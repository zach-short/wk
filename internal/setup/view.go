package setup

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/zach-short/wk/internal/theme"
)

var (
	brand   = lipgloss.NewStyle().Foreground(theme.HubInk).Bold(true)
	bold    = lipgloss.NewStyle().Bold(true)
	dim     = lipgloss.NewStyle().Foreground(theme.Muted)
	faint   = lipgloss.NewStyle().Foreground(theme.Rule)
	ink     = lipgloss.NewStyle().Foreground(theme.HubInk)
	good    = lipgloss.NewStyle().Foreground(theme.Green)
	bad     = lipgloss.NewStyle().Foreground(theme.Rose)
	keyName = lipgloss.NewStyle().Foreground(theme.Blue)
	value   = lipgloss.NewStyle().Foreground(theme.Amber)
)

const (
	railWidth  = 20
	railHeight = 16 // one line of padding, then eight steps and the seven lines joining them
	panelWidth = 64 // the panel's outer width when the terminal has room
)

var spinner = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

func (m Model) View() string {
	var body string
	if m.step == stepHome {
		body = m.panel(m.home())
	} else {
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.rail(), "  ", m.panel(m.content()))
	}
	return m.header() + "\n\n" + m.center(body) + "\n\n" + m.center(m.keys()) + "\n"
}

func (m Model) header() string {
	left := brand.Render("⬢ wk") + dim.Render(" · setup")
	if m.step == stepHome {
		left = brand.Render("⬢ wk") + dim.Render(" · home")
	}
	right := bold.Render(m.plan.Repo) + dim.Render("  "+tilde(m.plan.Primary))
	if m.width == 0 {
		return left + "   " + right
	}
	gap := max(m.width-lipgloss.Width(left)-lipgloss.Width(right)-2, 1)
	return " " + left + strings.Repeat(" ", gap) + right + "\n" + faint.Render(strings.Repeat("─", m.width))
}

func (m Model) center(s string) string {
	if m.width == 0 || lipgloss.Width(s) >= m.width {
		return s
	}
	return lipgloss.PlaceHorizontal(m.width, lipgloss.Center, s)
}

// rail lists the steps with a filled hex for each one done, the current one lit, and a line
// joining them.
func (m Model) rail() string {
	failedRun := m.failure() != nil
	var lines []string
	for i, r := range rail {
		done := m.step > r.step && m.step != stepHome
		var icon, label string
		switch {
		case r.step == stepRun && failedRun:
			icon, label = bad.Render("✖"), bad.Render(r.label)
		case done:
			icon, label = good.Render("⬢"), dim.Render(r.label)
		case m.step == r.step:
			icon, label = ink.Render("⬢"), brand.Render(r.label)
		default:
			icon, label = faint.Render("⬡"), faint.Render(r.label)
		}
		lines = append(lines, " "+icon+"  "+label)
		if i < len(rail)-1 {
			join := faint
			if done {
				join = good
			}
			lines = append(lines, " "+join.Render("│"))
		}
	}
	return lipgloss.NewStyle().Width(railWidth).PaddingTop(1).Render(strings.Join(lines, "\n"))
}

// inner is the panel's text width: its outer width less two border and four padding columns.
func (m Model) inner() int {
	w := panelWidth
	if m.width > 0 {
		avail := m.width - 4
		if m.step != stepHome {
			avail -= railWidth + 2
		}
		w = max(min(w, avail), 36)
	}
	return w - 6
}

func (m Model) panel(s string) string {
	edge := theme.HubInk
	switch {
	case m.failure() != nil:
		edge = theme.Rose
	case m.step == stepDone:
		edge = theme.Green
	}
	st := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(edge).
		Padding(1, 2).
		Width(m.inner() + 4)
	if m.step != stepHome {
		st = st.Height(railHeight - 1) // a little taller than the rail, so the box keeps its size from step to step
	}
	return st.Render(s)
}

// wrap fits prose to the panel.
func (m Model) wrap(st lipgloss.Style, s string) string {
	return st.Width(m.inner()).Render(s)
}

// heading is a step's title with its place in the wizard.
func (m Model) heading(title string) string {
	n := int(m.step) // the welcome step is not counted
	count := dim.Render(fmt.Sprintf("step %d of %d", n, len(rail)-2))
	gap := max(m.inner()-lipgloss.Width(title)-lipgloss.Width(count), 1)
	return bold.Render(title) + strings.Repeat(" ", gap) + count
}

func (m Model) content() string {
	switch m.step {
	case stepWelcome:
		return m.welcome()
	case stepBase:
		return m.heading("Base branch") + "\n\n" +
			m.wrap(dim, "New workspaces start from this branch, the template follows it, and wk gc counts work as landed once it is here.") +
			"\n\n" + m.radio(m.plan.Found.Branches, m.base)
	case stepClone:
		return m.heading("Dependencies to clone") + "\n\n" +
			m.wrap(dim, "Each workspace gets these folders as copy-on-write clones of the template's, which take no time and almost no disk.") +
			"\n\n" + m.checks(m.clone, "wk found no dependency folders. You can add clone_dirs to the config later.")
	case stepCopy:
		return m.heading("Local files to copy") + "\n\n" +
			m.wrap(dim, "Files git ignores never reach a new worktree. wk copies these in from the main checkout and never prints them.") +
			"\n\n" + m.checks(m.copy, "wk found no .env files. You can add copy_files to the config later.")
	case stepInstall:
		return m.heading("Install command") + "\n\n" +
			m.wrap(dim, "The template runs this after each refresh. New workspaces clone its result, so they never install.") +
			"\n\n" + m.checks(m.install, "wk found no lockfile. You can add template.post_clone to the config later.")
	case stepExtras:
		return m.heading("Options") + "\n\n" + m.checks(m.extras, "")
	case stepReview:
		return m.review()
	case stepRun:
		return m.running()
	case stepDone:
		return m.done()
	}
	return ""
}

// logo is a small honeycomb in the verdict colors, the hub in the middle.
func logo(lit bool) string {
	c := func(col lipgloss.AdaptiveColor, glyph string) string {
		if !lit {
			col = theme.Rule
		}
		return lipgloss.NewStyle().Foreground(col).Render(glyph)
	}
	return "  " + c(theme.Green, "⬢") + " " + c(theme.Amber, "⬢") + "\n" +
		c(theme.Blue, "⬢") + " " + ink.Render("⬡") + " " + c(theme.Rose, "⬢") + "\n" +
		"  " + c(theme.Slate, "⬢") + " " + c(theme.Green, "⬢")
}

func (m Model) welcome() string {
	title := lipgloss.JoinVertical(lipgloss.Left, bold.Render("Warm workspaces for "+m.plan.Repo), dim.Render("parallel agents, ready in seconds"))
	top := lipgloss.JoinHorizontal(lipgloss.Center, logo(true), "    ", title)
	return top + "\n\n" +
		m.wrap(lipgloss.NewStyle(), "Each workspace is a git worktree whose dependencies are copy-on-write clones of one warm template, so an agent can start work at once.") +
		"\n\n" +
		m.wrap(dim, "This setup reads your repository and writes "+tilde(m.plan.ConfigPath)+". Nothing changes until the last step.") +
		"\n\n" + ink.Render("Press enter to start.")
}

func (m Model) radio(items []string, chosen int) string {
	var rows []string
	for i, it := range items {
		mark, text := faint.Render("○"), it
		if i == chosen {
			mark, text = ink.Render("◉"), brand.Render(it)
		}
		rows = append(rows, m.pointer(i)+mark+" "+text)
	}
	return strings.Join(rows, "\n")
}

func (m Model) checks(opts []Option, empty string) string {
	if len(opts) == 0 {
		return m.wrap(dim, empty)
	}
	var rows []string
	for i, o := range opts {
		mark, text := faint.Render("◇"), dim.Render(o.Value)
		if o.On {
			mark, text = good.Render("◆"), o.Value
		}
		row := m.pointer(i) + mark + " " + text
		if note := clip(o.Note, m.inner()-lipgloss.Width(row)-2); note != "" {
			row += "  " + dim.Render(note)
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func (m Model) pointer(i int) string {
	if m.cursor[m.step] == i {
		return ink.Render("› ")
	}
	return "  "
}

// review shows the file setup will write, colored like an editor would, and scrolls when the
// terminal is short.
func (m Model) review() string {
	text := Render(m.plan.Repo, m.Choices())
	lines := strings.Split(strings.TrimRight(text, "\n"), "\n")
	room := len(lines)
	if m.height > 0 {
		room = max(m.height-16, 6)
	}
	scroll := min(m.scroll, max(len(lines)-room, 0))
	shown := lines[scroll:min(scroll+room, len(lines))]
	for i, l := range shown {
		shown[i] = highlight(clip(l, m.inner()-2))
	}
	more := ""
	if len(lines) > room {
		more = "\n" + dim.Render(fmt.Sprintf("lines %d-%d of %d", scroll+1, scroll+len(shown), len(lines)))
	}
	code := lipgloss.NewStyle().Border(lipgloss.NormalBorder(), false, false, false, true).BorderForeground(theme.Rule).PaddingLeft(1).Render(strings.Join(shown, "\n"))
	return m.heading("Review") + "\n" + dim.Render(tilde(m.plan.ConfigPath)) + "\n\n" + code + more
}

func highlight(l string) string {
	t := strings.TrimSpace(l)
	switch {
	case t == "":
		return l
	case strings.HasPrefix(t, "#"):
		return dim.Italic(true).Render(l)
	case strings.HasPrefix(t, "["):
		return brand.Render(l)
	}
	if k, v, ok := strings.Cut(l, " = "); ok {
		return keyName.Render(k) + faint.Render(" = ") + value.Render(v)
	}
	return l
}

func (m Model) running() string {
	title := "Setting up"
	if m.plan.Existing {
		title = "Refreshing"
	}
	var b strings.Builder
	b.WriteString(bold.Render(title) + "\n\n")
	for _, t := range m.tasks {
		var row string
		switch t.state {
		case pending:
			row = dim.Render("◇ " + t.name)
		case running:
			row = ink.Render(spinner[m.frame%len(spinner)]+" ") + t.name + dim.Render(fmt.Sprintf("  %s", seconds(m.now().Sub(m.started))))
		case succeeded:
			row = good.Render("◆ ") + t.name + dim.Render("  "+seconds(t.took))
		case failed:
			row = bad.Render("✖ " + t.name)
		}
		b.WriteString(row + "\n")
		if t.state == running {
			for _, l := range m.log {
				b.WriteString(ink.Render("│ ") + dim.Render(clip(l, m.inner()-2)) + "\n")
			}
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m Model) done() string {
	if t := m.failure(); t != nil {
		var b strings.Builder
		b.WriteString(bad.Bold(true).Render("✖ "+t.name+" failed") + "\n\n")
		b.WriteString(m.wrap(lipgloss.NewStyle(), t.err.Error()) + "\n\n")
		if m.tasks[0].state == succeeded {
			b.WriteString(m.wrap(dim, "The config is written. Fix the cause, then run wk template refresh.") + "\n")
		} else {
			b.WriteString(m.wrap(dim, "Nothing changed.") + "\n")
		}
		b.WriteString("\n" + m.choices([]string{"Open the hive", "Quit"}))
		return b.String()
	}
	title := lipgloss.JoinVertical(lipgloss.Left, good.Bold(true).Render("The hive is ready"), dim.Render(m.plan.Repo+" is set up for warm workspaces"))
	top := lipgloss.JoinHorizontal(lipgloss.Center, logo(true), "    ", title)
	cmds := [][2]string{
		{"wk new 1", "make a warm workspace for row 1"},
		{"wk hive", "watch every worktree, live"},
		{"wk viz", "the same hive in a browser"},
	}
	var b strings.Builder
	b.WriteString(top + "\n\n")
	for _, t := range m.tasks {
		b.WriteString(good.Render("◆ ") + t.name + dim.Render("  "+seconds(t.took)) + "\n")
	}
	b.WriteString("\n" + bold.Render("Next") + "\n")
	for _, c := range cmds {
		b.WriteString(ink.Render(fmt.Sprintf("  %-10s", c[0])) + dim.Render(c[1]) + "\n")
	}
	b.WriteString("\n" + m.wrap(dim, "Edit "+tilde(m.plan.ConfigPath)+" to add build steps, gate env or sparse rules.") + "\n\n")
	b.WriteString(m.choices([]string{"Open the hive", "Quit"}))
	return b.String()
}

func (m Model) home() string {
	title := lipgloss.JoinVertical(lipgloss.Left, bold.Render(m.plan.Repo+" is set up"), dim.Render(tilde(m.plan.ConfigPath)))
	var b strings.Builder
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Center, logo(m.plan.LoadErr == nil), "    ", title) + "\n\n")
	if m.plan.LoadErr != nil {
		b.WriteString(bad.Bold(true).Render("✖ The config does not load") + "\n")
		b.WriteString(m.wrap(lipgloss.NewStyle(), m.plan.LoadErr.Error()) + "\n\n")
		b.WriteString(m.wrap(dim, "Fix the file by hand. Setup never overwrites a config.") + "\n\n")
	} else {
		for _, f := range m.plan.Summary {
			b.WriteString(dim.Render(fmt.Sprintf("%-14s", f.Label)) + clip(f.Value, m.inner()-14) + "\n")
		}
		b.WriteString("\n")
	}
	if m.flash != "" {
		b.WriteString(good.Render("◆ "+m.flash) + "\n\n")
	}
	if t := m.failure(); t != nil {
		b.WriteString(bad.Render("✖ "+t.name+": ") + m.wrap(lipgloss.NewStyle(), t.err.Error()) + "\n\n")
	}
	b.WriteString(m.choices(m.menu()))
	return b.String()
}

func (m Model) choices(items []string) string {
	var rows []string
	for i, it := range items {
		if m.cursor[m.step] == i {
			rows = append(rows, ink.Render("› ")+brand.Render(it))
		} else {
			rows = append(rows, "  "+dim.Render(it))
		}
	}
	return strings.Join(rows, "\n")
}

func (m Model) keys() string {
	var hints [][2]string
	switch m.step {
	case stepWelcome:
		hints = [][2]string{{"enter", "start"}, {"q", "quit"}}
	case stepBase:
		hints = [][2]string{{"↑/↓", "choose"}, {"enter", "next"}, {"esc", "back"}}
	case stepClone, stepCopy, stepInstall, stepExtras:
		hints = [][2]string{{"↑/↓", "move"}, {"space", "toggle"}, {"enter", "next"}, {"esc", "back"}}
	case stepReview:
		hints = [][2]string{{"↑/↓", "scroll"}, {"enter", "write and set up"}, {"esc", "back"}}
	case stepRun:
		hints = [][2]string{{"ctrl+c", "stop"}}
	default:
		hints = [][2]string{{"↑/↓", "choose"}, {"enter", "select"}, {"q", "quit"}}
	}
	parts := make([]string, len(hints))
	for i, h := range hints {
		parts[i] = brand.Render(h[0]) + dim.Render(" "+h[1])
	}
	return strings.Join(parts, faint.Render("  ·  "))
}

func clip(s string, n int) string {
	r := []rune(s)
	if n <= 0 {
		return ""
	}
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

// tilde shortens a path under the home folder for display.
func tilde(p string) string {
	if home, err := os.UserHomeDir(); err == nil && home != "" && strings.HasPrefix(p, home+"/") {
		return "~" + p[len(home):]
	}
	return p
}
