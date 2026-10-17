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
	if marked == nil {
		// The marker was written; only the removal of the intent record was missed.
		return dir, os.Remove(e.trialIntent())
	}
	if err := e.removeInterruptedTrial(dir); err != nil {
		return "", err
	}
	return dir, e.makeTrial(dir)
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
		if _, lerr := os.Lstat(dir); lerr != nil {
			// Nothing was made, so the record may not outlive this pass and later name a
			// directory someone else puts there.
			_ = os.Remove(e.trialIntent())
		}
		return err
	}
	gd, err := gitx.GitDir(dir)
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(gd, GateTrialMarkerName), []byte("wk gate trial: the landing worker reuses it; gc skips it\n"), 0o644); err != nil {
		return err
	}
	return os.Remove(e.trialIntent())
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
// batch that changes bun.lock would be gated over the old dependencies. When the batch changes
// refresh_on (tree's blob is not tipTree's), a command that fails is a red verdict named
// post_clone with refresh_on as its path, so blame finds the changeset that changed it.
// Otherwise the batch's lockfile is the tip's, and the failure (a registry that is down, a
// dependency directory a stop left half written) says nothing about the batch: a start error.
// So is a command that cannot start.
func (e *Env) refreshDeps(dir, tree, tipTree string, out io.Writer) (verdict, error) {
	on, cmds := e.Cfg.Template.RefreshOn, e.Cfg.Template.PostClone
	if on == "" || len(cmds) == 0 {
		return verdict{}, nil
	}
	record, made, err := depsMadeAt(dir, on)
	if err != nil {
		return verdict{}, err
	}
	want := blobAt(dir, tree, on)
	if made == want {
		return verdict{}, nil
	}
	_, _ = fmt.Fprintf(out, "trial: the batch changes %s, so the post-clone commands run first\n", on)
	// Until the commands pass, the dependencies match no blob, so a stop or a red run here makes
	// the next batch run them again.
	if err := os.WriteFile(record, []byte("unknown\n"), 0o644); err != nil {
		return verdict{}, err
	}
	rs, err := gate.Run(dir, postCloneGates(cmds), nil, nil, out)
	if err != nil {
		return verdict{}, err
	}
	if r := rs[len(rs)-1]; r.Status == gate.Fail {
		if blobAt(dir, tipTree, on) == want {
			return verdict{}, fmt.Errorf("post_clone in the trial: %s, and the batch does not change %s, so the failure says nothing about it; the next batch runs post_clone again", r.Reason, on)
		}
		return verdict{results: []gate.Result{r}, red: &r, paths: []string{on}}, nil
	}
	return verdict{}, os.WriteFile(record, []byte(want+"\n"), 0o644)
}

// depsMadeAt is the path of the trial's deps record and the blob it holds.
func depsMadeAt(dir, on string) (string, string, error) {
	gd, err := gitx.GitDir(dir)
	if err != nil {
		return "", "", err
	}
	record := filepath.Join(gd, trialDepsName)
	data, err := os.ReadFile(record)
	if errors.Is(err, fs.ErrNotExist) {
		return record, blobAt(dir, "HEAD", on), nil
	}
	return record, strings.TrimSuffix(string(data), "\n"), err
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
