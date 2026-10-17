package wk

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/testrepo"
)

const agentID = "a0123456789abcdef"

func stopInput(t *testing.T, id, cwd string) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(map[string]string{"hook_event_name": "SubagentStop", "agent_id": id, "cwd": cwd})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}

func removeInput(t *testing.T, path string) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(map[string]string{"hook_event_name": "WorktreeRemove", "worktree_path": path})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}

func stoppedMarker(t *testing.T, dir string) string {
	t.Helper()
	gd, err := privateGitDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(gd, StoppedMarkerName)
}

// openAt opens the fixture with its clock d after now, so a worktree made by the test has been
// quiet for d.
func openAt(t *testing.T, f testrepo.Fixture, d time.Duration) *Env {
	t.Helper()
	e := open(t, f, io.Discard)
	at := time.Now().Add(d)
	e.clock = func() time.Time { return at }
	return e
}

func TestSubagentStopMarksOnlyTheAgentsOwnWorktree(t *testing.T) {
	f := testrepo.New(t)
	own := addWorktree(t, f, "agent-"+agentID)
	other := addWorktree(t, f, "agent-a1111111111111111")

	if got := SubagentStop(f.Home, stopInput(t, agentID, filepath.Join(own, "web"))); !strings.HasPrefix(got, "marked ") {
		t.Fatalf("SubagentStop = %q", got)
	}
	read(t, stoppedMarker(t, own))
	for name, in := range map[string]*bytes.Reader{
		"parent's checkout":    stopInput(t, agentID, f.Repo),
		"another agent's":      stopInput(t, agentID, other),
		"not an agent id":      stopInput(t, "general-purpose", own),
		"relative cwd":         stopInput(t, agentID, "demo"),
		"unreadable input":     bytes.NewReader([]byte("{")),
		"cwd in no git at all": stopInput(t, agentID, t.TempDir()),
	} {
		t.Run(name, func(t *testing.T) {
			if got := SubagentStop(f.Home, in); strings.HasPrefix(got, "marked ") {
				t.Fatalf("marked something: %q", got)
			}
		})
	}
	if _, err := os.Stat(stoppedMarker(t, other)); !os.IsNotExist(err) {
		t.Fatal("another agent's worktree was marked")
	}
	gd, err := privateGitDir(f.Repo)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(gd, StoppedMarkerName)); !os.IsNotExist(err) {
		t.Fatal("the main checkout was marked")
	}
}

func markStopped(t *testing.T, f testrepo.Fixture, dir string) {
	t.Helper()
	id := strings.TrimPrefix(filepath.Base(dir), "agent-")
	if got := SubagentStop(f.Home, stopInput(t, id, dir)); !strings.HasPrefix(got, "marked ") {
		t.Fatalf("SubagentStop = %q", got)
	}
}

// A stopped subagent's clean worktree goes after the short window even though main does not
// hold its commits; the branch stays for /land-worktrees (BD-11).
func TestGCRemovesAStoppedSubagentWorktreeAndKeepsItsBranch(t *testing.T) {
	f := testrepo.New(t)
	dir := addWorktree(t, f, "agent-"+agentID)
	writeIn(t, dir, "web/agent.ts", "export const done = 1;\n")
	commitIn(t, dir, "agent work")
	markStopped(t, f, dir)

	expect(t, openAt(t, f, 5*time.Minute), dir, Kept, "a stopped subagent's worktree waits 15 minutes")
	e := openAt(t, f, 16*time.Minute)
	j := expect(t, e, dir, Removable, "branch worktree-agent-"+agentID+" is kept")
	if j.DeleteBranch {
		t.Fatal("gc would delete a branch main does not hold")
	}
	apply(t, e)
	assertGone(t, f, dir)
	if !strings.Contains(f.Git(t, "branch", "--list", "worktree-agent-"+agentID), "worktree-agent-"+agentID) {
		t.Fatal("the agent's branch went with its worktree")
	}
}

func TestGCKeepsADirtyStoppedSubagentWorktree(t *testing.T) {
	f := testrepo.New(t)
	dir := addWorktree(t, f, "agent-"+agentID)
	markStopped(t, f, dir)
	writeIn(t, dir, "web/half.ts", "export const half = 1;\n")
	expect(t, openAt(t, f, 2*time.Hour), dir, KeptForLanding, "1 changed path")
}

// A running subagent leaves no process in its worktree between tool calls, so without a stop
// marker its worktree waits a day even when it is absorbed and clean.
func TestGCGivesAnUnmarkedSubagentWorktreeADay(t *testing.T) {
	f := testrepo.New(t)
	dir := addWorktree(t, f, "agent-"+agentID)
	expect(t, openAt(t, f, 2*time.Hour), dir, Kept, "no stop marker waits 24 hours")
	expect(t, openAt(t, f, 25*time.Hour), dir, Removable, "an ancestor of main")
	named := addWorktree(t, f, "swift-fox-ab12")
	expect(t, openAt(t, f, 2*time.Hour), named, Removable, "an ancestor of main")
}

func TestWorktreeRemoveForcesItAndKeepsAnUnlandedBranch(t *testing.T) {
	f := testrepo.New(t)
	dir := addWorktree(t, f, "agent-"+agentID)
	writeIn(t, dir, "web/agent.ts", "export const a = 1;\n")
	commitIn(t, dir, "agent work")
	writeIn(t, dir, "web/scratch.ts", "uncommitted\n")
	what, err := WorktreeRemove(f.Home, removeInput(t, dir))
	if err != nil {
		t.Fatal(err)
	}
	assertGone(t, f, dir)
	if !strings.Contains(what, "kept") || !strings.Contains(f.Git(t, "branch", "--list", "worktree-agent-"+agentID), agentID) {
		t.Fatalf("what = %q; the unlanded branch must stay", what)
	}

	landed := addWorktree(t, f, "landed")
	if what, err := WorktreeRemove(f.Home, removeInput(t, landed)); err != nil || !strings.Contains(what, "deleted") {
		t.Fatalf("what = %q, err = %v; want the branch main holds deleted", what, err)
	}
	if log := hookLog(t, f); !strings.Contains(log, "WorktreeRemove removed "+dir) {
		t.Fatalf("hook.log:\n%s", log)
	}
}

func TestWorktreeRemoveRefusals(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	if err := e.RefreshTemplate(); err != nil {
		t.Fatal(err)
	}
	plain := filepath.Join(f.Root, "not-a-worktree")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}
	for name, tc := range map[string]struct {
		path, want string
	}{
		"main checkout": {f.Repo, "main checkout"},
		"template":      {e.TemplateDir(), "belongs to wk itself"},
		"relative":      {"demo", "not absolute"},
		"not a repo":    {plain, "rev-parse"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := WorktreeRemove(f.Home, removeInput(t, tc.path)); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
		})
	}
	read(t, filepath.Join(e.TemplateDir(), "README.md"))
}

// A stopped subagent on a detached HEAD has no branch to keep its commits, so the stopped
// exception does not apply: gc keeps the worktree until main holds the commit (F1, 2026-10-16).
func TestGCKeepsAStoppedDetachedSubagentWorktreeWithUnlandedCommits(t *testing.T) {
	f := testrepo.New(t)
	dir := filepath.Join(f.Repo, ".claude/worktrees", "agent-"+agentID)
	f.Git(t, "worktree", "add", "--quiet", "--detach", dir, "main")
	writeIn(t, dir, "web/agent.ts", "export const done = 1;\n")
	commitIn(t, dir, "agent work on a detached head")
	markStopped(t, f, dir)

	e := openAt(t, f, 16*time.Minute)
	expect(t, e, dir, Kept, "its detached HEAD has commits main does not hold")
	apply(t, e)
	assertStays(t, dir)

	f.Git(t, "merge", "--ff-only", testrepo.Git(t, dir, "rev-parse", "HEAD"))
	j := expect(t, openAt(t, f, 16*time.Minute), dir, Removable, "detached HEAD is an ancestor of main")
	if j.DeleteBranch {
		t.Fatal("a detached HEAD has no branch to delete")
	}
}
