package slot

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Stats is what `wk stats` reports about the places: how the runs of runs.log waited and ran,
// and what missed.log and errors.log say about the hook, over one date range. It replaces the
// one-off scripts the slot reviews of 2026-10-26 were done with.
type Stats struct {
	Dir       string        `json:"dir"`
	Since     string        `json:"since,omitempty"`
	Until     string        `json:"until,omitempty"`
	Runs      int           `json:"runs"`
	Malformed int           `json:"malformed_lines"`
	Classes   []ClassStats  `json:"classes"`
	Repeats   Repeats       `json:"repeats"`
	Missed    MissedStats   `json:"missed"`
	Deadlines DeadlineStats `json:"deadlines"`
}

// ClassStats is one class's runs. Commands is ranked by total wait, the costliest first.
type ClassStats struct {
	Name      string         `json:"name"`
	Runs      int            `json:"runs"`
	Failed    int            `json:"failed"`
	Wait      Distribution   `json:"wait"`
	Run       Distribution   `json:"run"`
	Overlap   Overlap        `json:"overlap"`
	LongWaits LongWaits      `json:"long_waits"`
	Commands  []CommandStats `json:"commands"`
}

// CommandStats groups the runs of one command in one subdirectory of the worktrees, so the same
// `go test ./...` in backend/ of forty worktrees is one row.
type CommandStats struct {
	Subdir  string       `json:"subdir"`
	Command string       `json:"command"`
	Runs    int          `json:"runs"`
	Failed  int          `json:"failed"`
	Wait    Distribution `json:"wait"`
	Run     Distribution `json:"run"`
}

// Overlap says how the waits of a class stacked: AgentSeconds is every wait added up, WallSeconds
// the clock time in which at least one run waited, PeakQueued the most that waited at once.
type Overlap struct {
	AgentSeconds float64   `json:"agent_seconds"`
	WallSeconds  float64   `json:"wall_seconds"`
	PeakQueued   int       `json:"peak_queued"`
	PeakAt       time.Time `json:"peak_at,omitzero"`
}

// LongWaits are the waits over longWait, and the runs that held the class's places while one of
// them went on, ranked by the time they held a place inside those waits.
type LongWaits struct {
	Count        int      `json:"count"`
	TotalSeconds float64  `json:"total_seconds"`
	Holders      []Holder `json:"holders"`
}

type Holder struct {
	Subdir      string  `json:"subdir"`
	Command     string  `json:"command"`
	Runs        int     `json:"runs"`
	HeldSeconds float64 `json:"held_seconds"`
}

// Repeats counts runs issued again in the same directory within repeatWindow of the previous
// run of that command ending. AfterFailure is how many followed a nonzero exit.
type Repeats struct {
	Count        int           `json:"count"`
	AfterFailure int           `json:"after_failure"`
	RunSeconds   float64       `json:"run_seconds"`
	Commands     []RepeatGroup `json:"commands"`
}

type RepeatGroup struct {
	Subdir  string `json:"subdir"`
	Command string `json:"command"`
	Count   int    `json:"count"`
}

type MissedStats struct {
	Lines   int           `json:"lines"`
	Reasons []ReasonCount `json:"reasons"`
}

type ReasonCount struct {
	Reason string `json:"reason"`
	Count  int    `json:"count"`
}

// DeadlineStats are the hook's deadline misses in errors.log, with the latency each one saw.
// Other counts the lines of errors.log that are not a deadline miss.
type DeadlineStats struct {
	Count      int `json:"count"`
	DeadlineMS int `json:"deadline_ms,omitempty"`
	MinMS      int `json:"min_ms,omitempty"`
	MaxMS      int `json:"max_ms,omitempty"`
	Other      int `json:"other"`
}

// Distribution summarizes durations in seconds. Median and P90 are nearest-rank percentiles,
// so each is a value that was observed.
type Distribution struct {
	Count  int     `json:"count"`
	Total  float64 `json:"total_seconds"`
	Median float64 `json:"median_seconds"`
	P90    float64 `json:"p90_seconds"`
}

const (
	longWait     = 10 * time.Minute
	repeatWindow = 10 * time.Minute
)

// Distribute summarizes values; it does not change the slice it is given.
func Distribute(values []float64) Distribution {
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	d := Distribution{Count: len(sorted), Median: percentile(sorted, 50), P90: percentile(sorted, 90)}
	for _, v := range sorted {
		d.Total += v
	}
	return d
}

// percentile is the nearest-rank percentile of sorted values: the smallest value with at least
// p percent of the values at or below it. Zero values give zero.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p / 100 * float64(len(sorted))))
	return sorted[min(max(rank, 1), len(sorted))-1]
}

// Range is a span of local days; a zero end leaves that side open.
type Range struct {
	Since time.Time // first instant inside
	Until time.Time // first instant after
}

const dayLayout = "2006-01-02"

// ParseRange reads --since and --until as local dates, both days included.
func ParseRange(since, until string) (Range, error) {
	var r Range
	var err error
	if since != "" {
		if r.Since, err = time.ParseInLocation(dayLayout, since, time.Local); err != nil {
			return Range{}, fmt.Errorf("--since %q is not a YYYY-MM-DD date", since)
		}
	}
	if until != "" {
		if r.Until, err = time.ParseInLocation(dayLayout, until, time.Local); err != nil {
			return Range{}, fmt.Errorf("--until %q is not a YYYY-MM-DD date", until)
		}
		r.Until = r.Until.AddDate(0, 0, 1)
	}
	return r, nil
}

func (r Range) Contains(t time.Time) bool {
	return (r.Since.IsZero() || !t.Before(r.Since)) && (r.Until.IsZero() || t.Before(r.Until))
}

func (r Range) bounds() (since, until string) {
	if !r.Since.IsZero() {
		since = r.Since.Format(dayLayout)
	}
	if !r.Until.IsZero() {
		until = r.Until.AddDate(0, 0, -1).Format(dayLayout)
	}
	return since, until
}

// ReadStats reads the three logs of the slots directory named by machine.toml. It takes no lock:
// every line is written in one append, so a read sees whole lines.
func ReadStats(home string, r Range) (Stats, error) {
	m, _, err := LoadMachine(home)
	if err != nil {
		return Stats{}, err
	}
	s := Stats{Dir: m.Dir}
	s.Since, s.Until = r.bounds()
	lines, err := readLines(filepath.Join(m.Dir, runsLog))
	if err != nil {
		return Stats{}, err
	}
	runs, malformed := parseRuns(lines, r)
	s.Runs, s.Malformed = len(runs), malformed
	newRoots(home).resolve(runs)
	s.Classes, s.Repeats = classStats(runs), repeats(runs)
	if s.Missed, err = readMissed(filepath.Join(m.Dir, "missed.log"), r); err != nil {
		return Stats{}, err
	}
	s.Deadlines, err = readDeadlines(filepath.Join(m.Dir, errorsLog), r)
	return s, err
}

// timedRun is a run with its three instants: queued when wk run began to wait, started when it
// took a place, ended when the line was written.
type timedRun struct {
	RunRecord
	queued, started, ended time.Time
	subdir, command        string
}

func (tr timedRun) failed() bool { return tr.Exit != "0" }

// parseRuns keeps the runs that ended inside r and counts the lines it could not read. A blank
// line is not counted: an interrupted append leaves one.
func parseRuns(lines []string, r Range) ([]timedRun, int) {
	var runs []timedRun
	malformed := 0
	for _, line := range lines {
		rr, ok := parseRun(line)
		switch {
		case strings.TrimSpace(line) == "":
		case !ok:
			malformed++
		case r.Contains(rr.Time):
			runs = append(runs, timed(rr))
		}
	}
	return runs, malformed
}

func timed(rr RunRecord) timedRun {
	started := rr.Time.Add(-seconds(rr.RunSeconds))
	return timedRun{RunRecord: rr, ended: rr.Time, started: started,
		queued: started.Add(-seconds(rr.WaitSeconds)), command: strings.Join(strings.Fields(rr.Command), " ")}
}

func seconds(s float64) time.Duration { return time.Duration(s * float64(time.Second)) }
