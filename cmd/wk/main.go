// Command wk makes warm git workspaces for parallel agents.
package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/zach-short/landing-layer/internal/wk"
)

const usage = `usage:
  wk [--repo <absolute dir>] template refresh
  wk [--repo <absolute dir>] new <row> [--sparse]
  wk [--repo <absolute dir>] gc [--apply|--json]
  wk [--repo <absolute dir>] gc schedule install|uninstall

The repository is the one holding --repo, or the current directory. Config is read from
$WK_HOME/<repo>.toml, where WK_HOME defaults to ~/.wk. wk new prints the workspace's absolute
path on stdout and everything else on stderr.

wk gc lists every worktree with its verdict and changes nothing; --apply removes the removable
ones; --json prints the same verdicts as one JSON document for tools. wk gc schedule install writes and loads a launchd agent that runs wk gc --apply every
gc.apply_interval_minutes; uninstall unloads and deletes it.
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
	if err != nil || len(rest) == 0 {
		return fail(stderr, err, exitUsage)
	}
	home, err := wkHome()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	switch {
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
	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err != nil {
		return fail(stderr, err, exitError)
	}
	return fail(stderr, env.ScheduleInstall(exe, l, stdout), exitError)
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
