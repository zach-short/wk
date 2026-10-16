package wk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
)

// MarkerName is the file in the template's private git directory that tells wk, and later
// `wk gc`, that this worktree is the template and never anyone's work (the trial_marker
// pattern of the land-worktrees lib.sh).
const MarkerName = "wk-template"

type templateState struct {
	Commit      string    `json:"commit"`
	RefreshBlob string    `json:"refresh_blob"`
	RefreshedAt time.Time `json:"refreshed_at"`
}

func (e *Env) TemplateDir() string { return e.inPrimary(e.Cfg.Template.Path) }

func (e *Env) lockPath() string { return filepath.Join(e.State, "template.lock") }

func (e *Env) statePath() string { return filepath.Join(e.State, "template.json") }

// RefreshTemplate moves the template to the base branch's commit and rebuilds what workspaces
// clone from it. It holds the template lock exclusive throughout, so no clone reads a tree
// that is being written.
func (e *Env) RefreshTemplate() error {
	lock, err := flock.Take(e.lockPath(), true, func() { e.say("waiting for the template lock") })
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	return e.refreshLocked()
}

func (e *Env) refreshLocked() error {
	commit, err := gitx.Commit(e.Repo.Primary, e.Cfg.Workspace.Base)
	if err != nil {
		return fmt.Errorf("base %q: %w", e.Cfg.Workspace.Base, err)
	}
	blob, err := e.refreshBlob(commit)
	if err != nil {
		return err
	}
	dir := e.TemplateDir()
	if err := e.placeTemplate(dir, commit); err != nil {
		return err
	}
	if err := e.copyFiles(dir); err != nil {
		return err
	}
	if err := e.runCommands(dir, e.Cfg.Template.PostClone, nil); err != nil {
		return err
	}
	if err := e.runCommands(dir, e.Cfg.Template.Build, e.Cfg.Gates.Env); err != nil {
		return err
	}
	return e.writeState(templateState{Commit: commit, RefreshBlob: blob, RefreshedAt: time.Now().UTC()})
}

func (e *Env) refreshBlob(commit string) (string, error) {
	if e.Cfg.Template.RefreshOn == "" {
		return "", nil
	}
	return gitx.Blob(e.Repo.Primary, commit, e.Cfg.Template.RefreshOn)
}

// placeTemplate makes the template a detached worktree at commit. The base branch is checked
// out in the main checkout and git refuses a second checkout of a branch, so the template holds
// the commit detached (BD-3).
func (e *Env) placeTemplate(dir, commit string) error {
	wt, found, err := e.findWorktree(dir)
	if err != nil {
		return err
	}
	if !found {
		if _, err := os.Lstat(dir); err == nil {
			return fmt.Errorf("%s exists and is not a worktree; wk will not take it over", dir)
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		if err := e.timed("template worktree add", func() error {
			_, err := gitx.Run(e.Repo.Primary, "worktree", "add", "--quiet", "--detach", dir, commit)
			return err
		}); err != nil {
			return err
		}
		return writeMarker(dir)
	}
	if ok, err := hasMarker(dir); err != nil || !ok {
		return errors.Join(err, fmt.Errorf("%s is a worktree without the wk template marker; wk will not take it over", dir))
	}
	dirty, err := gitx.DirtyPaths(dir)
	if err != nil {
		return err
	}
	if len(dirty) > 0 {
		return fmt.Errorf("the template %s has %d changed paths; it is never used for work, so inspect it and remove it with `git worktree remove --force`", dir, len(dirty))
	}
	if wt.Head == commit {
		return nil
	}
	return e.timed("template checkout", func() error {
		_, err := gitx.Run(dir, "checkout", "--quiet", "--detach", commit)
		return err
	})
}

func (e *Env) findWorktree(dir string) (gitx.Worktree, bool, error) {
	list, err := gitx.Worktrees(e.Repo.Primary)
	if err != nil {
		return gitx.Worktree{}, false, err
	}
	want := physical(dir)
	for _, w := range list {
		if physical(w.Path) == want {
			return w, true, nil
		}
	}
	return gitx.Worktree{}, false, nil
}

// physical resolves symlinks where it can, so /tmp and /private/tmp compare equal.
func physical(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return filepath.Clean(p)
}

func markerPath(dir string) (string, error) {
	gd, err := gitx.GitDir(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(gd, MarkerName), nil
}

func writeMarker(dir string) error {
	p, err := markerPath(dir)
	if err != nil {
		return err
	}
	return os.WriteFile(p, []byte("wk template: never used for work, never removed by gc\n"), 0o644)
}

func hasMarker(dir string) (bool, error) {
	p, err := markerPath(dir)
	if err != nil {
		return false, err
	}
	_, err = os.Stat(p)
	if os.IsNotExist(err) {
		return false, nil
	}
	return err == nil, err
}

func (e *Env) readState() (templateState, bool, error) {
	data, err := os.ReadFile(e.statePath())
	if os.IsNotExist(err) {
		return templateState{}, false, nil
	}
	if err != nil {
		return templateState{}, false, err
	}
	var s templateState
	if err := json.Unmarshal(data, &s); err != nil {
		return templateState{}, false, fmt.Errorf("%s: %w", e.statePath(), err)
	}
	return s, true, nil
}

func (e *Env) writeState(s templateState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := e.statePath() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, e.statePath())
}

// staleReason says why the template must be refreshed before a clone, or "" when it is fresh.
func (e *Env) staleReason(baseCommit string) (string, error) {
	s, found, err := e.readState()
	if err != nil {
		return "", err
	}
	if !found {
		return "no template has been made yet", nil
	}
	dir := e.TemplateDir()
	if _, statErr := os.Stat(dir); statErr != nil {
		return "the template directory is missing", nil
	}
	if ok, err := hasMarker(dir); err != nil || !ok {
		return "the template has no marker", nil
	}
	blob, err := e.refreshBlob(baseCommit)
	if err != nil {
		return "", err
	}
	if blob != s.RefreshBlob {
		return fmt.Sprintf("%s changed on %s", e.Cfg.Template.RefreshOn, e.Cfg.Workspace.Base), nil
	}
	return "", nil
}
