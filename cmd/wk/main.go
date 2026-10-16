// Command wk makes warm git workspaces for parallel agents.
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/hive"
	"github.com/zach-short/wk/internal/setup"
	"github.com/zach-short/wk/internal/viz"
	"github.com/zach-short/wk/internal/wk"
)

const usage = `usage:
  wk [--repo <absolute dir>]
  wk [--repo <absolute dir>] template refresh
  wk [--repo <absolute dir>] new <row> [--sparse]
  wk [--repo <absolute dir>] gc [--apply|--json]
  wk [--repo <absolute dir>] gc schedule install|uninstall
  wk [--repo <absolute dir>] viz [--addr <host:port>]
  wk [--repo <absolute dir>] hive

wk alone, in a terminal, opens setup: it reads the repository, writes its config and warms the
template. Once the config exists it opens a home screen instead, and never overwrites the file.

The repository is the one holding --repo, or the current directory. Config is read from
$WK_HOME/<repo>.toml, where WK_HOME defaults to ~/.wk. wk new prints the workspace's absolute
path on stdout and everything else on stderr.

wk gc lists every worktree with its verdict and changes nothing; --apply removes the removable
ones; --json prints the same verdicts as one JSON document for tools.
wk gc schedule install writes and loads a launchd agent that runs wk gc --apply every
gc.apply_interval_minutes; uninstall unloads and deletes it.

wk viz serves the hive page on --addr (default 127.0.0.1:8787). The page reads the live
verdicts on every load and changes nothing.
wk hive draws the same hive in the terminal and reads the verdicts again every 3 seconds.
Arrow keys select a cell, q quits. Like wk gc without --apply, it changes nothing.
`

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	repoDir, rest, err := splitRepoFlag(args)
	if err != nil || (len(rest) == 0 && !interactive()) {
		return fail(stderr, err, exitUsage)
	}
	home, err := wkHome()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	switch {
	case len(rest) == 0:
		return runSetup(home, repoDir, stderr)
	case len(rest) == 2 && rest[0] == "template" && rest[1] == "refresh":
		env, err := wk.Open(home, repoDir, stderr)
		if err != nil {
			return fail(stderr, err, exitError)
		}
		return fail(stderr, env.RefreshTemplate(), exitError)
	case rest[0] == "new":
		row, sparse, err := parseNew(rest[1:])
		if err != nil {
			return fail(stderr, err, exitUsage)
		}
		env, err := wk.Open(home, repoDir, stderr)
		if err != nil {
			return fail(stderr, err, exitError)
		}
		path, err := env.New(row, sparse)
		if err != nil {
			return fail(stderr, err, exitError)
		}
		_, _ = fmt.Fprintln(stdout, path)
		return exitOK
	case rest[0] == "gc":
		return runGC(home, repoDir, rest[1:], stdout, stderr)
	case rest[0] == "viz":
		return runViz(home, repoDir, rest[1:], stderr)
	case rest[0] == "hive":
		return runHive(home, repoDir, rest[1:], stderr)
	}
	return fail(stderr, nil, exitUsage)
}

// launchd is swapped by the self-test, so no test loads an agent into the owner's session.
var launchd = realLaunchd

func realLaunchd() (wk.Launchd, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return wk.Launchd{}, err
	}
	return wk.Launchd{
		AgentsDir: filepath.Join(home, "Library", "LaunchAgents"),
		Domain:    fmt.Sprintf("gui/%d", os.Getuid()),
		Control: func(args ...string) error {
			out, err := exec.Command("launchctl", args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
			}
			return nil
		},
	}, nil
}

func runGC(home, repoDir string, args []string, stdout, stderr io.Writer) int {
	what := strings.Join(args, " ")
	switch what {
	case "", "--apply", "--json", "schedule install", "schedule uninstall":
	default:
		return fail(stderr, fmt.Errorf("unexpected arguments to wk gc: %q", what), exitUsage)
	}
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	switch what {
	case "":
		return fail(stderr, env.GC(stdout), exitError)
	case "--apply":
		return fail(stderr, env.GCApply(stdout), exitError)
	case "--json":
		return fail(stderr, env.GCJSON(stdout), exitError)
	}
	l, err := launchd()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	if what == "schedule uninstall" {
		return fail(stderr, env.ScheduleUninstall(l, stdout), exitError)
	}
	return fail(stderr, scheduleInstall(env, l, stdout), exitError)
}

func scheduleInstall(env *wk.Env, l wk.Launchd, out io.Writer) error {
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return err
	}
	return env.ScheduleInstall(exe, l, out)
}

// interactive is true when both ends are a terminal, so a bare wk may draw setup. A pipe or a
// test gets the usage text, as before setup existed. The self-test swaps it.
var interactive = func() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		info, err := f.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}

// runSetup opens setup for the repository holding repoDir: the wizard when it has no config,
// the home screen when it has one. Leaving with "Open the hive" opens wk hive.
func runSetup(home, repoDir string, stderr io.Writer) int {
	plan, err := setupPlan(home, repoDir)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	acts := setup.Actions{Warm: func(log io.Writer) error {
		env, err := wk.Open(home, plan.Primary, log)
		if err != nil {
			return err
		}
		return env.RefreshTemplate()
	}}
	if runtime.GOOS == "darwin" {
		acts.Schedule = func(log io.Writer) error {
			env, err := wk.Open(home, plan.Primary, log)
			if err != nil {
				return err
			}
			l, err := launchd()
			if err != nil {
				return err
			}
			return scheduleInstall(env, l, log)
		}
	}
	final, err := tea.NewProgram(setup.New(plan, acts), tea.WithAltScreen()).Run()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	if final.(setup.Model).Result() == setup.OpenHive {
		return runHive(home, plan.Primary, nil, stderr)
	}
	return exitOK
}

// setupPlan reads what setup starts from. The repository is only read for the wizard; the home
// screen needs the config alone.
func setupPlan(home, repoDir string) (setup.Plan, error) {
	repo, err := gitx.Resolve(repoDir)
	if err != nil {
		return setup.Plan{}, err
	}
	path := filepath.Join(home, repo.Name+".toml")
	cfg, found, loadErr := config.Load(path)
	plan := setup.Plan{Repo: repo.Name, Primary: repo.Primary, ConfigPath: path, Existing: found, LoadErr: loadErr}
	if !found {
		plan.Found = setup.Detect(repo.Primary)
		return plan, nil
	}
	if loadErr == nil {
		installs := make([]string, len(cfg.Template.PostClone))
		for i, c := range cfg.Template.PostClone {
			installs[i] = strings.Join(c.Run, " ")
		}
		plan.Summary = []setup.Fact{
			{Label: "base", Value: cfg.Workspace.Base},
			{Label: "template", Value: cfg.Template.Path},
			{Label: "clones", Value: orNone(strings.Join(cfg.Template.CloneDirs, ", "))},
			{Label: "copies", Value: orNone(strings.Join(cfg.Template.CopyFiles, ", "))},
			{Label: "installs", Value: orNone(strings.Join(installs, "; "))},
			{Label: "workspaces", Value: cfg.Workspace.Dir},
		}
	}
	return plan, nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

const defaultVizAddr = "127.0.0.1:8787"

// runViz serves the hive page until the process stops. Its report is built per request, so the
// page never shows a stale snapshot.
func runViz(home, repoDir string, args []string, stderr io.Writer) int {
	addr := defaultVizAddr
	switch {
	case len(args) == 0:
	case len(args) == 2 && args[0] == "--addr" && args[1] != "":
		addr = args[1]
	default:
		return fail(stderr, fmt.Errorf("unexpected arguments to wk viz: %q", strings.Join(args, " ")), exitUsage)
	}
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	_, _ = fmt.Fprintf(stderr, "wk viz: http://%s (Ctrl-C stops it)\n", addr)
	srv := &http.Server{Addr: addr, Handler: viz.Handler(env.GCJSON), ReadHeaderTimeout: 5 * time.Second}
	return fail(stderr, srv.ListenAndServe(), exitError)
}

const hiveEvery = 3 * time.Second

// runHive draws the terminal hive until q. It reads through hiveLoader only, which is the dry
// run, so the view cannot remove anything.
func runHive(home, repoDir string, args []string, stderr io.Writer) int {
	if len(args) != 0 {
		return fail(stderr, fmt.Errorf("unexpected arguments to wk hive: %q", strings.Join(args, " ")), exitUsage)
	}
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	_, err = tea.NewProgram(hive.New(hiveLoader(env), hiveEvery), tea.WithAltScreen()).Run()
	return fail(stderr, err, exitError)
}

// hiveLoader reads the report the page reads, through the same JSON, so both views keep one
// contract.
func hiveLoader(env *wk.Env) hive.Loader {
	return func() (wk.Report, error) {
		var buf bytes.Buffer
		if err := env.GCJSON(&buf); err != nil {
			return wk.Report{}, err
		}
		var r wk.Report
		err := json.Unmarshal(buf.Bytes(), &r)
		return r, err
	}
}

// fail prints err and returns code, or returns exitOK when err is nil and code is not a usage
// error. A usage error always prints the usage text.
func fail(stderr io.Writer, err error, code int) int {
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "wk: %v\n", err)
	}
	if code == exitUsage {
		_, _ = fmt.Fprint(stderr, usage)
		return code
	}
	if err == nil {
		return exitOK
	}
	return code
}

func splitRepoFlag(args []string) (string, []string, error) {
	if len(args) >= 2 && args[0] == "--repo" {
		if !filepath.IsAbs(args[1]) {
			return "", nil, fmt.Errorf("--repo %q is relative; wk takes absolute paths only", args[1])
		}
		return args[1], args[2:], nil
	}
	cwd, err := os.Getwd()
	return cwd, args, err
}

func parseNew(args []string) (row string, sparse bool, err error) {
	for _, a := range args {
		switch {
		case a == "--sparse":
			sparse = true
		case row == "" && len(a) > 0 && a[0] != '-':
			row = a
		default:
			return "", false, fmt.Errorf("unexpected argument %q", a)
		}
	}
	if row == "" {
		return "", false, fmt.Errorf("wk new needs a row")
	}
	return row, sparse, nil
}

func wkHome() (string, error) {
	if h := os.Getenv("WK_HOME"); h != "" {
		if !filepath.IsAbs(h) {
			return "", fmt.Errorf("WK_HOME %q is relative; wk takes absolute paths only", h)
		}
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wk"), nil
}
