package settings

import (
	"os"
	"strings"
	"testing"
)

const guarded = exe + " hook pre-tool-use || true"

// The owner's commit guard is the Bash hook that already sits on PreToolUse (SLOTS-PLAN.md 0.2).
const commitGuard = `{
  "hooks": {
    "PreToolUse": [
      { "matcher": "Bash", "hooks": [ { "type": "command", "command": "/x/commit-guard.sh" } ] }
    ]
  }
}
`

func preToolUse(t *testing.T, path string) []any {
	t.Helper()
	hooks, _ := load(t, path)["hooks"].(map[string]any)
	groups, _ := hooks["PreToolUse"].([]any)
	return groups
}

func TestInstallAddsThePreToolUseEntryBesideTheOwnersHook(t *testing.T) {
	p := file(t, commitGuard)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	groups := preToolUse(t, p)
	if len(groups) != 2 || !strings.Contains(mustJSON(t, groups[0]), "/x/commit-guard.sh") {
		t.Fatalf("PreToolUse = %s, want the commit guard's group, then wk's", mustJSON(t, groups))
	}
	group := groups[1].(map[string]any)
	entry := group["hooks"].([]any)[0].(map[string]any)
	if group["matcher"] != "Bash" || entry["command"] != guarded || entry["timeout"] != float64(5) || entry["type"] != "command" {
		t.Fatalf("wk's group = %s", mustJSON(t, group))
	}
}

// The matcher leads the group, as in Claude Code's own files, and the hook keeps the sorted keys
// earlier wk versions wrote.
func TestThePreToolUseGroupsBytes(t *testing.T) {
	p := file(t, "")
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	want := `
      {
        "matcher": "Bash",
        "hooks": [
          {
            "command": "` + guarded + `",
            "timeout": 5,
            "type": "command"
          }
        ]
      }`
	if data, _ := os.ReadFile(p); !strings.Contains(string(data), want) {
		t.Fatalf("no group %s in:\n%s", want, data)
	}
}

func TestInstallOfThePreToolUseEntryIsIdempotent(t *testing.T) {
	p := file(t, commitGuard)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(p)
	changed, err := Install(p, exe)
	if err != nil || changed {
		t.Fatalf("second install: changed = %v, err = %v", changed, err)
	}
	if after, _ := os.ReadFile(p); string(after) != string(before) {
		t.Fatal("a second install rewrote the file")
	}
	if n := strings.Count(string(before), "hook pre-tool-use"); n != 1 {
		t.Fatalf("%d pre-tool-use entries, want 1", n)
	}
}

func TestInstallReplacesAnUnguardedPreToolUseEntry(t *testing.T) {
	p := file(t, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/old/wk hook pre-tool-use"}]}]}}`)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	groups := preToolUse(t, p)
	if len(groups) != 1 || !strings.Contains(mustJSON(t, groups), guarded) || strings.Contains(mustJSON(t, groups), "/old/") {
		t.Fatalf("PreToolUse = %s, want only the guarded entry", mustJSON(t, groups))
	}
}

func TestUninstallRemovesThePreToolUseEntryAndKeepsTheOwnersHook(t *testing.T) {
	p := file(t, commitGuard)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	if changed, err := Uninstall(p); err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	groups := preToolUse(t, p)
	if len(groups) != 1 || !strings.Contains(mustJSON(t, groups), "/x/commit-guard.sh") || strings.Contains(mustJSON(t, groups), "pre-tool-use") {
		t.Fatalf("after uninstall PreToolUse = %s", mustJSON(t, groups))
	}
}

// A group another tool shares with wk keeps that tool's hook and its matcher when wk's goes.
func TestUninstallFromASharedPreToolUseGroup(t *testing.T) {
	p := file(t, `{"hooks":{"PreToolUse":[{"matcher":"Bash","hooks":[{"type":"command","command":"/x/commit-guard.sh"},{"type":"command","command":"`+guarded+`","timeout":5}]}]}}`)
	if _, err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	groups := preToolUse(t, p)
	if len(groups) != 1 || mustJSON(t, groups[0]) != `{"hooks":[{"command":"/x/commit-guard.sh","type":"command"}],"matcher":"Bash"}` {
		t.Fatalf("PreToolUse = %s", mustJSON(t, groups))
	}
}

func TestIsWKOnThePlainAndTheGuardedForms(t *testing.T) {
	for cmd, want := range map[string]bool{
		exe + " hook pre-tool-use": true,
		guarded:                    true,
		guarded + "  ":             true,
		"'/Users/my dir/wk' hook pre-tool-use || true":    true,
		"/x/wk hook worktree-create || true":              true,
		"WK_HOOK_FAIL=1 /x/wk hook subagent-stop || true": true,
		"/x/notwk hook pre-tool-use || true":              false,
		"/x/wk hook pre-tool-use || false":                false,
		"/x/wk hook pre-tool-use; true":                   false,
		"/x/commit-guard.sh || true":                      false,
		"|| true":                                         false,
	} {
		if got := IsWK(cmd); got != want {
			t.Errorf("IsWK(%q) = %v, want %v", cmd, got, want)
		}
	}
	for _, en := range Entries {
		if got := en.command("/Users/my dir/wk"); !IsWK(got) {
			t.Errorf("IsWK(%q) = false for wk's own %s entry", got, en.Event)
		}
	}
}
