package changeset

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/testrepo"
)

func TestMain(m *testing.M) {
	testrepo.Isolate()
	os.Exit(m.Run())
}

// workspace is a linked worktree on its own branch, the shape `wk new` hands a session.
func workspace(t *testing.T, f testrepo.Fixture, name string) string {
	t.Helper()
	dir := filepath.Join(f.Root, name)
	f.Git(t, "worktree", "add", "--quiet", "-b", "wk/"+name, dir)
	return dir
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func indexPath(t *testing.T, dir string) string {
	t.Helper()
	return filepath.Join(testrepo.Git(t, dir, "rev-parse", "--absolute-git-dir"), "index")
}

func readFile(t *testing.T, p string) []byte {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestTreeTakesTheWorkingTreeNotTheIndex(t *testing.T) {
	f := testrepo.New(t)
	ws := workspace(t, f, "a")
	write(t, ws, "web/new.ts", "export const fresh = 1;\n")
	write(t, ws, ".env", "SECRET=workspace\n")
	write(t, ws, "README.md", "changed\n")
	if err := os.Remove(filepath.Join(ws, "web", "page.ts")); err != nil {
		t.Fatal(err)
	}
	tree, err := Tree(ws)
	if err != nil {
		t.Fatal(err)
	}
	paths := strings.Split(testrepo.Git(t, ws, "ls-tree", "-r", "--name-only", tree), "\n")
	if !slices.Contains(paths, "web/new.ts") {
		t.Fatalf("tree %v lacks the untracked file", paths)
	}
	if slices.Contains(paths, ".env") {
		t.Fatalf("tree %v holds the ignored file", paths)
	}
	if slices.Contains(paths, "web/page.ts") {
		t.Fatalf("tree %v still holds the deleted file", paths)
	}
	if got := testrepo.Git(t, ws, "cat-file", "-p", tree+":README.md"); got != "changed" {
		t.Fatalf("README.md in the tree = %q; want the working tree's content", got)
	}
	if testrepo.Git(t, ws, "diff", "--cached", "--name-only") != "" {
		t.Fatal("the workspace's own index was staged")
	}
}

func TestTreeLeavesTheIndexAndNoTempFile(t *testing.T) {
	f := testrepo.New(t)
	for name, dir := range map[string]string{"main checkout": f.Repo, "worktree": workspace(t, f, "b")} {
		t.Run(name, func(t *testing.T) {
			write(t, dir, "web/new.ts", "export const fresh = 1;\n")
			index := indexPath(t, dir)
			before := readFile(t, index)
			if _, err := Tree(dir); err != nil {
				t.Fatal(err)
			}
			if after := readFile(t, index); !slices.Equal(before, after) {
				t.Fatal("the workspace's index changed")
			}
			left, err := filepath.Glob(filepath.Join(filepath.Dir(index), "wk-index-*"))
			if err != nil || len(left) != 0 {
				t.Fatalf("temporary index left behind: %v %v", left, err)
			}
		})
	}
}

// A sparse workspace (`wk new` with workspace.sparse_exclude) has no file outside its cone.
// `add -A` must not read that absence as a deletion: the path keeps its HEAD blob.
func TestTreeKeepsPathsOutsideTheSparseCone(t *testing.T) {
	f := testrepo.New(t)
	ws := workspace(t, f, "sparse")
	testrepo.Git(t, ws, "sparse-checkout", "set", "--no-cone", "/*", "!/docs/incomplete/")
	if _, err := os.Stat(filepath.Join(ws, "docs", "incomplete", "plan", "PLAN.md")); err == nil {
		t.Fatal("the sparse checkout still holds docs/incomplete")
	}
	write(t, ws, "web/new.ts", "export const fresh = 1;\n")
	tree, err := Tree(ws)
	if err != nil {
		t.Fatal(err)
	}
	want := testrepo.Git(t, ws, "rev-parse", "HEAD:docs/incomplete/plan/PLAN.md")
	if got := testrepo.Git(t, ws, "rev-parse", tree+":docs/incomplete/plan/PLAN.md"); got != want {
		t.Fatalf("docs/incomplete/plan/PLAN.md = %q; want HEAD's blob %q", got, want)
	}
}

func TestBase(t *testing.T) {
	f := testrepo.New(t)
	root := testrepo.Git(t, f.Repo, "rev-parse", "HEAD")
	ws := workspace(t, f, "c")
	write(t, ws, "feature.ts", "export const feature = 1;\n")
	testrepo.Git(t, ws, "add", "--all")
	testrepo.Git(t, ws, "commit", "--quiet", "--no-verify", "-m", "feature")
	f.Write(t, "main.ts", "export const main = 1;\n")
	f.Commit(t, "main moves")
	base, err := Base(ws, "main")
	if err != nil || base != root {
		t.Fatalf("Base = %q, %v; want the fork point %s", base, err, root)
	}
	orphan := testrepo.Git(t, f.Repo, "commit-tree", "HEAD^{tree}", "-m", "orphan")
	head := testrepo.Git(t, ws, "rev-parse", "HEAD")
	_, err = Base(ws, orphan)
	if err == nil || !strings.Contains(err.Error(), head) || !strings.Contains(err.Error(), orphan) {
		t.Fatalf("Base on an unrelated history = %v; want an error naming %s and %s", err, head, orphan)
	}
	if _, err := Base(ws, "no-such-ref"); err == nil {
		t.Fatal("Base accepted a tip that is not a commit")
	}
}

func TestHead(t *testing.T) {
	f := testrepo.New(t)
	ws := workspace(t, f, "d")
	want := testrepo.Git(t, ws, "rev-parse", "HEAD")
	commit, branch, err := Head(ws)
	if err != nil || commit != want || branch != "wk/d" {
		t.Fatalf("Head = %q, %q, %v; want %s on wk/d", commit, branch, err, want)
	}
	testrepo.Git(t, ws, "checkout", "--quiet", "--detach")
	if _, branch, err = Head(ws); err != nil || branch != "" {
		t.Fatalf("detached Head branch = %q, %v; want empty", branch, err)
	}
}

func TestQueueFoldsToTheLastLine(t *testing.T) {
	q := Queue{Path: filepath.Join(t.TempDir(), "queue.jsonl")}
	if got, err := q.Load(); err != nil || len(got) != 0 {
		t.Fatalf("Load of a missing file = %v, %v; want empty", got, err)
	}
	at := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	for _, r := range []Record{
		{ID: "aaa", State: Submitted, Tree: "t1", Base: "b1", Message: "one", Workspace: "/ws/a", At: at},
		{ID: "bbb", State: Submitted, Tree: "t2", Base: "b1", Message: "two", Workspace: "/ws/b", At: at},
		{ID: "aaa", State: Refused, Paths: []string{"web/page.ts"}, At: at},
		{ID: "aaa", State: Landed, Commit: "c1", At: at},
	} {
		if err := q.Append(r); err != nil {
			t.Fatal(err)
		}
	}
	if lines := strings.Count(string(readFile(t, q.Path)), "\n"); lines != 4 {
		t.Fatalf("the file holds %d lines; want 4, one per Append", lines)
	}
	got, err := q.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "aaa" || got[0].State != Landed || got[0].Commit != "c1" || got[1].ID != "bbb" {
		t.Fatalf("Load = %+v; want aaa landed then bbb submitted", got)
	}
}

func TestQueueNamesTheCorruptLine(t *testing.T) {
	q := Queue{Path: filepath.Join(t.TempDir(), "queue.jsonl")}
	if err := q.Append(Record{ID: "aaa", State: Submitted}); err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(q.Path, os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("{\"id\":\"bbb\",\"state\n"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := q.Load(); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Fatalf("Load = %v; want an error naming line 2", err)
	}
}

func TestNewID(t *testing.T) {
	at := time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)
	id := NewID("tree", "base", at)
	if len(id) != 12 || strings.Trim(id, "0123456789abcdef") != "" {
		t.Fatalf("NewID = %q; want 12 hex characters", id)
	}
	if NewID("tree", "base", at) != id {
		t.Fatal("NewID is not deterministic")
	}
	if NewID("tree", "base", at.Add(time.Nanosecond)) == id {
		t.Fatal("NewID ignores the time")
	}
}

func TestTreeRemovesTheTempIndexOnFailure(t *testing.T) {
	dir := t.TempDir()
	testrepo.Git(t, dir, "init", "--quiet", "--initial-branch=main")
	if _, err := Tree(dir); err == nil {
		t.Fatal("Tree succeeded with no HEAD to read")
	}
	left, err := filepath.Glob(filepath.Join(dir, ".git", "wk-index-*"))
	if err != nil || len(left) != 0 {
		t.Fatalf("temporary index left behind after the failure: %v %v", left, err)
	}
}
