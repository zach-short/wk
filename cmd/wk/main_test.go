package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/landing-layer/internal/testrepo"
	"github.com/zach-short/landing-layer/internal/wk"
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
	t.Setenv("WK_HOME", t.TempDir())
	for name, tc := range map[string]struct {
		args []string
		code int
		want string
	}{
		"no args":       {nil, exitUsage, "usage:"},
		"unknown":       {[]string{"frobnicate"}, exitUsage, "usage:"},
		"new no row":    {[]string{"new"}, exitUsage, "needs a row"},
		"new two rows":  {[]string{"new", "1", "2"}, exitUsage, "unexpected argument"},
		"new bad flag":  {[]string{"new", "1", "--full"}, exitUsage, "unexpected argument"},
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
	t.Setenv("WK_HOME", t.TempDir())
	var stdout, stderr bytes.Buffer
	if code := run([]string{"viz", "--port", "9"}, &stdout, &stderr); code != exitUsage || !strings.Contains(stderr.String(), "unexpected arguments to wk viz") {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
}

func TestHiveRefusesBadArguments(t *testing.T) {
	t.Setenv("WK_HOME", t.TempDir())
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
	t.Setenv("WK_HOME", t.TempDir())
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
