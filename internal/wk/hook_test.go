package wk

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

func hookInput(t *testing.T, name, cwd string) *bytes.Reader {
	t.Helper()
	data, err := json.Marshal(map[string]string{
		"session_id": "s1", "transcript_path": "/dev/null", "cwd": cwd,
		"hook_event_name": "WorktreeCreate", "name": name,
	})
	if err != nil {
		t.Fatal(err)
	}
	return bytes.NewReader(data)
}

func hookLog(t *testing.T, f testrepo.Fixture) string {
	t.Helper()
	return read(t, filepath.Join(f.Home, "demo", "hook.log"))
}

func TestHookMakesAWarmWorktreeFromTheSessionsHead(t *testing.T) {
	f := testrepo.New(t)
	session := mustNew(t, open(t, f, io.Discard), "40", false)
	testrepo.Git(t, session, "commit", "--quiet", "--allow-empty", "-m", "session work")
	head := testrepo.Git(t, session, "rev-parse", "HEAD")

	var stderr bytes.Buffer
	refilled := false
	path, err := WorktreeCreate(f.Home, hookInput(t, "agent-a1b2c3", session), &stderr, func(e *Env) {
		e.Cfg.Pool.Size = 1
		e.Refill = func() { refilled = true }
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, stderr.String())
	}
	if want := filepath.Join(f.Repo, ".claude/worktrees/agent-a1b2c3"); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	if got := testrepo.Git(t, path, "branch", "--show-current"); got != "worktree-agent-a1b2c3" {
		t.Fatalf("branch = %q", got)
	}
	if got := testrepo.Git(t, path, "rev-parse", "HEAD"); got != head {
		t.Fatalf("HEAD = %s, want the session's %s (BD-7)", got, head)
	}
	read(t, filepath.Join(path, "node_modules/.installed-from"))
	read(t, filepath.Join(path, "web/.env"))
	assertClean(t, path)
	if !refilled {
		t.Fatal("the hook did not top the pool up")
	}
	assertWarmLine(t, hookLog(t, f), path, "template")
	if stderr.Len() != 0 {
		t.Fatalf("stderr on success: %q", stderr.String())
	}
}

// assertWarmLine checks that hook.log ends the create of path with a warm line naming source,
// in the shape wk stats reads.
func assertWarmLine(t *testing.T, log, path, source string) {
	t.Helper()
	want := regexp.MustCompile(`(?m)^\S+ warm ` + regexp.QuoteMeta(path) + ` in [0-9.]+s from ` + source + `$`)
	if !want.MatchString(log) {
		t.Fatalf("hook.log has no warm line for %s from %s:\n%s", path, source, log)
	}
	if at, name, _, got, ok := warmLine(want.FindString(log)); !ok || at.IsZero() || name != filepath.Base(path) || got != source {
		t.Fatalf("wk stats reads the warm line as %v %q %q %v", at, name, got, ok)
	}
}

func TestHookWarmLineNamesAPoolSlot(t *testing.T) {
	f := testrepo.New(t)
	mustFill(t, openPool(t, f, io.Discard, 1, 0))
	var stderr bytes.Buffer
	path, err := WorktreeCreate(f.Home, hookInput(t, "agent-pool01", f.Repo), &stderr, func(e *Env) {
		e.Cfg.Pool.Size = 1
		e.Refill = func() {}
	})
	if err != nil {
		t.Fatalf("%v\n%s", err, stderr.String())
	}
	log := hookLog(t, f)
	if !strings.Contains(log, "claim pool slot") {
		t.Fatalf("the hook did not take the slot:\n%s", log)
	}
	assertWarmLine(t, log, path, "pool")
}

func TestHookFallsBackToAPlainWorktree(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv(HookFailEnv, "1")
	var stderr bytes.Buffer
	path, err := WorktreeCreate(f.Home, hookInput(t, "swift-fox-ab12", f.Repo), &stderr, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := testrepo.Git(t, path, "branch", "--show-current"); got != "worktree-swift-fox-ab12" {
		t.Fatalf("branch = %q", got)
	}
	if _, err := os.Stat(filepath.Join(path, "node_modules")); !os.IsNotExist(err) {
		t.Fatal("the fallback worktree has node_modules; it should be plain")
	}
	if !strings.Contains(stderr.String(), HookFailEnv+" is set") {
		t.Fatalf("stderr does not say why: %q", stderr.String())
	}
	if log := hookLog(t, f); !strings.Contains(log, "plain fallback "+path) || !strings.Contains(log, HookFailEnv) {
		t.Fatalf("hook.log:\n%s", log)
	}
}

func TestHookFallsBackWhenTheConfigIsBroken(t *testing.T) {
	f := testrepo.New(t)
	if err := os.WriteFile(filepath.Join(f.Home, "demo.toml"), []byte("[template\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	path, err := WorktreeCreate(f.Home, hookInput(t, "calm-owl-cd34", f.Repo), &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(f.Repo, ".claude/worktrees/calm-owl-cd34"); path != want {
		t.Fatalf("path = %q, want the default hook.dir %q", path, want)
	}
}

func TestHookReusesItsOwnWorktreeAndAnOldBranch(t *testing.T) {
	f := testrepo.New(t)
	first, err := WorktreeCreate(f.Home, hookInput(t, "reuse-me", f.Repo), &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	again, err := WorktreeCreate(f.Home, hookInput(t, "reuse-me", f.Repo), &bytes.Buffer{}, nil)
	if err != nil || again != first {
		t.Fatalf("second call = %q, %v; want the same %q", again, err, first)
	}

	f.Git(t, "branch", "worktree-left-over", "main")
	path, err := WorktreeCreate(f.Home, hookInput(t, "left-over", f.Repo), &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := testrepo.Git(t, path, "branch", "--show-current"); got != "worktree-left-over" {
		t.Fatalf("branch = %q, want the left-over branch checked out", got)
	}
}

func TestHookRefusesBadInput(t *testing.T) {
	f := testrepo.New(t)
	for name, in := range map[string]*bytes.Reader{
		"dot segment":  hookInput(t, "..", f.Repo),
		"double dot":   hookInput(t, "a..b", f.Repo),
		"slash":        hookInput(t, "a/b", f.Repo),
		"empty":        hookInput(t, "", f.Repo),
		"relative cwd": hookInput(t, "ok-name", "demo"),
		"not json":     bytes.NewReader([]byte("{")),
	} {
		t.Run(name, func(t *testing.T) {
			if path, err := WorktreeCreate(f.Home, in, &bytes.Buffer{}, nil); err == nil {
				t.Fatalf("made %q from bad input", path)
			}
		})
	}
	if entries, _ := os.ReadDir(filepath.Join(f.Repo, ".claude/worktrees")); len(entries) != 0 {
		t.Fatalf("bad input left directories: %v", entries)
	}
}

// A reused worktree has a session in it again, so the stop marker a SubagentStop left there
// must go, or gc would give the live worktree a stopped subagent's short window (F2, 2026-10-16).
func TestHookReuseClearsTheStopMarker(t *testing.T) {
	f := testrepo.New(t)
	name := "agent-" + agentID
	first, err := WorktreeCreate(f.Home, hookInput(t, name, f.Repo), &bytes.Buffer{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	markStopped(t, f, first)
	marker := stoppedMarker(t, first)
	read(t, marker)

	again, err := WorktreeCreate(f.Home, hookInput(t, name, f.Repo), &bytes.Buffer{}, nil)
	if err != nil || again != first {
		t.Fatalf("second call = %q, %v; want the same %q", again, err, first)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("the stop marker survived the reuse (stat err %v)", err)
	}
	if log := hookLog(t, f); !strings.Contains(log, "reused "+first) {
		t.Fatalf("hook.log:\n%s", log)
	}
}
