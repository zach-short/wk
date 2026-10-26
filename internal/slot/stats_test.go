package slot

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

var t0 = time.Date(2026, 10, 24, 6, 0, 0, 0, time.UTC)

// runLine writes a runs.log line for a run that waited wait and ran run, ending at end.
func runLine(end time.Time, class, exit string, wait, run time.Duration, dir, cmd string) string {
	return fmt.Sprintf("%s\tclass=%s\tpid=7\texit=%s\twait=%.1fs\trun=%.1fs\tdir=%s\tcmd=%s",
		end.Format(logTimeLayout), class, exit, wait.Seconds(), run.Seconds(), dir, cmd)
}

func TestDistributeTakesNearestRankPercentiles(t *testing.T) {
	t.Parallel()
	values := []float64{10, 9, 8, 7, 6, 5, 4, 3, 2, 1}
	d := Distribute(values)
	if want := (Distribution{Count: 10, Total: 55, Median: 5, P90: 9}); d != want {
		t.Fatalf("Distribute = %+v, want %+v", d, want)
	}
	if values[0] != 10 {
		t.Fatalf("Distribute sorted its input: %v", values)
	}
	if d := Distribute(nil); d != (Distribution{}) {
		t.Fatalf("Distribute(nil) = %+v, want zero", d)
	}
	if d := Distribute([]float64{3}); d.Median != 3 || d.P90 != 3 {
		t.Fatalf("one value gives %+v, want median and p90 of 3", d)
	}
	if got := percentile([]float64{1, 2, 3}, 90); got != 3 {
		t.Fatalf("p90 of three = %v, want 3", got)
	}
}

func TestParseRunsReadsSignalsAndCountsMalformedLines(t *testing.T) {
	t.Parallel()
	lines := []string{
		runLine(t0, "check", "SIGTERM", 30*time.Second, 20*time.Second, "/w", "go  test   ./... "),
		runLine(t0, "check", "0", 0, time.Second, "/w", "true"),
		"not a run",
		"2026-13-45T00:00:00Z\tclass=check",
		t0.Format(logTimeLayout) + "\tpid=1\texit=0",
		"",
		runLine(t0.Add(48*time.Hour), "check", "0", 0, 0, "/w", "late"),
	}
	r := Range{Until: t0.Add(24 * time.Hour)}
	runs, malformed := parseRuns(lines, r)
	if malformed != 3 {
		t.Fatalf("malformed = %d, want 3 (no stamp, bad stamp, no class); the blank line is not one", malformed)
	}
	if len(runs) != 2 {
		t.Fatalf("runs = %d, want 2: the one after --until is left out", len(runs))
	}
	sig := runs[0]
	if sig.Exit != "SIGTERM" || !sig.failed() || runs[1].failed() {
		t.Fatalf("exits %q and %q: want SIGTERM to count as nonzero and 0 not", sig.Exit, runs[1].Exit)
	}
	if !sig.started.Equal(t0.Add(-20*time.Second)) || !sig.queued.Equal(t0.Add(-50*time.Second)) {
		t.Fatalf("started %v queued %v, want end-run and start-wait", sig.started, sig.queued)
	}
	if sig.command != "go test ./..." {
		t.Fatalf("command %q, want whitespace collapsed", sig.command)
	}
}

func TestParseRangeIncludesBothDays(t *testing.T) {
	t.Parallel()
	r, err := ParseRange("2026-10-17", "2026-10-26")
	if err != nil {
		t.Fatal(err)
	}
	in := time.Date(2026, 10, 26, 23, 59, 0, 0, time.Local)
	out := time.Date(2026, 10, 27, 0, 0, 0, 0, time.Local)
	before := time.Date(2026, 10, 16, 23, 59, 0, 0, time.Local)
	if !r.Contains(in) || r.Contains(out) || r.Contains(before) {
		t.Fatalf("range %+v: want the last minute of the 26th in, the 27th and the 16th out", r)
	}
	if since, until := r.bounds(); since != "2026-10-17" || until != "2026-10-26" {
		t.Fatalf("bounds %s %s, want the dates given", since, until)
	}
	if _, err := ParseRange("10/17", ""); err == nil || !strings.Contains(err.Error(), "--since") {
		t.Fatalf("bad --since gave %v, want a refusal naming it", err)
	}
	if !(Range{}).Contains(t0) {
		t.Fatal("an open range must hold every time")
	}
}

func timedRuns(t *testing.T, lines ...string) []timedRun {
	t.Helper()
	runs, malformed := parseRuns(lines, Range{})
	if malformed != 0 {
		t.Fatalf("fixture has %d malformed lines", malformed)
	}
	return runs
}

// Three waits: A from 0 to 10 min, B from 5 to 15, C from 15 to 20, which touches B's end. Two
// were queued at most at once, the wall time is 20 min, and the agent time is 25 min.
func TestOverlapSweepsTheWaits(t *testing.T) {
	t.Parallel()
	m := time.Minute
	runs := timedRuns(t,
		runLine(t0.Add(11*m), "check", "0", 10*m, m, "/a", "a"),
		runLine(t0.Add(16*m), "check", "0", 10*m, m, "/b", "b"),
		runLine(t0.Add(21*m), "check", "0", 5*m, m, "/c", "c"),
		runLine(t0.Add(40*m), "check", "0", 0, m, "/d", "d"),
	)
	o := overlap(runs)
	if o.AgentSeconds != 1500 || o.WallSeconds != 1200 || o.PeakQueued != 2 {
		t.Fatalf("overlap %+v, want 1500 s of agent time in 1200 s of wall time, peak 2", o)
	}
	if !o.PeakAt.Equal(t0.Add(5 * m)) {
		t.Fatalf("peak at %v, want when B joined A", o.PeakAt)
	}
	if o := overlap(nil); o.WallSeconds != 0 || o.PeakQueued != 0 {
		t.Fatalf("no runs gave %+v", o)
	}
}

// Two long waits cover 0 to 20 min together. H1 holds a place from -5 to 15 min, H2 from 15 to
// 25: H1 held 15 min inside them and H2 5, and neither is counted twice for the two waiters.
func TestLongWaitsRankTheHolders(t *testing.T) {
	t.Parallel()
	m := time.Minute
	runs := timedRuns(t,
		runLine(t0.Add(21*m), "check", "0", 20*m, m, "/w1/backend", "go test ./..."),
		runLine(t0.Add(19*m), "check", "0", 15*m, m, "/w2/backend", "go test ./..."),
		runLine(t0.Add(15*m), "check", "1", 0, 20*m, "/w3", "bun run test"),
		runLine(t0.Add(25*m), "check", "0", 0, 10*m, "/w4", "bun run lint"),
		runLine(t0.Add(30*m), "check", "0", 9*m, m, "/w5", "short"),
	)
	lw := longWaits(runs)
	if lw.Count != 2 || lw.TotalSeconds != 35*60 {
		t.Fatalf("long waits %d, %v s; want 2 and 2100 s (the 9 min wait is not long)", lw.Count, lw.TotalSeconds)
	}
	want := []Holder{{Command: "bun run test", Runs: 1, HeldSeconds: 900}, {Command: "bun run lint", Runs: 1, HeldSeconds: 300}}
	got := lw.Holders[:2]
	for i := range got {
		got[i].Subdir = ""
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("holders %+v, want %+v", lw.Holders, want)
	}
}

func TestRepeatsAreReissuesWithinTenMinutes(t *testing.T) {
	t.Parallel()
	m := time.Minute
	runs := timedRuns(t,
		runLine(t0, "check", "1", 0, m, "/w/web", "bunx tsc --noEmit"),
		runLine(t0.Add(6*m), "check", "0", 4*m, m, "/w/web", "bunx  tsc --noEmit"),              // issued 1 min after: a repeat after a failure
		runLine(t0.Add(30*m), "check", "0", 0, m, "/w/web", "bunx tsc --noEmit"),                // issued 23 min after: not one
		runLine(t0.Add(30*m+30*time.Second), "check", "0", 0, m, "/w/web", "bunx tsc --noEmit"), // issued while the last ran
		runLine(t0.Add(2*m), "check", "0", 0, m, "/other/web", "bunx tsc --noEmit"),             // another directory
	)
	newRoots(t.TempDir()).resolve(runs)
	rep := repeats(runs)
	if rep.Count != 1 || rep.AfterFailure != 1 || rep.RunSeconds != 60 {
		t.Fatalf("repeats %+v, want one, after a failure, of 60 s", rep)
	}
	if len(rep.Commands) != 1 || rep.Commands[0].Command != "bunx tsc --noEmit" {
		t.Fatalf("repeat groups %+v", rep.Commands)
	}
}

func TestSubdirFromWorktreeRepoOrLearnedSuffix(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	for _, f := range []string{"ezh.toml", "ezh.slots.toml", "machine.toml"} {
		if err := os.WriteFile(filepath.Join(home, f), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	dirs := map[string]string{
		"/p/ezh/.claude/worktrees/agent-1/packages/core": "packages/core",
		"/p/ezh/.claude/worktrees/agent-1":               ".",
		"/p/ezh/web":                                     "web",
		"/p/ezh":                                         ".",
		"/tmp/x/land/trial/packages/core":                "packages/core",
		"/tmp/x/land/trial/web":                          "web",
		"/tmp/x/land/trial":                              ".",
	}
	var runs []timedRun
	for dir := range dirs {
		runs = append(runs, timedRun{RunRecord: RunRecord{Workspace: dir}})
	}
	newRoots(home).resolve(runs)
	for _, tr := range runs {
		if tr.subdir != dirs[tr.Workspace] {
			t.Errorf("subdir of %s = %q, want %q", tr.Workspace, tr.subdir, dirs[tr.Workspace])
		}
	}
}

func writeLog(t *testing.T, path string, lines ...string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadStatsFromFixtureLogs(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	dir := filepath.Join(home, "slots")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeLog(t, filepath.Join(home, "machine.toml"), `[slots]`, `dir = "`+dir+`"`, `[slots.check]`, `capacity = 2`)
	wt := "/p/ezh/.claude/worktrees/agent-1/backend"
	writeLog(t, filepath.Join(dir, runsLog),
		runLine(t0, "check", "0", 2*time.Second, 10*time.Second, wt, "go test ./..."),
		runLine(t0.Add(time.Hour), "check", "SIGTERM", 0, 4*time.Second, wt, "go test ./..."),
		runLine(t0, "heavy", "2", 0, time.Second, wt, "go run ./cmd/lintgate"),
		"torn line",
	)
	stamp := t0.Format(logTimeLayout)
	writeLog(t, filepath.Join(dir, "missed.log"),
		stamp+"\tezh inside time: \"go test\" in \"time go test\"",
		stamp+"\tezh inside time: \"bun run test\" in \"time bun run test\"",
		stamp+"\tezh does not parse: 1:62: bad: \"x\" in \"y\"",
		"no stamp at all")
	writeLog(t, filepath.Join(dir, errorsLog),
		stamp+"\thook pre-tool-use ezh: deadline of 200 ms passed after 370 ms; the command runs unslotted",
		stamp+"\thook pre-tool-use ezh: deadline of 200 ms passed after 235 ms; the command runs unslotted",
		stamp+"\tsomething else")
	s, err := ReadStats(home, Range{})
	if err != nil {
		t.Fatal(err)
	}
	checkFixtureStats(t, s)
}

func checkFixtureStats(t *testing.T, s Stats) {
	t.Helper()
	if s.Runs != 3 || s.Malformed != 1 || len(s.Classes) != 2 || s.Classes[0].Name != "check" {
		t.Fatalf("stats %+v, want 3 runs, 1 malformed line, check then heavy", s)
	}
	check := s.Classes[0]
	if check.Failed != 1 || len(check.Commands) != 1 || check.Commands[0].Subdir != "backend" || check.Commands[0].Runs != 2 {
		t.Fatalf("check %+v, want one backend go test group of 2 runs, 1 nonzero", check)
	}
	wantMissed := []ReasonCount{{"inside time", 2}, {"does not parse", 1}}
	if s.Missed.Lines != 3 || !reflect.DeepEqual(s.Missed.Reasons, wantMissed) {
		t.Fatalf("missed %+v, want %v", s.Missed, wantMissed)
	}
	if want := (DeadlineStats{Count: 2, DeadlineMS: 200, MinMS: 235, MaxMS: 370, Other: 1}); s.Deadlines != want {
		t.Fatalf("deadlines %+v, want %+v", s.Deadlines, want)
	}
	text := s.Text()
	for _, want := range []string{"3 runs, 1 unreadable line", "check: 2 runs, 1 nonzero exit", "backend  go test ./...", "2 hook deadline misses (deadline 200 ms), latency 235 to 370 ms"} {
		if !strings.Contains(text, want) {
			t.Errorf("text lacks %q:\n%s", want, text)
		}
	}
}

func TestDurText(t *testing.T) {
	t.Parallel()
	for sec, want := range map[float64]string{0: "0.0s", 59.94: "59.9s", 61: "1m01s", 3599: "59m59s", 5400: "1.5h"} {
		if got := dur(sec); got != want {
			t.Errorf("dur(%v) = %q, want %q", sec, got, want)
		}
	}
}
