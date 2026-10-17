package wk

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zach-short/wk/internal/flock"
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

// ensureTrial returns the trial, making it on first use by Env.create with a detached HEAD at
// the base branch and marking it wk-trial in its private git directory, so gc skips it (PD-9 as
// amended). A directory there without the marker is someone else's, and is never taken over.
func (e *Env) ensureTrial() (string, error) {
	dir := e.TrialDir()
	if _, err := os.Lstat(dir); err == nil {
		return dir, checkTrialMarker(dir)
	}
	commit, err := e.baseCommit()
	if err != nil {
		return "", err
	}
	if _, err := e.create(dir, "", commit, false); err != nil {
		return "", err
	}
	gd, err := gitx.GitDir(dir)
	if err != nil {
		return "", err
	}
	return dir, os.WriteFile(filepath.Join(gd, GateTrialMarkerName), []byte("wk gate trial: the landing worker reuses it; gc skips it\n"), 0o644)
}

func checkTrialMarker(dir string) error {
	gd, err := gitx.GitDir(dir)
	if err == nil && exists(filepath.Join(gd, GateTrialMarkerName)) {
		return nil
	}
	return fmt.Errorf("%s exists without the %s marker, so it is not the trial; move it, or remove it with git worktree remove, and the worker makes the trial there", dir, GateTrialMarkerName)
}
