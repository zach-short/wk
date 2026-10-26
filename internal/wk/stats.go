package wk

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/slot"
)

// Stats is `wk stats`: how the places of the machine and the warm creates of every repository
// performed over one date range, read from wk's own logs.
type Stats struct {
	Slots slot.Stats  `json:"slots"`
	Warm  []WarmStats `json:"warm"`
}

// WarmStats is one repository's warm creates from hook.log, split by where the workspace came
// from: a ready pool slot, a clone of the template, or Unattributed when the log cannot say.
type WarmStats struct {
	Repo         string            `json:"repo"`
	Pool         slot.Distribution `json:"pool"`
	Template     slot.Distribution `json:"template"`
	Unattributed slot.Distribution `json:"unattributed"`
}

func ReadStats(home string, r slot.Range) (Stats, error) {
	s, err := slot.ReadStats(home, r)
	if err != nil {
		return Stats{}, err
	}
	warm, err := readWarmStats(home, r)
	return Stats{Slots: s, Warm: warm}, err
}

func readWarmStats(home string, r slot.Range) ([]WarmStats, error) {
	logs, err := filepath.Glob(filepath.Join(home, "*", "hook.log"))
	if err != nil {
		return nil, err
	}
	sort.Strings(logs)
	out := []WarmStats{}
	for _, path := range logs {
		w, err := readWarm(path, r)
		if err != nil {
			return nil, err
		}
		if len(w.pool)+len(w.template)+len(w.unattributed) > 0 {
			out = append(out, WarmStats{Repo: filepath.Base(filepath.Dir(path)), Pool: slot.Distribute(w.pool),
				Template: slot.Distribute(w.template), Unattributed: slot.Distribute(w.unattributed)})
		}
	}
	return out, nil
}

var (
	createLineRE = regexp.MustCompile(`^(\S+) WorktreeCreate name=(\S+)`)
	warmLineRE   = regexp.MustCompile(`^(\S+) warm (\S.*) in ([0-9.]+)s$`)
	// endLineRE is every other way a create ends with its path: hook.go's "reused", "plain
	// fallback" and "cold after a partial warm failure".
	endLineRE  = regexp.MustCompile(`^\S+ (?:reused|plain fallback|cold after a partial warm failure) (\S.*) in [0-9.]+s$`)
	failLineRE = regexp.MustCompile(`^\S+ failed after `)
)

// The lines create writes for each source. "pool: no ready sparse slot" also names the pool but
// is followed by a template clone, so a bare "pool" is not enough.
const (
	poolClaim     = "claim pool slot"
	templateClone = "worktree add --no-checkout"
)

// staleCreate drops a create that never wrote an end line, such as a hook that was killed, so it
// does not mark every later create as crossed. The slowest warm create seen took under a minute.
const staleCreate = 10 * time.Minute

// openCreate is one create between its WorktreeCreate line and its end line. crossed is set when
// another create was open at any moment of that span: the pool and template lines carry no
// create name, so the lines of crossed creates cannot be told apart.
type openCreate struct {
	at                      time.Time
	crossed, pool, template bool
}

type warmTimes struct{ pool, template, unattributed []float64 }

// warmScan matches each warm line to its WorktreeCreate line by name, the base name of the warm
// path. A warm create counts as pool or template only when no other create was open between its
// two lines and exactly one of the claim line and the template clone line came between them.
// Every other warm line, including one whose WorktreeCreate line is missing, is unattributed.
type warmScan struct {
	r     slot.Range
	open  map[string]*openCreate
	times warmTimes
}

func readWarm(path string, r slot.Range) (warmTimes, error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return warmTimes{}, nil
	}
	if err != nil {
		return warmTimes{}, err
	}
	defer func() { _ = f.Close() }()
	w := warmScan{r: r, open: map[string]*openCreate{}}
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		w.line(sc.Text())
	}
	return w.times, sc.Err()
}

func (w *warmScan) line(line string) {
	if m := createLineRE.FindStringSubmatch(line); m != nil {
		w.start(m[1], m[2])
		return
	}
	if m := endLineRE.FindStringSubmatch(line); m != nil {
		delete(w.open, filepath.Base(m[1]))
		return
	}
	// A failure line names no create; it can only be placed when one create is open.
	if failLineRE.MatchString(line) && len(w.open) == 1 {
		clear(w.open)
		return
	}
	w.evidence(strings.Contains(line, poolClaim), strings.Contains(line, templateClone))
	if at, name, sec, ok := warmLine(line); ok {
		w.warm(at, name, sec)
	}
}

func (w *warmScan) start(stamp, name string) {
	at, _ := time.Parse(time.RFC3339, stamp)
	for n, c := range w.open {
		if at.Sub(c.at) > staleCreate {
			delete(w.open, n)
		}
	}
	for _, c := range w.open {
		c.crossed = true
	}
	w.open[name] = &openCreate{at: at, crossed: len(w.open) > 0}
}

func (w *warmScan) evidence(pool, template bool) {
	for _, c := range w.open {
		c.pool = c.pool || pool
		c.template = c.template || template
	}
}

func (w *warmScan) warm(at time.Time, name string, sec float64) {
	c := w.open[name]
	delete(w.open, name)
	if !w.r.Contains(at) {
		return
	}
	switch {
	case c == nil || c.crossed || c.pool == c.template:
		w.times.unattributed = append(w.times.unattributed, sec)
	case c.pool:
		w.times.pool = append(w.times.pool, sec)
	default:
		w.times.template = append(w.times.template, sec)
	}
}

func warmLine(line string) (at time.Time, name string, sec float64, ok bool) {
	m := warmLineRE.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, "", 0, false
	}
	at, err := time.Parse(time.RFC3339, m[1])
	sec, serr := strconv.ParseFloat(m[3], 64)
	return at, filepath.Base(m[2]), sec, err == nil && serr == nil
}

func (s Stats) Text() string {
	var b strings.Builder
	b.WriteString(s.Slots.Text())
	b.WriteString("\nwarm creates in hook.log. Unattributed creates came from the pool or the template, but the log\n" +
		"cannot separate them: another create was open at the same time, or the log has neither source\n" +
		"line or both. So a low pool or template count does not mean few such creates happened.\n")
	if len(s.Warm) == 0 {
		b.WriteString("  none\n")
	}
	for _, w := range s.Warm {
		fmt.Fprintf(&b, "  %s\n    pool:         %s\n    template:     %s\n    unattributed: %s\n",
			w.Repo, w.attributed(w.Pool), w.attributed(w.Template), w.Unattributed.Text())
	}
	return b.String()
}

// attributed says "none attributed" instead of "none" when some creates are unattributed, since
// any of them may have come from this source.
func (w WarmStats) attributed(d slot.Distribution) string {
	if d.Count == 0 && w.Unattributed.Count > 0 {
		return "none attributed"
	}
	return d.Text()
}
