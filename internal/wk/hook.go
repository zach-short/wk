package wk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/gitx"
)

// HookFailEnv makes the warm path of the hook fail on purpose, so the fallback can be proven in
// a live session (PLAN.md phase 3). Nothing else reads it.
const HookFailEnv = "WK_HOOK_FAIL"

// HookInput is the part of Claude Code's WorktreeCreate input that wk reads (PLAN.md 0.1).
type HookInput struct {
	Name string `json:"name"`
	Cwd  string `json:"cwd"`
}

// A name becomes one path component and one branch suffix, so it may hold no separator and no
// dot segment, which Claude Code also refuses in the printed path.
var hookName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,99}$`)

// WorktreeCreate answers the hook and returns the path to print. It never fails while git can
// still make a plain worktree (BD-6), because Claude Code gives a session no worktree at all when
// the hook fails. Progress goes to hook.log; stderr gets only the reason for a fallback or a
// failure. configure runs on the Env before any workspace is made, to set Refill.
func WorktreeCreate(home string, in io.Reader, stderr io.Writer, configure func(*Env)) (string, error) {
	var input HookInput
	if err := json.NewDecoder(in).Decode(&input); err != nil {
		return "", fmt.Errorf("WorktreeCreate input: %w", err)
	}
	if !hookName.MatchString(input.Name) || strings.Contains(input.Name, "..") {
		return "", fmt.Errorf("WorktreeCreate name %q cannot be a directory name", input.Name)
	}
	repo, err := gitx.Resolve(input.Cwd)
	if err != nil {
		return "", err
	}
	base, err := gitx.Commit(input.Cwd, "HEAD")
	if err != nil {
		return "", fmt.Errorf("HEAD of %s: %w", input.Cwd, err)
	}
	log, closeLog := openHookLog(home, repo.Name)
	defer closeLog()
	start := time.Now()
	_, _ = fmt.Fprintf(log, "%s WorktreeCreate name=%s cwd=%s base=%.10s\n", start.Format(time.RFC3339), input.Name, input.Cwd, base)

	h := hookRun{repo: repo, name: input.Name, base: base, log: log, stderr: stderr}
	path, how, err := h.create(home, configure)
	if err != nil {
		_, _ = fmt.Fprintf(log, "%s failed after %.2fs: %v\n", time.Now().Format(time.RFC3339), time.Since(start).Seconds(), err)
		return "", err
	}
	_, _ = fmt.Fprintf(log, "%s %s %s in %.2fs\n", time.Now().Format(time.RFC3339), how, path, time.Since(start).Seconds())
	return path, nil
}

type hookRun struct {
	repo   gitx.Repo
	name   string
	base   string
	log    io.Writer
	stderr io.Writer
	dir    string
	branch string
}

func (h *hookRun) create(home string, configure func(*Env)) (path, how string, err error) {
	env, openErr := Open(home, h.repo.Primary, h.log)
	cfg := config.Defaults().Hook
	if openErr == nil {
		cfg = env.Cfg.Hook
	}
	h.dir = filepath.Join(h.repo.Primary, filepath.FromSlash(strings.ReplaceAll(cfg.Dir, config.NamePlaceholder, h.name)))
	h.branch = strings.ReplaceAll(cfg.Branch, config.NamePlaceholder, h.name)
	if isWorktree(h.repo.Primary, h.dir) {
		return h.dir, "reused", nil
	}

	warmErr := openErr
	switch {
	case os.Getenv(HookFailEnv) != "":
		warmErr = errors.New(HookFailEnv + " is set")
	case openErr == nil:
		if configure != nil {
			configure(env)
		}
		_, warmErr = env.create(h.dir, h.branch, h.base, env.Cfg.Workspace.Sparse)
	}
	if warmErr == nil {
		return h.dir, "warm", nil
	}
	_, _ = fmt.Fprintf(h.stderr, "wk hook: warm workspace failed: %v\n", warmErr)
	_, _ = fmt.Fprintf(h.log, "warm workspace failed: %v\n", warmErr)
	if isWorktree(h.repo.Primary, h.dir) {
		return h.dir, "cold after a partial warm failure", nil
	}
	if err := h.plain(); err != nil {
		return "", "", fmt.Errorf("the plain worktree fallback failed too: %w", err)
	}
	return h.dir, "plain fallback", nil
}

// plain is the worktree Claude Code makes when no hook is set: git alone, no clones and no
// copied files. A branch left over from an earlier worktree of the same name is checked out
// again instead of refused, which a new branch could not do.
func (h *hookRun) plain() error {
	if err := os.MkdirAll(filepath.Dir(h.dir), 0o755); err != nil {
		return err
	}
	args := []string{"worktree", "add", "--quiet", "-b", h.branch, h.dir, h.base}
	if gitx.BranchExists(h.repo.Primary, h.branch) {
		args = []string{"worktree", "add", "--quiet", h.dir, h.branch}
	}
	_, err := gitx.Run(h.repo.Primary, args...)
	return err
}

func isWorktree(primary, dir string) bool {
	_, found := worktreeAt(primary, dir)
	return found
}

// openHookLog appends to home/<repo>/hook.log. A log that cannot be opened is no reason to fail
// the hook, so the writes then go nowhere.
func openHookLog(home, repo string) (io.Writer, func()) {
	dir := filepath.Join(home, repo)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return io.Discard, func() {}
	}
	f, err := os.OpenFile(filepath.Join(dir, "hook.log"), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return io.Discard, func() {}
	}
	return f, func() { _ = f.Close() }
}
