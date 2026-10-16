package setup

import (
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Plan is what the wizard starts from: the repository, where its config goes, and what wk found.
type Plan struct {
	Repo       string
	Primary    string
	ConfigPath string
	Existing   bool   // the config file is there, so the home screen opens, never the wizard
	LoadErr    error  // the existing config does not load
	Summary    []Fact // the existing config's main dials, for the home screen
	Found      Found
}

// Fact is one label and value on the home screen.
type Fact struct{ Label, Value string }

// Actions are the steps with effects outside the config file. Each writes its progress to log.
// Schedule is nil where wk cannot install the agent.
type Actions struct {
	Warm     func(log io.Writer) error
	Schedule func(log io.Writer) error
}

// Result is what the person asked for on leaving.
type Result int

const (
	Quit Result = iota
	OpenHive
)

type step int

const (
	stepWelcome step = iota
	stepBase
	stepClone
	stepCopy
	stepInstall
	stepExtras
	stepReview
	stepRun
	stepDone
	stepHome
)

// rail is the wizard's steps as the side rail names them, in order.
var rail = []struct {
	step  step
	label string
}{
	{stepWelcome, "Welcome"},
	{stepBase, "Base branch"},
	{stepClone, "Dependencies"},
	{stepCopy, "Local files"},
	{stepInstall, "Install"},
	{stepExtras, "Options"},
	{stepReview, "Review"},
	{stepRun, "Set up"},
}

// task is one effect of the run step.
type task struct {
	name  string
	run   func(log io.Writer) error
	state taskState
	took  time.Duration
	err   error
}

type taskState int

const (
	pending taskState = iota
	running
	succeeded
	failed
)

// Model is the bubbletea model of setup and of the home screen.
type Model struct {
	plan    Plan
	acts    Actions
	now     func() time.Time
	step    step
	cursor  map[step]int
	base    int
	clone   []Option
	copy    []Option
	install []Option
	extras  []Option // warm, then schedule when there is one
	scroll  int

	tasks   []task
	current int
	started time.Time
	log     []string
	events  chan runMsg
	frame   int
	flash   string // the home screen's note after a refresh

	result Result
	width  int
	height int
}

// New returns the model: the home screen when the config exists, else the wizard's welcome.
func New(plan Plan, acts Actions) Model {
	m := Model{plan: plan, acts: acts, now: time.Now, cursor: map[step]int{}}
	m.clone = slices.Clone(plan.Found.CloneDirs)
	m.copy = slices.Clone(plan.Found.CopyFiles)
	m.install = slices.Clone(plan.Found.Installs)
	m.extras = []Option{{Value: "Warm the template now", Note: "runs the install once, so the first wk new is fast", On: true}}
	if acts.Schedule != nil {
		m.extras = append(m.extras, Option{Value: "Clean up idle worktrees every hour", Note: "a launchd agent runs wk gc --apply"})
	}
	if plan.Existing {
		m.step = stepHome
	}
	return m
}

// Result is what the person chose on leaving; main opens the hive for OpenHive.
func (m Model) Result() Result { return m.result }

// Choices reads the wizard's picks.
func (m Model) Choices() Choices {
	c := Choices{Base: m.plan.Found.Branches[m.base], CloneDirs: []string{}, CopyFiles: []string{}}
	for _, o := range m.clone {
		if o.On {
			c.CloneDirs = append(c.CloneDirs, o.Value)
		}
	}
	for _, o := range m.copy {
		if o.On {
			c.CopyFiles = append(c.CopyFiles, o.Value)
		}
	}
	for i, o := range m.install {
		if o.On {
			c.Installs = append(c.Installs, m.plan.Found.installs[i])
		}
	}
	c.Warm = m.extras[0].On
	c.Schedule = len(m.extras) > 1 && m.extras[1].On
	return c
}

func (m Model) Init() tea.Cmd { return nil }

// runMsg is one line of a task's log, or, with done set, its end.
type runMsg struct {
	line string
	done bool
	err  error
}

type spinMsg struct{}

func spin() tea.Cmd {
	return tea.Tick(90*time.Millisecond, func(time.Time) tea.Msg { return spinMsg{} })
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case spinMsg:
		if m.step != stepRun || m.current >= len(m.tasks) {
			return m, nil
		}
		m.frame++
		return m, spin()
	case runMsg:
		return m.onRun(msg)
	case tea.KeyMsg:
		return m.onKey(msg.String())
	}
	return m, nil
}

func (m Model) onKey(key string) (tea.Model, tea.Cmd) {
	if key == "ctrl+c" {
		return m, tea.Quit
	}
	if m.step == stepRun {
		return m, nil // the run cannot be stopped half way, except by ctrl+c
	}
	switch key {
	case "q":
		return m, tea.Quit
	case "up", "k":
		m.move(-1)
		return m, nil
	case "down", "j":
		m.move(1)
		return m, nil
	case " ", "x":
		m.toggle()
		return m, nil
	case "esc", "left", "h", "shift+tab":
		switch m.step {
		case stepWelcome, stepHome:
			return m, tea.Quit
		case stepDone:
			return m, nil
		}
		m.step--
		return m, nil
	case "enter", "right", "l", "tab":
		return m.next()
	}
	return m, nil
}

// options is the toggle list of the step, if it has one.
func (m *Model) options() []Option {
	switch m.step {
	case stepClone:
		return m.clone
	case stepCopy:
		return m.copy
	case stepInstall:
		return m.install
	case stepExtras:
		return m.extras
	}
	return nil
}

// rows is how many lines the cursor moves over on this step.
func (m *Model) rows() int {
	switch m.step {
	case stepBase:
		return len(m.plan.Found.Branches)
	case stepHome:
		return len(m.menu())
	case stepDone:
		return 2
	}
	return len(m.options())
}

func (m *Model) move(by int) {
	if m.step == stepReview {
		m.scroll = max(m.scroll+by, 0)
		return
	}
	n := m.rows()
	if n == 0 {
		return
	}
	c := (m.cursor[m.step] + by + n) % n
	m.cursor[m.step] = c
	if m.step == stepBase {
		m.base = c
	}
}

func (m *Model) toggle() {
	if opts := m.options(); len(opts) > 0 {
		i := m.cursor[m.step]
		opts[i].On = !opts[i].On
	}
}

func (m Model) next() (tea.Model, tea.Cmd) {
	switch m.step {
	case stepReview:
		return m.startRun(true)
	case stepDone:
		if m.cursor[stepDone] == 0 {
			m.result = OpenHive
		}
		return m, tea.Quit
	case stepHome:
		switch m.menu()[m.cursor[stepHome]] {
		case menuHive:
			m.result = OpenHive
			return m, tea.Quit
		case menuRefresh:
			return m.startRun(false)
		}
		return m, tea.Quit
	}
	m.step++
	return m, nil
}

const (
	menuHive    = "Open the hive"
	menuRefresh = "Refresh the template"
	menuQuit    = "Quit"
)

// menu is the home screen's choices; a config that does not load leaves only quit.
func (m Model) menu() []string {
	if m.plan.LoadErr != nil {
		return []string{menuQuit}
	}
	return []string{menuHive, menuRefresh, menuQuit}
}

// startRun lists the tasks and starts the first. From the wizard it writes the config first;
// from the home screen it only refreshes the template.
func (m Model) startRun(wizard bool) (tea.Model, tea.Cmd) {
	m.tasks = nil
	if wizard {
		c := m.Choices()
		text := Render(m.plan.Repo, c)
		m.tasks = append(m.tasks, task{name: "Write " + tilde(m.plan.ConfigPath), run: func(io.Writer) error {
			return Write(m.plan.ConfigPath, text)
		}})
		if c.Warm {
			m.tasks = append(m.tasks, task{name: "Warm the template", run: m.acts.Warm})
		}
		if c.Schedule {
			m.tasks = append(m.tasks, task{name: "Install the cleanup schedule", run: m.acts.Schedule})
		}
	} else {
		m.tasks = append(m.tasks, task{name: "Refresh the template", run: m.acts.Warm})
	}
	m.step, m.current, m.log, m.flash = stepRun, 0, nil, ""
	return m, tea.Batch(m.begin(), spin())
}

// begin starts the current task in its own goroutine; its log lines and its end come back as
// runMsg through one channel, read one message at a time.
func (m *Model) begin() tea.Cmd {
	t := &m.tasks[m.current]
	t.state = running
	m.started = m.now()
	m.events = make(chan runMsg, 64)
	events, run := m.events, t.run
	go func() {
		w := &lineWriter{out: events}
		err := run(w)
		w.flush()
		events <- runMsg{done: true, err: err}
	}()
	return wait(events)
}

func wait(events chan runMsg) tea.Cmd {
	return func() tea.Msg { return <-events }
}

// keepLog is how many of the running task's last log lines show.
const keepLog = 6

func (m Model) onRun(msg runMsg) (tea.Model, tea.Cmd) {
	if !msg.done {
		m.log = append(m.log, msg.line)
		if len(m.log) > keepLog {
			m.log = m.log[len(m.log)-keepLog:]
		}
		return m, wait(m.events)
	}
	t := &m.tasks[m.current]
	t.took, t.err = m.now().Sub(m.started), msg.err
	if msg.err != nil {
		t.state = failed
		m.current = len(m.tasks)
		m.step = stepDone
		return m, nil
	}
	t.state = succeeded
	m.current++
	m.log = nil
	if m.current < len(m.tasks) {
		return m, m.begin()
	}
	if m.plan.Existing {
		m.step, m.flash = stepHome, fmt.Sprintf("Template refreshed in %s.", seconds(t.took))
		return m, nil
	}
	m.step = stepDone
	return m, nil
}

// failure is the task that failed, if one did.
func (m Model) failure() *task {
	for i := range m.tasks {
		if m.tasks[i].state == failed {
			return &m.tasks[i]
		}
	}
	return nil
}

// lineWriter turns a task's log into lines. wk's own lines start "wk: ", which the view drops.
type lineWriter struct {
	out  chan<- runMsg
	part string
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.part += string(p)
	for {
		i := strings.IndexByte(w.part, '\n')
		if i < 0 {
			break
		}
		w.send(w.part[:i])
		w.part = w.part[i+1:]
	}
	return len(p), nil
}

func (w *lineWriter) flush() {
	if w.part != "" {
		w.send(w.part)
		w.part = ""
	}
}

func (w *lineWriter) send(line string) {
	line = strings.TrimSpace(strings.TrimPrefix(line, "wk: "))
	if line != "" {
		w.out <- runMsg{line: line}
	}
}

func seconds(d time.Duration) string {
	return fmt.Sprintf("%.1fs", d.Seconds())
}
