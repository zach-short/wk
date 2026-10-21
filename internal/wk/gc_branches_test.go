package wk

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/testrepo"
)

// orphan makes branch b with no worktree. With work, it carries one commit main does not hold,
// made in a worktree that is then removed, as gc leaves a stopped subagent's branch.
func orphan(t *testing.T, f testrepo.Fixture, b string, work bool) {
	t.Helper()
	if !work {
		f.Git(t, "branch", b, "main")
		return
	}
	dir := t.TempDir() + "/wt"
	f.Git(t, "worktree", "add", "--quiet", "-b", b, dir, "main")
	writeIn(t, dir, "work.txt", "unlanded\n")
	commitIn(t, dir, "unlanded work")
	f.Git(t, "worktree", "remove", dir)
}

func dryRun(t *testing.T, e *Env) string {
	t.Helper()
	var out bytes.Buffer
	if err := e.GC(&out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	return out.String()
}

// branchLine is the line gc printed for branch b, or "" when it printed none.
func branchLine(out, b string) string {
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, " branch "+b+" ") {
			return line
		}
	}
	return ""
}

func TestGCDeletesAMergedOrphanBranch(t *testing.T) {
	f := testrepo.New(t)
	orphan(t, f, "worktree-agent-a0123456789abcdef", false)
	orphan(t, f, "wk/12", false)
	f.Write(t, "later.txt", "main moved on\n")
	f.Commit(t, "main moves past the branches")
	e := idle(t, f)

	for _, b := range []string{"worktree-agent-a0123456789abcdef", "wk/12"} {
		if line := branchLine(dryRun(t, e), b); !strings.HasPrefix(line, string(Removable)) {
			t.Fatalf("dry run line for %s: %q", b, line)
		}
	}
	out := apply(t, e)
	for _, b := range []string{"worktree-agent-a0123456789abcdef", "wk/12"} {
		if gitx.BranchExists(f.Repo, b) {
			t.Fatalf("merged orphan %s was kept\n%s", b, out)
		}
		if !strings.HasPrefix(branchLine(out, b), "deleted") {
			t.Fatalf("apply did not say it deleted %s:\n%s", b, out)
		}
	}
	if !strings.Contains(out, "2 of 2 orphan branches deleted") {
		t.Fatalf("apply summary:\n%s", out)
	}
}

func TestGCKeepsAnUnmergedOrphanBranch(t *testing.T) {
	f := testrepo.New(t)
	orphan(t, f, "worktree-stopped", true)
	e := idle(t, f)

	line := branchLine(dryRun(t, e), "worktree-stopped")
	if !strings.HasPrefix(line, string(Kept)) || !strings.Contains(line, "1 commit main does not hold") {
		t.Fatalf("dry run line: %q", line)
	}
	out := apply(t, e)
	if !gitx.BranchExists(f.Repo, "worktree-stopped") {
		t.Fatalf("an unmerged orphan was deleted\n%s", out)
	}
}

// A pool claim writes its branch before the slot's HEAD points at it, so a branch that moved
// inside the quiet window is kept even when it is an orphan at the base.
func TestGCKeepsAnOrphanBranchThatJustMoved(t *testing.T) {
	f := testrepo.New(t)
	orphan(t, f, "worktree-claiming", false)
	e := open(t, f, io.Discard)

	out := apply(t, e)
	if !gitx.BranchExists(f.Repo, "worktree-claiming") {
		t.Fatalf("a branch that moved a moment ago was deleted\n%s", out)
	}
	if line := branchLine(out, "worktree-claiming"); !strings.Contains(line, "a new workspace may be taking it") {
		t.Fatalf("apply line: %q", line)
	}
}

func TestGCSkipsABranchAWorktreeHasCheckedOut(t *testing.T) {
	f := testrepo.New(t)
	dir := addWorktree(t, f, "held")
	f.Git(t, "worktree", "lock", "--reason", "held by the test", dir)
	e := idle(t, f)

	if line := branchLine(dryRun(t, e), "worktree-held"); line != "" {
		t.Fatalf("gc judged a checked-out branch: %q", line)
	}
	out := apply(t, e)
	assertStays(t, dir)
	if !gitx.BranchExists(f.Repo, "worktree-held") || branchLine(out, "worktree-held") != "" {
		t.Fatalf("gc touched a checked-out branch\n%s", out)
	}
}

func TestGCSkipsBranchesOutsideItsPatterns(t *testing.T) {
	f := testrepo.New(t)
	skipped := []string{"feature-x", "wk/landing", "wk/twelve", "worktree-", "row-65-build"}
	for _, b := range skipped {
		orphan(t, f, b, false)
	}
	e := idle(t, f)

	dry := dryRun(t, e)
	out := apply(t, e)
	for _, b := range append(skipped, "main") {
		if branchLine(dry, b) != "" || branchLine(out, b) != "" {
			t.Fatalf("gc judged %s, which no pattern names:\n%s\n%s", b, dry, out)
		}
		if !gitx.BranchExists(f.Repo, b) {
			t.Fatalf("gc deleted %s, which no pattern names\n%s", b, out)
		}
	}
	if strings.Contains(dry, "orphan branches") {
		t.Fatalf("with no orphan of its own, gc printed a branch summary:\n%s", dry)
	}
}

func TestGCDeletesARenamedBranchAPatternNames(t *testing.T) {
	f := testrepo.New(t)
	orphan(t, f, "row-65-build", false)
	orphan(t, f, "row-66-build", true)
	e := idle(t, f)
	e.Cfg.GC.BranchPatterns = []string{"row-*"}

	out := apply(t, e)
	if gitx.BranchExists(f.Repo, "row-65-build") {
		t.Fatalf("the merged branch gc.branch_patterns names was kept\n%s", out)
	}
	if !gitx.BranchExists(f.Repo, "row-66-build") {
		t.Fatalf("the unmerged branch gc.branch_patterns names was deleted\n%s", out)
	}
}

func TestGCDryRunDeletesNoBranch(t *testing.T) {
	f := testrepo.New(t)
	orphan(t, f, "worktree-landed", false)
	orphan(t, f, "worktree-stopped", true)
	e := idle(t, f)
	before := f.Git(t, "for-each-ref", "refs/heads/")

	out := dryRun(t, e)
	if after := f.Git(t, "for-each-ref", "refs/heads/"); after != before {
		t.Fatalf("the dry run changed the branches:\nbefore\n%s\nafter\n%s", before, after)
	}
	if !strings.Contains(out, "1 of 2 orphan branches removable; dry run, nothing deleted") {
		t.Fatalf("dry run output:\n%s", out)
	}
}
