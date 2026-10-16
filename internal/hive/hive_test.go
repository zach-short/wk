package hive

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zach-short/wk/internal/wk"
)

var stamp = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func sample() wk.Report {
	at := func(d time.Duration) string { return stamp.Add(-d).Format(time.RFC3339) }
	return wk.Report{
		Base: "main", Generated: at(4 * time.Second), Removable: 1,
		Worktrees: []wk.ReportItem{
			{Path: ".claude/worktrees/wk-41", Branch: "wk/41", Head: "a1b2c3d", Verdict: wk.Kept,
				Reasons: []string{"it changed 3 minutes ago, so a session may still be in it"}, Dirty: 3, LastActivity: at(3 * time.Minute)},
			{Path: ".claude/worktrees/wk-44", Branch: "wk/44", Head: "e4f5a6b", Verdict: wk.Kept,
				Reasons: []string{"a process is working in it: claude (pid 8121)"}},
			{Path: ".claude/worktrees/wk-50", Branch: "wk/50", Head: "f00ba44", Verdict: wk.Kept,
				Reasons: []string{"branch wk/50 has commits main does not hold"}},
			{Path: ".claude/worktrees/wk-38", Branch: "wk/38", Head: "c7d8e9f", Verdict: wk.KeptForLanding,
				Reasons: []string{"4 changed paths"}, Dirty: 4},
			{Path: ".claude/worktrees/wk-29", Branch: "wk/29", Head: "0a9b8c7", Verdict: wk.Removable,
				Reasons: []string{"clean and idle"}, DeleteBranch: true},
			{Path: ".claude/worktrees/wk-12", Head: "9f8e7d6", Verdict: wk.Skipped, Reasons: []string{"the wk template"}},
			{Path: ".claude/worktrees/wk-7", Branch: "wk/7", Head: "1234abc", Verdict: wk.Prunable,
				Reasons: []string{"its directory is gone; git worktree prune clears the entry"}},
		},
	}
}

func TestStyleOfMatchesThePage(t *testing.T) {
	want := []Key{Working, Working, Idle, Land, Removable, Skip, Prunable}
	for i, it := range sample().Worktrees {
		if got := StyleOf(it); got != want[i] {
			t.Errorf("%s (%s): style %s, want %s", it.Path, it.Verdict, got, want[i])
		}
	}
	if got := StyleOf(wk.ReportItem{Verdict: wk.Kept, Reasons: []string{"it changed 1 minute ago, so a session may still be in it"}}); got != Working {
		t.Errorf("one minute ago: %s, want working", got)
	}
}

func TestSlotsFillRingsOutwardWithoutRepeats(t *testing.T) {
	slots := Slots(18)
	seen := map[[2]int]bool{}
	for i, s := range slots {
		wantRing := 1
		if i >= 6 {
			wantRing = 2
		}
		if s.D != wantRing {
			t.Errorf("slot %d is in ring %d, want %d", i, s.D, wantRing)
		}
		if seen[[2]int{s.Q, s.R}] {
			t.Errorf("slot %d repeats (%d,%d)", i, s.Q, s.R)
		}
		seen[[2]int{s.Q, s.R}] = true
	}
	// The page sorts by angle atan2(r + q/2, q), so the first cell is the left neighbour.
	if first := slots[0]; first.Q != -1 || first.R != 0 {
		t.Errorf("first slot = %+v, want q=-1 r=0", first)
	}
}

// fixture counts loads so a test can see that ticks, and only ticks, read again.
type fixture struct {
	calls  int
	report wk.Report
	err    error
}

func (f *fixture) load() (wk.Report, error) {
	f.calls++
	return f.report, f.err
}

func newModel(f *fixture) Model {
	m := New(f.load, 3*time.Second)
	m.now = func() time.Time { return stamp }
	return m
}

func step(t *testing.T, m Model, msg tea.Msg) (Model, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	return next.(Model), cmd
}

func TestModelReadsOnStartAndOnEachTick(t *testing.T) {
	f := &fixture{report: sample()}
	m := newModel(f)

	msg := m.Init()()
	if f.calls != 1 {
		t.Fatalf("Init read %d times, want 1", f.calls)
	}
	m, cmd := step(t, m, msg)
	if !m.loaded || len(m.report.Worktrees) != 7 || cmd == nil {
		t.Fatalf("after the first report: loaded=%v worktrees=%d cmd=%v", m.loaded, len(m.report.Worktrees), cmd)
	}
	if f.calls != 1 {
		t.Fatalf("a report read again by itself (%d reads); only a tick may", f.calls)
	}

	m, cmd = step(t, m, tickMsg(stamp))
	if cmd == nil {
		t.Fatal("a tick returned no read")
	}
	if _, ok := cmd().(reportMsg); !ok || f.calls != 2 {
		t.Fatalf("the tick's command did not read once more (%d reads)", f.calls)
	}
}

func TestModelKeepsTheLastReportWhenAReadFails(t *testing.T) {
	f := &fixture{report: sample()}
	m := newModel(f)
	m, _ = step(t, m, m.Init()())
	m, cmd := step(t, m, reportMsg{err: errors.New("base \"main\": not found")})
	if cmd == nil {
		t.Fatal("a failed read stopped the refresh")
	}
	view := m.View()
	if !strings.Contains(view, `wk hive: base "main": not found`) || !strings.Contains(view, "wk/41") {
		t.Fatalf("view after a failed read:\n%s", view)
	}
}

func TestModelQuitsOnQAndCtrlC(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune("q")},
		{Type: tea.KeyCtrlC},
		{Type: tea.KeyEsc},
	} {
		_, cmd := step(t, newModel(&fixture{}), key)
		if cmd == nil {
			t.Fatalf("%q: no command", key.String())
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Fatalf("%q did not quit", key.String())
		}
	}
}

func TestSelectionWrapsAndClampsWhenWorktreesGo(t *testing.T) {
	f := &fixture{report: sample()}
	m := newModel(f)
	m, _ = step(t, m, m.Init()())

	left := tea.KeyMsg{Type: tea.KeyLeft}
	m, _ = step(t, m, left)
	if m.selected != 6 {
		t.Fatalf("left from the first cell selected %d, want 6", m.selected)
	}
	m, _ = step(t, m, tea.KeyMsg{Type: tea.KeyRight})
	if m.selected != 0 {
		t.Fatalf("right from the last cell selected %d, want 0", m.selected)
	}

	m, _ = step(t, m, left)
	short := sample()
	short.Worktrees = short.Worktrees[:2]
	m, _ = step(t, m, reportMsg{report: short})
	if m.selected != 1 {
		t.Fatalf("after the list shrank to 2, selected = %d, want 1", m.selected)
	}
}

func TestViewDrawsTheLegendHiveAndDetail(t *testing.T) {
	f := &fixture{report: sample()}
	m := newModel(f)
	m, _ = step(t, m, m.Init()())
	view := m.View()
	for _, want := range []string{
		"base main", "updated 4s ago", "read-only",
		"2 working", "1 kept", "1 run /land-worktrees", "1 removable", "1 skipped", "1 prunable",
		"wk/41", "wk/44", "wk-12", "wk/7",
		".claude/worktrees/wk-41", "it changed 3 minutes ago", "activity 3 min ago",
		"q quit",
	} {
		if !strings.Contains(view, want) {
			t.Errorf("view lacks %q", want)
		}
	}
}

// The hub sits between its left and right neighbours on one line, and the rows above and below
// sit half a cell across, as hexagons interlock.
func TestHoneycombPlacesTheHubInTheMiddle(t *testing.T) {
	r := sample()
	r.Worktrees = r.Worktrees[:6]
	m := newModel(&fixture{})
	m.report, m.loaded = r, true
	lines := strings.Split(m.honeycomb(), "\n")
	if len(lines) != 8 {
		t.Fatalf("ring 1 drew %d lines, want 8:\n%s", len(lines), strings.Join(lines, "\n"))
	}
	var hubLine string
	for _, l := range lines {
		if strings.Contains(l, "main") {
			hubLine = l
		}
	}
	slots := Slots(6)
	var leftName, rightName string
	for i, s := range slots {
		if s.R == 0 && s.Q < 0 {
			leftName = name(r.Worktrees[i])
		}
		if s.R == 0 && s.Q > 0 {
			rightName = name(r.Worktrees[i])
		}
	}
	l, h, rt := strings.Index(hubLine, leftName), strings.Index(hubLine, "main"), strings.Index(hubLine, rightName)
	if l < 0 || rt < 0 || l >= h || h >= rt {
		t.Fatalf("hub line %q: want %s, main, %s in order", hubLine, leftName, rightName)
	}
	if lines[0] == "" || strings.Contains(lines[0], "main") {
		t.Fatalf("the top row should hold ring-1 cells only: %q", lines[0])
	}
}

func TestEmptyHiveSaysHowToStart(t *testing.T) {
	m := newModel(&fixture{})
	m, _ = step(t, m, reportMsg{report: wk.Report{Base: "main", Generated: stamp.Format(time.RFC3339)}})
	if view := m.View(); !strings.Contains(view, "No worktrees yet") || !strings.Contains(view, "main") {
		t.Fatalf("empty view:\n%s", view)
	}
}

func TestInkReadsOnEveryFill(t *testing.T) {
	for k, c := range palette {
		for _, fill := range []string{c.Light, c.Dark} {
			ink := inkFor(fill)
			hi, lo := luminance(fill), luminance(ink)
			if lo > hi {
				hi, lo = lo, hi
			}
			if ratio := (hi + 0.05) / (lo + 0.05); ratio < 3 {
				t.Errorf("%s fill %s with ink %s: contrast %.2f, want at least 3", k, fill, ink, ratio)
			}
		}
	}
}

func TestAgo(t *testing.T) {
	for d, want := range map[time.Duration]string{
		10 * time.Second: "10s ago",
		20 * time.Minute: "20 min ago",
		5 * time.Hour:    "5 h ago",
		72 * time.Hour:   "3 days ago",
	} {
		if got := ago(stamp, stamp.Add(-d).Format(time.RFC3339)); got != want {
			t.Errorf("ago(%s) = %q, want %q", d, got, want)
		}
	}
	if got := ago(stamp, ""); got != "unknown" {
		t.Errorf("ago(empty) = %q", got)
	}
}
