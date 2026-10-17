package wk

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gate"
	"github.com/zach-short/wk/internal/gitx"
)

// TrialDir is the trial workspace at gates.trial_dir, where the landing worker gates each batch.
func (e *Env) TrialDir() string { return e.inPrimary(e.Cfg.Gates.TrialDir) }

// withTrial puts tree in the trial and runs gate over it, holding ~/.wk/<repo>/trial.lock from
// the read to the last gate: next build type-checks the working tree, so nothing may gate a
// trial mid-read (DESIGN.md section 6). The trial is made once and reused; the next batch's read
// replaces whatever a red one left, so nothing is cleaned up after a gate.
func (e *Env) withTrial(tree string, gate func(dir string) error) error {
	lock, err := flock.Take(filepath.Join(e.State, "trial.lock"), true, func() { e.say("waiting for trial.lock") })
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	dir, err := e.ensureTrial()
	if err != nil {
		return err
	}
	// --reset with -u overwrites the files of the last batch and removes those this tree lacks;
	// ignored files (dependencies, build caches) stay, as in any checkout.
	if _, err := gitx.Run(dir, "read-tree", "-u", "--reset", tree); err != nil {
		return err
	}
	return gate(dir)
}

// ensureTrial returns the trial, making it on first use. A directory there without the marker
// is someone else's and is never taken over, unless trial.making names it: then it is a trial
// this worker's repository started to make and never marked, which is made again.
func (e *Env) ensureTrial() (string, error) {
	dir := e.TrialDir()
	if _, err := os.Lstat(dir); err != nil {
		return dir, e.makeTrial(dir)
	}
	marked := checkTrialMarker(dir)
	if !e.interruptedTrial(dir) {
		return dir, marked
	}
	return dir, e.finishInterruptedTrial(dir, marked)
}

// finishInterruptedTrial finishes the trial at dir, which trial.making names: marked, only the
// record's removal was missed; unmarked, it is removed and made again.
func (e *Env) finishInterruptedTrial(dir string, marked error) error {
	if marked == nil {
		return os.Remove(e.trialIntent())
	}
	if err := e.removeInterruptedTrial(dir); err != nil {
		return err
	}
	return e.makeTrial(dir)
}

// makeTrial makes the trial by Env.create with a detached HEAD at the base branch and marks it
// wk-trial in its private git directory, so gc skips it (PD-9 as amended). The marker can only
// follow the create, because the private git directory does not exist before worktree add, so
// trial.making names dir from before the create until the marker is written (row 5's N3).
func (e *Env) makeTrial(dir string) error {
	commit, err := e.baseCommit()
	if err != nil {
		return err
	}
	if err := os.WriteFile(e.trialIntent(), []byte(dir+"\n"), 0o644); err != nil {
		return err
	}
	if _, err := e.create(dir, "", commit, false); err != nil {
		return e.failedTrialCreate(dir, err)
	}
	if err := markTrial(dir); err != nil {
		return err
	}
	return os.Remove(e.trialIntent())
}

// markTrial writes the wk-trial marker into dir's private git directory.
func markTrial(dir string) error {
	gd, err := gitx.GitDir(dir)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(gd, GateTrialMarkerName), []byte("wk gate trial: the landing worker reuses it; gc skips it\n"), 0o644)
}

// failedTrialCreate is makeTrial's error when Env.create fails. With nothing at dir, the record
// is removed, so it cannot outlive this pass and later name a directory someone else puts there.
// With dir there, the record stays, because dir is the unfinished trial it exists for and the
// next worker removes it and makes it again without a person. create's own error still ends
// "left for inspection", which is true for wk new, so the trial's error adds what happens here
// instead (row 15's audit F2).
func (e *Env) failedTrialCreate(dir string, err error) error {
	if _, lerr := os.Lstat(dir); lerr != nil {
		_ = os.Remove(e.trialIntent())
		return err
	}
	return fmt.Errorf("%w; but %s names it as an unfinished trial, so the next worker (the landing timer starts one every minute) removes it with git worktree remove --force and makes it again: copy anything you need out of it first", err, e.trialIntent())
}

// trialIntent is ~/.wk/<repo>/trial.making. Only a holder of trial.lock writes or reads it.
func (e *Env) trialIntent() string { return filepath.Join(e.State, "trial.making") }

func (e *Env) interruptedTrial(dir string) bool {
	data, err := os.ReadFile(e.trialIntent())
	return err == nil && strings.TrimSuffix(string(data), "\n") == dir
}

// removeInterruptedTrial removes an unfinished trial with git worktree remove --force, because
// a create that stopped partway may have no files or no dependencies. The trial is made
// detached, so a directory on a branch is refused; git removes only a worktree of this
// repository, so a directory that is not one is refused too, and nothing else is deleted.
func (e *Env) removeInterruptedTrial(dir string) error {
	if _, err := gitx.Run(dir, "symbolic-ref", "-q", "HEAD"); err == nil {
		return fmt.Errorf("%s names %s as an unfinished trial, but it is on a branch, so it is not the trial; move it, or remove it with git worktree remove", e.trialIntent(), dir)
	}
	e.say("%s is a trial a stopped worker did not finish making; removing it to make it again", dir)
	if _, err := gitx.Run(e.Repo.Primary, "worktree", "remove", "--force", dir); err != nil {
		return fmt.Errorf("%s names %s as an unfinished trial, but git worktree remove refused it: %w; remove it by hand, and the worker makes the trial there", e.trialIntent(), dir, err)
	}
	return nil
}

// trialDepsName is the file beside the marker that holds the template.refresh_on blob the
// trial's dependencies were last made at. Before it exists, that is the blob at the trial's
// HEAD, the commit the trial was made from.
const trialDepsName = "wk-trial-deps"

// refreshDeps runs template.post_clone in the trial when tree's template.refresh_on blob is not
// the one its dependencies were made at, because read-tree changes tracked files only and a
// batch that changes bun.lock would be gated over the old dependencies. A command that fails is
// a red verdict or a start error, as postCloneFailed decides; one that cannot start is a start
// error.
func (e *Env) refreshDeps(dir, tree, tipTree string, out io.Writer) (verdict, error) {
	on := e.Cfg.Template.RefreshOn
	if on == "" || len(e.Cfg.Template.PostClone) == 0 {
		return verdict{}, nil
	}
	d, err := readDeps(dir, tree, on)
	if err != nil || d.made == d.want {
		return verdict{}, err
	}
	_, _ = fmt.Fprintf(out, "trial: the batch changes %s, so the post-clone commands run first\n", on)
	r, err := e.remakeDeps(dir, d, out)
	if err != nil || r.Status != gate.Fail {
		return verdict{}, err
	}
	return postCloneFailed(r, on, blobAt(dir, tipTree, on) != d.want)
}

// trialDeps is where the trial's dependencies stand: the record beside the marker, the
// template.refresh_on blob they were made at, and the one the batch's tree holds.
type trialDeps struct{ record, made, want string }

// readDeps reads the trial's deps record (before it exists, the blob at the trial's HEAD), then
// the blob tree holds at on.
func readDeps(dir, tree, on string) (trialDeps, error) {
	gd, err := gitx.GitDir(dir)
	if err != nil {
		return trialDeps{}, err
	}
	d := trialDeps{record: filepath.Join(gd, trialDepsName)}
	data, err := os.ReadFile(d.record)
	if errors.Is(err, fs.ErrNotExist) {
		data, err = []byte(blobAt(dir, "HEAD", on)), nil
	}
	if err != nil {
		return trialDeps{}, err
	}
	d.made, d.want = strings.TrimSuffix(string(data), "\n"), blobAt(dir, tree, on)
	return d, nil
}

// remakeDeps runs template.post_clone in the trial (through gate.Run, one gate named post_clone
// per command, no [gates.env], as Env.create runs them) and records d.want once they pass. Until
// then the record says unknown, so a stop or a red run here makes the next batch run them again.
func (e *Env) remakeDeps(dir string, d trialDeps, out io.Writer) (gate.Result, error) {
	if err := os.WriteFile(d.record, []byte("unknown\n"), 0o644); err != nil {
		return gate.Result{}, err
	}
	rs, err := gate.Run(dir, postCloneGates(e.Cfg.Template.PostClone), nil, nil, out)
	if err != nil {
		return gate.Result{}, err
	}
	if r := rs[len(rs)-1]; r.Status == gate.Fail {
		return r, nil
	}
	return gate.Result{}, os.WriteFile(d.record, []byte(d.want+"\n"), 0o644)
}

// postCloneFailed is the answer to a failed post-clone run. When the batch changes refresh_on,
// it is a red verdict named post_clone with refresh_on as its path, so blame finds the changeset
// that changed it. Otherwise the batch's lockfile is the tip's, and the failure (a registry that
// is down, a dependency directory a stop left half written) says nothing about the batch: a
// start error.
func postCloneFailed(r gate.Result, on string, batchChangesIt bool) (verdict, error) {
	if !batchChangesIt {
		return verdict{}, fmt.Errorf("post_clone in the trial: %s, and the batch does not change %s, so the failure says nothing about it; the next batch runs post_clone again", r.Reason, on)
	}
	return verdict{results: []gate.Result{r}, red: &r, paths: []string{on}}, nil
}

// blobAt is the object path holds in rev, a commit or a tree, or "" when it holds none. It reads
// rev-parse directly, because gitx.Blob checks rev as a commit, which a tree is not.
func blobAt(dir, rev, path string) string {
	out, err := gitx.Run(dir, "rev-parse", "--verify", "--quiet", rev+":"+path)
	if err != nil {
		return ""
	}
	return out
}

func postCloneGates(cmds []config.Command) []gate.Gate {
	gates := make([]gate.Gate, len(cmds))
	for i, c := range cmds {
		gates[i] = gate.Gate{Name: "post_clone", Dir: c.Dir, Run: c.Run}
	}
	return gates
}

func checkTrialMarker(dir string) error {
	gd, err := gitx.GitDir(dir)
	if err == nil && exists(filepath.Join(gd, GateTrialMarkerName)) {
		return nil
	}
	return fmt.Errorf("%s exists without the %s marker, so it is not the trial; move it, or remove it with git worktree remove, and the worker makes the trial there", dir, GateTrialMarkerName)
}
