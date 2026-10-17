package settings

import (
	"strings"
	"testing"
)

const primary = "/Users/my dir/proj"

func TestInstallAddsTheTwoLeaseEntries(t *testing.T) {
	p := file(t, commitGuard)
	if _, err := Install(p, exe, LeaseEntries(primary)...); err != nil {
		t.Fatal(err)
	}
	groups := preToolUse(t, p)
	if len(groups) != 4 || !strings.Contains(mustJSON(t, groups[0]), "/x/commit-guard.sh") {
		t.Fatalf("PreToolUse = %s; want the commit guard, the slots hook and the two lease entries", mustJSON(t, groups))
	}
	edit := groups[2].(map[string]any)
	hook := edit["hooks"].([]any)[0].(map[string]any)
	wantCmd := exe + ` hook lease --repo '/Users/my dir/proj'`
	if edit["matcher"] != "Edit|Write|MultiEdit|NotebookEdit" || hook["command"] != wantCmd || hook["onFailure"] != "block" || hook["timeout"] != float64(5) {
		t.Fatalf("edit entry = %s", mustJSON(t, edit))
	}
	bash := groups[3].(map[string]any)
	hook = bash["hooks"].([]any)[0].(map[string]any)
	if bash["matcher"] != "Bash" || hook["command"] != wantCmd+guard || hook["onFailure"] != nil {
		t.Fatalf("bash entry = %s", mustJSON(t, bash))
	}
	for _, g := range groups[1:] {
		cmd := g.(map[string]any)["hooks"].([]any)[0].(map[string]any)["command"].(string)
		if !IsWK(cmd) {
			t.Errorf("IsWK(%q) = false", cmd)
		}
	}
}

func TestInstallWithoutLeasesRemovesThem(t *testing.T) {
	p := file(t, commitGuard)
	if _, err := Install(p, exe, LeaseEntries(primary)...); err != nil {
		t.Fatal(err)
	}
	changed, err := Install(p, exe, LeaseEntries(primary)...)
	if err != nil || changed {
		t.Fatalf("second install: changed = %v, err = %v; want no change", changed, err)
	}
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	if groups := preToolUse(t, p); len(groups) != 2 || strings.Contains(mustJSON(t, groups), "hook lease") {
		t.Fatalf("PreToolUse = %s; want the commit guard and the slots hook only", mustJSON(t, groups))
	}
}

func TestUninstallRemovesTheLeaseEntries(t *testing.T) {
	p := file(t, commitGuard)
	if _, err := Install(p, exe, LeaseEntries(primary)...); err != nil {
		t.Fatal(err)
	}
	if _, err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	if groups := preToolUse(t, p); len(groups) != 1 || !strings.Contains(mustJSON(t, groups), "/x/commit-guard.sh") {
		t.Fatalf("PreToolUse = %s; want the commit guard alone", mustJSON(t, groups))
	}
}

func TestIsWKOnTheLeaseHook(t *testing.T) {
	for cmd, want := range map[string]bool{
		"/u/go/bin/wk hook lease --repo /p":          true,
		"'/a b/wk' hook lease --repo '/x y' || true": true,
		"/u/go/bin/other hook lease --repo /p":       false,
	} {
		if got := IsWK(cmd); got != want {
			t.Errorf("IsWK(%q) = %v, want %v", cmd, got, want)
		}
	}
}
