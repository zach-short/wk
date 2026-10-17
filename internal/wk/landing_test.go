package wk

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/changeset"
	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/landing"
	"github.com/zach-short/wk/internal/testrepo"
)

// landingFixture is a repository with landing and leases on, a bare origin beside it, and two
// worktrees a and b on branches of their own. extra is appended to the config.
func landingFixture(t *testing.T, log io.Writer, extra string) (f testrepo.Fixture, e *Env, a, b string) {
	t.Helper()
	f = testrepo.New(t)
	cfg := filepath.Join(f.Home, "demo.toml")
	body := read(t, cfg) + "\n[leases]\nenabled = true\n\n[landing]\nenabled = true\n" + extra
	if err := os.WriteFile(cfg, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	f.Write(t, "PASSOFF.md", leaseBoard)
	origin := filepath.Join(f.Root, "origin.git")
	testrepo.Git(t, f.Root, "init", "--quiet", "--bare", origin)
	f.Git(t, "remote", "add", "origin", origin)
	a, b = filepath.Join(f.Root, "a"), filepath.Join(f.Root, "b")
	f.Git(t, "worktree", "add", "--quiet", "-b", "a", a)
	f.Git(t, "worktree", "add", "--quiet", "-b", "b", b)
	return f, open(t, f, log), a, b
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	return strings.TrimSpace(testrepo.Git(t, dir, args...))
}

func submit(t *testing.T, e *Env, dir, message, review string) string {
	t.Helper()
	var out bytes.Buffer
	if err := e.Submit(dir, message, "", review, &out); err != nil {
		t.Fatalf("submit in %s: %v", dir, err)
	}
	id := strings.TrimSuffix(out.String(), "\n")
	if len(id) != 12 || strings.Contains(id, "\n") {
		t.Fatalf("stdout = %q; want the id alone", out.String())
	}
	return id
}

func loop(t *testing.T, e *Env) string {
	t.Helper()
	var out bytes.Buffer
	if err := e.LandLoop(&out); err != nil {
		t.Fatalf("land --loop: %v\n%s", err, out.String())
	}
	return out.String()
}

func record(t *testing.T, e *Env, id string) changeset.Record {
	t.Helper()
	records, err := e.queue().Load()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range records {
		if r.ID == id {
			return r
		}
	}
	t.Fatalf("no record %s in %v", id, records)
	return changeset.Record{}
}

// The proof of PLAN-P2-P4.md section 3: two overlapping changesets, the first lands, the
// second is refused with the path named, wk sync carries the conflict into the tree, and a
// resubmit after the fix lands. Every landing commit's tree is the predicted tree.
func TestSubmitLandsInOrderAndRefusesTheOverlap(t *testing.T) {
	f, e, a, b := landingFixture(t, io.Discard, "")
	mustClaim(t, e, a, "1")
	writeIn(t, a, "web/page.ts", "export const page = 'a';\n")
	writeIn(t, a, "notes.txt", "untracked\n")
	main := gitOut(t, f.Repo, "rev-parse", "refs/heads/main")
	tree, err := changeset.Tree(a)
	if err != nil {
		t.Fatal(err)
	}
	predicted, _, err := landing.Predict(f.Repo, main, main, tree)
	if err != nil {
		t.Fatal(err)
	}
	id := submit(t, e, a, "Add The Page", "")
	loop(t, e)
	r := record(t, e, id)
	if r.State != changeset.Landed || r.Message != "row 1: add the page" || r.Row != "1" {
		t.Fatalf("record = %+v", r)
	}
	if got := gitOut(t, f.Repo, "rev-parse", r.Commit+"^{tree}"); got != predicted {
		t.Fatalf("landed tree %s, predicted %s", got, predicted)
	}
	if tip := gitOut(t, f.Repo, "rev-parse", "refs/heads/wk/landing"); tip != r.Commit {
		t.Fatalf("landing ref at %s, commit %s", tip, r.Commit)
	}
	if got := gitOut(t, f.Repo, "rev-parse", "refs/heads/main"); got != main {
		t.Fatal("main moved; only wk land --apply moves it (PD-6)")
	}
	if leases, _ := loadLeases(e.State); leases.Workspaces[a].Row != "" {
		t.Fatalf("leases of a kept after landing: %+v", leases.Workspaces[a])
	}

	writeIn(t, b, "web/page.ts", "export const page = 'b';\n")
	second := submit(t, e, b, "Change the page", "")
	loop(t, e)
	r = record(t, e, second)
	if r.State != changeset.Refused || len(r.Paths) != 1 || r.Paths[0] != "web/page.ts" || !strings.Contains(r.Reason, "wk sync") {
		t.Fatalf("second record = %+v", r)
	}
	var out bytes.Buffer
	err = e.Sync(b, &out)
	if err == nil || !strings.Contains(err.Error(), "1 path conflicts") || !strings.Contains(out.String(), "conflict: web/page.ts") {
		t.Fatalf("sync: %v\n%s", err, out.String())
	}
	if !strings.Contains(read(t, filepath.Join(b, "web/page.ts")), "<<<<<<<") {
		t.Fatal("no markers in the synced tree")
	}
	if head := gitOut(t, b, "rev-parse", "HEAD"); head != record(t, e, id).Commit {
		t.Fatalf("b is at %s, not the landing tip", head)
	}
	writeIn(t, b, "web/page.ts", "export const page = 'ab';\n")
	third := submit(t, e, b, "Change the page", "")
	loop(t, e)
	r = record(t, e, third)
	if r.State != changeset.Landed || r.Message != "change the page" {
		t.Fatalf("third record = %+v", r)
	}
	if got := gitOut(t, f.Repo, "show", r.Commit+":web/page.ts"); got != "export const page = 'ab';" {
		t.Fatalf("landed page = %q", got)
	}
	if got := gitOut(t, f.Repo, "show", r.Commit+":notes.txt"); got != "untracked" {
		t.Fatalf("the untracked file of a is missing: %q", got)
	}
}

// A landing line whose tree is the tip's tree is the crash between the swap and the landed line
// (F18): the next pass lands it without a commit. A submit whose tree is the tip's tree lands
// the same way, so an empty landing commit is never written.
func TestLoopLandsWhatIsAlreadyOnTheTipWithoutACommit(t *testing.T) {
	f, e, a, _ := landingFixture(t, io.Discard, "")
	writeIn(t, a, "web/page.ts", "export const page = 'a';\n")
	first := submit(t, e, a, "first", "")
	loop(t, e)
	tip := record(t, e, first).Commit
	tipTree := gitOut(t, f.Repo, "rev-parse", tip+"^{tree}")
	journal := changeset.Record{ID: "0123456789ab", State: changeset.Landing, Tree: tipTree, Base: tip, Message: "crashed", Workspace: a, At: e.clock()}
	if err := e.queue().Append(journal); err != nil {
		t.Fatal(err)
	}
	loop(t, e)
	if r := record(t, e, journal.ID); r.State != changeset.Landed || r.Commit != tip {
		t.Fatalf("journal record = %+v; want landed at %s", r, tip)
	}
	empty := submit(t, e, a, "nothing new", "")
	loop(t, e)
	if r := record(t, e, empty); r.State != changeset.Landed || r.Commit != tip {
		t.Fatalf("empty record = %+v; want landed at %s", r, tip)
	}
	if got := gitOut(t, f.Repo, "rev-parse", "refs/heads/wk/landing"); got != tip {
		t.Fatalf("landing ref moved to %s", got)
	}
}

func TestRequireReviewRefusesAChangesetWithoutOne(t *testing.T) {
	_, e, a, _ := landingFixture(t, io.Discard, "require_review = true\n")
	writeIn(t, a, "web/page.ts", "export const page = 'a';\n")
	bare := submit(t, e, a, "no review", "")
	loop(t, e)
	if r := record(t, e, bare); r.State != changeset.Refused || !strings.Contains(r.Reason, "--review") {
		t.Fatalf("record = %+v", r)
	}
	reviewed := submit(t, e, a, "reviewed", "audit 3")
	loop(t, e)
	if r := record(t, e, reviewed); r.State != changeset.Landed || r.Review != "audit 3" {
		t.Fatalf("record = %+v", r)
	}
}

func TestSubmitAndLoopRefusals(t *testing.T) {
	f, e, a, _ := landingFixture(t, io.Discard, "")
	if err := e.Submit(f.Repo, "x", "", "", io.Discard); err == nil || !strings.Contains(err.Error(), "main checkout") {
		t.Errorf("submit in the primary: %v", err)
	}
	if err := e.Submit(a, "", "", "", io.Discard); err == nil || !strings.Contains(err.Error(), "-m") {
		t.Errorf("submit without a message: %v", err)
	}
	if err := e.Sync(f.Repo, io.Discard); err == nil || !strings.Contains(err.Error(), "main checkout") {
		t.Errorf("sync in the primary: %v", err)
	}
	off := testrepo.New(t)
	offEnv := open(t, off, io.Discard)
	ws := filepath.Join(off.Root, "ws")
	off.Git(t, "worktree", "add", "--quiet", "-b", "ws", ws)
	if err := offEnv.Submit(ws, "x", "", "", io.Discard); err == nil || !strings.Contains(err.Error(), "landing.enabled is false") {
		t.Errorf("submit with landing off: %v", err)
	}
	if err := offEnv.LandLoop(io.Discard); err == nil || !strings.Contains(err.Error(), "landing.enabled is false") {
		t.Errorf("loop with landing off: %v", err)
	}
}

// One worker at a time: a loop that finds landing.lock held leaves without touching the queue.
func TestLoopLeavesWhenAnotherWorkerHoldsTheLock(t *testing.T) {
	_, e, a, _ := landingFixture(t, io.Discard, "")
	writeIn(t, a, "web/page.ts", "export const page = 'a';\n")
	id := submit(t, e, a, "held", "")
	lock, ok, err := flock.TryTake(filepath.Join(e.State, "landing.lock"), true)
	if err != nil || !ok {
		t.Fatalf("take the lock: %v %v", ok, err)
	}
	loop(t, e)
	if r := record(t, e, id); r.State != changeset.Submitted {
		t.Fatalf("record = %+v; want it still submitted", r)
	}
	_ = lock.Release()
	loop(t, e)
	if r := record(t, e, id); r.State != changeset.Landed {
		t.Fatalf("record = %+v; want it landed once the lock is free", r)
	}
}

// A branch with commits of its own is folded: the commits become uncommitted work and the
// branch moves to the tip (owner amendment, 2026-10-16).
func TestSyncFoldsABranchWithCommits(t *testing.T) {
	f, e, a, _ := landingFixture(t, io.Discard, "")
	var log bytes.Buffer
	e.log = &syncWriter{w: &log}
	writeIn(t, a, "web/page.ts", "export const page = 'committed';\n")
	testrepo.Git(t, a, "commit", "--quiet", "--no-verify", "-am", "on a")
	writeIn(t, a, "web/extra.ts", "export const extra = 1;\n")
	if err := e.Sync(a, io.Discard); err != nil {
		t.Fatal(err)
	}
	main := gitOut(t, f.Repo, "rev-parse", "refs/heads/main")
	if head := gitOut(t, a, "rev-parse", "HEAD"); head != main {
		t.Fatalf("a is at %s, main at %s", head, main)
	}
	if got := read(t, filepath.Join(a, "web/page.ts")); got != "export const page = 'committed';\n" {
		t.Fatalf("the committed work was lost: %q", got)
	}
	if status := gitOut(t, a, "status", "--porcelain"); !strings.Contains(status, "M web/page.ts") || !strings.Contains(status, "?? web/extra.ts") {
		t.Fatalf("status after the fold:\n%s", status)
	}
	if !strings.Contains(log.String(), "folded 1 commits of a") {
		t.Fatalf("log:\n%s", log.String())
	}
}

func TestLandPrintsThePlanAndApplyMovesAndPushesMain(t *testing.T) {
	f, e, a, _ := landingFixture(t, io.Discard, "")
	writeIn(t, a, "web/page.ts", "export const page = 'a';\n")
	id := submit(t, e, a, "page", "")
	loop(t, e)
	tip := record(t, e, id).Commit
	var out bytes.Buffer
	if err := e.Land(&out); err != nil || !strings.Contains(out.String(), "fast-forward possible: wk land --apply --expect "+tip) {
		t.Fatalf("land: %v\n%s", err, out.String())
	}
	writeIn(t, f.Repo, "web/page.ts", "dirty in the main checkout\n")
	out.Reset()
	if err := e.Land(&out); err != nil || !strings.Contains(out.String(), "blocker: web/page.ts") || strings.Contains(out.String(), "possible") {
		t.Fatalf("land with a blocker: %v\n%s", err, out.String())
	}
	if err := e.LandApply(tip, io.Discard); err == nil || !strings.Contains(err.Error(), "block the fast-forward") {
		t.Fatalf("apply over a blocker: %v", err)
	}
	f.Git(t, "checkout", "--quiet", "--", "web/page.ts")
	if err := e.LandApply(strings.Repeat("0", 40), io.Discard); err == nil || !strings.Contains(err.Error(), "not the expected") {
		t.Fatalf("apply with a stale hash: %v", err)
	}
	out.Reset()
	if err := e.LandApply(tip, &out); err != nil {
		t.Fatalf("apply: %v\n%s", err, out.String())
	}
	for _, dir := range []string{f.Repo, filepath.Join(f.Root, "origin.git")} {
		if got := gitOut(t, dir, "rev-parse", "refs/heads/main"); got != tip {
			t.Fatalf("main in %s is %s, not %s", dir, got, tip)
		}
	}
	if !strings.Contains(out.String(), "pushed main to origin") {
		t.Fatalf("apply output:\n%s", out.String())
	}
	out.Reset()
	if err := e.Land(&out); err != nil || !strings.Contains(out.String(), "nothing to land") {
		t.Fatalf("land after apply: %v\n%s", err, out.String())
	}
}

// When main and the ref diverge the worker stops with the reason and wk status says what to do.
func TestStatusListsTheQueueAndNamesADivergedRef(t *testing.T) {
	f, e, a, _ := landingFixture(t, io.Discard, "")
	writeIn(t, a, "web/page.ts", "export const page = 'a';\n")
	id := submit(t, e, a, "page", "")
	loop(t, e)
	var out bytes.Buffer
	if err := e.Status(&out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"landing.enabled = true", id + "  landed", "1 commits ahead of main", "commit: " + record(t, e, id).Commit} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("status lacks %q:\n%s", want, out.String())
		}
	}
	f.Git(t, "commit", "--quiet", "--allow-empty", "-m", "by hand on main")
	writeIn(t, a, "web/page.ts", "export const page = 'again';\n")
	submit(t, e, a, "again", "")
	if err := e.LandLoop(io.Discard); err == nil || !strings.Contains(err.Error(), "diverged") {
		t.Fatalf("loop on a diverged ref: %v", err)
	}
	out.Reset()
	if err := e.QueueStatus(&out); err != nil || !strings.Contains(out.String(), "diverged from main") || !strings.Contains(out.String(), "git update-ref refs/heads/wk/landing") {
		t.Fatalf("status: %v\n%s", err, out.String())
	}
}
