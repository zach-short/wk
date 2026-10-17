package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

// The lease hook exits 0 on a deny too: the deny is the JSON on stdout, and a non-zero exit would
// block a call by accident.
func TestLeaseHookDeniesWithJSONAndExitsZero(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	cfg := filepath.Join(f.Home, "demo.toml")
	data, _ := os.ReadFile(cfg)
	if err := os.WriteFile(cfg, append(data, "\n[leases]\nenabled = true\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	feed(t, `{"cwd":"`+f.Repo+`","tool_name":"Edit","tool_input":{"file_path":"`+f.Repo+`/README.md"}}`)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"hook", "lease", "--repo", f.Repo}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `"permissionDecision":"deny"`) || !strings.Contains(stdout.String(), "has not claimed a row") {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestClaimStatusAndConfigShow(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	var stdout, stderr bytes.Buffer
	for _, args := range [][]string{{"--repo", f.Repo, "claim", "--none"}, {"--repo", f.Repo, "status"}, {"--repo", f.Repo, "config", "show"}} {
		if code := run(args, &stdout, &stderr); code != exitOK {
			t.Fatalf("%v: exit %d\n%s", args, code, stderr.String())
		}
	}
	for _, want := range []string{"claimed: row none in " + f.Repo, f.Repo + "  row none", "# " + filepath.Join(f.Home, "demo.toml") + " (found)", "[leases]"} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, stdout.String())
		}
	}
	if code := run([]string{"--repo", f.Repo, "claim"}, &stdout, &stderr); code != exitUsage {
		t.Errorf("claim with no row: exit %d", code)
	}
}

// panicOnce panics on its first write and keeps every later one.
type panicOnce struct {
	bytes.Buffer
	panicked bool
}

func (p *panicOnce) Write(b []byte) (int, error) {
	if !p.panicked {
		p.panicked = true
		panic("write failed")
	}
	return p.Buffer.Write(b)
}

// Audit 2, N7: a panic outside wk.LeaseHook, here in printing its deny, still prints a deny and
// exits 0.
func TestLeaseHookPanicOutsideTheJudgmentDenies(t *testing.T) {
	feed(t, `{`)
	var out panicOnce
	if code := runLeaseHook([]string{"--repo", "/nowhere"}, &out); code != exitOK {
		t.Fatalf("exit %d", code)
	}
	if !strings.Contains(out.String(), `"permissionDecision":"deny"`) || !strings.Contains(out.String(), "the lease hook failed (write failed)") {
		t.Fatalf("stdout = %q", out.String())
	}
}
