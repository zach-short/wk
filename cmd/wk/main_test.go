package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
	"github.com/zach-short/wk/internal/wk"
)

func TestMain(m *testing.M) {
	testrepo.Isolate()
	os.Exit(m.Run())
}

func TestNewPrintsOnlyThePath(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--repo", f.Repo, "new", "12", "--sparse"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	want := filepath.Join(f.Repo, ".claude/worktrees/wk-12") + "\n"
	if stdout.String() != want {
		t.Fatalf("stdout = %q, want exactly %q", stdout.String(), want)
	}
	if !strings.Contains(stderr.String(), "wk: sparse checkout") {
		t.Fatalf("progress did not go to stderr:\n%s", stderr.String())
	}
	if strings.Contains(stderr.String(), "main-checkout") {
		t.Fatal("a copied secret reached stderr")
	}
}

func TestUsageAndRefusals(t *testing.T) {
	isolated(t)
	for name, tc := range map[string]struct {
		args []string
		code int
		want string
	}{
		"no args":       {nil, exitUsage, "usage:"},
		"unknown":       {[]string{"frobnicate"}, exitUsage, "usage:"},
		"new no row":    {[]string{"new"}, exitUsage, "needs a row"},
		"new two rows":  {[]string{"new", "1", "2"}, exitUsage, "unexpected argument"},
		"new bad flag":  {[]string{"new", "1", "--fat"}, exitUsage, "unexpected argument"},
		"new two modes": {[]string{"new", "1", "--full", "--sparse"}, exitUsage, "exclude each other"},
		"relative repo": {[]string{"--repo", "demo", "new", "1"}, exitUsage, "relative"},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != tc.code {
				t.Fatalf("exit %d, want %d\n%s", code, tc.code, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want nothing", stdout.String())
			}
			if !strings.Contains(stderr.String(), tc.want) {
				t.Fatalf("stderr = %q, want %q", stderr.String(), tc.want)
			}
		})
	}
}

func TestRelativeWKHomeIsRefused(t *testing.T) {
	t.Setenv("WK_HOME", "rel")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--repo", t.TempDir(), "template", "refresh"}, &stdout, &stderr); code != exitError {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "WK_HOME") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestGCDryRunPrintsAVerdictPerWorktree(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	f.Git(t, "worktree", "add", "--quiet", "-b", "worktree-x", filepath.Join(f.Repo, ".claude/worktrees/x"), "main")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--repo", f.Repo, "gc"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "kept                       .claude/worktrees/x") ||
		!strings.Contains(stdout.String(), "0 of 1 removable; dry run") {
		t.Fatalf("stdout:\n%s", stdout.String())
	}
}

func TestVizRefusesBadArguments(t *testing.T) {
	isolated(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"viz", "--port", "9"}, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), "unexpected arguments to wk viz") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestHiveRefusesBadArguments(t *testing.T) {
	isolated(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"hive", "--every", "1s"}, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), "unexpected arguments to wk hive") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

// The terminal view reads the dry-run report and nothing else: the worktree is still on disk
// after the read.
func TestHiveLoaderReadsTheDryRunReport(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	dir := filepath.Join(f.Repo, ".claude/worktrees/x")
	f.Git(t, "worktree", "add", "--quiet", "-b", "worktree-x", dir, "main")
	var stderr bytes.Buffer
	env, err := wk.Open(f.Home, f.Repo, &stderr)
	if err != nil {
		t.Fatal(err)
	}
	r, err := hiveLoader(env)()
	if err != nil {
		t.Fatal(err)
	}
	if r.Base != "main" || len(r.Worktrees) != 1 || r.Worktrees[0].Path != ".claude/worktrees/x" {
		t.Fatalf("report = %+v", r)
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("the read touched the worktree: %v", err)
	}
}

func TestGCRefusals(t *testing.T) {
	isolated(t)
	for _, args := range [][]string{{"gc", "--force"}, {"gc", "schedule"}, {"gc", "--apply", "now"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), "unexpected arguments to wk gc") {
			t.Errorf("%q: exit %d, stderr %q", args, code, stderr.String())
		}
	}
}

// A test binary is a go-build binary, so the install refuses it, which is the refusal an agent
// pointed at a `go run` binary would hit. No launchctl call is made.
func TestScheduleInstallRefusesTheTestBinary(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	called := false
	launchd = func() (wk.Launchd, error) {
		return wk.Launchd{AgentsDir: filepath.Join(f.Root, "agents"), Domain: "gui/0", Control: func(...string) error { called = true; return nil }}, nil
	}
	t.Cleanup(func() { launchd = realLaunchd })
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--repo", f.Repo, "gc", "schedule", "install"}, &stdout, &stderr); code != exitError {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "go install ./cmd/wk") || called {
		t.Fatalf("stderr %q, launchctl called %v", stderr.String(), called)
	}
}

func feed(t *testing.T, input string) {
	t.Helper()
	stdin = strings.NewReader(input)
	t.Cleanup(func() { stdin = os.Stdin })
}

func TestHookPrintsOnlyThePath(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	feed(t, `{"hook_event_name":"WorktreeCreate","name":"agent-0123456789abcdef0","cwd":"`+f.Repo+`"}`)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"hook", "worktree-create"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	if want := filepath.Join(f.Repo, ".claude/worktrees/agent-0123456789abcdef0") + "\n"; stdout.String() != want {
		t.Fatalf("stdout = %q, want exactly %q", stdout.String(), want)
	}
}

// A failed hook prints nothing on stdout, because Claude Code takes any stdout line as the path,
// and exits 2.
func TestHookFailurePrintsNoPath(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	feed(t, `{"hook_event_name":"WorktreeCreate","name":"a/b","cwd":"`+f.Repo+`"}`)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"hook", "worktree-create"}, &stdout, &stderr); code != exitUsage {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "cannot be a directory name") {
		t.Fatalf("stdout %q, stderr %q", stdout.String(), stderr.String())
	}
}

func TestHooksInstallRefusesTheTestBinary(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--repo", f.Repo, "hooks", "install"}, &stdout, &stderr); code != exitError {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "go install ./cmd/wk") {
		t.Fatalf("stderr %q", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(f.Repo, ".claude/settings.local.json")); !os.IsNotExist(err) {
		t.Fatal("a refused install wrote the settings file")
	}
}

func TestPoolAndHooksRefuseBadArguments(t *testing.T) {
	isolated(t)
	for _, args := range [][]string{{"pool", "drain"}, {"hooks", "add"}, {"pool"}, {"hook", "worktree-delete"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != exitUsage {
			t.Errorf("%q: exit %d, stderr %q", args, code, stderr.String())
		}
	}
}

// isolated runs a refusal test from an empty directory with an empty WK_HOME, so a case that
// is not refused acts on no real repository. The test's own working directory is inside the
// wk repository, which a `wk new` there would change.
func isolated(t *testing.T) {
	t.Helper()
	t.Setenv("WK_HOME", t.TempDir())
	t.Chdir(t.TempDir())
}

// Exit 2 from a SubagentStop hook would keep the subagent running, so any input, even junk,
// exits 0 and prints nothing on stdout.
func TestSubagentStopNeverBlocksTheStop(t *testing.T) {
	isolated(t)
	for _, input := range []string{"{", `{"agent_id":"a0123456789abcdef","cwd":"/nowhere"}`, ""} {
		feed(t, input)
		var stdout, stderr bytes.Buffer
		if code := run([]string{"hook", "subagent-stop"}, &stdout, &stderr); code != exitOK || stdout.Len() != 0 {
			t.Errorf("input %q: exit %d, stdout %q", input, code, stdout.String())
		}
	}
}

func TestWorktreeRemoveRefusesBadInput(t *testing.T) {
	isolated(t)
	feed(t, `{"worktree_path":"relative/path"}`)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"hook", "worktree-remove"}, &stdout, &stderr); code != exitError || !strings.Contains(stderr.String(), "not absolute") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}
