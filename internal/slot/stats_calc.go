package slot

import (
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// roots finds the subdirectory of a worktree a run ran in. runs.log keeps only the directory,
// and the worktree it was in may be gone, so this reads the path, not git.
type roots struct {
	repos   map[string]bool // the repositories with a <repo>.toml in WK_HOME
	learned []string        // subdirectories found by the first two rules, longest first
}

const worktreesDir = "/.claude/worktrees/"

func newRoots(home string) *roots {
	r := &roots{repos: map[string]bool{}}
	tomls, _ := filepath.Glob(filepath.Join(home, "*.toml"))
	for _, p := range tomls {
		name := strings.TrimSuffix(filepath.Base(p), ".toml")
		if !strings.Contains(name, ".") && p != machinePath(home) {
			r.repos[name] = true
		}
	}
	return r
}

// known applies the two sure rules: a path under .claude/worktrees/<name>/, then a path under a
// directory named like a repository wk knows. The deepest match wins.
func (r *roots) known(dir string) (string, bool) {
	if i := strings.LastIndex(dir, worktreesDir); i >= 0 {
		_, rest, _ := strings.Cut(dir[i+len(worktreesDir):], "/")
		return orDot(rest), true
	}
	parts := strings.Split(filepath.Clean(dir), "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if r.repos[parts[i]] {
			return orDot(strings.Join(parts[i+1:], "/")), true
		}
	}
	return "", false
}

// guess serves a directory neither rule knows, such as a landing trial copied to /tmp: it takes
// the longest subdirectory seen elsewhere that ends the path.
func (r *roots) guess(dir string) string {
	for _, sub := range r.learned {
		if strings.HasSuffix(dir, "/"+sub) {
			return sub
		}
	}
	return "."
}

func orDot(s string) string {
	if s == "" {
		return "."
	}
	return s
}

func (r *roots) resolve(runs []timedRun) {
	seen := map[string]bool{}
	var unknown []int
	for i := range runs {
		sub, ok := r.known(runs[i].Workspace)
		if !ok {
			unknown = append(unknown, i)
			continue
		}
		runs[i].subdir = sub
		if sub != "." && !seen[sub] {
			seen[sub] = true
			r.learned = append(r.learned, sub)
		}
	}
	sort.Slice(r.learned, func(a, b int) bool {
		if len(r.learned[a]) != len(r.learned[b]) {
			return len(r.learned[a]) > len(r.learned[b])
		}
		return r.learned[a] < r.learned[b]
	})
	for _, i := range unknown {
		runs[i].subdir = r.guess(runs[i].Workspace)
	}
}

func classStats(runs []timedRun) []ClassStats {
	byClass := map[string][]timedRun{}
	for _, tr := range runs {
		byClass[tr.Class] = append(byClass[tr.Class], tr)
	}
	out := []ClassStats{}
	for name, rs := range byClass {
		cs := ClassStats{Name: name, Runs: len(rs), Overlap: overlap(rs), LongWaits: longWaits(rs), Commands: commandStats(rs)}
		cs.Failed, cs.Wait, cs.Run = summarize(rs)
		out = append(out, cs)
	}
	sort.Slice(out, func(a, b int) bool { return out[a].Name < out[b].Name })
	return out
}

func summarize(rs []timedRun) (failed int, wait, run Distribution) {
	waits, runTimes := make([]float64, len(rs)), make([]float64, len(rs))
	for i, tr := range rs {
		waits[i], runTimes[i] = tr.WaitSeconds, tr.RunSeconds
		if tr.failed() {
			failed++
		}
	}
	return failed, Distribute(waits), Distribute(runTimes)
}

type commandKey struct{ subdir, command string }

func commandStats(rs []timedRun) []CommandStats {
	groups := map[commandKey][]timedRun{}
	for _, tr := range rs {
		k := commandKey{tr.subdir, tr.command}
		groups[k] = append(groups[k], tr)
	}
	out := make([]CommandStats, 0, len(groups))
	for k, g := range groups {
		cs := CommandStats{Subdir: k.subdir, Command: k.command, Runs: len(g)}
		cs.Failed, cs.Wait, cs.Run = summarize(g)
		out = append(out, cs)
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Wait.Total != out[b].Wait.Total {
			return out[a].Wait.Total > out[b].Wait.Total
		}
		if out[a].Runs != out[b].Runs {
			return out[a].Runs > out[b].Runs
		}
		if out[a].Command != out[b].Command {
			return out[a].Command < out[b].Command
		}
		return out[a].Subdir < out[b].Subdir
	})
	return out
}

// edge is one end of a wait: +1 when a run starts to wait, -1 when it takes its place.
type edge struct {
	at    time.Time
	delta int
}

// overlap sweeps the waits in time order. At equal instants an end comes before a start, so two
// waits that only touch never count as queued together.
func overlap(rs []timedRun) Overlap {
	var edges []edge
	var o Overlap
	for _, tr := range rs {
		if tr.WaitSeconds > 0 {
			o.AgentSeconds += tr.WaitSeconds
			edges = append(edges, edge{tr.queued, 1}, edge{tr.started, -1})
		}
	}
	sort.Slice(edges, func(a, b int) bool {
		if !edges[a].at.Equal(edges[b].at) {
			return edges[a].at.Before(edges[b].at)
		}
		return edges[a].delta < edges[b].delta
	})
	queued := 0
	for i, e := range edges {
		if queued > 0 {
			o.WallSeconds += e.at.Sub(edges[i-1].at).Seconds()
		}
		if queued += e.delta; queued > o.PeakQueued {
			o.PeakQueued, o.PeakAt = queued, e.at
		}
	}
	return o
}

type span struct{ from, to time.Time }

// longWaits merges the long waits into disjoint spans first, so a run that held a place while
// twenty runs waited is counted for its held time once, not twenty times.
func longWaits(rs []timedRun) LongWaits {
	var lw LongWaits
	var spans []span
	for _, tr := range rs {
		if seconds(tr.WaitSeconds) > longWait {
			lw.Count++
			lw.TotalSeconds += tr.WaitSeconds
			spans = append(spans, span{tr.queued, tr.started})
		}
	}
	lw.Holders = holders(rs, merge(spans))
	return lw
}

func merge(spans []span) []span {
	sort.Slice(spans, func(a, b int) bool {
		if !spans[a].from.Equal(spans[b].from) {
			return spans[a].from.Before(spans[b].from)
		}
		return spans[a].to.Before(spans[b].to)
	})
	var out []span
	for _, s := range spans {
		if n := len(out); n > 0 && !s.from.After(out[n-1].to) {
			if s.to.After(out[n-1].to) {
				out[n-1].to = s.to
			}
			continue
		}
		out = append(out, s)
	}
	return out
}

func holders(rs []timedRun, spans []span) []Holder {
	byKey := map[commandKey]*Holder{}
	for _, tr := range rs {
		held := heldInside(span{tr.started, tr.ended}, spans)
		if held <= 0 {
			continue
		}
		k := commandKey{tr.subdir, tr.command}
		if byKey[k] == nil {
			byKey[k] = &Holder{Subdir: k.subdir, Command: k.command}
		}
		byKey[k].Runs++
		byKey[k].HeldSeconds += held
	}
	out := []Holder{}
	for _, h := range byKey {
		out = append(out, *h)
	}
	sort.Slice(out, func(a, b int) bool {
		x, y := out[a], out[b]
		if x.HeldSeconds != y.HeldSeconds {
			return x.HeldSeconds > y.HeldSeconds
		}
		if x.Runs != y.Runs {
			return x.Runs > y.Runs
		}
		if x.Command != y.Command {
			return x.Command < y.Command
		}
		return x.Subdir < y.Subdir
	})
	return out
}

func heldInside(run span, spans []span) float64 {
	total := 0.0
	for _, s := range spans {
		from, to := later(run.from, s.from), earlier(run.to, s.to)
		if to.After(from) {
			total += to.Sub(from).Seconds()
		}
	}
	return total
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func earlier(a, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

// repeats walks each command in each directory in the order the runs were issued. A run issued
// while the previous one still ran is not a repeat: the two ran side by side.
func repeats(runs []timedRun) Repeats {
	byKey := map[commandKey][]timedRun{}
	for _, tr := range runs {
		k := commandKey{tr.Workspace, tr.command}
		byKey[k] = append(byKey[k], tr)
	}
	var rep Repeats
	counts := map[commandKey]int{}
	// Float sums depend on the order of the terms, so walk the keys in a fixed order.
	keys := make([]commandKey, 0, len(byKey))
	for k := range byKey {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(a, b int) bool {
		if keys[a].subdir != keys[b].subdir {
			return keys[a].subdir < keys[b].subdir
		}
		return keys[a].command < keys[b].command
	})
	for _, k := range keys {
		rs := byKey[k]
		sort.Slice(rs, func(a, b int) bool {
			if !rs[a].queued.Equal(rs[b].queued) {
				return rs[a].queued.Before(rs[b].queued)
			}
			if !rs[a].started.Equal(rs[b].started) {
				return rs[a].started.Before(rs[b].started)
			}
			return rs[a].ended.Before(rs[b].ended)
		})
		for i := 1; i < len(rs); i++ {
			if gap := rs[i].queued.Sub(rs[i-1].ended); gap >= 0 && gap <= repeatWindow {
				rep.add(rs[i-1], rs[i])
				counts[commandKey{rs[i].subdir, rs[i].command}]++
			}
		}
	}
	rep.Commands = rankRepeats(counts)
	return rep
}

func (rep *Repeats) add(prev, next timedRun) {
	rep.Count++
	rep.RunSeconds += next.RunSeconds
	if prev.failed() {
		rep.AfterFailure++
	}
}

func rankRepeats(counts map[commandKey]int) []RepeatGroup {
	out := []RepeatGroup{}
	for k, n := range counts {
		out = append(out, RepeatGroup{Subdir: k.subdir, Command: k.command, Count: n})
	}
	sort.Slice(out, func(a, b int) bool {
		if out[a].Count != out[b].Count {
			return out[a].Count > out[b].Count
		}
		if out[a].Command != out[b].Command {
			return out[a].Command < out[b].Command
		}
		return out[a].Subdir < out[b].Subdir
	})
	return out
}

// stampedLines returns the text after the stamp of each line of path inside r; a line without a
// stamp has no known age and is left out.
func stampedLines(path string, r Range) ([]string, error) {
	lines, err := readLines(path)
	var out []string
	for _, line := range lines {
		stamp, rest, _ := strings.Cut(line, "\t")
		if at, perr := time.Parse(logTimeLayout, stamp); perr == nil && r.Contains(at) {
			out = append(out, rest)
		}
	}
	return out, err
}

// readMissed counts missed.log by its reason, the words between the repository and the first
// colon: `<repo> <reason>: "<command>" in "<line>"`. A reason's own detail after a colon, such as
// the parse error of "does not parse", is left out so the same reason counts once.
func readMissed(path string, r Range) (MissedStats, error) {
	lines, err := stampedLines(path, r)
	counts := map[string]int{}
	for _, line := range lines {
		counts[missedReason(line)]++
	}
	ms := MissedStats{Lines: len(lines), Reasons: []ReasonCount{}}
	for reason, n := range counts {
		ms.Reasons = append(ms.Reasons, ReasonCount{reason, n})
	}
	sort.Slice(ms.Reasons, func(a, b int) bool {
		x, y := ms.Reasons[a], ms.Reasons[b]
		return x.Count > y.Count || x.Count == y.Count && x.Reason < y.Reason
	})
	return ms, err
}

func missedReason(line string) string {
	head, _, _ := strings.Cut(line, ": ")
	if _, reason, ok := strings.Cut(head, " "); ok && reason != "" {
		return reason
	}
	return "unreadable"
}

var deadlineRE = regexp.MustCompile(`deadline of (\d+) ms passed after (\d+) ms`)

func readDeadlines(path string, r Range) (DeadlineStats, error) {
	lines, err := stampedLines(path, r)
	var ds DeadlineStats
	for _, line := range lines {
		m := deadlineRE.FindStringSubmatch(line)
		if m == nil {
			ds.Other++
			continue
		}
		ds.DeadlineMS, _ = strconv.Atoi(m[1])
		ds.add(m[2])
	}
	return ds, err
}

func (ds *DeadlineStats) add(latency string) {
	ms, _ := strconv.Atoi(latency)
	if ds.Count == 0 || ms < ds.MinMS {
		ds.MinMS = ms
	}
	ds.MaxMS = max(ds.MaxMS, ms)
	ds.Count++
}
