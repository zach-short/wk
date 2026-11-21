package wk

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/testrepo"
)

// idle opens the fixture with a clock two hours ahead, so every worktree made by the test is
// past the 60-minute quiet window unless the test touches it later than that.
func idle(t *testing.T, f testrepo.Fixture) *Env {
	t.Helper()
	e := open(t, f, io.Discard)
	ahead := time.Now().Add(2 * time.Hour)
	e.clock = func() time.Time { return ahead }
	return e
}

func addWorktree(t *testing.T, f testrepo.Fixture, name string) string {
	t.Helper()
	dir := filepath.Join(f.Repo, ".claude/worktrees", name)
	f.Git(t, "worktree", "add", "--quiet", "-b", "worktree-"+name, dir, "main")
	return dir
}

func writeIn(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func commitIn(t *testing.T, dir, message string) {
	t.Helper()
	testrepo.Git(t, dir, "add", "--all")
	testrepo.Git(t, dir, "commit", "--quiet", "--no-verify", "-m", message)
}

func judgments(t *testing.T, e *Env) map[string]Judgment {
	t.Helper()
	list, err := e.judgeAll()
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]Judgment{}
	for _, j := range list {
		byPath[j.Worktree.Path] = j
	}
	return byPath
}

func expect(t *testing.T, e *Env, dir string, verdict Verdict, reason string) Judgment {
	t.Helper()
	j, ok := judgments(t, e)[dir]
	if !ok {
		t.Fatalf("gc did not judge %s", dir)
	}
	if j.Verdict != verdict || !strings.Contains(strings.Join(j.Reasons, "; "), reason) {
		t.Fatalf("%s: verdict %q, reasons %q; want %q containing %q", filepath.Base(dir), j.Verdict, j.Reasons, verdict, reason)
	}
	return j
}

func apply(t *testing.T, e *Env) string {
	t.Helper()
	var out bytes.Buffer
	if err := e.GCApply(&out); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	return out.String()
}

func assertGone(t *testing.T, f testrepo.Fixture, dir string) {
	t.Helper()
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("%s still exists (err %v)", dir, err)
	}
	if strings.Contains(f.Git(t, "worktree", "list", "--porcelain"), dir) {
		t.Fatalf("git still lists %s", dir)
	}
}

func assertStays(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("%s was removed: %v", dir, err)
	}
}

func TestGCRemovesAnAncestorAndItsBranch(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "landed")
	j := expect(t, e, dir, Removable, "an ancestor of main")
	if !j.DeleteBranch {
		t.Fatal("an ancestor's branch must be deleted with it (BD-11)")
	}
	out := apply(t, e)
	assertGone(t, f, dir)
	if gitx.BranchExists(f.Repo, "worktree-landed") {
		t.Fatalf("branch worktree-landed was kept\n%s", out)
	}
	if !strings.Contains(out, "branch worktree-landed deleted") {
		t.Fatalf("apply did not say it deleted the branch:\n%s", out)
	}
}

// Work that reached main by another commit (a squash or a copy) is absorbed by content: the
// worktree goes, and its branch stays for /land-worktrees, because `git branch -d` would call
// it unmerged and BD-11 never forces it.
func TestGCKeepsTheBranchOfWorkAbsorbedByContent(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "squashed")
	writeIn(t, dir, "web/feature.ts", "export const feature = 1;\n")
	commitIn(t, dir, "feature on the branch")
	f.Write(t, "web/feature.ts", "export const feature = 1;\n")
	f.Commit(t, "the same feature, squashed onto main")

	j := expect(t, e, dir, Removable, "by content only")
	if j.DeleteBranch {
		t.Fatal("a branch absorbed by content alone must be kept (BD-11)")
	}
	apply(t, e)
	assertGone(t, f, dir)
	if !gitx.BranchExists(f.Repo, "worktree-squashed") {
		t.Fatal("the branch of content-absorbed work was deleted")
	}
}

func TestGCKeepsUnlandedCommits(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "unlanded")
	writeIn(t, dir, "web/new.ts", "export const n = 1;\n")
	commitIn(t, dir, "work main does not have")
	expect(t, e, dir, Kept, "branch worktree-unlanded has commits main does not hold")
	apply(t, e)
	assertStays(t, dir)
}

func TestGCKeepsDirtyAbsorbedWorktreesForLandWorktrees(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	tracked := addWorktree(t, f, "tracked")
	writeIn(t, tracked, "README.md", "edited\n")
	untracked := addWorktree(t, f, "untracked")
	writeIn(t, untracked, "notes/draft.md", "a draft\n")
	staged := addWorktree(t, f, "staged")
	writeIn(t, staged, "web/staged.ts", "x\n")
	testrepo.Git(t, staged, "add", "web/staged.ts")
	deleted := addWorktree(t, f, "deleted")
	if err := os.Remove(filepath.Join(deleted, "web/page.ts")); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{tracked, untracked, staged, deleted} {
		expect(t, e, dir, KeptForLanding, "1 changed path")
	}
	apply(t, e)
	for _, dir := range []string{tracked, untracked, staged, deleted} {
		assertStays(t, dir)
	}
}

// Ignored files are not work: node_modules and a copied .env go with the worktree, and
// `git worktree remove` without --force agrees.
func TestGCRemovesAWorktreeWithOnlyIgnoredChanges(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "ignored")
	writeIn(t, dir, "node_modules/react/index.js", "react\n")
	writeIn(t, dir, ".env", "SECRET=copy\n")
	writeIn(t, dir, "web/.next/cache/x", "cache\n")
	expect(t, e, dir, Removable, "clean and idle")
	apply(t, e)
	assertGone(t, f, dir)
}

func TestGCKeepsALockedWorktree(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "locked")
	f.Git(t, "worktree", "lock", "--reason", "a person is using it", dir)
	expect(t, e, dir, Kept, "it is locked (a person is using it)")
	apply(t, e)
	assertStays(t, dir)
}

// A live session is not locked (PLAN.md section 0.2), so a process standing in the worktree
// is the only thing between gc and removing it. Here it stands in a subdirectory.
func TestGCKeepsAWorktreeWithAProcessInside(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "live")
	sleeper := exec.Command("sleep", "60")
	sleeper.Dir = filepath.Join(dir, "web")
	if err := sleeper.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sleeper.Process.Kill(); _ = sleeper.Wait() })

	expect(t, e, dir, Kept, "a process is working in it")
	apply(t, e)
	assertStays(t, dir)
}

// BD-10: with no HEAD reflog, lib.sh reads a worktree as decades idle. gc reads the files too.
// The worktree is detached because on a branch git falls back to the branch's own reflog.
func TestGCReadsAnEmptyReflogWithRecentFilesAsBusy(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := filepath.Join(f.Repo, ".claude/worktrees/noreflog")
	f.Git(t, "worktree", "add", "--quiet", "--detach", dir, "main")
	gd := testrepo.Git(t, dir, "rev-parse", "--absolute-git-dir")
	if err := os.Remove(filepath.Join(gd, "logs/HEAD")); err != nil {
		t.Fatal(err)
	}
	if out := testrepo.Git(t, dir, "reflog", "-1", "HEAD"); out != "" {
		t.Fatalf("the reflog is not empty: %q", out)
	}
	recent := e.clock().Add(-5 * time.Minute)
	if err := os.Chtimes(dir, recent, recent); err != nil {
		t.Fatal(err)
	}
	expect(t, e, dir, Kept, "it changed 5 minutes ago")

	old := e.clock().Add(-3 * time.Hour)
	if err := os.Chtimes(dir, old, old); err != nil {
		t.Fatal(err)
	}
	expect(t, e, dir, Removable, "clean and idle")
}

func TestGCKeepsAWorktreeMadeJustNow(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	dir := addWorktree(t, f, "fresh")
	expect(t, e, dir, Kept, "it changed 0 minutes ago")
}

// gc's own reads must not count as activity: `git diff HEAD` rewrites the index unless
// GIT_OPTIONAL_LOCKS=0, and the index mtime is one of BD-10's clocks.
func TestGCReadsDoNotTouchTheIndex(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "quiet")
	writeIn(t, dir, "README.md", "edited\n")
	gd := testrepo.Git(t, dir, "rev-parse", "--absolute-git-dir")
	index := filepath.Join(gd, "index")
	old := time.Now().Add(-48 * time.Hour)
	if err := os.Chtimes(index, old, old); err != nil {
		t.Fatal(err)
	}
	judgments(t, e)
	info, err := os.Stat(index)
	if err != nil {
		t.Fatal(err)
	}
	if !info.ModTime().Equal(old) {
		t.Fatalf("gc moved the index mtime from %v to %v", old, info.ModTime())
	}
}

func TestGCSkipsTheTemplateAndATrial(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	if err := e.RefreshTemplate(); err != nil {
		t.Fatal(err)
	}
	trial := addWorktree(t, f, "trial")
	gd := testrepo.Git(t, trial, "rev-parse", "--absolute-git-dir")
	if err := os.WriteFile(filepath.Join(gd, TrialMarkerName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	gateTrial := addWorktree(t, f, "gate-trial")
	gd = testrepo.Git(t, gateTrial, "rev-parse", "--absolute-git-dir")
	if err := os.WriteFile(filepath.Join(gd, GateTrialMarkerName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	expect(t, e, e.TemplateDir(), Skipped, "the wk template")
	expect(t, e, trial, Skipped, "a land-worktrees trial")
	expect(t, e, gateTrial, Skipped, "the wk gate trial")
	apply(t, e)
	assertStays(t, e.TemplateDir())
	assertStays(t, trial)
	assertStays(t, gateTrial)
}

func TestGCNeverJudgesTheMainCheckout(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	if _, ok := judgments(t, e)[f.Repo]; ok {
		t.Fatal("gc judged the main checkout")
	}
}

func TestGCDetachedHeadIsRemovedWithNoBranch(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := filepath.Join(f.Repo, ".claude/worktrees/detached")
	f.Git(t, "worktree", "add", "--quiet", "--detach", dir, "main")
	expect(t, e, dir, Removable, "detached HEAD is an ancestor of main")
	apply(t, e)
	assertGone(t, f, dir)
}

func TestGCPrunesAnEntryWhoseDirectoryIsGone(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "vanished")
	// Unlanded work, so the branch pass keeps the orphan too and only prune is under test.
	writeIn(t, dir, "work.txt", "unlanded\n")
	commitIn(t, dir, "unlanded work")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	expect(t, e, dir, Prunable, "its directory is gone")
	apply(t, e)
	assertGone(t, f, dir)
	if !gitx.BranchExists(f.Repo, "worktree-vanished") {
		t.Fatal("prune deleted a branch; only the worktree entry may go")
	}
}

// The judgment of a pass can go stale before its removal runs: a session may start in the
// worktree. The removal judges again and keeps it.
func TestGCApplyJudgesAgainAtTheMomentOfRemoval(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "late")
	j := expect(t, e, dir, Removable, "clean and idle")
	writeIn(t, dir, "web/late.ts", "work that started after the pass\n")
	var out bytes.Buffer
	removed, err := e.removeOne(&out, j.Worktree)
	if err != nil || removed {
		t.Fatalf("removed = %v, err = %v; want it kept\n%s", removed, err, out.String())
	}
	assertStays(t, dir)
	if !strings.Contains(out.String(), string(KeptForLanding)) {
		t.Fatalf("the re-check did not say why it kept it:\n%s", out.String())
	}
}

// A `wk new` holding the template lock does not hold up removals, so gc never stalls the
// creates of a burst; only the prune waits (BD-5 as amended).
func TestGCApplyRemovesWhileAWkNewHoldsTheLock(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "busy-template")
	held, err := flock.Take(e.lockPath(), false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Release() }()
	done := make(chan error, 1)
	go func() { done <- e.GCApply(io.Discard) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("gc --apply waited for the template lock to remove a worktree")
	}
	assertGone(t, f, dir)
}

func TestGCApplyPruneWaitsForAWkNewHoldingTheLock(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "racing")
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	held, err := flock.Take(e.lockPath(), false, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &waitWriter{want: "waiting for the template lock to prune", seen: make(chan struct{})}
	e.log = &syncWriter{w: w}
	done := make(chan error, 1)
	go func() { done <- e.GCApply(io.Discard) }()

	select {
	case <-w.seen:
	case err := <-done:
		t.Fatalf("gc --apply finished while wk new held the lock: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("gc --apply never said it was waiting")
	}
	select {
	case err := <-done:
		t.Fatalf("gc --apply finished while wk new held the lock: %v", err)
	case <-time.After(200 * time.Millisecond):
	}
	if !strings.Contains(f.Git(t, "worktree", "list", "--porcelain"), dir) {
		t.Fatal("gc pruned while wk new held the lock")
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("gc --apply did not finish after the lock was released")
	}
	assertGone(t, f, dir)
}

func TestGCDryRunChangesNothing(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "candidate")
	var out bytes.Buffer
	if err := e.GC(&out); err != nil {
		t.Fatal(err)
	}
	assertStays(t, dir)
	if !gitx.BranchExists(f.Repo, "worktree-candidate") {
		t.Fatal("the dry run deleted a branch")
	}
	want := "removable                  .claude/worktrees/candidate"
	if !strings.Contains(out.String(), want) || !strings.Contains(out.String(), "1 of 1 removable; dry run") {
		t.Fatalf("dry run output:\n%s", out.String())
	}
}

const finding = "docs/incomplete/audit/finding.md"

// auditWorktree is an /audit agent's worktree after the lead copied its finding file into the
// main checkout and committed it on main: the worktree still has its own untracked copy.
func auditWorktree(t *testing.T, f testrepo.Fixture, name, content string) string {
	t.Helper()
	dir := addWorktree(t, f, name)
	f.Write(t, finding, "a finding\n")
	f.Commit(t, "the lead commits the finding")
	writeIn(t, dir, finding, content)
	return dir
}

func TestGCRemovesAWorktreeWhoseOnlyChangeTheBaseHolds(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := auditWorktree(t, f, "audit", "a finding\n")
	j := expect(t, e, dir, Removable, "1 untracked file the base already holds is deleted with it")
	if !j.DeleteBranch {
		t.Fatal("the branch is an ancestor of main, so it must go with the worktree (BD-11)")
	}
	out := apply(t, e)
	assertGone(t, f, dir)
	if gitx.BranchExists(f.Repo, "worktree-audit") {
		t.Fatalf("branch worktree-audit was kept\n%s", out)
	}
	if got := read(t, filepath.Join(f.Repo, finding)); got != "a finding\n" {
		t.Fatalf("the main checkout's copy changed: %q", got)
	}
}

func TestGCKeepsAnUntrackedFileTheBaseDoesNotHold(t *testing.T) {
	for _, tc := range []struct{ name, rel, content string }{
		{"differs", finding, "a finding, edited later\n"},
		{"new-path", "docs/incomplete/audit/other.md", "a finding\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := testrepo.New(t)
			e := idle(t, f)
			dir := addWorktree(t, f, tc.name)
			f.Write(t, finding, "a finding\n")
			f.Commit(t, "the lead commits the finding")
			writeIn(t, dir, tc.rel, tc.content)
			expect(t, e, dir, KeptForLanding, "1 changed path")
			apply(t, e)
			assertStays(t, dir)
			if got := read(t, filepath.Join(dir, tc.rel)); got != tc.content {
				t.Fatalf("%s changed: %q", tc.rel, got)
			}
		})
	}
}

func TestGCKeepsAHeldFileBesideATrackedEdit(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := auditWorktree(t, f, "mixed", "a finding\n")
	writeIn(t, dir, "README.md", "edited\n")
	expect(t, e, dir, KeptForLanding, "2 changed paths")
	apply(t, e)
	assertStays(t, dir)
	if got := read(t, filepath.Join(dir, finding)); got != "a finding\n" {
		t.Fatalf("the held file was deleted from a kept worktree: %q", got)
	}
}

// After `git rm --cached`, a path is a staged deletion and an untracked file with the base's
// bytes. The staged deletion is a tracked change, so the file is not held (rule 1 of row 31).
func TestGCKeepsAnUntrackedFileWhoseDeletionIsStaged(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "rm-cached")
	testrepo.Git(t, dir, "rm", "--cached", "--quiet", "README.md")
	j := expect(t, e, dir, KeptForLanding, "1 changed path")
	if len(j.held) != 0 {
		t.Fatalf("held = %v, want none", j.held)
	}
	var dry bytes.Buffer
	if err := e.GC(&dry); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(dry.String(), "the base already holds") {
		t.Fatalf("the dry run called the file held:\n%s", dry.String())
	}
	apply(t, e)
	assertStays(t, dir)
	if got := read(t, filepath.Join(dir, "README.md")); got != "demo\n" {
		t.Fatalf("README.md = %q, want it kept", got)
	}
	if st := testrepo.Git(t, dir, "status", "--porcelain"); !strings.Contains(st, "D  README.md") {
		t.Fatalf("the staged deletion is gone: %q", st)
	}
}

// A change only in the index leaves the working tree equal to HEAD, so `git diff HEAD` sees
// nothing, and `git worktree remove` still refuses. gc must keep the worktree and delete nothing.
func TestGCKeepsAHeldFileBesideAChangeOnlyInTheIndex(t *testing.T) {
	for _, tc := range []struct {
		name  string
		stage func(t *testing.T, dir string)
	}{
		{"staged-edit-undone", func(t *testing.T, dir string) {
			writeIn(t, dir, "README.md", "staged\n")
			testrepo.Git(t, dir, "add", "README.md")
			writeIn(t, dir, "README.md", "demo\n")
		}},
		{"added-then-deleted", func(t *testing.T, dir string) {
			writeIn(t, dir, "N.txt", "n\n")
			testrepo.Git(t, dir, "add", "N.txt")
			if err := os.Remove(filepath.Join(dir, "N.txt")); err != nil {
				t.Fatal(err)
			}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := testrepo.New(t)
			e := idle(t, f)
			dir := auditWorktree(t, f, tc.name, "a finding\n")
			tc.stage(t, dir)
			j, ok := judgments(t, e)[dir]
			if !ok || j.Verdict == Removable || len(j.held) != 0 {
				t.Fatalf("verdict %q, reasons %q, held %v; want it kept with nothing held", j.Verdict, j.Reasons, j.held)
			}
			var dry bytes.Buffer
			if err := e.GC(&dry); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(dry.String(), "the base already holds") {
				t.Fatalf("the dry run called the file held:\n%s", dry.String())
			}
			apply(t, e)
			assertStays(t, dir)
			if got := read(t, filepath.Join(dir, finding)); got != "a finding\n" {
				t.Fatalf("the held file was deleted: %q", got)
			}
		})
	}
}

func TestGCKeepsAnEqualFileWhoseExecutableBitDiffers(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := auditWorktree(t, f, "chmod", "a finding\n")
	if err := os.Chmod(filepath.Join(dir, finding), 0o755); err != nil {
		t.Fatal(err)
	}
	expect(t, e, dir, KeptForLanding, "1 changed path")
	apply(t, e)
	assertStays(t, dir)
	if got := read(t, filepath.Join(dir, finding)); got != "a finding\n" {
		t.Fatalf("the executable copy was deleted: %q", got)
	}
}

// Deleting an untracked .gitignore can turn the files it hides into untracked files, and then
// git refuses the removal after gc deleted files, so a .gitignore is never held.
func TestGCKeepsAnUntrackedGitignoreTheBaseHolds(t *testing.T) {
	const ignore = "docs/incomplete/audit/.gitignore"
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "ignore")
	f.Write(t, ignore, "*.log\n")
	f.Commit(t, "the lead commits the .gitignore")
	writeIn(t, dir, ignore, "*.log\n")
	writeIn(t, dir, "docs/incomplete/audit/run.log", "hidden\n")
	j := expect(t, e, dir, KeptForLanding, "1 changed path")
	if len(j.held) != 0 {
		t.Fatalf("held = %v, want none", j.held)
	}
	apply(t, e)
	assertStays(t, dir)
	if got := read(t, filepath.Join(dir, ignore)); got != "*.log\n" {
		t.Fatalf("the .gitignore was deleted: %q", got)
	}
}

func TestGCKeepsASymbolicLinkToAnEqualFile(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "link")
	f.Write(t, finding, "a finding\n")
	f.Commit(t, "the lead commits the finding")
	if err := os.MkdirAll(filepath.Dir(filepath.Join(dir, finding)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(f.Repo, finding), filepath.Join(dir, finding)); err != nil {
		t.Fatal(err)
	}
	expect(t, e, dir, KeptForLanding, "1 changed path")
	apply(t, e)
	assertStays(t, dir)
	if _, err := os.Lstat(filepath.Join(dir, finding)); err != nil {
		t.Fatalf("the link was deleted: %v", err)
	}
}

func TestGCApplyKeepsAHeldFileThatChangesBeforeRemoval(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := auditWorktree(t, f, "late-edit", "a finding\n")
	j := expect(t, e, dir, Removable, "the base already holds")
	writeIn(t, dir, finding, "edited after the pass\n")
	var out bytes.Buffer
	removed, err := e.removeOne(&out, j.Worktree)
	if err != nil || removed {
		t.Fatalf("removed = %v, err = %v; want it kept\n%s", removed, err, out.String())
	}
	assertStays(t, dir)
	if got := read(t, filepath.Join(dir, finding)); got != "edited after the pass\n" {
		t.Fatalf("the edited file was touched: %q", got)
	}
	if !strings.Contains(out.String(), string(KeptForLanding)) {
		t.Fatalf("the re-check did not say why it kept it:\n%s", out.String())
	}
}

// The re-judge can pass and the file still change before the delete; deleteHeld checks every
// file again and deletes none of them when one fails.
func TestGCDeleteHeldDeletesNothingWhenOneFileChanged(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "two")
	f.Write(t, "docs/incomplete/audit/a.md", "a\n")
	f.Write(t, "docs/incomplete/audit/b.md", "b\n")
	f.Commit(t, "the lead commits two findings")
	writeIn(t, dir, "docs/incomplete/audit/a.md", "a\n")
	writeIn(t, dir, "docs/incomplete/audit/b.md", "b\n")
	j := expect(t, e, dir, Removable, "2 untracked files the base already holds are deleted with it")
	writeIn(t, dir, "docs/incomplete/audit/b.md", "b, edited\n")
	var out bytes.Buffer
	ok, err := e.deleteHeld(&out, j)
	if ok || err != nil {
		t.Fatalf("deleteHeld = %v, %v; want it to refuse\n%s", ok, err, out.String())
	}
	if !strings.Contains(out.String(), "docs/incomplete/audit/b.md changed after it was judged; nothing deleted") {
		t.Fatalf("deleteHeld did not say why:\n%s", out.String())
	}
	for rel, want := range map[string]string{"docs/incomplete/audit/a.md": "a\n", "docs/incomplete/audit/b.md": "b, edited\n"} {
		if got := read(t, filepath.Join(dir, rel)); got != want {
			t.Fatalf("%s = %q, want %q", rel, got, want)
		}
	}
}

func TestGCDryRunLeavesTheHeldFiles(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := auditWorktree(t, f, "dry", "a finding\n")
	var out bytes.Buffer
	if err := e.GC(&out); err != nil {
		t.Fatal(err)
	}
	assertStays(t, dir)
	if got := read(t, filepath.Join(dir, finding)); got != "a finding\n" {
		t.Fatalf("the dry run touched the held file: %q", got)
	}
	if !strings.Contains(out.String(), "1 untracked file the base already holds is deleted with it") || !strings.Contains(out.String(), "1 of 1 removable; dry run") {
		t.Fatalf("dry run output:\n%s", out.String())
	}
	var report bytes.Buffer
	if err := e.GCJSON(&report); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(report.String(), `"held": 1`) {
		t.Fatalf("the --json report has no held count:\n%s", report.String())
	}
	assertStays(t, dir)
}

type recorder struct{ calls []string }

func (r *recorder) control(args ...string) error {
	r.calls = append(r.calls, strings.Join(args, " "))
	return nil
}

func TestScheduleInstallWritesAndLoadsTheAgent(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	r := &recorder{}
	l := Launchd{AgentsDir: filepath.Join(f.Root, "LaunchAgents"), Domain: "gui/501", Control: r.control}
	var out bytes.Buffer
	if err := e.ScheduleInstall("/opt/wk/bin/wk", l, &out); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(l.AgentsDir, "com.github.zach-short.wk.gc.demo.plist")
	body := read(t, path)
	for _, want := range []string{
		"<array>\n\t\t<string>/bin/sh</string>\n\t\t<string>-c</string>\n\t\t<string>exec &#34;$0&#34; &#34;$@&#34;</string>\n\t\t<string>/opt/wk/bin/wk</string>",
		"<string>" + f.Repo + "</string>",
		"<key>RunAtLoad</key>\n\t<false/>",
		"<key>ProcessType</key>\n\t<string>Background</string>",
		"<string>gc</string>\n\t\t<string>--apply</string>",
		"<key>StartInterval</key>\n\t<integer>3600</integer>",
		"<string>" + filepath.Join(f.Home, "demo/gc.log") + "</string>",
		"<key>WK_HOME</key>\n\t\t<string>" + f.Home + "</string>",
		"<key>WorkingDirectory</key>\n\t<string>/</string>",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("plist lacks %q:\n%s", want, body)
		}
	}
	if lint, err := exec.LookPath("plutil"); err == nil {
		if out, err := exec.Command(lint, "-lint", path).CombinedOutput(); err != nil {
			t.Fatalf("plutil -lint: %v\n%s", err, out)
		}
	}
	wantCalls := []string{"bootout gui/501/com.github.zach-short.wk.gc.demo", "bootstrap gui/501 " + path}
	if strings.Join(r.calls, "\n") != strings.Join(wantCalls, "\n") {
		t.Fatalf("launchctl calls = %q, want %q", r.calls, wantCalls)
	}

	r.calls = nil
	if err := e.ScheduleUninstall(l, &out); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("uninstall left the plist")
	}
	if len(r.calls) != 1 || r.calls[0] != wantCalls[0] {
		t.Fatalf("uninstall launchctl calls = %q", r.calls)
	}
}

func TestScheduleInstallRefusesATransientBinary(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	l := Launchd{AgentsDir: filepath.Join(f.Root, "LaunchAgents"), Domain: "gui/501", Control: (&recorder{}).control}
	for exe, want := range map[string]string{
		"wk": "relative",
		"/var/folders/x/T/go-build123/b001/exe/wk": "go run build",
	} {
		if err := e.ScheduleInstall(exe, l, io.Discard); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("install %s: err = %v, want %q", exe, err, want)
		}
	}
	if _, err := os.Stat(l.AgentsDir); !os.IsNotExist(err) {
		t.Fatal("a refused install wrote the agents directory")
	}
}

// programArguments reads the ProgramArguments array of a plist the way launchd does: as XML,
// one string per element, with the entities decoded.
func programArguments(t *testing.T, body string) []string {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(body))
	var args []string
	var key, elem string
	inArgs := false
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			elem = tk.Name.Local
			if elem == "array" && key == "ProgramArguments" {
				inArgs = true
			}
			if elem == "string" && inArgs {
				args = append(args, "")
			}
		case xml.EndElement:
			if tk.Name.Local == "array" {
				inArgs = false
			}
			elem = ""
		case xml.CharData:
			switch {
			case elem == "key":
				key = string(tk)
			case elem == "string" && inArgs:
				args[len(args)-1] += string(tk)
			}
		}
	}
	return args
}

// The agent starts wk through /bin/sh so that a brew upgrade, which changes wk's ad-hoc
// signature, does not break launchd's inferred code requirement. The wk path goes in as its own
// string, so a space or a quote in it reaches exec whole. The test runs the parsed arguments
// through the real /bin/sh with a stand-in wk that prints its argv; it loads no agent.
func TestScheduleAgentStartsWkThroughShell(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	dir := filepath.Join(f.Root, `my "wk" bin`)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "wk")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nprintf '%s\\n' \"$0\" \"$@\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	l := Launchd{AgentsDir: filepath.Join(f.Root, "LaunchAgents"), Domain: "gui/501", Control: (&recorder{}).control}
	if err := e.ScheduleInstall(exe, l, io.Discard); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(l.AgentsDir, "com.github.zach-short.wk.gc.demo.plist")
	args := programArguments(t, read(t, path))
	want := []string{"/bin/sh", "-c", `exec "$0" "$@"`, exe, "--repo", f.Repo, "gc", "--apply"}
	if strings.Join(args, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("ProgramArguments = %q, want %q", args, want)
	}
	if lint, err := exec.LookPath("plutil"); err == nil {
		if out, err := exec.Command(lint, "-lint", path).CombinedOutput(); err != nil {
			t.Fatalf("plutil -lint: %v\n%s", err, out)
		}
	} else {
		t.Log("plutil not on PATH; lint skipped")
	}
	out, err := exec.Command(args[0], args[1:]...).Output()
	if err != nil {
		t.Fatalf("sh: %v", err)
	}
	if got, wantOut := string(out), strings.Join(want[3:], "\n")+"\n"; got != wantOut {
		t.Fatalf("wk got argv %q, want %q", got, wantOut)
	}
}

func TestScheduleCheckReportsOnlyAFailedAgent(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	const label = "com.github.zach-short.wk.gc.demo"
	printed := func(exit string) string {
		return "gui/501/" + label + " = {\n\tactive count = 0\n\tpath = /x.plist\n\tstate = not running\n\n\tprogram = /bin/sh\n\truns = 3\n\tlast exit code = " + exit + "\n\n\tproperties = inferred program | managed LWCR | has LWCR\n}\n"
	}
	for name, tc := range map[string]struct {
		text string
		err  error
		want string
	}{
		"exit 78":      {printed("78: EX_CONFIG"), nil, "gc agent " + label + ": last exit code 78: EX_CONFIG, so scheduled gc is not running; fix: wk gc schedule install\n"},
		"exit 1":       {printed("1"), nil, "gc agent " + label + ": the last scheduled gc failed with exit code 1; see " + e.GCLogPath() + "\n"},
		"exit 0":       {printed("0"), nil, ""},
		"never exited": {printed("(never exited)"), nil, ""},
		"missing":      {"Bad request.\nCould not find service \"" + label + "\" in domain for user gui: 501\n", errors.New("exit status 113"), ""},
	} {
		t.Run(name, func(t *testing.T) {
			var targets []string
			l := Launchd{Domain: "gui/501", Control: func(...string) error { t.Fatal("the check changed launchd"); return nil },
				Print: func(target string) (string, error) { targets = append(targets, target); return tc.text, tc.err }}
			var out bytes.Buffer
			e.ScheduleCheck(l, &out)
			if out.String() != tc.want {
				t.Fatalf("printed %q, want %q", out.String(), tc.want)
			}
			if len(targets) != 1 || targets[0] != "gui/501/"+label {
				t.Fatalf("launchctl print targets = %q", targets)
			}
		})
	}
	var out bytes.Buffer
	e.ScheduleCheck(Launchd{Domain: "gui/501"}, &out)
	if out.Len() != 0 {
		t.Fatalf("no Print seam printed %q", out.String())
	}
}
