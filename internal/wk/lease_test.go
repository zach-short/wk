package wk

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/testrepo"
)

const leaseBoard = `# Board

| # | Task | Files it owns | Model |
|---|------|---------------|-------|
| 1 | web | ` + "`web/page.ts`; `docs/`; `packages/*/index.ts`; per item in the audit" + ` | Opus |
| 2 | overlap | ` + "`web/`" + ` | Opus |
| 3 | other | ` + "`README.md`; `nothing/*.go`" + ` | Opus |
`

// leaseFixture is a repository with leases on, a board, and two worktrees a and b.
func leaseFixture(t *testing.T) (f testrepo.Fixture, e *Env, a, b string) {
	t.Helper()
	f = testrepo.New(t)
	cfg := filepath.Join(f.Home, "demo.toml")
	if err := os.WriteFile(cfg, []byte(read(t, cfg)+"\n[leases]\nenabled = true\nsingle_writer = [\"PASSOFF.md\"]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.Write(t, "PASSOFF.md", leaseBoard)
	a, b = filepath.Join(f.Root, "a"), filepath.Join(f.Root, "b")
	f.Git(t, "worktree", "add", "--quiet", "-b", "a", a)
	f.Git(t, "worktree", "add", "--quiet", "-b", "b", b)
	return f, open(t, f, io.Discard), a, b
}

func mustClaim(t *testing.T, e *Env, dir, row string) string {
	t.Helper()
	var out bytes.Buffer
	if err := e.Claim(dir, row, &out); err != nil {
		t.Fatalf("claim %s in %s: %v", row, dir, err)
	}
	return out.String()
}

func TestClaimLeasesTheRowsCell(t *testing.T) {
	_, e, a, b := leaseFixture(t)
	got := mustClaim(t, e, a, "1")
	for _, want := range []string{"unread: ; per item in the audit", "claimed: row 1 in " + a, "lease: docs/", "lease: packages/core/index.ts", "lease: web/page.ts"} {
		if !strings.Contains(got, want) {
			t.Errorf("claim output lacks %q:\n%s", want, got)
		}
	}
	if err := e.Claim(b, "2", io.Discard); err == nil || !strings.Contains(err.Error(), "held by row 1 in "+a) {
		t.Fatalf("overlapping claim: %v; want a refusal naming row 1 and %s", err, a)
	}
	if got := mustClaim(t, e, a, "1"); !strings.HasPrefix(got, "unread: ; per item in the audit\nalready claimed: row 1") {
		t.Errorf("re-claim printed %q", got)
	}
	if err := e.Claim(a, "3", io.Discard); err == nil || !strings.Contains(err.Error(), "already claimed row 1") {
		t.Errorf("claim of a second row: %v", err)
	}
	if got := mustClaim(t, e, b, "3"); !strings.Contains(got, "unread: nothing/*.go (matches no tracked file)") {
		t.Errorf("claim 3 printed %q", got)
	}
}

func TestReleaseAndLeaseAdd(t *testing.T) {
	_, e, a, b := leaseFixture(t)
	mustClaim(t, e, a, "1")
	if err := e.ClaimNone(b, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := e.LeaseAdd(b, []string{"web/page.ts"}, io.Discard); err == nil {
		t.Fatal("lease add of a path row 1 holds succeeded")
	}
	if err := e.Release(a, io.Discard); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := e.LeaseAdd(filepath.Join(b, "web"), []string{"page.ts", b + "/docs/"}, &out); err != nil {
		t.Fatal(err)
	}
	if out.String() != "lease: web/page.ts\nlease: docs/\n" {
		t.Errorf("lease add printed %q", out.String())
	}
	out.Reset()
	if err := e.LeaseStatus(&out); err != nil || !strings.Contains(out.String(), b+"  row none") || strings.Contains(out.String(), a) {
		t.Errorf("status = %q, %v", out.String(), err)
	}
}

func TestAGoneWorktreeLosesItsClaim(t *testing.T) {
	f, e, a, b := leaseFixture(t)
	mustClaim(t, e, a, "1")
	f.Git(t, "worktree", "remove", "--force", a)
	mustClaim(t, e, b, "2")
}

func TestClaimRefusesAnotherRepositorysStore(t *testing.T) {
	_, e, a, _ := leaseFixture(t)
	if err := os.WriteFile(leasesPath(e.State), []byte(`{"common_dir": "/elsewhere/.git"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.Claim(a, "1", io.Discard); err == nil || !strings.Contains(err.Error(), "two repositories share the name demo") {
		t.Fatalf("claim over another repository's store: %v", err)
	}
}

func TestAStaleTempFileIsReaped(t *testing.T) {
	_, e, a, _ := leaseFixture(t)
	stale := filepath.Join(e.State, ".leases.json.123.tmp")
	if err := os.WriteFile(stale, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Minute)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}
	mustClaim(t, e, a, "1")
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale temp file still there: %v", err)
	}
}

// hook runs the lease hook on one call and returns the deny reason, empty on an allow.
func hook(t *testing.T, f testrepo.Fixture, tool, cwd, pathOrCommand string) string {
	t.Helper()
	ti := map[string]string{"file_path": pathOrCommand}
	if tool == "Bash" {
		ti = map[string]string{"command": pathOrCommand}
	}
	in, _ := json.Marshal(map[string]any{"cwd": cwd, "tool_name": tool, "tool_input": ti})
	var out bytes.Buffer
	LeaseHook(f.Home, []string{"--repo", f.Repo}, bytes.NewReader(in), &out)
	if out.Len() == 0 {
		return ""
	}
	var d hookDenial
	if err := json.Unmarshal(out.Bytes(), &d); err != nil || d.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("hook printed %q (%v)", out.String(), err)
	}
	return d.HookSpecificOutput.PermissionDecisionReason
}

func TestHookJudgesEditsAndBash(t *testing.T) {
	f, e, a, b := leaseFixture(t)
	mustClaim(t, e, a, "1")
	mustClaim(t, e, b, "3")
	heldByA := "web/page.ts is leased by row 1 in " + a
	for _, c := range []struct{ name, tool, cwd, arg, want string }{
		{"own lease", "Edit", a, a + "/web/page.ts", ""},
		{"leased elsewhere", "Edit", b, b + "/web/page.ts", heldByA},
		{"relative path", "Write", b + "/web", "page.ts", heldByA},
		{"sed -i", "Bash", b, "sed -i '' s/1/2/ web/page.ts", heldByA},
		{"no write target", "Bash", b, "ls web && cat web/page.ts", ""},
		{"a single wk command", "Bash", b, "wk claim 3", ""},
		{"a wk command where nothing is claimed", "Bash", f.Repo, `wk --repo "$PWD" claim --none`, ""},
		{"an expansion where nothing is claimed", "Bash", f.Repo, `touch "$PWD/x"`, "cannot tell which file"},
		{"wk then a write", "Bash", b, "wk status; rm web/page.ts", heldByA},
		{"single-writer", "Edit", b, b + "/PASSOFF.md", "PASSOFF.md is single-writer"},
		{"another worktree's file", "Edit", b, a + "/README.md", "is in the workspace " + a + ", and this session works in " + b},
		{"cd into another worktree", "Bash", b, "cd " + a + " && touch new.txt", "is in the workspace " + a},
		{"unclaimed main checkout", "Edit", f.Repo, f.Repo + "/README.md", "has not claimed a row; run `wk claim <row>` or `wk claim --none` in " + f.Repo},
		{"outside every worktree", "Edit", b, f.Root + "/elsewhere.txt", ""},
	} {
		if got := hook(t, f, c.tool, c.cwd, c.arg); c.want == "" && got != "" || !strings.Contains(got, c.want) {
			t.Errorf("%s: hook = %q; want %q", c.name, got, c.want)
		}
	}
	log := read(t, filepath.Join(f.Home, "demo", "hook.log"))
	if !strings.Contains(log, "lease Edit "+b+"/web/page.ts cwd="+b) || !strings.Contains(log, "deny: wk: "+heldByA) {
		t.Errorf("hook.log lacks the deny line:\n%s", log)
	}
}

// A worktree made after the last claim is not in the store's roots; the hook finds it on disk, so
// it is unclaimed rather than part of the main checkout.
func TestHookFindsAWorktreeTheStoreDoesNotList(t *testing.T) {
	f, e, a, _ := leaseFixture(t)
	if err := e.ClaimNone(f.Repo, io.Discard); err != nil {
		t.Fatal(err)
	}
	mustClaim(t, e, a, "1")
	c := filepath.Join(f.Repo, ".claude", "worktrees", "c")
	f.Git(t, "worktree", "add", "--quiet", "-b", "c", c)
	if got := hook(t, f, "Edit", c, c+"/README.md"); !strings.Contains(got, "has not claimed a row") || !strings.Contains(got, "in "+c) {
		t.Errorf("hook = %q; want the new worktree unclaimed", got)
	}
}

// A path through a symlink is judged where it leads (row 7 close-out, "Canonical paths").
func TestHookPlacesPathsThroughSymlinks(t *testing.T) {
	f, e, a, b := leaseFixture(t)
	mustClaim(t, e, a, "1")
	mustClaim(t, e, b, "3")
	link := filepath.Join(f.Root, "link")
	if err := os.Symlink(b, link); err != nil {
		t.Fatal(err)
	}
	want := "web/page.ts is leased by row 1 in " + a
	if got := hook(t, f, "Edit", link, link+"/web/page.ts"); !strings.Contains(got, want) {
		t.Errorf("edit through a link: %q", got)
	}
	if got := hook(t, f, "Bash", b, "rm "+link+"/web/page.ts"); !strings.Contains(got, want) {
		t.Errorf("rm through a link: %q", got)
	}
}

func TestHookWithLeasesOffAllowsAndLogs(t *testing.T) {
	f := testrepo.New(t)
	if got := hook(t, f, "Edit", f.Repo, f.Repo+"/README.md"); got != "" {
		t.Fatalf("hook = %q; want an allow", got)
	}
	if log := read(t, filepath.Join(f.Home, "demo", "hook.log")); !strings.Contains(log, "allow: leases.enabled is false") {
		t.Errorf("hook.log = %q", log)
	}
}

func TestHookFailsClosed(t *testing.T) {
	f, _, a, _ := leaseFixture(t)
	for _, c := range []struct {
		name, input string
		args        []string
		want        string
	}{
		{"no --repo", `{}`, nil, "the lease hook needs --repo"},
		{"bad input", `{`, []string{"--repo", f.Repo}, "cannot read the hook input"},
		{"other tool with no path", `{"cwd":"` + a + `","tool_name":"Frob","tool_input":{}}`, []string{"--repo", f.Repo}, "cannot tell which file"},
	} {
		var out bytes.Buffer
		LeaseHook(f.Home, c.args, strings.NewReader(c.input), &out)
		if !strings.Contains(out.String(), c.want) {
			t.Errorf("%s: hook printed %q; want %q", c.name, out.String(), c.want)
		}
	}
	cfg := filepath.Join(f.Home, "demo.toml")
	if err := os.WriteFile(cfg, []byte("[leases]\nenabled = true\nunknown = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := hook(t, f, "Edit", a, a+"/README.md"); !strings.Contains(got, "unknown keys: leases.unknown") {
		t.Errorf("broken config: hook = %q", got)
	}
}

func TestHooksInstallWritesTheLeaseEntriesOnlyWhenEnabled(t *testing.T) {
	e, f, _ := hooksTestSetup(t, "")
	local := filepath.Join(f.Repo, ".claude", "settings.local.json")
	if err := e.HooksInstall(hooksTestExe, io.Discard); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(read(t, local), "hook lease") {
		t.Fatalf("leases off, yet the lease hook was installed:\n%s", read(t, local))
	}
	e.Cfg.Leases.Enabled = true
	if err := e.HooksInstall(hooksTestExe, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := read(t, local)
	if strings.Count(got, hooksTestExe+" hook lease --repo "+f.Repo) != 2 || !strings.Contains(got, `"onFailure": "block"`) || !strings.Contains(got, "hook pre-tool-use") {
		t.Fatalf("leases on: settings =\n%s", got)
	}
}
