package wk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/zach-short/wk/internal/clone"
	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
)

var rowPattern = regexp.MustCompile(`^[1-9][0-9]{0,8}$`)

// errExists is create's refusal of a dir that is already there. create made nothing then, so
// the trial can tell a directory it never touched from one its own create left (row 16's A1).
var errExists = errors.New("already exists")

// maxRefreshes bounds how often one `wk new` refreshes a template that goes stale again while
// it waits, so a base branch whose lockfile moves every minute cannot hold it forever.
const maxRefreshes = 2

// Checkout is the shape a caller asks for. DefaultCheckout follows workspace.sparse.
type Checkout int

const (
	DefaultCheckout Checkout = iota
	FullCheckout
	SparseCheckout
)

func (e *Env) sparseFor(c Checkout) bool {
	switch c {
	case FullCheckout:
		return false
	case SparseCheckout:
		return true
	}
	return e.Cfg.Workspace.Sparse
}

// New makes the workspace for a board row and returns its absolute path.
func (e *Env) New(row string, c Checkout) (string, error) {
	if !rowPattern.MatchString(row) {
		return "", fmt.Errorf("row %q is not a positive integer", row)
	}
	dir := e.inPrimary(strings.ReplaceAll(e.Cfg.Workspace.Dir, config.RowPlaceholder, row))
	branch := strings.ReplaceAll(e.Cfg.Workspace.Branch, config.RowPlaceholder, row)
	commit, err := gitx.Commit(e.Repo.Primary, e.Cfg.Workspace.Base)
	if err != nil {
		return "", fmt.Errorf("base %q: %w", e.Cfg.Workspace.Base, err)
	}
	return e.create(dir, branch, commit, e.sparseFor(c))
}

// create makes the workspace at dir on a new branch at commit, or with a detached HEAD at
// commit when branch is "" (the trial): from a pool slot when one is ready, else from the
// template. Either way the pool is topped up afterwards, in the background.
func (e *Env) create(dir, branch, commit string, sparse bool) (string, error) {
	if _, err := os.Lstat(dir); err == nil {
		return "", fmt.Errorf("%s %w", dir, errExists)
	}
	if branch != "" && gitx.BranchExists(e.Repo.Primary, branch) {
		return "", fmt.Errorf("branch %s already exists", branch)
	}
	defer e.refill()
	path, claimed, err := e.claim(dir, branch, commit, sparse)
	if claimed {
		return path, err
	}
	if err != nil {
		e.say("pool: %v; making the workspace from the template", err)
	}
	return e.createCold(dir, branch, commit, sparse)
}

func (e *Env) createCold(dir, branch, commit string, sparse bool) (string, error) {
	slot, err := e.takeCreateSlot()
	if err != nil {
		return "", err
	}
	lock, state, err := e.takeFreshTemplate()
	if err != nil {
		_ = slot.Release()
		return "", err
	}
	err = e.timed("worktree add --no-checkout", func() error {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		_, err := gitx.Run(e.Repo.Primary, worktreeAdd(dir, branch, commit)...)
		return err
	})
	if err == nil {
		err = e.fill(dir, sparse)
	}
	err = errors.Join(err, lock.Release(), slot.Release())
	if err == nil {
		err = e.copyFiles(dir)
	}
	if err == nil {
		err = e.postClone(dir, commit, state.Commit)
	}
	if err != nil {
		return "", fmt.Errorf("%w; the workspace is left at %s for inspection", err, dir)
	}
	return dir, nil
}

// worktreeAdd is the `git worktree add` that puts dir on a new branch, or detached at commit
// when branch is "".
func worktreeAdd(dir, branch, commit string) []string {
	args := []string{"worktree", "add", "--quiet", "--no-checkout"}
	if branch == "" {
		return append(args, "--detach", dir, commit)
	}
	return append(args, "-b", branch, dir, commit)
}

// postClone runs the post-clone commands unless the workspace sits at the commit its clones
// were installed and generated at: then they would rewrite what was cloned, byte for byte.
func (e *Env) postClone(dir, commit, clonedAt string) error {
	if commit == clonedAt {
		e.say("skip the post-clone commands: the clones were made at this commit")
		return nil
	}
	return e.runCommands(dir, e.Cfg.Template.PostClone, nil)
}

// takeCreateSlot holds one of new.max_parallel slots while a workspace clones and checks out.
// It polls rather than blocking on one slot, so a waiter takes whichever slot frees first.
func (e *Env) takeCreateSlot() (*flock.Lock, error) {
	said := false
	for {
		for i := range e.Cfg.New.MaxParallel {
			lock, ok, err := flock.TryTake(filepath.Join(e.State, fmt.Sprintf("create-%d.lock", i)), true)
			if err != nil || ok {
				return lock, err
			}
		}
		if !said {
			e.say("waiting: %d workspaces are already cloning (new.max_parallel)", e.Cfg.New.MaxParallel)
			said = true
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// takeFreshTemplate returns the template lock held shared, and the template's state, on a
// template that matches the base branch, refreshing it first when it does not (D7). The base
// is read on every pass, because a refresh can take long enough for it to move again.
func (e *Env) takeFreshTemplate() (*flock.Lock, templateState, error) {
	for refreshes := 0; ; refreshes++ {
		lock, err := flock.Take(e.lockPath(), false, func() { e.say("waiting: the template is being refreshed") })
		if err != nil {
			return nil, templateState{}, err
		}
		why, _, err := e.staleNow()
		if err == nil && why == "" {
			state, _, err := e.readState()
			if err == nil {
				return lock, state, nil
			}
		}
		if relErr := lock.Release(); err == nil {
			err = relErr
		}
		if err != nil {
			return nil, templateState{}, err
		}
		if refreshes == maxRefreshes {
			return nil, templateState{}, fmt.Errorf("the template is still stale after %d refreshes: %s", maxRefreshes, why)
		}
		if err := e.refreshStale(); err != nil {
			return nil, templateState{}, fmt.Errorf("template refresh: %w", err)
		}
	}
}

// fill checks the tree out and clones every dependency tree at the same time (BD-4). git
// creates directories that already exist without complaint, and every clone target is
// ignored, so the two never write the same path.
func (e *Env) fill(dir string, sparse bool) error {
	var wg sync.WaitGroup
	errs := make([]error, len(e.Cfg.Template.CloneDirs)+1)
	wg.Add(1)
	go func() {
		defer wg.Done()
		errs[0] = e.checkout(dir, sparse)
	}()
	template := e.TemplateDir()
	for i, rel := range e.Cfg.Template.CloneDirs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i+1] = e.cloneOne(template, dir, rel)
		}()
	}
	wg.Wait()
	return errors.Join(errs...)
}

func (e *Env) checkout(dir string, sparse bool) error {
	step := "checkout"
	if sparse {
		step = "sparse checkout"
	}
	return e.timed(step, func() error {
		if sparse {
			args := append([]string{"sparse-checkout", "set", "--no-cone", "/*"}, sparseExcludes(e.Cfg.Workspace.SparseExclude)...)
			if _, err := gitx.Run(dir, args...); err != nil {
				return err
			}
		}
		_, err := gitx.Run(dir, "read-tree", "-m", "-u", "HEAD")
		return err
	})
}

// sparseExcludes turns workspace.sparse_exclude into negated non-cone patterns: a directory
// excludes everything under it, and an entry with a glob character is a pattern as written.
func sparseExcludes(entries []string) []string {
	out := make([]string, 0, len(entries))
	for _, p := range entries {
		p = strings.Trim(filepath.ToSlash(p), "/")
		if strings.ContainsAny(p, "*?[") {
			out = append(out, "!/"+p)
		} else {
			out = append(out, "!/"+p+"/")
		}
	}
	return out
}

func (e *Env) cloneOne(template, dir, rel string) error {
	src := filepath.Join(template, filepath.FromSlash(rel))
	if _, err := os.Lstat(src); os.IsNotExist(err) {
		e.say("skip clone of %s: the template has none", rel)
		return nil
	}
	dst := filepath.Join(dir, filepath.FromSlash(rel))
	return e.timed("clone "+rel, func() error {
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := clone.Dir(src, dst); err != nil {
			return fmt.Errorf("clone %s: %w", rel, err)
		}
		return nil
	})
}
