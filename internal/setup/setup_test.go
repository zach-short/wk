package setup

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/testrepo"
)

func TestMain(m *testing.M) {
	testrepo.Isolate()
	os.Exit(m.Run())
}

// monorepo is a small repository with two package.json files, a bun lockfile and three .env
// files: one ignored, one committed, one example.
func monorepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	for path, body := range map[string]string{
		"package.json":          "{}",
		"bun.lock":              "lock",
		"web/package.json":      "{}",
		"web/.env":              "SECRET=1",
		".env.production":       "PUBLIC=1",
		".env.example":          "SECRET=",
		"deep/a/b/package.json": "{}",
		".gitignore":            "web/.env\nnode_modules\n",
	} {
		p := filepath.Join(dir, filepath.FromSlash(path))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{
		{"init", "-q", "-b", "trunk"}, {"add", "-A"}, {"commit", "-q", "-m", "init"}, {"branch", "dev"}, {"branch", "wk/3"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

func values(opts []Option) []string {
	var out []string
	for _, o := range opts {
		out = append(out, fmt.Sprintf("%s=%v", o.Value, o.On))
	}
	return out
}

func TestDetectReadsTheCheckout(t *testing.T) {
	f := Detect(monorepo(t))
	if want := []string{"trunk", "dev"}; !reflect.DeepEqual(f.Branches, want) {
		t.Errorf("branches = %v, want %v (current first, wk branches left out)", f.Branches, want)
	}
	if got, want := values(f.CloneDirs), []string{"node_modules=true", "web/node_modules=true"}; !reflect.DeepEqual(got, want) {
		t.Errorf("clone dirs = %v, want %v (deep/a/b is past the depth limit)", got, want)
	}
	if got, want := values(f.CopyFiles), []string{".env.production=false", "web/.env=true"}; !reflect.DeepEqual(got, want) {
		t.Errorf("copy files = %v, want %v (tracked off, ignored on, example left out)", got, want)
	}
	if got, want := values(f.Installs), []string{"bun install --frozen-lockfile=true"}; !reflect.DeepEqual(got, want) {
		t.Errorf("installs = %v, want %v", got, want)
	}
}

func TestRenderLoadsAndWriteNeverOverwrites(t *testing.T) {
	c := Choices{
		Base: "trunk", CloneDirs: []string{"node_modules"}, CopyFiles: []string{"web/.env"},
		Installs: []Install{{Dir: ".", Run: []string{"bun", "install"}, Lockfile: "bun.lock"}},
	}
	path := filepath.Join(t.TempDir(), "home", "demo.toml")
	if err := Write(path, Render("demo", c)); err != nil {
		t.Fatal(err)
	}
	cfg, found, err := config.Load(path)
	if err != nil || !found {
		t.Fatalf("load: found=%v err=%v", found, err)
	}
	if cfg.Workspace.Base != "trunk" || cfg.Template.RefreshOn != "bun.lock" ||
		!reflect.DeepEqual(cfg.Template.CopyFiles, []string{"web/.env"}) || len(cfg.Template.PostClone) != 1 {
		t.Fatalf("config read back wrong: %+v", cfg.Template)
	}

	if err := os.WriteFile(path, []byte("# mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, Render("demo", c)); !errors.Is(err, ErrExists) {
		t.Fatalf("second write: %v, want ErrExists", err)
	}
	if b, _ := os.ReadFile(path); string(b) != "# mine\n" {
		t.Fatalf("the existing config changed: %q", b)
	}
	entries, _ := os.ReadDir(filepath.Dir(path))
	if len(entries) != 1 {
		t.Fatalf("setup left files behind: %v", entries)
	}
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "space":
			msg = tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		next, _ := m.Update(msg)
		m = next.(Model)
	}
	return m
}

// settle runs commands until none is left, skipping the spinner's ticks.
func settle(t *testing.T, m Model, cmd tea.Cmd) Model {
	t.Helper()
	queue := []tea.Cmd{cmd}
	for len(queue) > 0 {
		c := queue[0]
		queue = queue[1:]
		if c == nil {
			continue
		}
		switch msg := c().(type) {
		case tea.BatchMsg:
			queue = append(queue, msg...)
		case spinMsg, nil:
		default:
			next, more := m.Update(msg)
			m = next.(Model)
			queue = append(queue, more)
		}
	}
	return m
}

type recorder struct{ warmed, scheduled int }

func (r *recorder) actions(warmErr error) Actions {
	return Actions{
		Warm: func(log io.Writer) error {
			r.warmed++
			_, _ = io.WriteString(log, "wk: checkout\nwk: bun install   1.20s\n")
			return warmErr
		},
		Schedule: func(io.Writer) error { r.scheduled++; return nil },
	}
}

func wizard(t *testing.T, acts Actions) (Model, string) {
	dir := monorepo(t)
	path := filepath.Join(t.TempDir(), "demo.toml")
	return New(Plan{Repo: "demo", Primary: dir, ConfigPath: path, Found: Detect(dir)}, acts), path
}

func TestWizardWritesWhatWasPicked(t *testing.T) {
	r := &recorder{}
	m, path := wizard(t, r.actions(nil))
	// welcome → base: pick dev → clone: turn web/node_modules off → copy → install → options:
	// turn the schedule on → review.
	m = press(m, "enter", "down", "enter", "down", "space", "enter", "enter", "enter", "down", "space", "enter")
	if m.step != stepReview {
		t.Fatalf("at step %d, want review", m.step)
	}
	view := m.View()
	for _, want := range []string{`base = "dev"`, `clone_dirs = ["node_modules"]`, `copy_files = ["web/.env"]`, "bun.lock"} {
		if !strings.Contains(view, want) {
			t.Errorf("review lacks %q", want)
		}
	}
	if _, err := os.Stat(path); err == nil {
		t.Fatal("the config was written before the run step")
	}

	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = settle(t, next.(Model), cmd)
	if m.step != stepDone || r.warmed != 1 || r.scheduled != 1 {
		t.Fatalf("after the run: step %d, warmed %d, scheduled %d", m.step, r.warmed, r.scheduled)
	}
	cfg, _, err := config.Load(path)
	if err != nil || cfg.Workspace.Base != "dev" || !reflect.DeepEqual(cfg.Template.CloneDirs, []string{"node_modules"}) {
		t.Fatalf("written config: %+v, %v", cfg.Workspace, err)
	}
	if view := m.View(); !strings.Contains(view, "The hive is ready") || !strings.Contains(view, "wk new 1") {
		t.Fatalf("done view:\n%s", view)
	}
	m = press(m, "enter")
	if m.Result() != OpenHive {
		t.Fatal("enter on the first choice did not open the hive")
	}
}

func TestWizardShowsAFailedWarm(t *testing.T) {
	r := &recorder{}
	m, path := wizard(t, r.actions(errors.New("bun: command not found")))
	m = press(m, "enter", "enter", "enter", "enter", "enter", "enter")
	next, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = settle(t, next.(Model), cmd)
	view := m.View()
	if m.failure() == nil || !strings.Contains(view, "Warm the template failed") || !strings.Contains(view, "bun: command not found") ||
		!strings.Contains(view, "Fix the cause") {
		t.Fatalf("failure view:\n%s", view)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal("the config should stay written after the warm failed")
	}
	if r.scheduled != 0 {
		t.Fatal("the schedule ran after a failed warm")
	}
}

func TestEscGoesBackAndQuitsFromWelcome(t *testing.T) {
	m, _ := wizard(t, Actions{})
	m = press(m, "enter", "enter", "esc")
	if m.step != stepBase {
		t.Fatalf("esc from dependencies went to %d, want base", m.step)
	}
	m = press(m, "esc")
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if _, ok := cmd().(tea.QuitMsg); !ok {
		t.Fatal("esc on welcome did not quit")
	}
}

func TestHomeScreenForAnExistingConfig(t *testing.T) {
	r := &recorder{}
	m := New(Plan{Repo: "demo", ConfigPath: "/x/demo.toml", Existing: true,
		Summary: []Fact{{Label: "base", Value: "main"}}}, r.actions(nil))
	if m.step != stepHome || !strings.Contains(m.View(), "demo is set up") {
		t.Fatalf("home view:\n%s", m.View())
	}
	next, cmd := press(m, "down").Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = settle(t, next.(Model), cmd)
	if r.warmed != 1 || m.step != stepHome || !strings.Contains(m.View(), "Template refreshed") {
		t.Fatalf("after refresh: warmed %d, step %d\n%s", r.warmed, m.step, m.View())
	}

	broken := New(Plan{Repo: "demo", Existing: true, LoadErr: errors.New("unknown keys: template.follow_base")}, Actions{})
	if got := broken.menu(); !reflect.DeepEqual(got, []string{menuQuit}) || !strings.Contains(broken.View(), "follow_base") {
		t.Fatalf("broken config: menu %v\n%s", got, broken.View())
	}
}
