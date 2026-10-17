package landing

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/zach-short/wk/internal/gitx"
)

// Sync moves workspace from its HEAD to tip and carries its uncommitted work across: the
// working tree, built into a tree through a private index, is merged onto tip with base as
// the merge base (Predict); the result is read into the index and the working tree; the
// branch is moved to tip with its old value as the guard; the index is reset to tip so the
// work shows as uncommitted again (DESIGN.md option R). A conflict leaves git's markers in
// the working tree and comes back as the paths; nothing is committed on any path. A workspace
// whose HEAD is not an ancestor of tip has commits of its own and is refused (PD-7), because
// the branch move keeps no commits.
func Sync(workspace, base, tip string) ([]string, error) {
	if err := notPrimary(workspace); err != nil {
		return nil, err
	}
	head, branch, err := headOf(workspace)
	if err != nil {
		return nil, err
	}
	if !gitx.IsAncestor(workspace, head, tip) {
		return nil, fmt.Errorf("%s is at %s, which is not an ancestor of %s: it has commits of its own, and sync moves the branch without them", branch, head, tip)
	}
	tree, err := workTree(workspace)
	if err != nil {
		return nil, err
	}
	result, conflicts, err := Predict(workspace, base, tip, tree)
	if err != nil {
		return nil, err
	}
	if _, err := gitx.Run(workspace, "read-tree", "-u", "--reset", result); err != nil {
		return nil, err
	}
	// The CAS is the last point of refusal, and by then the working tree has changed: say so.
	if _, err := gitx.Run(workspace, "update-ref", branch, tip, head); err != nil {
		return nil, fmt.Errorf("%s is no longer at %s, where sync read it, so it was not moved; the working tree and the index hold the merge of %s: run wk sync again (%w)", branch, head, tip, err)
	}
	if _, err := gitx.Run(workspace, "reset", "-q"); err != nil {
		return nil, fmt.Errorf("%s moved to %s, but the index still holds the merged tree: run git reset -q in %s (%w)", branch, tip, workspace, err)
	}
	return conflicts, nil
}

// notPrimary refuses the repository's main checkout: Sync moves the branch the checkout has,
// and on the primary that is main, which only FastForward moves (D15 reserves the primary
// for landing; `wk submit` refuses it too). Both paths come back from git as real paths, and
// are resolved once more so a symlinked workspace path cannot slip past.
func notPrimary(workspace string) error {
	repo, err := gitx.Resolve(workspace)
	if err != nil {
		return err
	}
	top, err := gitx.Run(workspace, "rev-parse", "--show-toplevel")
	if err != nil {
		return err
	}
	if realPath(top) == realPath(repo.Primary) {
		return fmt.Errorf("%s is the repository's main checkout, and wk sync runs in a workspace", workspace)
	}
	return nil
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// headOf is the workspace's HEAD and the full name of the branch it is on. A detached HEAD is
// refused: Sync moves a branch, and workspaces keep one (option R).
func headOf(workspace string) (head, branch string, err error) {
	head, err = gitx.Run(workspace, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return "", "", err
	}
	branch, err = gitx.Run(workspace, "symbolic-ref", "--quiet", "HEAD")
	if err != nil {
		return "", "", fmt.Errorf("%s has a detached HEAD, and sync moves a branch", workspace)
	}
	return head, branch, nil
}

// workTree is the workspace's working tree as a tree object: tracked edits, deletions and
// untracked files that are not ignored, through a private index so the workspace's own index
// is never opened for writing. The index lives in the workspace's git directory, outside the
// working tree, and goes on every path. This is the sequence `wk submit` builds its tree with
// (PLAN-P2-P4.md section 3, item 1); row 4 may replace it with changeset.Tree.
func workTree(workspace string) (string, error) {
	gitDir, err := gitx.GitDir(workspace)
	if err != nil {
		return "", err
	}
	dir, err := os.MkdirTemp(gitDir, "wk-sync-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	env := []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")}
	for _, args := range [][]string{{"read-tree", "HEAD"}, {"add", "-A"}} {
		if _, _, err := run(workspace, env, args...); err != nil {
			return "", err
		}
	}
	tree, _, err := run(workspace, env, "write-tree")
	return tree, err
}
