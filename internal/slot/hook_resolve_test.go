package slot

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// From 2026-10-17 to 2026-10-26, 42 lines of ezhomesteading's errors.log were the hook's own
// 200 ms deadline, each a gate that ran unslotted, all while slotted suites filled the machine.
// The one git run was 85 to 95 percent of the hook's own time when measured, and the only child
// process it starts, so the resolver reads the files git reads. With no git on PATH the hook
// must still find the repository and rewrite the gate.
func TestHookPreToolUseResolvesWithoutGit(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	repo := filepath.Join(root, "ezhomesteading")
	wt := filepath.Join(root, "agent-1")
	for _, args := range [][]string{
		{"init", "-q", "--initial-branch=main", repo},
		{"-C", repo, "-c", "user.name=t", "-c", "user.email=t@t.invalid", "commit", "-q", "--allow-empty", "-m", "init"},
		{"-C", repo, "worktree", "add", "-q", "-b", "wk/1", wt},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	backend := filepath.Join(wt, "backend")
	if err := os.MkdirAll(backend, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", t.TempDir())
	env := hookTestSetup(t, matchTestSlotsTOML)
	env.deps.Resolve = nil
	input := strings.Replace(hookTestInput("Bash", `{"command":"go test ./..."}`), "/r/ezhomesteading", backend, 1)
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"command":"/x/wk run --slot check -- go test ./...","timeout":600000}}}` + "\n"
	if got := env.run(t, input); got != want {
		t.Errorf("stdout\n got %q\nwant %q", got, want)
	}
	if lines := env.logLines(t, "errors.log"); lines != nil {
		t.Errorf("errors.log: %q", lines)
	}
}
