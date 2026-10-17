package wk

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/testrepo"
)

func TestRemoveDeadRemovesARemovableWorktreeByItsRelativePath(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "dead")
	expect(t, e, dir, Removable, "an ancestor of main")

	var out bytes.Buffer
	if err := e.RemoveDead(&out, filepath.Join(".claude", "worktrees", "dead")); err != nil {
		t.Fatalf("RemoveDead: %v\n%s", err, out.String())
	}
	assertGone(t, f, dir)
	if gitx.BranchExists(f.Repo, "worktree-dead") {
		t.Fatal("the branch of an ancestor worktree was kept")
	}
}

func TestRemoveDeadKeepsAWorktreeThatIsNotDead(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard) // the real clock: a worktree made just now is still busy
	dir := addWorktree(t, f, "fresh")

	var out bytes.Buffer
	err := e.RemoveDead(&out, dir)
	if err == nil || !strings.Contains(err.Error(), "not dead") {
		t.Fatalf("err = %v, want a refusal because the worktree is not dead", err)
	}
	assertStays(t, dir)
}

func TestRemoveDeadPrunesAnEntryWhoseDirectoryIsGone(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "vanished")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := e.RemoveDead(&out, dir); err != nil {
		t.Fatalf("RemoveDead: %v", err)
	}
	assertGone(t, f, dir)
	if !gitx.BranchExists(f.Repo, "worktree-vanished") {
		t.Fatal("prune deleted a branch; only the worktree entry may go")
	}
}

func TestRemoveDeadRefusesAPathThatIsNotAWorktree(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	outside := t.TempDir()

	var out bytes.Buffer
	if err := e.RemoveDead(&out, outside); err == nil || !strings.Contains(err.Error(), "not a worktree") {
		t.Fatalf("err = %v, want a refusal for a path outside the repository", err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Fatalf("the outside directory was touched: %v", err)
	}
}

func TestRemoveDeadNeverRemovesTheMainCheckout(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)

	var out bytes.Buffer
	if err := e.RemoveDead(&out, f.Repo); err == nil {
		t.Fatal("RemoveDead accepted the main checkout")
	}
	assertStays(t, f.Repo)
}
