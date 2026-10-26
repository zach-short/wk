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
// from: a ready pool slot, or a clone of the template.
type WarmStats struct {
	Repo     string            `json:"repo"`
	Pool     slot.Distribution `json:"pool"`
	Template slot.Distribution `json:"template"`
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
		pool, template, err := readWarm(path, r)
		if err != nil {
			return nil, err
		}
		if len(pool)+len(template) > 0 {
			out = append(out, WarmStats{Repo: filepath.Base(filepath.Dir(path)), Pool: slot.Distribute(pool), Template: slot.Distribute(template)})
		}
	}
	return out, nil
}

var (
	createLineRE = regexp.MustCompile(`^\S+ WorktreeCreate `)
	warmLineRE   = regexp.MustCompile(`^(\S+) warm \S.* in ([0-9.]+)s$`)
)

// poolClaim is the line create writes when it takes a pool slot. "pool: no ready sparse slot"
// also names the pool but is followed by a template clone, so a bare "pool" is not enough.
const poolClaim = "claim pool slot"

// readWarm reads the seconds of each warm create inside r. A create counts as from the pool when
// a claim line comes between its WorktreeCreate line and its warm line. Two hooks that run at
// once can interleave their lines; the warm line then takes the claim of the nearer create.
func readWarm(path string, r slot.Range) (pool, template []float64, err error) {
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = f.Close() }()
	claimed := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 64*1024), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case createLineRE.MatchString(line):
			claimed = false
		case strings.Contains(line, poolClaim):
			claimed = true
		default:
			at, sec, ok := warmLine(line)
			if !ok || !r.Contains(at) {
				continue
			}
			if claimed {
				pool = append(pool, sec)
			} else {
				template = append(template, sec)
			}
		}
	}
	return pool, template, sc.Err()
}

func warmLine(line string) (time.Time, float64, bool) {
	m := warmLineRE.FindStringSubmatch(line)
	if m == nil {
		return time.Time{}, 0, false
	}
	at, err := time.Parse(time.RFC3339, m[1])
	sec, serr := strconv.ParseFloat(m[2], 64)
	return at, sec, err == nil && serr == nil
}

func (s Stats) Text() string {
	var b strings.Builder
	b.WriteString(s.Slots.Text())
	b.WriteString("\nwarm creates in hook.log:\n")
	if len(s.Warm) == 0 {
		b.WriteString("  none\n")
	}
	for _, w := range s.Warm {
		fmt.Fprintf(&b, "  %s\n    pool:     %s\n    template: %s\n", w.Repo, w.Pool.Text(), w.Template.Text())
	}
	return b.String()
}
