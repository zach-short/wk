package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	exe     = "/Users/me/go/bin/wk"
	command = exe + " hook worktree-create"
)

// The shape of the owner's real file: a permission list and personal-config's merged key.
const shared = `{
  "permissions": {
    "allow": [
      "Bash(git status:*)",
      "Bash(bun run test:*)"
    ]
  },
  "availableModels": ["opus", "sonnet"],
  "hooks": {
    "SessionStart": [
      { "hooks": [ { "type": "command", "command": "echo hi" } ] }
    ]
  }
}
`

func file(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".claude", "settings.local.json")
	if body != "" {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return p
}

func load(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("%v\n%s", err, data)
	}
	return m
}

func worktreeCreate(t *testing.T, m map[string]any) []any {
	t.Helper()
	hooks, _ := m["hooks"].(map[string]any)
	groups, _ := hooks["WorktreeCreate"].([]any)
	return groups
}

func TestInstallKeepsEveryOtherKeyInOrder(t *testing.T) {
	p := file(t, shared)
	changed, err := Install(p, exe)
	if err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	data, _ := os.ReadFile(p)
	text := string(data)
	order := []string{`"permissions"`, `"Bash(git status:*)"`, `"Bash(bun run test:*)"`, `"availableModels"`, `"hooks"`, `"SessionStart"`, `"WorktreeCreate"`}
	last := -1
	for _, s := range order {
		i := strings.Index(text, s)
		if i < 0 || i < last {
			t.Fatalf("%s missing or out of order in:\n%s", s, text)
		}
		last = i
	}
	m := load(t, p)
	groups := worktreeCreate(t, m)
	if len(groups) != 1 {
		t.Fatalf("WorktreeCreate groups = %v, want one", groups)
	}
	entry := groups[0].(map[string]any)["hooks"].([]any)[0].(map[string]any)
	if entry["command"] != command || entry["type"] != "command" || entry["timeout"] != float64(600) {
		t.Fatalf("entry = %v", entry)
	}
	if info, _ := os.Stat(p); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v, want the file's own 0600 kept", info.Mode().Perm())
	}
}

func TestInstallTwiceChangesNothing(t *testing.T) {
	p := file(t, shared)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)
	changed, err := Install(p, exe)
	if err != nil || changed {
		t.Fatalf("second install: changed = %v, err = %v", changed, err)
	}
	after, _ := os.ReadFile(p)
	if string(before) != string(after) {
		t.Fatal("a second install rewrote the file")
	}
}

func TestInstallReplacesAnOlderWKEntry(t *testing.T) {
	p := file(t, `{"hooks":{"WorktreeCreate":[{"hooks":[{"type":"command","command":"WK_HOOK_FAIL=1 /old/wk hook worktree-create"}]}]}}`)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	groups := worktreeCreate(t, load(t, p))
	if len(groups) != 1 || !strings.Contains(mustJSON(t, groups), command) || strings.Contains(mustJSON(t, groups), "/old/") {
		t.Fatalf("groups = %s, want only the new command", mustJSON(t, groups))
	}
}

func TestInstallIntoAMissingFile(t *testing.T) {
	p := file(t, "")
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	if len(worktreeCreate(t, load(t, p))) != 1 {
		t.Fatal("no entry written to a new file")
	}
}

func TestInstallRefusals(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"not json":     {"{\"permissions\": ", "will not rewrite it"},
		"not object":   {"[1, 2]", "not a JSON object"},
		"trailing":     {"{} {}", "trailing data"},
		"foreign hook": {`{"hooks":{"WorktreeCreate":[{"hooks":[{"type":"command","command":"/x/make-tree.sh"}]}]}}`, "not wk's"},
	} {
		t.Run(name, func(t *testing.T) {
			p := file(t, tc.body)
			if _, err := Install(p, exe); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if data, _ := os.ReadFile(p); string(data) != tc.body {
				t.Fatal("a refused install changed the file")
			}
		})
	}
}

func TestUninstallRemovesOnlyWK(t *testing.T) {
	p := file(t, shared)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	changed, err := Uninstall(p)
	if err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	m := load(t, p)
	if worktreeCreate(t, m) != nil {
		t.Fatal("the WorktreeCreate entry is still there")
	}
	if _, ok := m["hooks"].(map[string]any)["SessionStart"]; !ok {
		t.Fatal("uninstall removed another tool's hook")
	}
	if _, ok := m["availableModels"]; !ok {
		t.Fatal("uninstall removed availableModels")
	}
	if changed, err := Uninstall(p); err != nil || changed {
		t.Fatalf("second uninstall: changed = %v, err = %v", changed, err)
	}
}

func TestUninstallDropsTheHooksKeyItAlonePutThere(t *testing.T) {
	p := file(t, `{"permissions":{"allow":[]}}`)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	if _, ok := load(t, p)["hooks"]; ok {
		t.Fatal("an empty hooks object was left behind")
	}
}

func TestUninstallOfAMissingFile(t *testing.T) {
	p := file(t, "")
	if changed, err := Uninstall(p); err != nil || changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatal("uninstall created a file")
	}
}

func TestIsWK(t *testing.T) {
	for cmd, want := range map[string]bool{
		command: true,
		"WK_HOOK_FAIL=1 /Users/me/go/bin/wk hook worktree-create": true,
		"'/Users/my dir/wk' hook worktree-create":                 true,
		"/x/notwk hook worktree-create":                           false,
		"/x/wk hook worktree-remove":                              true,
		"/x/wk hook subagent-stop":                                true,
		"/x/wk hook worktree-delete":                              false,
		"echo hook worktree-create":                               false,
	} {
		if got := IsWK(cmd); got != want {
			t.Errorf("IsWK(%q) = %v, want %v", cmd, got, want)
		}
	}
	if got := Command("/Users/my dir/wk", "hook worktree-create"); !IsWK(got) || !strings.HasPrefix(got, "'") {
		t.Fatalf("Command quoted a spaced path as %q", got)
	}
}

func mustJSON(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestInstallWritesEveryHook(t *testing.T) {
	p := file(t, shared)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	hooks := load(t, p)["hooks"].(map[string]any)
	for event, verb := range map[string]string{"WorktreeCreate": "worktree-create", "WorktreeRemove": "worktree-remove", "SubagentStop": "subagent-stop", "PreToolUse": "pre-tool-use || true"} {
		groups, _ := hooks[event].([]any)
		if len(groups) != 1 || !strings.Contains(mustJSON(t, groups), exe+" hook "+verb) {
			t.Errorf("%s = %s", event, mustJSON(t, groups))
		}
	}
}

// SubagentStop is not a single-answer event, so wk's hook sits beside another tool's there and
// uninstall leaves that one.
func TestSubagentStopSharesTheEvent(t *testing.T) {
	p := file(t, `{"hooks":{"SubagentStop":[{"hooks":[{"type":"command","command":"/x/notify.sh"}]}]}}`)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	stop := load(t, p)["hooks"].(map[string]any)["SubagentStop"].([]any)
	if len(stop) != 2 {
		t.Fatalf("SubagentStop = %s, want the other tool's group and wk's", mustJSON(t, stop))
	}
	if _, err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	stop = load(t, p)["hooks"].(map[string]any)["SubagentStop"].([]any)
	if len(stop) != 1 || !strings.Contains(mustJSON(t, stop), "notify.sh") {
		t.Fatalf("after uninstall SubagentStop = %s", mustJSON(t, stop))
	}
}

func TestInstallRefusesAForeignWorktreeRemoveHook(t *testing.T) {
	body := `{"hooks":{"WorktreeRemove":[{"hooks":[{"type":"command","command":"/x/rm.sh"}]}]}}`
	p := file(t, body)
	if _, err := Install(p, exe); err == nil || !strings.Contains(err.Error(), "WorktreeRemove hook that is not wk's") {
		t.Fatalf("err = %v", err)
	}
}
