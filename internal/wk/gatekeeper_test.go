package wk

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/changeset"
	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gate"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/slot"
	"github.com/zach-short/wk/internal/testrepo"
)

// The three gates of the P4 proof, as shell over the trial: count logs each gate run to
// $WK_RUNS; types fails when web/use.ts calls page and web/page.ts no longer exports it, naming
// use.ts relative to its dir, as tsc does; bad fails on any file under web holding BAD, naming
// each; db needs a variable no test sets, so it always skips.
const testGates = `
[gates]
batch_max = %MAX%
batch_idle_seconds = 0
min_free_mb = 0
fresh_caches = ["WK_TEST_CACHE"]

[[gates.run]]
name = "count"
run = ["sh", "-c", 'test -d "$WK_TEST_CACHE" && echo run >> "$WK_RUNS"']

[[gates.run]]
name = "types"
dir = "web"
run = ["sh", "-c", 'if [ -f use.ts ] && ! grep -q "const page" page.ts; then echo "use.ts:1:8 - error TS2305: page is not exported"; exit 1; fi']

[[gates.run]]
name = "db"
run = ["sh", "-c", "exit 1"]
needs_env = ["WK_TEST_NEVER_SET"]

[[gates.run]]
name = "bad"
dir = "web"
run = ["sh", "-c", '! grep -rl BAD .']
`

// gateFixture is landingFixture with the four gates, batch_max max, ff_main on, and use.ts on
// main, plus a workspace per name. runs reads how many gate runs there were.
func gateFixture(t *testing.T, max string, names ...string) (f testrepo.Fixture, e *Env, ws map[string]string, runs func() int) {
	t.Helper()
	f, _, _, _ = landingFixture(t, io.Discard, "ff_main = true\n"+strings.ReplaceAll(testGates, "%MAX%", max))
	f.Write(t, "web/use.ts", "import { page } from './page';\n")
	f.Commit(t, "use the page")
	runsLog := filepath.Join(f.Root, "runs.log")
	e = open(t, f, io.Discard)
	e.Cfg.Gates.Env["WK_RUNS"] = runsLog
	ws = map[string]string{}
	for _, n := range names {
		ws[n] = filepath.Join(f.Root, "ws-"+n)
		f.Git(t, "worktree", "add", "--quiet", "-b", "ws-"+n, ws[n])
	}
	return f, e, ws, func() int {
		data, _ := os.ReadFile(runsLog)
		return strings.Count(string(data), "run\n")
	}
}

func mustState(t *testing.T, e *Env, id, state string) changeset.Record {
	t.Helper()
	r := record(t, e, id)
	if r.State != state {
		t.Fatalf("%s is %s (%s); want %s", id, r.State, r.Reason, state)
	}
	return r
}

func mainMovedTo(t *testing.T, f testrepo.Fixture, tip string) {
	t.Helper()
	if got := gitOut(t, f.Repo, "rev-parse", "refs/heads/main"); got != tip {
		t.Fatalf("main at %s, landing tip %s", got, tip)
	}
	if got := gitOut(t, filepath.Join(f.Root, "origin.git"), "rev-parse", "refs/heads/main"); got != tip {
		t.Fatalf("origin's main at %s, landing tip %s", got, tip)
	}
}

// Two changesets form one batch, the gates run once over their merged tree in the trial, both
// land in order, and main is fast-forwarded and pushed. The skipped gate is in the record, never
// a pass. The trial is detached, marked, and gc leaves it alone.
func TestGateLandsAGreenBatchOnceAndMovesMain(t *testing.T) {
	f, e, ws, runs := gateFixture(t, "2", "a", "b")
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	writeIn(t, ws["b"], "packages/core/b.ts", "export const b = 1;\n")
	first, second := submit(t, e, ws["a"], "add a", ""), submit(t, e, ws["b"], "add b", "")
	loop(t, e)
	if n := runs(); n != 1 {
		t.Fatalf("%d gate runs; want 1 for the batch", n)
	}
	r1, r2 := mustState(t, e, first, changeset.Landed), mustState(t, e, second, changeset.Landed)
	if gitOut(t, f.Repo, "rev-parse", r2.Commit+"^") != r1.Commit || !strings.Contains(r2.Reason, "db skip (WK_TEST_NEVER_SET is unset)") {
		t.Fatalf("landed out of order or without the skip: %+v %+v", r1, r2)
	}
	if got := gitOut(t, f.Repo, "rev-parse", r2.Commit+"^{tree}"); got != r2.Predicted {
		t.Fatalf("landed tree %s, gated tree %s", got, r2.Predicted)
	}
	mainMovedTo(t, f, r2.Commit)
	trial := e.TrialDir()
	if _, err := gitx.Run(trial, "symbolic-ref", "-q", "HEAD"); err == nil {
		t.Fatal("the trial is on a branch; it is detached")
	}
	expect(t, e, trial, Skipped, "the wk gate trial")
}

// The proof's three changesets: the middle one breaks the bad gate, which names its file
// relative to web; Blame names it, the batch without it is gated green, and the other two land.
// One run plus one re-gate. The trial then holds the re-gated tree, without the bad file.
func TestGateFailsTheBlamedChangesetAfterAGreenRegate(t *testing.T) {
	f, e, ws, runs := gateFixture(t, "3", "a", "b", "c")
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	writeIn(t, ws["b"], "web/b.ts", "BAD\n")
	writeIn(t, ws["c"], "packages/core/c.ts", "export const c = 1;\n")
	ids := []string{submit(t, e, ws["a"], "a", ""), submit(t, e, ws["b"], "b", ""), submit(t, e, ws["c"], "c", "")}
	loop(t, e)
	if n := runs(); n != 2 {
		t.Fatalf("%d gate runs; want the batch and one re-gate", n)
	}
	failed := mustState(t, e, ids[1], changeset.Failed)
	if failed.Gate != "bad" || !slices.Equal(failed.Paths, []string{"web/b.ts"}) || !strings.Contains(failed.Reason, "exit status 1") {
		t.Fatalf("failed record = %+v", failed)
	}
	mustState(t, e, ids[0], changeset.Landed)
	tip := mustState(t, e, ids[2], changeset.Landed).Commit
	mainMovedTo(t, f, tip)
	if exists(filepath.Join(e.TrialDir(), "web/b.ts")) {
		t.Fatal("the trial kept web/b.ts from the red batch; read-tree --reset -u removes it")
	}
	var out bytes.Buffer
	if err := e.QueueStatus(&out); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"failed", "gate: bad", "named: web/b.ts", "last gate run: green"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("status lacks %q:\n%s", want, out.String())
		}
	}
}

// Row 12's probe B11: a removes page, and the type gate names use.ts, which b changed. Blame
// names b, the batch without b is still red, so the batch is bisected: a alone is red and
// failed, b alone is green and lands. b is never failed on the guess.
func TestGateBisectsWhenTheBlamedChangesetWasInnocent(t *testing.T) {
	f, e, ws, runs := gateFixture(t, "2", "a", "b")
	writeIn(t, ws["a"], "web/page.ts", "export const pg = 1;\n")
	writeIn(t, ws["b"], "web/use.ts", "import { page } from './page';\nconsole.log(page);\n")
	a, b := submit(t, e, ws["a"], "rename page", ""), submit(t, e, ws["b"], "log the page", "")
	loop(t, e)
	failed := mustState(t, e, a, changeset.Failed)
	if failed.Gate != "types" || !slices.Equal(failed.Paths, []string{"web/use.ts"}) {
		t.Fatalf("failed record = %+v", failed)
	}
	mainMovedTo(t, f, mustState(t, e, b, changeset.Landed).Commit)
	if n := runs(); n != 4 {
		t.Fatalf("%d gate runs; want the batch, the re-gate without b, then a and b alone", n)
	}
}

// A changeset that conflicts only with an earlier member of its batch waits for the next batch,
// because that member could still fail; once the member lands, it conflicts with the tip and
// is refused as P3 refuses it.
func TestGateDefersAConflictWithinTheBatch(t *testing.T) {
	_, e, ws, runs := gateFixture(t, "2", "a", "b")
	writeIn(t, ws["a"], "web/page.ts", "export const page = 'a';\n")
	writeIn(t, ws["b"], "web/page.ts", "export const page = 'b';\n")
	a, b := submit(t, e, ws["a"], "a", ""), submit(t, e, ws["b"], "b", "")
	out := loop(t, e)
	mustState(t, e, a, changeset.Landed)
	r := mustState(t, e, b, changeset.Refused)
	if !slices.Equal(r.Paths, []string{"web/page.ts"}) || !strings.Contains(out, "deferred "+b) || runs() != 1 {
		t.Fatalf("refused = %+v, runs %d, log:\n%s", r, runs(), out)
	}
}

// A run in which every gate skipped is not green: nothing lands, the worker stops with the
// reason, and wk status says so.
func TestGateAllSkippedLandsNothing(t *testing.T) {
	f, e, ws, _ := gateFixture(t, "1", "a")
	e.Cfg.Gates.Run = e.Cfg.Gates.Run[2:3]
	main := gitOut(t, f.Repo, "rev-parse", "refs/heads/main")
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	id := submit(t, e, ws["a"], "a", "")
	err := e.LandLoop(io.Discard)
	if err == nil || !strings.Contains(err.Error(), "every gate skipped") {
		t.Fatalf("loop = %v; want the all-skip refusal", err)
	}
	mustState(t, e, id, changeset.Submitted)
	if got := gitOut(t, f.Repo, "rev-parse", "refs/heads/main"); got != main {
		t.Fatal("main moved on an all-skip run")
	}
	var out bytes.Buffer
	if err := e.QueueStatus(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "every gate skipped; nothing lands") {
		t.Fatalf("status:\n%s", out.String())
	}
}

// PD-11: with no [[gates.run]], the worker lands onto the ref as P3 does and never moves main,
// even with ff_main on, and wk status says why.
func TestNoGatesLandsOnTheRefAndNeverMovesMain(t *testing.T) {
	f, e, ws, _ := gateFixture(t, "1", "a")
	e.Cfg.Gates.Run = nil
	main := gitOut(t, f.Repo, "rev-parse", "refs/heads/main")
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	id := submit(t, e, ws["a"], "a", "")
	loop(t, e)
	mustState(t, e, id, changeset.Landed)
	if got := gitOut(t, f.Repo, "rev-parse", "refs/heads/main"); got != main {
		t.Fatal("main moved with no gates (PD-11)")
	}
	if exists(e.TrialDir()) {
		t.Fatal("a trial was made with no gates")
	}
	var out bytes.Buffer
	if err := e.QueueStatus(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "has no [[gates.run]]") {
		t.Fatalf("status:\n%s", out.String())
	}
}

// Row 12's finding: a gated line a stopped worker left is pending, so the next worker gates it
// again before it lands; a landing line whose prediction is the tip's tree is landed without a
// gate run or a commit.
func TestGatedRecordIsGatedAgainAfterAStop(t *testing.T) {
	f, e, ws, runs := gateFixture(t, "1", "a")
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	id := submit(t, e, ws["a"], "a", "")
	r := record(t, e, id)
	r.State, r.Predicted = changeset.Gated, "0000000000000000000000000000000000000000"
	if err := e.queue().Append(r); err != nil {
		t.Fatal(err)
	}
	stale := r
	stale.ID, stale.State, stale.Predicted = "stalelanding", changeset.Landing, gitOut(t, f.Repo, "rev-parse", "main^{tree}")
	if err := e.queue().Append(stale); err != nil {
		t.Fatal(err)
	}
	loop(t, e)
	if n := runs(); n != 1 {
		t.Fatalf("%d gate runs; want the gated record gated once more", n)
	}
	mainMovedTo(t, f, mustState(t, e, id, changeset.Landed).Commit)
	if got := mustState(t, e, "stalelanding", changeset.Landed); got.Commit == "" {
		t.Fatalf("stale landing = %+v", got)
	}
}

// A batch that is not full waits for gates.batch_idle_seconds of quiet before it forms.
func TestGateWaitsForQuietBeforeABatchForms(t *testing.T) {
	_, e, ws, _ := gateFixture(t, "2", "a")
	e.Cfg.Gates.BatchIdleSeconds = 10
	now := time.Now()
	e.clock = func() time.Time { return now }
	var slept []time.Duration
	e.sleep = func(d time.Duration) { slept = append(slept, d); now = now.Add(d) }
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	id := submit(t, e, ws["a"], "a", "")
	loop(t, e)
	mustState(t, e, id, changeset.Landed)
	if len(slept) != 10 || slept[0] != time.Second {
		t.Fatalf("slept %d times (%v first); want 10 one-second waits", len(slept), slept[:min(1, len(slept))])
	}
}

// A gate that cannot start says nothing about the tree: it is blamed on no changeset, the pass
// stops with the error, and wk status shows it.
func TestGateThatCannotStartBlamesNothing(t *testing.T) {
	_, e, ws, _ := gateFixture(t, "1", "a")
	e.Cfg.Gates.Run[1].Run = []string{"/nonexistent/wk-gate"}
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	id := submit(t, e, ws["a"], "a", "")
	if err := e.LandLoop(io.Discard); err == nil || !strings.Contains(err.Error(), "gate types") {
		t.Fatalf("loop = %v; want the start error", err)
	}
	mustState(t, e, id, changeset.Submitted)
	var out bytes.Buffer
	if err := e.QueueStatus(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "last gate run: error") || !strings.Contains(out.String(), "/nonexistent/wk-gate") {
		t.Fatalf("status:\n%s", out.String())
	}
}

// trial.lock is held from the read of the tree to the last gate.
func TestTrialLockIsHeldWhileGating(t *testing.T) {
	f, e, _, _ := gateFixture(t, "1")
	tree := gitOut(t, f.Repo, "rev-parse", "main^{tree}")
	err := e.withTrial(tree, func(dir string) error {
		_, ok, err := flock.TryTake(filepath.Join(e.State, "trial.lock"), true)
		if err != nil || ok {
			t.Errorf("trial.lock free while gating (ok %v, err %v)", ok, err)
		}
		if !exists(filepath.Join(dir, "web/use.ts")) {
			t.Error("the trial does not hold the tree")
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A directory at gates.trial_dir without the marker is never taken over.
func TestTrialRefusesAnUnmarkedDirectory(t *testing.T) {
	f, e, _, _ := gateFixture(t, "1")
	f.Git(t, "worktree", "add", "--quiet", "--detach", e.TrialDir(), "main")
	err := e.withTrial(gitOut(t, f.Repo, "rev-parse", "main^{tree}"), func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "without the wk-trial marker") {
		t.Fatalf("withTrial = %v; want the refusal", err)
	}
}

// PD-10: a gate whose command the slots file recognises runs through wk run --slot; one it does
// not, or a nested shell it leaves alone, runs as it is.
func TestSlottedRoutesOnlyAMatchedGate(t *testing.T) {
	sf := slot.SlotsFile{Match: []slot.MatchEntry{
		{Spelling: "go test", Class: "check"},
		{Spelling: "bun run build", Class: "heavy", TrimAfter: []string{"web/.next/cache"}},
	}}
	for _, tc := range []struct {
		run  []string
		want []string
	}{
		{[]string{"go", "test", "-race", "./..."}, []string{"/bin/sh", "-c", "/opt/wk run --slot check -- go test -race ./..."}},
		{[]string{"bun", "run", "build"}, []string{"/bin/sh", "-c", "/opt/wk run --slot heavy --trim web/.next/cache -- bun run build"}},
		{[]string{"go", "test", "it's here"}, []string{"/bin/sh", "-c", `/opt/wk run --slot check -- go test "it's here"`}},
		{[]string{"go", "vet", "./..."}, []string{"go", "vet", "./..."}},
		{[]string{"sh", "-c", "go test ./..."}, []string{"sh", "-c", "go test ./..."}},
	} {
		got, ok := slotted(gate.Gate{Name: "g", Run: tc.run}, "/opt/wk", sf)
		if !slices.Equal(got.Run, tc.want) || ok != (got.Run[0] == "/bin/sh") {
			t.Errorf("slotted(%q) = %q, %v; want %q", tc.run, got.Run, ok, tc.want)
		}
	}
}

// slotFixture is gateFixture with a slots file whose one spelling is word, the types gate
// replaced by run, and WKExe a script that does what exe says, standing in for wk run.
func slotFixture(t *testing.T, word, exe string, run ...string) (*Env, string) {
	t.Helper()
	_, e, ws, _ := gateFixture(t, "1", "a")
	e.WKExe = filepath.Join(t.TempDir(), "wk")
	if err := os.WriteFile(e.WKExe, []byte("#!/bin/sh\n"+exe+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	slots := "[[slots.match]]\nspelling = \"" + word + "\"\nclass = \"check\"\n"
	if err := os.WriteFile(filepath.Join(e.home, e.Repo.Name+".slots.toml"), []byte(slots), 0o644); err != nil {
		t.Fatal(err)
	}
	e.Cfg.Gates.Run[1].Run = run
	if !e.gateList()[1].slotted {
		t.Fatalf("the gate %q is not slotted", run)
	}
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	return e, submit(t, e, ws["a"], "a", "")
}

// Fix round 1, B1: a slotted gate whose command is not on the gate's PATH is a start error, as
// it is unslotted, and never a fail blamed on the changeset. wk run is never reached.
func TestSlottedGateWithAMissingCommandBlamesNothing(t *testing.T) {
	e, id := slotFixture(t, "wk-test-missing-tool", "echo reached > \"$0.reached\"; exit 1", "wk-test-missing-tool", "--noEmit")
	err := e.LandLoop(io.Discard)
	if err == nil || !strings.Contains(err.Error(), "wk-test-missing-tool is not on the gate's PATH") {
		t.Fatalf("loop = %v; want the start error", err)
	}
	mustState(t, e, id, changeset.Submitted)
	if exists(e.WKExe + ".reached") {
		t.Fatal("wk run was started for a command that is not there")
	}
}

// Fix round 1, B1, and fix round 2: wk run exits 127 or 126 for a command it cannot find or
// run, and 75 when slots.max_wait_seconds ran out, each after a line of its own; through the
// slot each is a start error, and the changeset stays submitted. Exit 1 is still a fail.
func TestSlottedGateExit127IsAStartError(t *testing.T) {
	for _, tc := range []struct {
		exe, state string
	}{
		{"echo 'wk run: true: command not found' >&2; exit 127", changeset.Submitted},
		{"echo 'wk run: true: permission denied' >&2; exit 126", changeset.Submitted},
		{"echo 'wk run: waiting for a check place: 1 of 1 held; 0 ahead of this run' >&2; echo 'wk run: gave up after 1s waiting for a check place (slots.max_wait_seconds)' >&2; exit 75", changeset.Submitted},
		// The review's N1: wk run can still say something after it gives up.
		{"echo 'wk run: gave up after 1s waiting for a check place (slots.max_wait_seconds)' >&2; echo 'wk run: cannot write /x/runs.log: permission denied' >&2; exit 75", changeset.Submitted},
		{"exit 1", changeset.Failed},
	} {
		e, id := slotFixture(t, "true", tc.exe, "true")
		err := e.LandLoop(io.Discard)
		if (tc.state == changeset.Submitted) != (err != nil && strings.Contains(err.Error(), "could not run")) {
			t.Errorf("%s: loop = %v", tc.exe, err)
		}
		mustState(t, e, id, tc.state)
	}
}

// Row 5's N-a: a slotted command that exits 127 (or 126, or 75) itself writes output of its own,
// as bun 1.2.9 does for a package script naming a missing command, so the gate is a fail and the
// changeset is blamed, and the queue keeps moving. So is a silent exit with no wk run line, and
// a wk run line followed by the command's own.
func TestSlottedCommandExit127IsAFail(t *testing.T) {
	for _, exe := range []string{
		`echo 'error: script "lint" exited with code 127' >&2; exit 127`,
		"echo '/bin/sh: eslint: command not found' >&2; exit 127",
		"exit 127",
		"echo 'wk run: took a check place after 1s' >&2; echo 'lint: not found' >&2; exit 127",
		"echo 'wk run: running unslotted: no machine file at /x' >&2; exit 126",
		"echo 'wk run: took a check place after 1s' >&2; exit 75",
	} {
		e, id := slotFixture(t, "true", exe, "true")
		if err := e.LandLoop(io.Discard); err != nil {
			t.Errorf("%s: loop = %v; want the changeset failed and the loop on", exe, err)
		}
		mustState(t, e, id, changeset.Failed)
	}
}

// Row 15's call on the shell in front of wk run: when the wk binary itself is gone, /bin/sh
// exits 127 with a line naming that binary. The gate never ran, so it is a start error.
func TestSlottedGateWithoutTheWkBinaryIsAStartError(t *testing.T) {
	e, id := slotFixture(t, "true", "exit 0", "true")
	if err := os.Remove(e.WKExe); err != nil {
		t.Fatal(err)
	}
	err := e.LandLoop(io.Discard)
	if err == nil || !strings.Contains(err.Error(), "could not run") || !strings.Contains(err.Error(), e.WKExe) {
		t.Fatalf("loop = %v; want the start error naming %s", err, e.WKExe)
	}
	mustState(t, e, id, changeset.Submitted)
}

// The landing log names a start error as one: the gate's output and the error, never the
// `gate types: fail, exit status 75` line gate.Run would have echoed first.
func TestSlottedStartErrorIsNotLoggedAsAFail(t *testing.T) {
	e, id := slotFixture(t, "true", "echo 'wk run: gave up after 1s waiting for a check place (slots.max_wait_seconds)' >&2; exit 75", "true")
	var out bytes.Buffer
	if err := e.LandLoop(&out); err == nil {
		t.Fatal("loop = nil; want the start error")
	}
	mustState(t, e, id, changeset.Submitted)
	log := out.String()
	if strings.Contains(log, "gate types: fail") || !strings.Contains(log, "gate types: exit status 75 through wk run --slot, which means its command could not run") || !strings.Contains(log, "  wk run: gave up after 1s") {
		t.Fatalf("log:\n%s", log)
	}
	if !strings.Contains(log, "gate count: pass") {
		t.Fatalf("the gate before the start error lost its line:\n%s", log)
	}
}

// A slotted gate that waited for a place and then ran out of time lost its time to another
// workspace's run, not to the tree: a start error, whether it was still waiting or had taken the
// place (the review's B2). One that never waited and runs out of time is a fail like any timeout.
func TestSlottedGateTimeoutWhileWaitingIsAStartError(t *testing.T) {
	for _, tc := range []struct{ exe, state string }{
		{"echo 'wk run: waiting for a check place: 1 of 1 held; 0 ahead of this run' >&2; sleep 60", changeset.Submitted},
		{"echo 'wk run: waiting for a check place: 1 of 1 held; 0 ahead of this run' >&2; echo 'wk run: took a check place after 1s' >&2; echo 'checking' >&2; sleep 60", changeset.Submitted},
		{"echo 'checking' >&2; sleep 60", changeset.Failed},
	} {
		e, id := slotFixture(t, "true", tc.exe, "true")
		e.Cfg.Gates.Run[1].TimeoutSeconds = 1
		err := e.LandLoop(io.Discard)
		if (tc.state == changeset.Submitted) != (err != nil && strings.Contains(err.Error(), "timed out after 1s through wk run --slot")) {
			t.Errorf("%s: loop = %v", tc.exe, err)
		}
		r := mustState(t, e, id, tc.state)
		if tc.state == changeset.Failed && !strings.Contains(r.Reason, "timed out after 1s") {
			t.Errorf("failed record = %+v; want the timeout reason", r)
		}
	}
}

// Row 5's N4 at the start check: a slotted gate whose needs_env is met only by [gates.env] runs,
// so canStart checks it and refuses its missing command; set nowhere, the gate skips unchecked.
func TestSlottedGateNeedsEnvMetByGatesEnvIsChecked(t *testing.T) {
	const v = "WK_TEST_SLOT_NEEDS"
	for _, set := range []bool{true, false} {
		e, id := slotFixture(t, "wk-test-missing-tool", "exit 1", "wk-test-missing-tool")
		e.Cfg.Gates.Run[1].NeedsEnv = []string{v}
		if set {
			e.Cfg.Gates.Env[v] = "x"
		}
		err := e.LandLoop(io.Discard)
		if set != (err != nil && strings.Contains(err.Error(), "wk-test-missing-tool is not on the gate's PATH")) {
			t.Errorf("set in [gates.env] %v: loop = %v", set, err)
		}
		if !set {
			mustState(t, e, id, changeset.Landed)
		}
	}
}

// Row 5's N-c: canStart decides executability as wk run does, with access(2). A file whose only
// execute bit is the group's is executable by the mode bits but not for its owner.
func TestCanStartUsesAccessLikeWkRun(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("as root, access(2) grants execute for any execute bit, so the mode bits and access agree and the case cannot be made")
	}
	dir := t.TempDir()
	tool := filepath.Join(dir, "tool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(tool, 0o010); err != nil {
		t.Fatal(err)
	}
	if fi, err := os.Stat(tool); err != nil || fi.Mode()&0o111 == 0 {
		t.Fatalf("stat = %v, %v; want an execute bit set", fi, err)
	}
	s := gateSpec{g: gate.Gate{Name: "g"}, argv0: tool, slotted: true}
	if err := s.canStart(dir, nil); err == nil || !strings.Contains(err.Error(), "not executable") {
		t.Errorf("canStart(%s) = %v; want the refusal", tool, err)
	}
	s.argv0 = "tool"
	if err := s.canStart(dir, map[string]string{"PATH": dir}); err == nil || !strings.Contains(err.Error(), "not on the gate's PATH") {
		t.Errorf("canStart(tool on PATH) = %v; want the refusal", err)
	}
}

// Row 5's N3: a worker stopped between the create and the marker leaves an unmarked trial and
// trial.making naming it. The next worker removes it, makes the trial again and gates the
// batch; the trial is marked and the intent record gone afterwards.
func TestTrialInterruptedBeforeItsMarkerIsMadeAgain(t *testing.T) {
	f, e, ws, _ := gateFixture(t, "1", "a")
	f.Git(t, "worktree", "add", "--quiet", "--no-checkout", "--detach", e.TrialDir(), "main")
	if err := os.WriteFile(e.trialIntent(), []byte(e.TrialDir()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	id := submit(t, e, ws["a"], "a", "")
	loop(t, e)
	mustState(t, e, id, changeset.Landed)
	if err := checkTrialMarker(e.TrialDir()); err != nil {
		t.Fatal(err)
	}
	if exists(e.trialIntent()) {
		t.Fatal("trial.making is still there after the trial was marked")
	}
	if !exists(filepath.Join(e.TrialDir(), "web/a.ts")) {
		t.Fatal("the trial made again does not hold the gated tree")
	}
}

// The intent record never hands over a directory on a branch, which the trial never is.
func TestTrialIntentNeverTakesABranch(t *testing.T) {
	f, e, _, _ := gateFixture(t, "1")
	f.Git(t, "worktree", "add", "--quiet", "-b", "someone", e.TrialDir(), "main")
	if err := os.WriteFile(e.trialIntent(), []byte(e.TrialDir()+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	err := e.withTrial(gitOut(t, f.Repo, "rev-parse", "main^{tree}"), func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "it is on a branch") || !exists(filepath.Join(e.TrialDir(), "web/use.ts")) {
		t.Fatalf("withTrial = %v; want the refusal and the directory kept", err)
	}
}

// postCloneFixture is gateFixture with template.post_clone replaced by a command that logs each
// run and fails while bun.lock holds BROKEN; change submits one changeset made from main.
func postCloneFixture(t *testing.T) (e *Env, ran func() int, change func(path, body string) string) {
	t.Helper()
	f, e, _, _ := gateFixture(t, "1")
	log := filepath.Join(f.Root, "post-clone.log")
	e.Cfg.Template.PostClone = []config.Command{{Run: []string{"sh", "-c", "echo run >> " + log + "; ! grep -q BROKEN bun.lock"}}}
	n := 0
	return e, func() int {
			data, _ := os.ReadFile(log)
			return strings.Count(string(data), "run\n")
		}, func(path, body string) string {
			n++
			ws := filepath.Join(f.Root, fmt.Sprintf("pc-%d", n))
			f.Git(t, "worktree", "add", "--quiet", "-b", fmt.Sprintf("pc-%d", n), ws, "main")
			writeIn(t, ws, path, body)
			return submit(t, e, ws, path, "")
		}
}

// The trial's dependencies follow template.refresh_on: a batch that changes bun.lock runs
// template.post_clone once more, the next batch with the same bun.lock does not, and a batch
// that changes it back runs it again.
func TestTrialRunsPostCloneWhenTheBatchChangesRefreshOn(t *testing.T) {
	e, ran, change := postCloneFixture(t)
	batch := func(path, body string) string {
		id := change(path, body)
		loop(t, e)
		return id
	}
	lock := read(t, filepath.Join(e.Repo.Primary, "bun.lock"))
	mustState(t, e, batch("web/a.ts", "export const a = 1;\n"), changeset.Landed)
	base := ran()
	for _, step := range []struct {
		path, body string
		more       int
	}{
		{"bun.lock", lock + "# new\n", 1},
		{"web/b.ts", "export const b = 1;\n", 1},
		{"bun.lock", lock, 2},
	} {
		mustState(t, e, batch(step.path, step.body), changeset.Landed)
		if got := ran() - base; got != step.more {
			t.Fatalf("after the %s batch post_clone ran %d more times; want %d", step.path, got, step.more)
		}
	}
}

// A post-clone run that fails is a red verdict named post_clone with bun.lock as its path, so
// the changeset that changed bun.lock is failed and the queue keeps moving; the next batch,
// back on the old bun.lock, runs post_clone again, because the failed run left the
// dependencies in no known state.
func TestTrialPostCloneFailureBlamesTheLockfileChange(t *testing.T) {
	e, ran, change := postCloneFixture(t)
	batch := func(path, body string) string {
		id := change(path, body)
		loop(t, e)
		return id
	}
	lock := read(t, filepath.Join(e.Repo.Primary, "bun.lock"))
	mustState(t, e, batch("web/a.ts", "export const a = 1;\n"), changeset.Landed)
	base := ran()
	r := mustState(t, e, batch("bun.lock", lock+"BROKEN\n"), changeset.Failed)
	if r.Gate != "post_clone" || !slices.Equal(r.Paths, []string{"bun.lock"}) {
		t.Fatalf("failed record = %+v; want gate post_clone naming bun.lock", r)
	}
	mustState(t, e, batch("web/b.ts", "export const b = 1;\n"), changeset.Landed)
	if got := ran() - base; got != 2 {
		t.Fatalf("post_clone ran %d more times; want the red run and the run back on the old bun.lock", got)
	}
}

// The review's B1: a post-clone run that fails for a batch whose lockfile is the tip's (a
// registry that is down, say, after the last run left the dependencies unknown) is a start
// error. The changeset stays submitted, and the next batch runs post_clone again.
func TestTrialPostCloneFailureWithoutALockfileChangeBlamesNothing(t *testing.T) {
	e, ran, change := postCloneFixture(t)
	batch := func(path, body string) string {
		id := change(path, body)
		loop(t, e)
		return id
	}
	mustState(t, e, batch("web/a.ts", "export const a = 1;\n"), changeset.Landed)
	gd, err := gitx.GitDir(e.TrialDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gd, trialDepsName), []byte("unknown\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	e.Cfg.Template.PostClone[0].Run[2] += "; exit 1"
	base := ran()
	id := change("web/b.ts", "export const b = 1;\n")
	for pass := 1; pass <= 2; pass++ {
		if err := e.LandLoop(io.Discard); err == nil || !strings.Contains(err.Error(), "the batch does not change bun.lock") {
			t.Fatalf("pass %d: loop = %v; want the start error", pass, err)
		}
		if r := mustState(t, e, id, changeset.Submitted); ran() != base+pass || r.Gate != "" {
			t.Fatalf("pass %d: post_clone ran %d more times, record %+v; want one run a pass and no blame", pass, ran()-base, r)
		}
	}
}

// Fix round 1, B2 (the auditor's probe): gated lines are written before any swap, so a gated
// record whose prediction happens to equal the tip's tree never landed. It is gated again, and
// its change is on the landing tip afterwards.
func TestGatedRecordEqualToTheTipIsNotTakenAsLanded(t *testing.T) {
	f, e, ws, runs := gateFixture(t, "2", "a", "b")
	writeIn(t, ws["a"], "web/a.ts", "export const a = 1;\n")
	writeIn(t, ws["b"], "packages/core/b.ts", "export const b = 1;\n")
	a, b := submit(t, e, ws["a"], "a", ""), submit(t, e, ws["b"], "b", "")
	ra, rb := record(t, e, a), record(t, e, b)
	ra.State, ra.Predicted = changeset.Gated, "1111111111111111111111111111111111111111"
	rb.State, rb.Predicted = changeset.Gated, gitOut(t, f.Repo, "rev-parse", "main^{tree}")
	for _, r := range []changeset.Record{ra, rb} {
		if err := e.queue().Append(r); err != nil {
			t.Fatal(err)
		}
	}
	loop(t, e)
	tip := mustState(t, e, b, changeset.Landed).Commit
	mustState(t, e, a, changeset.Landed)
	if files := gitOut(t, f.Repo, "ls-tree", "-r", "--name-only", tip); !strings.Contains(files, "packages/core/b.ts") || !strings.Contains(files, "web/a.ts") {
		t.Fatalf("the landing tip lacks a change marked landed:\n%s", files)
	}
	if n := runs(); n != 1 {
		t.Fatalf("%d gate runs; want the two gated again as one batch", n)
	}
}

// Fix round 1, N1: a gating record left by a stopped worker reads as stale, not in flight; one
// with no pid, from an older binary, reads as in flight by an unknown worker (fix round 2, N-b).
func TestStatusSaysAStoppedWorkersBatchIsStale(t *testing.T) {
	_, e, _, _ := gateFixture(t, "1")
	dead := exec.Command("true")
	if err := dead.Run(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		pid  int
		want string
	}{
		{dead.Process.Pid, "stale gate run: abc, gating"},
		{os.Getpid(), "batch in flight: abc, gating"},
		{0, "batch in flight: abc, gating tree 0123456789ab since 0001-01-01T00:00:00Z, worker unknown"},
	} {
		e.noteGates(func(l *gateLog) {
			l.PID, l.Batch, l.State, l.Tree = tc.pid, []string{"abc"}, "gating", "0123456789abcdef"
		})
		var out bytes.Buffer
		if err := e.QueueStatus(&out); err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(out.String(), tc.want) {
			t.Errorf("pid %d: status lacks %q:\n%s", tc.pid, tc.want, out.String())
		}
	}
}
