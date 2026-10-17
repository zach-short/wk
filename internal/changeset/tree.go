// Package changeset builds what `wk submit` sends to the daemon, the workspace's tree and its
// base against the landing tip, and keeps the queue file the daemon appends every state change
// to (PD-8). A workspace never commits: its tree is written through a private index.
package changeset

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"strings"

	"github.com/zach-short/wk/internal/gitx"
)

// Tree writes the workspace's working tree as a git tree object: tracked edits, deletions and
// untracked files that are not ignored. It goes through a private index built from HEAD, so
// the workspace's own index is never opened for writing and whatever is staged there does not
// matter. The private index is removed on every path.
func Tree(workspace string) (string, error) {
	gitDir, err := gitx.GitDir(workspace)
	if err != nil {
		return "", err
	}
	index, err := tempIndex(gitDir)
	if err != nil {
		return "", err
	}
	defer removeIndex(index)
	for _, args := range [][]string{{"read-tree", "HEAD"}, {"add", "-A"}} {
		if _, err := runIndexed(workspace, index, args...); err != nil {
			return "", err
		}
	}
	return runIndexed(workspace, index, "write-tree")
}

// tempIndex reserves a unique name beside the workspace's index, inside its git directory so
// `git status` never lists it. The empty file is removed again: git refuses an index file
// shorter than its header, and read-tree creates one that is absent.
func tempIndex(gitDir string) (string, error) {
	f, err := os.CreateTemp(gitDir, "wk-index-*")
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	return f.Name(), os.Remove(f.Name())
}

// removeIndex also drops the lock git leaves when it is killed between two steps.
func removeIndex(index string) {
	for _, p := range []string{index, index + ".lock"} {
		if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
			fmt.Fprintf(os.Stderr, "wk: %v\n", err)
		}
	}
}

// runIndexed is gitx.Run with GIT_INDEX_FILE set; gitx takes no environment, and the index
// file has no -c equivalent.
func runIndexed(dir, index string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_INDEX_FILE="+index)
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

// Base is the commit a changeset is diffed from: the merge base of the workspace's HEAD and
// the landing tip (PD-7), so a branch that carries its own commits still submits one tree.
// Two histories with no common commit are an error naming both.
func Base(workspace, tip string) (string, error) {
	head, err := gitx.Commit(workspace, "HEAD")
	if err != nil {
		return "", err
	}
	if _, err := gitx.Commit(workspace, tip); err != nil {
		return "", fmt.Errorf("%s is not a commit in %s", tip, workspace)
	}
	base, err := gitx.Run(workspace, "merge-base", "HEAD", tip)
	if err != nil {
		return "", fmt.Errorf("no merge base between HEAD %s and %s in %s", head, tip, workspace)
	}
	return base, nil
}

// Head is the workspace's commit and branch; branch is "" when HEAD is detached.
func Head(workspace string) (commit, branch string, err error) {
	commit, err = gitx.Commit(workspace, "HEAD")
	if err != nil {
		return "", "", err
	}
	ref, err := gitx.Run(workspace, "rev-parse", "--symbolic-full-name", "HEAD")
	if err != nil {
		return "", "", err
	}
	if ref == "HEAD" {
		return commit, "", nil
	}
	return commit, strings.TrimPrefix(ref, "refs/heads/"), nil
}
