package hive

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zach-short/landing-layer/internal/wk"
)

// Loader reads the current verdicts. wk hive passes the dry-run report, so the view can only
// read: nothing in this package removes or changes a worktree.
type Loader func() (wk.Report, error)

// Model is the bubbletea model of the terminal hive.
type Model struct {
	load     Loader
	every    time.Duration
	now      func() time.Time
	report   wk.Report
	loaded   bool
	err      error
	selected int
}

type reportMsg struct {
	report wk.Report
	err    error
}

type tickMsg time.Time

// New returns a model that reads load once at start and again every interval after each read
// lands, so a slow read never stacks reads behind it.
func New(load Loader, every time.Duration) Model {
	return Model{load: load, every: every, now: time.Now}
}

func (m Model) Init() tea.Cmd { return m.fetch() }

func (m Model) fetch() tea.Cmd {
	load := m.load
	return func() tea.Msg {
		r, err := load()
		return reportMsg{report: r, err: err}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case reportMsg:
		if msg.err != nil {
			m.err = msg.err
		} else {
			m.report, m.err, m.loaded = msg.report, nil, true
			if n := len(m.report.Worktrees); m.selected >= n {
				m.selected = max(n-1, 0)
			}
		}
		return m, tea.Tick(m.every, func(t time.Time) tea.Msg { return tickMsg(t) })
	case tickMsg:
		return m, m.fetch()
	case tea.KeyMsg:
		n := len(m.report.Worktrees)
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case "right", "l", "down", "j", "tab":
			if n > 0 {
				m.selected = (m.selected + 1) % n
			}
		case "left", "h", "up", "k", "shift+tab":
			if n > 0 {
				m.selected = (m.selected - 1 + n) % n
			}
		}
	}
	return m, nil
}

const (
	cellWidth = 11 // a cell is two lines of this many columns
	halfStep  = 6  // half the distance between neighbours in a row, so rows interlock
	rowStep   = 3  // two lines of cell and one of gap
)

var (
	title    = lipgloss.NewStyle().Bold(true)
	dim      = lipgloss.NewStyle().Foreground(muted)
	errStyle = lipgloss.NewStyle().Foreground(palette[Land]).Bold(true)
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(title.Render("wk hive"))
	if m.loaded {
		b.WriteString(dim.Render(fmt.Sprintf("   base %s · updated %s · every %s · read-only",
			m.report.Base, ago(m.now(), m.report.Generated), m.every)))
	}
	b.WriteString("\n")
	if m.err != nil {
		b.WriteString(errStyle.Render("wk hive: "+m.err.Error()) + "\n")
	}
	if !m.loaded {
		if m.err == nil {
			b.WriteString(dim.Render("reading worktrees...") + "\n")
		}
		b.WriteString("\n" + dim.Render("q quit") + "\n")
		return b.String()
	}

	b.WriteString(m.legend() + "\n\n")
	b.WriteString(m.honeycomb() + "\n\n")
	b.WriteString(m.detail())
	b.WriteString("\n" + dim.Render("←/→ select · q quit") + "\n")
	return b.String()
}

func (m Model) legend() string {
	counts := map[Key]int{}
	for _, it := range m.report.Worktrees {
		counts[StyleOf(it)]++
	}
	var parts []string
	for _, k := range Keys {
		dot := lipgloss.NewStyle().Foreground(palette[k]).Render("⬢")
		part := fmt.Sprintf("%s %d %s", dot, counts[k], k.Label())
		if counts[k] == 0 {
			part = dim.Render(fmt.Sprintf("⬡ 0 %s", k.Label()))
		}
		parts = append(parts, part)
	}
	return strings.Join(parts, "   ")
}

// placed is one cell's two lines at a column of the honeycomb canvas.
type placed struct {
	x, y       int
	top, under string
}

// honeycomb lays the cells out like the page: the hub in the middle and worktrees ring by ring.
// Axial (q, r) maps to column (2q+r)*halfStep and line r*rowStep, so each row sits half a cell
// across from the next, the way hexagons interlock.
func (m Model) honeycomb() string {
	hub := lipgloss.NewStyle().Background(hubFill).Foreground(hubInk).Bold(true).Width(cellWidth).Align(lipgloss.Center)
	cells := []placed{{x: 0, y: 0, top: hub.Render(clip(m.report.Base, cellWidth-2)), under: hub.Render("base")}}
	for i, s := range Slots(len(m.report.Worktrees)) {
		it := m.report.Worktrees[i]
		k := StyleOf(it)
		st := cellStyle(k)
		if i == m.selected {
			st = st.Bold(true).Underline(true)
		}
		cells = append(cells, placed{
			x: (2*s.Q + s.R) * halfStep, y: s.R * rowStep,
			top: st.Render(clip(name(it), cellWidth-2)), under: st.Render(k.Short()),
		})
	}

	minX, minY, maxY := 0, 0, 0
	for _, c := range cells {
		minX, minY, maxY = min(minX, c.x), min(minY, c.y), max(maxY, c.y+1)
	}
	type seg struct {
		x int
		s string
	}
	lines := make([][]seg, maxY-minY+1)
	for _, c := range cells {
		lines[c.y-minY] = append(lines[c.y-minY], seg{c.x - minX, c.top})
		lines[c.y-minY+1] = append(lines[c.y-minY+1], seg{c.x - minX, c.under})
	}
	out := make([]string, len(lines))
	for i, segs := range lines {
		sort.Slice(segs, func(a, b int) bool { return segs[a].x < segs[b].x })
		var line strings.Builder
		col := 0
		for _, s := range segs {
			line.WriteString(strings.Repeat(" ", max(s.x-col, 0)))
			line.WriteString(s.s)
			col = s.x + cellWidth
		}
		out[i] = "  " + line.String()
	}
	return strings.Join(out, "\n")
}

func (m Model) detail() string {
	if len(m.report.Worktrees) == 0 {
		return dim.Render("No worktrees yet. wk new <row> makes a warm one.") + "\n"
	}
	it := m.report.Worktrees[m.selected]
	k := StyleOf(it)
	branch := it.Branch
	if branch == "" {
		branch = "detached"
	}
	var b strings.Builder
	b.WriteString(title.Render(it.Path) + "  " + lipgloss.NewStyle().Foreground(palette[k]).Render("⬢ "+k.Label()) + "\n")
	b.WriteString(dim.Render(fmt.Sprintf("branch %s · head %s · %d changed · activity %s",
		branch, it.Head, it.Dirty, ago(m.now(), it.LastActivity))) + "\n")
	for _, r := range it.Reasons {
		b.WriteString("  • " + r + "\n")
	}
	return b.String()
}

func name(it wk.ReportItem) string {
	if it.Branch != "" {
		return it.Branch
	}
	return it.Path[strings.LastIndex(it.Path, "/")+1:]
}

func clip(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// ago reads like the page: seconds, minutes, hours, then days.
func ago(now time.Time, stamp string) string {
	t, err := time.Parse(time.RFC3339, stamp)
	if stamp == "" || err != nil {
		return "unknown"
	}
	s := max(now.Sub(t).Seconds(), 0)
	switch {
	case s < 90:
		return fmt.Sprintf("%ds ago", int(s+0.5))
	case s < 5400:
		return fmt.Sprintf("%d min ago", int(s/60+0.5))
	case s < 172800:
		return fmt.Sprintf("%d h ago", int(s/3600+0.5))
	}
	return fmt.Sprintf("%d days ago", int(s/86400+0.5))
}
