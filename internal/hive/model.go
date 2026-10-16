package hive

import (
	"fmt"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/zach-short/wk/internal/wk"
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
	reads    int // successful reads, so the live mark beats once per read
	width    int // the terminal's width; 0 until the first resize, and then nothing is centered
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
			m.reads++
			if n := len(m.report.Worktrees); m.selected >= n {
				m.selected = max(n-1, 0)
			}
		}
		return m, tea.Tick(m.every, func(t time.Time) tea.Msg { return tickMsg(t) })
	case tickMsg:
		return m, m.fetch()
	case tea.WindowSizeMsg:
		m.width = msg.Width
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
	cellWidth   = 11 // a cell is two lines of this many columns
	halfStep    = 6  // half the distance between neighbours in a row, so rows interlock
	rowStep     = 3  // two lines of cell and one of gap
	detailWidth = 76 // the detail card's outer width when the terminal has room for it
)

var (
	brand    = lipgloss.NewStyle().Foreground(hubInk).Bold(true)
	title    = lipgloss.NewStyle().Bold(true)
	dim      = lipgloss.NewStyle().Foreground(muted)
	faint    = lipgloss.NewStyle().Foreground(rule)
	keycap   = lipgloss.NewStyle().Foreground(hubInk).Bold(true)
	errStyle = lipgloss.NewStyle().Foreground(palette[Land]).Bold(true)
)

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.header() + "\n")
	if m.err != nil {
		b.WriteString(m.center(errStyle.Render("✖ wk hive: "+m.err.Error())) + "\n")
	}
	if !m.loaded {
		if m.err == nil {
			b.WriteString("\n" + m.center(dim.Render("⬡ reading worktrees...")) + "\n")
		}
		b.WriteString("\n" + m.center(m.keys(false)) + "\n")
		return b.String()
	}

	b.WriteString("\n" + m.center(m.legend()) + "\n\n")
	b.WriteString(m.center(m.honeycomb()) + "\n\n")
	b.WriteString(m.center(m.detail()) + "\n")
	b.WriteString("\n" + m.center(m.keys(len(m.report.Worktrees) > 0)) + "\n")
	return b.String()
}

// header is the brand on the left and the read's state on the right, over a rule. With no
// width yet the two sides sit next to each other.
func (m Model) header() string {
	left := brand.Render("⬢ wk hive")
	mark := lipgloss.NewStyle().Foreground(palette[Working])
	beat := "●"
	if m.reads%2 == 0 {
		beat = "◉"
	}
	if m.err != nil {
		mark, beat = errStyle, "●"
	}
	right := mark.Render(beat + " live")
	if m.loaded {
		right = dim.Render(fmt.Sprintf("base %s · updated %s · every %s · read-only  ",
			m.report.Base, ago(m.now(), m.report.Generated), m.every)) + right
	}
	if m.width == 0 {
		return left + "   " + right
	}
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right)-2, 1)
	line := " " + left + strings.Repeat(" ", gap) + right
	return line + "\n" + faint.Render(strings.Repeat("─", m.width))
}

// center places a block in the middle of the terminal, keeping its lines aligned to each other.
func (m Model) center(s string) string {
	if m.width == 0 || lipgloss.Width(s) >= m.width {
		return s
	}
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, s)
}

// legend is one pill per style: filled with its count when the hive holds any, an outline when
// it holds none.
func (m Model) legend() string {
	counts := map[Key]int{}
	for _, it := range m.report.Worktrees {
		counts[StyleOf(it)]++
	}
	var parts []string
	for _, k := range Keys {
		if counts[k] == 0 {
			parts = append(parts, faint.Render("⬡ 0 "+k.Label()))
			continue
		}
		edge := lipgloss.NewStyle().Foreground(palette[k])
		pill := cellStyle(k).Width(0)
		parts = append(parts, edge.Render("▐")+pill.Render(fmt.Sprintf("%d %s", counts[k], k.Label()))+edge.Render("▌"))
	}
	return strings.Join(parts, "  ")
}

// placed is one cell's two lines at a column of the honeycomb canvas; w is how many columns
// the cell takes, which is wider for the framed selection.
type placed struct {
	x, y, w    int
	top, under string
}

// honeycomb lays the cells out like the page: the hub in the middle and worktrees ring by ring.
// Axial (q, r) maps to column (2q+r)*halfStep and line r*rowStep, so each row sits half a cell
// across from the next, the way hexagons interlock. The selected cell takes the one-column gaps
// at its sides for a frame.
func (m Model) honeycomb() string {
	hub := lipgloss.NewStyle().Background(hubFill).Foreground(hubInk).Bold(true).Width(cellWidth).Align(lipgloss.Center)
	hubSub := hub.Bold(false).Foreground(muted)
	cells := []placed{{x: 0, y: 0, w: cellWidth, top: hub.Render(clip(m.report.Base, cellWidth-2)), under: hubSub.Render("⬢ base")}}
	frame := lipgloss.NewStyle().Foreground(hubInk)
	for i, s := range Slots(len(m.report.Worktrees)) {
		it := m.report.Worktrees[i]
		k := StyleOf(it)
		st := cellStyle(k)
		c := placed{x: (2*s.Q + s.R) * halfStep, y: s.R * rowStep, w: cellWidth}
		if i == m.selected {
			st = st.Bold(true)
			c.x, c.w = c.x-1, cellWidth+2
			c.top = frame.Render("▐") + st.Render(clip(name(it), cellWidth-2)) + frame.Render("▌")
			c.under = frame.Render("▐") + st.Render(k.Short()) + frame.Render("▌")
		} else {
			c.top, c.under = st.Render(clip(name(it), cellWidth-2)), st.Faint(k == Removable || k == Prunable).Render(k.Short())
		}
		cells = append(cells, c)
	}

	minX, minY, maxY := 0, 0, 0
	for _, c := range cells {
		minX, minY, maxY = min(minX, c.x), min(minY, c.y), max(maxY, c.y+1)
	}
	type seg struct {
		x, w int
		s    string
	}
	lines := make([][]seg, maxY-minY+1)
	for _, c := range cells {
		lines[c.y-minY] = append(lines[c.y-minY], seg{c.x - minX, c.w, c.top})
		lines[c.y-minY+1] = append(lines[c.y-minY+1], seg{c.x - minX, c.w, c.under})
	}
	out := make([]string, len(lines))
	for i, segs := range lines {
		sort.Slice(segs, func(a, b int) bool { return segs[a].x < segs[b].x })
		var line strings.Builder
		col := 0
		for _, s := range segs {
			line.WriteString(strings.Repeat(" ", max(s.x-col, 0)))
			line.WriteString(s.s)
			col = s.x + s.w
		}
		out[i] = "  " + line.String()
	}
	return strings.Join(out, "\n")
}

// detail is a card for the selected worktree, edged in its verdict's color.
func (m Model) detail() string {
	if len(m.report.Worktrees) == 0 {
		return dim.Render("No worktrees yet. wk new <row> makes a warm one.")
	}
	it := m.report.Worktrees[m.selected]
	k := StyleOf(it)
	color := lipgloss.NewStyle().Foreground(palette[k])
	branch := it.Branch
	if branch == "" {
		branch = "detached"
	}

	width := detailWidth
	if m.width > 0 {
		width = min(width, m.width-2)
	}
	inner := width - 4 // two border columns and one column of padding each side

	path, badge := title.Render(clip(it.Path, inner-lipgloss.Width(k.Label())-4)), color.Bold(true).Render("⬢ "+k.Label())
	head := path + strings.Repeat(" ", max(inner-lipgloss.Width(path)-lipgloss.Width(badge), 1)) + badge

	field := func(label, value string) string { return dim.Render(label+" ") + value }
	facts := strings.Join([]string{
		field("branch", branch), field("head", it.Head),
		field("changed", fmt.Sprint(it.Dirty)), field("activity", ago(m.now(), it.LastActivity)),
	}, faint.Render("  │  "))

	var b strings.Builder
	b.WriteString(head + "\n" + facts)
	if len(it.Reasons) > 0 {
		b.WriteString("\n" + faint.Render(strings.Repeat("─", inner)))
	}
	reason := lipgloss.NewStyle().Width(inner - 2)
	for _, r := range it.Reasons {
		b.WriteString("\n" + lipgloss.JoinHorizontal(lipgloss.Top, color.Render("› "), reason.Render(r)))
	}
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(palette[k]).
		Padding(0, 1).
		Width(width - 2).
		Render(b.String())
}

// keys is the footer of key hints; the selection keys show only when there is a cell to move to.
func (m Model) keys(selectable bool) string {
	var hints []string
	if selectable {
		hints = append(hints, keycap.Render("←/→")+dim.Render(" select"))
	}
	hints = append(hints, keycap.Render("q")+dim.Render(" quit"))
	return strings.Join(hints, faint.Render("  ·  "))
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
