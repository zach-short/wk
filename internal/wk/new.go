package wk

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/zach-short/wk/internal/clone"
	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
)

var rowPattern = regexp.MustCompile(`^[1-9][0-9]{0,8}$`)

// maxRefreshes bounds how often one `wk new` refreshes a template that goes stale again while
// it waits, so a base branch whose lockfile moves every minute cannot hold it forever.
const maxRefreshes = 2

// New makes the workspace for a board row and returns its absolute path.
func (e *Env) New(row string, sparse bool) (string, error) {
	if !rowPattern.MatchString(row) {
		return "", fmt.Errorf("row %q is not a positive integer", row)
	}
	dir := e.inPrimary(strings.ReplaceAll(e.Cfg.Workspace.Dir, config.RowPlaceholder, row))
	branch := strings.ReplaceAll(e.Cfg.Workspace.Branch, config.RowPlaceholder, row)
	return e.create(dir, branch, sparse || e.Cfg.Workspace.Sparse)
}

func (e *Env) create(dir, branch string, sparse bool) (string, error) {
	if _, err := os.Lstat(dir); err == nil {
		return "", fmt.Errorf("%s already exists", dir)
	}
	if gitx.BranchExists(e.Repo.Primary, branch) {
		return "", fmt.Errorf("branch %s already exists", branch)
	}
	commit, err := gitx.Commit(e.Repo.Primary, e.Cfg.Workspace.Base)
	if err != nil {
		return "", fmt.Errorf("base %q: %w", e.Cfg.Workspace.Base, err)
	}
	lock, err := e.takeFreshTemplate(commit)
	if err != nil {
		return "", err
	}
	err = e.timed("worktree add --no-checkout", func() error {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		_, err := gitx.Run(e.Repo.Primary, "worktree", "add", "--quiet", "--no-checkout", "-b", branch, dir, commit)
		return err
	})
	if err != nil {
		_ = lock.Release()
		return "", err
	}
	err = e.fill(dir, sparse)
	if relErr := lock.Release(); err == nil {
		err = relErr
	}
	if err == nil {
		err = e.copyFiles(dir)
	}
	if err == nil {
		err = e.runCommands(dir, e.Cfg.Template.PostClone, nil)
	}
	if err != nil {
		return "", fmt.Errorf("%w; the workspace is left at %s for inspection", err, dir)
	}
	return dir, nil
}

// takeFreshTemplate returns the template lock held shared, on a template that matches the base
// branch's lockfile, refreshing it first when it does not (D7).
func (e *Env) takeFreshTemplate(commit string) (*flock.Lock, error) {
	for refreshes := 0; ; refreshes++ {
		lock, err := flock.Take(e.lockPath(), false, func() { e.say("waiting: the template is being refreshed") })
		if err != nil {
			return nil, err
		}
		why, err := e.staleReason(commit)
		if err != nil || why == "" {
			if err != nil {
				_ = lock.Release()
				return nil, err
			}
			return lock, nil
		}
		if err := lock.Release(); err != nil {
			return nil, err
		}
		if refreshes == maxRefreshes {
			return nil, fmt.Errorf("the template is still stale after %d refreshes: %s", maxRefreshes, why)
		}
		e.say("refreshing the template first: %s", why)
		if err := e.RefreshTemplate(); err != nil {
			return nil, fmt.Errorf("template refresh: %w", err)
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
			args := []string{"sparse-checkout", "set", "--no-cone", "/*"}
			for _, p := range e.Cfg.Workspace.SparseExclude {
				args = append(args, "!/"+strings.Trim(filepath.ToSlash(p), "/")+"/")
			}
			if _, err := gitx.Run(dir, args...); err != nil {
				return err
			}
		}
		_, err := gitx.Run(dir, "read-tree", "-m", "-u", "HEAD")
		return err
	})
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
