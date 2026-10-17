package slot

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
)

// Status is what `wk slots` prints (SB-18). Its JSON is the document `wk slots --json` gives
// the live proof's sampler.
type Status struct {
	MachineFile string        `json:"machine_file"`
	Found       bool          `json:"found"`
	Dir         string        `json:"dir"`
	Order       string        `json:"order"`
	Now         time.Time     `json:"now"`
	Classes     []ClassStatus `json:"classes"`
	Missed      LogSummary    `json:"missed"`
	Errors      LogSummary    `json:"errors"`
}

type ClassStatus struct {
	Name     string        `json:"name"`
	Capacity int           `json:"capacity"`
	Places   []PlaceStatus `json:"places"`
	Waiters  []Occupant    `json:"waiters"`
	LastRuns []RunRecord   `json:"last_runs"`
}

// PlaceStatus is one place of a class; Holder is nil while the place is free.
type PlaceStatus struct {
	Index  int       `json:"index"`
	Holder *Occupant `json:"holder"`
}

// Occupant is a run that holds a place or waits for one; Seconds is how long it has done so.
type Occupant struct {
	PID       int       `json:"pid"`
	Workspace string    `json:"workspace"`
	Command   string    `json:"command"`
	Since     time.Time `json:"since"`
	Seconds   float64   `json:"seconds"`
}

// RunRecord is one line of runs.log.
type RunRecord struct {
	Time        time.Time `json:"time"`
	Class       string    `json:"class"`
	PID         int       `json:"pid"`
	Exit        string    `json:"exit"`
	WaitSeconds float64   `json:"wait_seconds"`
	RunSeconds  float64   `json:"run_seconds"`
	Workspace   string    `json:"workspace"`
	Command     string    `json:"command"`
}

// LogSummary is a log's lines of the last 24 hours: how many, and the newest.
type LogSummary struct {
	Last24h int    `json:"last_24h"`
	Newest  string `json:"newest,omitempty"`
}

const lastRunsShown = 5

// ReadStatus reads the places, the queue and the logs under admit.lock (SB-18).
func ReadStatus(home string) (Status, error) {
	m, found, err := LoadMachine(home)
	if err != nil {
		return Status{}, err
	}
	s := Status{MachineFile: machinePath(home), Found: found, Dir: m.Dir, Order: m.Order, Now: time.Now(), Classes: []ClassStatus{}}
	runs, err := readRuns(filepath.Join(m.Dir, runsLog))
	if err != nil {
		return Status{}, err
	}
	for _, name := range m.classNames() {
		cs, err := readClass(m, name, s.Now)
		if err != nil {
			return Status{}, err
		}
		cs.LastRuns = lastRuns(runs, name)
		s.Classes = append(s.Classes, cs)
	}
	if s.Missed, err = summarizeLog(filepath.Join(m.Dir, "missed.log"), s.Now); err != nil {
		return Status{}, err
	}
	s.Errors, err = summarizeLog(filepath.Join(m.Dir, errorsLog), s.Now)
	return s, err
}

func readClass(m Machine, name string, now time.Time) (ClassStatus, error) {
	q := newQueue(m, name, nil)
	cs := ClassStatus{Name: name, Capacity: q.capacity, Places: make([]PlaceStatus, q.capacity), Waiters: []Occupant{}}
	for i := range cs.Places {
		cs.Places[i].Index = i
	}
	if _, err := os.Stat(q.dir); errors.Is(err, fs.ErrNotExist) {
		return cs, nil
	}
	err := q.withAdmit(func() error { return q.fill(&cs, now) })
	return cs, err
}

func (q queue) fill(cs *ClassStatus, now time.Time) error {
	for i := range cs.Places {
		held, err := heldElsewhere(q.placePath(i))
		if err != nil {
			return err
		}
		if held {
			o := readRecord(q.placePath(i)).occupant(now)
			cs.Places[i].Holder = &o
		}
	}
	tickets, err := q.tickets()
	for _, t := range tickets {
		held, lockErr := heldElsewhere(t.path)
		if lockErr != nil {
			return lockErr
		}
		if held {
			cs.Waiters = append(cs.Waiters, readRecord(t.path).occupant(now))
		}
	}
	return err
}

// heldElsewhere probes a lock with a shared, non-blocking flock on a file it opens read-only, so
// a status read neither creates a ticket nor removes a dead one: only waiters change the queue.
func heldElsewhere(path string) (bool, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	defer func() { _ = f.Close() }()
	err = unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB)
	if errors.Is(err, unix.EWOULDBLOCK) {
		return true, nil
	}
	return false, err
}

func (rec record) occupant(now time.Time) Occupant {
	seconds := now.Sub(rec.Start).Round(100 * time.Millisecond).Seconds()
	return Occupant{PID: rec.PID, Workspace: rec.Workspace, Command: rec.Command, Since: rec.Start, Seconds: seconds}
}

func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	var lines []string
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

func readRuns(path string) ([]RunRecord, error) {
	lines, err := readLines(path)
	var runs []RunRecord
	for _, line := range lines {
		if rr, ok := parseRun(line); ok {
			runs = append(runs, rr)
		}
	}
	return runs, err
}

// parseRun reads a line logRun wrote; a line it cannot read is left out of the status.
func parseRun(line string) (RunRecord, bool) {
	fields := strings.Split(line, "\t")
	at, err := time.Parse(logTimeLayout, fields[0])
	if err != nil {
		return RunRecord{}, false
	}
	rr := RunRecord{Time: at}
	for _, f := range fields[1:] {
		key, value, _ := strings.Cut(f, "=")
		rr.set(key, value)
	}
	return rr, rr.Class != ""
}

func (rr *RunRecord) set(key, value string) {
	switch key {
	case "class":
		rr.Class = value
	case "pid":
		rr.PID, _ = strconv.Atoi(value)
	case "exit":
		rr.Exit = value
	case "wait":
		rr.WaitSeconds = durationSeconds(value)
	case "run":
		rr.RunSeconds = durationSeconds(value)
	case "dir":
		rr.Workspace = value
	case "cmd":
		rr.Command = value
	}
}

func durationSeconds(s string) float64 {
	d, _ := time.ParseDuration(s)
	return d.Seconds()
}

func lastRuns(runs []RunRecord, class string) []RunRecord {
	out := []RunRecord{}
	for _, rr := range runs {
		if rr.Class == class {
			out = append(out, rr)
		}
	}
	return out[max(0, len(out)-lastRunsShown):]
}

// summarizeLog counts by the stamp appendLog puts first on every line; a line without one is
// not counted, since its age is unknown.
func summarizeLog(path string, now time.Time) (LogSummary, error) {
	lines, err := readLines(path)
	var sum LogSummary
	for _, line := range lines {
		stamp, _, _ := strings.Cut(line, "\t")
		at, perr := time.Parse(logTimeLayout, stamp)
		if perr == nil && now.Sub(at) <= 24*time.Hour {
			sum.Last24h++
			sum.Newest = line
		}
	}
	return sum, err
}

// Text is the status for a terminal. Every line names a place with its class, because "slot"
// already means a pool slot or a create slot in wk.
func (s Status) Text() string {
	var b strings.Builder
	if !s.Found {
		fmt.Fprintf(&b, "no machine file at %s: every wk run runs unslotted\n", s.MachineFile)
	} else {
		fmt.Fprintf(&b, "machine file %s: places under %s, order %s\n", s.MachineFile, s.Dir, s.Order)
	}
	for _, cs := range s.Classes {
		cs.text(&b)
	}
	fmt.Fprintf(&b, "missed.log: %s\n", s.Missed.text())
	fmt.Fprintf(&b, "errors.log: %s\n", s.Errors.text())
	return b.String()
}

func (cs ClassStatus) text(b *strings.Builder) {
	fmt.Fprintf(b, "%s: %s\n", cs.Name, plural(cs.Capacity, "place"))
	for _, p := range cs.Places {
		holder := "free"
		if p.Holder != nil {
			holder = p.Holder.text("for")
		}
		fmt.Fprintf(b, "  %s place %d: %s\n", cs.Name, p.Index, holder)
	}
	for i, w := range cs.Waiters {
		fmt.Fprintf(b, "  %s waiter %d: %s\n", cs.Name, i+1, w.text("waiting"))
	}
	if len(cs.LastRuns) > 0 {
		fmt.Fprintf(b, "  last %s runs:\n", cs.Name)
	}
	for _, rr := range cs.LastRuns {
		fmt.Fprintf(b, "    %s exit %s, waited %.1fs, ran %.1fs, pid %d in %s: %s\n",
			rr.Time.Format(logTimeLayout), rr.Exit, rr.WaitSeconds, rr.RunSeconds, rr.PID, rr.Workspace, rr.Command)
	}
}

func (o Occupant) text(how string) string {
	if o.PID == 0 {
		return "an unknown run"
	}
	age := time.Duration(o.Seconds * float64(time.Second)).Round(time.Second)
	return fmt.Sprintf("pid %d in %s, %s, %s %s", o.PID, o.Workspace, o.Command, how, age)
}

func (l LogSummary) text() string {
	if l.Last24h == 0 {
		return "no lines in the last 24 hours"
	}
	return fmt.Sprintf("%s in the last 24 hours, newest: %s", plural(l.Last24h, "line"), l.Newest)
}

func plural(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}
