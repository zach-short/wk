package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/landing-layer/internal/testrepo"
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
