// Package gitx runs git and resolves which repository a command is about.
package gitx

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Run runs git in dir and returns its stdout with the trailing newline removed. A failure
// carries git's stderr, so the reason reaches whoever reads the error.
//
// GIT_OPTIONAL_LOCKS=0 stops `git diff HEAD` and `git status` from rewriting the index they
// read. Without it every read wk makes bumps the index mtime, and `wk gc` reads that mtime as
// a session's activity (BD-10), so an hourly gc would keep every worktree "busy" for ever.
func Run(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimRight(stdout.String(), "\n"), nil
}

// Repo is one repository as wk sees it: the main checkout every relative config path hangs
// from, and the name its config file is read under.
type Repo struct {
	Primary   string
	CommonDir string
	Name      string
}

// Resolve finds the repository that holds dir, which must be absolute. Any worktree of a
// repository resolves to the same Repo, because the answer comes from the common git
// directory, not from dir itself.
func Resolve(dir string) (Repo, error) {
	if !filepath.IsAbs(dir) {
		return Repo{}, fmt.Errorf("%q is relative; wk takes absolute paths only", dir)
	}
	common, err := Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return Repo{}, err
	}
	primary, err := mainCheckout(dir)
	if err != nil {
		return Repo{}, err
	}
	return Repo{
		Primary:   primary,
		CommonDir: common,
		Name:      filepath.Base(filepath.Dir(common)),
	}, nil
}

func mainCheckout(dir string) (string, error) {
	out, err := Run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return "", err
	}
	first, _, _ := strings.Cut(out, "\n")
	path, ok := strings.CutPrefix(first, "worktree ")
	if !ok {
		return "", errors.New("git worktree list printed no main checkout")
	}
	return path, nil
}

// Worktree is one entry of `git worktree list --porcelain`.
type Worktree struct {
	Path       string
	Head       string
	Branch     string
	Detached   bool
	Bare       bool
	Locked     bool
	LockReason string
	Prunable   bool
}

func Worktrees(dir string) ([]Worktree, error) {
	out, err := Run(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var list []Worktree
	for _, block := range strings.Split(out, "\n\n") {
		var w Worktree
		for _, line := range strings.Split(block, "\n") {
			key, value, _ := strings.Cut(line, " ")
			switch key {
			case "worktree":
				w.Path = value
			case "HEAD":
				w.Head = value
			case "branch":
				w.Branch = strings.TrimPrefix(value, "refs/heads/")
			case "detached":
				w.Detached = true
			case "bare":
				w.Bare = true
			case "locked":
				w.Locked, w.LockReason = true, value
			case "prunable":
				w.Prunable = true
			}
		}
		if w.Path != "" {
			list = append(list, w)
		}
	}
	return list, nil
}

func BranchExists(dir, branch string) bool {
	_, err := Run(dir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch)
	return err == nil
}

func Commit(dir, rev string) (string, error) {
	return Run(dir, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
}

// Blob is the object a path holds at rev, or "" when rev does not hold the path.
func Blob(dir, rev, path string) (string, error) {
	out, err := Run(dir, "rev-parse", "--verify", "--quiet", rev+":"+path)
	if err != nil {
		if _, cerr := Commit(dir, rev); cerr != nil {
			return "", cerr
		}
		return "", nil
	}
	return out, nil
}

// DirtyPaths lists every path in dir that differs from its HEAD, sorted: tracked edits staged
// or not, deletions, and untracked files that are not ignored. It runs the same two commands as
// the land-worktrees lib.sh dirty_paths, because `wk gc` must call clean exactly what that
// skill calls clean. Ignored files such as node_modules and .env are not work.
func DirtyPaths(dir string) ([]string, error) {
	tracked, err := Run(dir, "diff", "--name-only", "--no-renames", "-z", "HEAD")
	if err != nil {
		return nil, err
	}
	untracked, err := Run(dir, "ls-files", "--others", "--exclude-standard", "-z")
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var paths []string
	for _, p := range strings.Split(tracked+"\x00"+untracked, "\x00") {
		if p != "" && !seen[p] {
			seen[p] = true
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

// IsAncestor says whether commit is already in into's history.
func IsAncestor(dir, commit, into string) bool {
	_, err := Run(dir, "merge-base", "--is-ancestor", commit, into)
	return err == nil
}

// IsAbsorbed is lib.sh is_absorbed: everything on commit is already in into, because it is an
// ancestor, or because merging it would change nothing (the work landed by a squash merge, a
// cherry-pick or a copy). Any git failure answers false, as it does there. ancestor reports
// which of the two held, for BD-11.
func IsAbsorbed(dir, commit, into string) (absorbed, ancestor bool) {
	if IsAncestor(dir, commit, into) {
		return true, true
	}
	merged, err := Run(dir, "merge-tree", "--write-tree", into, commit)
	if err != nil {
		return false, false
	}
	tree, err := Run(dir, "rev-parse", into+"^{tree}")
	if err != nil {
		return false, false
	}
	first, _, _ := strings.Cut(merged, "\n")
	return first == tree, false
}

// GitDir is the worktree's private git directory, where markers live out of sight of the
// working tree and of `git status`.
func GitDir(dir string) (string, error) {
	return Run(dir, "rev-parse", "--absolute-git-dir")
}
