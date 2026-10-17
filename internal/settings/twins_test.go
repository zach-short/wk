package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const (
	heavyTrim = exe + " run --slot heavy --trim web/.next/cache -- "
	heavy     = exe + " run --slot heavy -- "
	check     = exe + " run --slot check -- "
)

// Spellings of SLOTS-PLAN.md section 3, each with the prefix its class and trim give.
var gateTwins = []Twin{
	{Spelling: "bun run build", Prefix: heavyTrim},
	{Spelling: "next build", Prefix: heavyTrim},
	{Spelling: "bun run e2e", Prefix: heavy},
	{Spelling: "go run ./cmd/lintgate", Prefix: heavy},
	{Spelling: "bun run lint", Prefix: check},
	{Spelling: "bun run test", Prefix: check},
	{Spelling: "bunx tsc", Prefix: check},
	{Spelling: "go test", Prefix: check},
}

func bash(body string) string { return "Bash(" + body + ")" }

func source(t *testing.T, lists map[string][]string) []byte {
	t.Helper()
	data, err := json.Marshal(map[string]any{"permissions": lists})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func rules(t *testing.T, path, list string) []string {
	t.Helper()
	perms, _ := load(t, path)["permissions"].(map[string]any)
	raw, _ := perms[list].([]any)
	out := make([]string, len(raw))
	for i, r := range raw {
		out[i], _ = r.(string)
	}
	return out
}

func twinsInto(t *testing.T, path string, sources ...[]byte) bool {
	t.Helper()
	changed, err := InstallTwins(path, sources, gateTwins)
	if err != nil {
		t.Fatal(err)
	}
	return changed
}

func wantRules(t *testing.T, path, list string, want ...string) {
	t.Helper()
	if got := rules(t, path, list); !slices.Equal(got, want) {
		t.Fatalf("%s = %q,\nwant %q", list, got, want)
	}
}

// SB-17's own example: Bash(bun *) in the user's file gains its three wrapped twins in a local
// file that has no permissions key yet, and the user's and the tracked file are only read.
func TestTwinsOfBunStar(t *testing.T) {
	p := file(t, `{"availableModels":["opus"],"hooks":{"SessionStart":[]}}`)
	dir := filepath.Dir(p)
	user := source(t, map[string][]string{"allow": {bash("bun *")}})
	tracked := source(t, map[string][]string{"allow": {bash("git status:*")}})
	local, _ := os.ReadFile(p)
	if !twinsInto(t, p, user, tracked, local) {
		t.Fatal("changed = false")
	}
	wantRules(t, p, "allow", bash(heavyTrim+"bun *"), bash(heavy+"bun *"), bash(check+"bun *"))
	data, _ := os.ReadFile(p)
	if i, j := strings.Index(string(data), `"hooks"`), strings.Index(string(data), `"permissions"`); i < 0 || j < i {
		t.Fatalf("permissions not appended after the other keys:\n%s", data)
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 1 {
		t.Fatalf("%d files beside the settings file, want none", len(entries)-1)
	}
}

func TestTwinsOfAnAskAndADenyRule(t *testing.T) {
	p := file(t, "{}")
	user := source(t, map[string][]string{
		"allow": {bash("git log *")},
		"ask":   {bash("go test *")},
		"deny":  {bash("bun run e2e:*")},
	})
	twinsInto(t, p, user)
	wantRules(t, p, "allow")
	wantRules(t, p, "ask", bash(check+"go test *"))
	wantRules(t, p, "deny", bash(heavy+"bun run e2e:*"))
}

// Section 0.2 proved Bash(/bin/echo PROBE_REWRITTEN:*): an absolute path first, the
// wildcard last. Both trailing forms appear in the owner's files, and each twin keeps its own.
func TestTwinSyntax(t *testing.T) {
	p := file(t, "")
	twinsInto(t, p, source(t, map[string][]string{"allow": {bash("bun run build:*"), bash("go test *")}}))
	wantRules(t, p, "allow",
		"Bash(/Users/me/go/bin/wk run --slot heavy --trim web/.next/cache -- bun run build:*)",
		"Bash(/Users/me/go/bin/wk run --slot check -- go test *)",
	)
}

// A deny rule narrower than a spelling still matches commands the hook wraps, so it gets a
// twin; otherwise the wrapped command would no longer be refused.
func TestARuleNarrowerThanASpellingGetsATwin(t *testing.T) {
	p := file(t, "")
	twinsInto(t, p, source(t, map[string][]string{"deny": {
		bash("bun run build --prod"),
		bash("go test * -count=0"),
		bash("bun run lint --fix *"),
	}}))
	wantRules(t, p, "deny",
		bash(heavyTrim+"bun run build --prod"),
		bash(check+"go test * -count=0"),
		bash(check+"bun run lint --fix *"),
	)
}

func TestARuleThatCoversNoSpellingGetsNoTwin(t *testing.T) {
	for _, rule := range []string{
		bash("git status:*"),
		bash("bun install"),
		bash("bun run builder"),
		bash("bun run build-storybook"),
		bash("bun run builder*"),
		bash("bunx eslint *"),
		bash("go testify"),
		"Bash",
		"Bash()",
		"Read(./web/**)",
		// These already match the wrapped command themselves.
		bash("* --version"),
		bash("*"),
		bash(exe + " run *"),
		// A twin is never made of a twin.
		bash(check + "go test *"),
	} {
		t.Run(rule, func(t *testing.T) {
			p := file(t, "")
			if twinsInto(t, p, source(t, map[string][]string{"allow": {rule}, "deny": {rule}})) {
				t.Fatalf("%s got a twin:\n%s", rule, mustJSON(t, load(t, p)))
			}
			if _, err := os.Stat(p); !os.IsNotExist(err) {
				t.Fatal("a file with nothing to hold was created")
			}
		})
	}
}

// A twin already there keeps its place, a rule held in two sources gets one twin, and a second
// run changes no byte.
func TestATwinAlreadyPresentIsNotDoubled(t *testing.T) {
	p := file(t, `{"permissions":{"allow":["Bash(bun *)","`+bash(heavy+"bun *")+`","Bash(git diff *)"]}}`)
	local, _ := os.ReadFile(p)
	user := source(t, map[string][]string{"allow": {bash("bun *")}})
	twinsInto(t, p, user, local)
	wantRules(t, p, "allow",
		bash("bun *"), bash(heavy+"bun *"), bash("git diff *"),
		bash(heavyTrim+"bun *"), bash(check+"bun *"),
	)
	before, _ := os.ReadFile(p)
	if twinsInto(t, p, user, before) {
		t.Fatal("a second run reported a change")
	}
	if after, _ := os.ReadFile(p); string(after) != string(before) {
		t.Fatal("a second run rewrote the file")
	}
}

// A twin whose original rule is gone is removed at the next install, so revoking the rule
// revokes the wrapped form too.
func TestATwinWithoutItsRuleIsRemoved(t *testing.T) {
	p := file(t, `{"permissions":{"allow":["Bash(git diff *)","`+bash(check+"bun *")+`"],"ask":[]}}`)
	local, _ := os.ReadFile(p)
	if !twinsInto(t, p, local) {
		t.Fatal("changed = false")
	}
	wantRules(t, p, "allow", bash("git diff *"))
	if _, ok := load(t, p)["permissions"].(map[string]any)["ask"]; !ok {
		t.Fatal("the owner's empty ask list was removed")
	}
}

// The fixture holds what wk must never change: another tool's hook, the owner's rules with
// characters json.Marshal would escape, a non-string rule, an empty list, and a rule that names
// a relative wk, which wk never writes.
const owners = `{
  "permissions": {
    "allow": [
      "Bash(bun *)",
      "Bash(cd web && bun run lint 2>&1 | tail -5)",
      "Bash(wk run --slot heavy -- make)",
      7
    ],
    "ask": [],
    "deny": [
      "Bash(bunx tsc --build *)"
    ]
  },
  "availableModels": [
    "opus"
  ],
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          {
            "type": "command",
            "command": "/x/commit-guard.sh"
          }
        ]
      }
    ]
  }
}
`

func TestUninstallRemovesTwinsAndNothingElse(t *testing.T) {
	p := file(t, owners)
	if _, err := Install(p, exe); err != nil {
		t.Fatal(err)
	}
	user := source(t, map[string][]string{"allow": {bash("go test *")}})
	twinsInto(t, p, user, []byte(owners))
	wantRules(t, p, "allow",
		"Bash(bun *)", "Bash(cd web && bun run lint 2>&1 | tail -5)", "Bash(wk run --slot heavy -- make)", "",
		bash(check+"go test *"), bash(heavyTrim+"bun *"), bash(heavy+"bun *"), bash(check+"bun *"),
	)
	wantRules(t, p, "deny", "Bash(bunx tsc --build *)", bash(check+"bunx tsc --build *"))
	if data, _ := os.ReadFile(p); strings.Contains(string(data), `\u00`) {
		t.Fatalf("a kept rule was escaped:\n%s", data)
	}
	if changed, err := Uninstall(p); err != nil || !changed {
		t.Fatalf("changed = %v, err = %v", changed, err)
	}
	if data, _ := os.ReadFile(p); string(data) != owners {
		t.Fatalf("uninstall left:\n%s\nwant the file as it was:\n%s", data, owners)
	}
}

func TestUninstallRemovesATwinOfAQuotedWK(t *testing.T) {
	quoted := Command("/Users/my dir/wk", "run --slot check -- ")
	p := file(t, `{"permissions":{"allow":["`+bash(quoted+"go test *")+`","Bash(/bin/echo /x/wk run --slot check)"]}}`)
	if _, err := Uninstall(p); err != nil {
		t.Fatal(err)
	}
	wantRules(t, p, "allow", "Bash(/bin/echo /x/wk run --slot check)")
}

func TestInstallTwinsRefusals(t *testing.T) {
	good := source(t, map[string][]string{"allow": {bash("bun *")}})
	for name, tc := range map[string]struct {
		local   string
		sources [][]byte
		twins   []Twin
		want    string
	}{
		"broken source":       {"{}", [][]byte{good, []byte(`{"permissions": `)}, gateTwins, "settings source 2"},
		"permissions string":  {"{}", [][]byte{[]byte(`{"permissions":"all"}`)}, gateTwins, "permissions: not a JSON object"},
		"allow object":        {"{}", [][]byte{[]byte(`{"permissions":{"allow":{}}}`)}, gateTwins, "permissions.allow"},
		"broken local":        {`{"permissions": `, [][]byte{good}, gateTwins, "will not rewrite it"},
		"local allow string":  {`{"permissions":{"allow":"x"}}`, [][]byte{good}, gateTwins, "permissions.allow"},
		"relative prefix":     {"{}", [][]byte{good}, []Twin{{Spelling: "bun run build", Prefix: "wk run --slot heavy -- "}}, "absolute path to wk"},
		"prefix not run slot": {"{}", [][]byte{good}, []Twin{{Spelling: "bun run build", Prefix: exe + " run -- "}}, "absolute path to wk"},
		"empty spelling":      {"{}", [][]byte{good}, []Twin{{Spelling: " ", Prefix: heavy}}, "needs a spelling"},
	} {
		t.Run(name, func(t *testing.T) {
			p := file(t, tc.local)
			if _, err := InstallTwins(p, tc.sources, tc.twins); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want %q", err, tc.want)
			}
			if data, _ := os.ReadFile(p); string(data) != tc.local {
				t.Fatal("a refused install changed the file")
			}
		})
	}
}
