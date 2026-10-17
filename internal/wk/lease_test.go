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
		if got := hook(t, f, c.tool, c.cwd, c.arg); c.want == "" && got != "" || !containsFold(got, c.want) {
			t.Errorf("%s: hook = %q; want %q", c.name, got, c.want)
		}
	}
	log := read(t, filepath.Join(f.Home, "demo", "hook.log"))
	if !containsFold(log, "lease Edit "+b+"/web/page.ts cwd="+b) || !containsFold(log, "deny: wk: "+heldByA) {
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
	if got := hook(t, f, "Edit", c, c+"/README.md"); !containsFold(got, "has not claimed a row") || !containsFold(got, "in "+c) {
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
	if got := hook(t, f, "Edit", link, link+"/web/page.ts"); !containsFold(got, want) {
		t.Errorf("edit through a link: %q", got)
	}
	if got := hook(t, f, "Bash", b, "rm "+link+"/web/page.ts"); !containsFold(got, want) {
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

// containsFold is strings.Contains without letter case: on a volume that ignores case the hook
// folds every path, so its messages name them in lower case.
func containsFold(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// Audit 1, B1: on a volume that ignores case, another spelling of a leased file is that file.
func TestHookFoldsCaseOnACaseInsensitiveVolume(t *testing.T) {
	f, e, a, b := leaseFixture(t)
	if _, err := os.Stat(filepath.Join(f.Repo, ".GIT")); err != nil {
		t.Skip("this volume is case-sensitive: WEB/Page.ts is another file")
	}
	mustClaim(t, e, a, "1")
	mustClaim(t, e, b, "3")
	want := "web/page.ts is leased by row 1 in " + a
	for _, c := range []struct{ tool, arg string }{{"Edit", b + "/WEB/Page.ts"}, {"Bash", "rm Web/page.ts"}} {
		if got := hook(t, f, c.tool, b, c.arg); !containsFold(got, want) {
			t.Errorf("%s %s: hook = %q", c.tool, c.arg, got)
		}
	}
}

// Audit 1, B2: a session whose cwd is in no worktree may write no worktree.
func TestHookDeniesAWriteFromOutsideEveryWorkspace(t *testing.T) {
	f, e, a, _ := leaseFixture(t)
	mustClaim(t, e, a, "1")
	want := "this session works outside every workspace of the repository"
	for _, c := range []struct{ tool, arg string }{{"Edit", a + "/web/page.ts"}, {"Bash", "rm " + a + "/web/page.ts"}} {
		if got := hook(t, f, c.tool, f.Root, c.arg); !containsFold(got, want) {
			t.Errorf("%s from %s: hook = %q", c.tool, f.Root, got)
		}
	}
	if got := hook(t, f, "Edit", f.Root, f.Root+"/elsewhere.txt"); got != "" {
		t.Errorf("a path outside every workspace: hook = %q; want an allow", got)
	}
}

// Audit 1, B3 and B4: a dangling link and a link followed by `..` are judged where the kernel
// writes; a link loop denies.
func TestHookFollowsLinksAsTheKernelDoes(t *testing.T) {
	f, e, a, b := leaseFixture(t)
	mustClaim(t, e, a, "1")
	mustClaim(t, e, b, "3")
	if err := e.LeaseAdd(a, []string{"web/new.ts"}, io.Discard); err != nil {
		t.Fatal(err)
	}
	link := func(target, name string) {
		t.Helper()
		_ = os.MkdirAll(filepath.Dir(name), 0o755)
		if err := os.Symlink(target, name); err != nil {
			t.Fatal(err)
		}
	}
	link(a+"/web/new.ts", b+"/dang.ts")
	link(a+"/docs", b+"/sub/e")
	link(b+"/loop2", b+"/loop1")
	link(b+"/loop1", b+"/loop2")
	other := "is in the workspace " + a
	for _, c := range []struct{ tool, arg, want string }{
		{"Write", b + "/dang.ts", other},
		{"Bash", "echo x > dang.ts", other},
		{"Edit", b + "/sub/e/../web/page.ts", other},
		{"Bash", "rm sub/e/../web/page.ts", other},
		{"Write", b + "/loop1", "a symlink loop or an unreadable link"},
		{"Bash", "touch loop1", "a symlink loop or an unreadable link"},
	} {
		if got := hook(t, f, c.tool, b, c.arg); !containsFold(got, c.want) {
			t.Errorf("%s %s: hook = %q; want %q", c.tool, c.arg, got, c.want)
		}
	}
}

// Audit 1, N2: a panic inside the hook is a deny, never silence.
func TestHookPanicIsADeny(t *testing.T) {
	f, _, _, _ := leaseFixture(t)
	var out bytes.Buffer
	LeaseHook(f.Home, []string{"--repo", f.Repo}, nil, &out)
	if !strings.Contains(out.String(), `"permissionDecision":"deny"`) || !strings.Contains(out.String(), "the lease hook failed") {
		t.Fatalf("hook printed %q", out.String())
	}
}

// Audit 2, B5, B6 and B7: a cd through a link and then `..`, a link at a leased path, and an edit
// path a tool may clean by text before it writes.
func TestHookJudgesEverySpellingOfAPath(t *testing.T) {
	f, e, a, b := leaseFixture(t)
	mustClaim(t, e, a, "1")
	mustClaim(t, e, b, "3")
	_ = os.MkdirAll(filepath.Join(f.Root, "outside", "d"), 0o755)
	_ = os.MkdirAll(filepath.Join(b, "docs", "inner"), 0o755)
	for target, name := range map[string]string{
		a + "/docs":           b + "/sub/e",
		"../web/x.ts":         b + "/docs/lnk",
		f.Root + "/outside/d": b + "/out",
		"docs/inner":          b + "/p",
		"../docs/inner":       b + "/web/q",
	} {
		_ = os.MkdirAll(filepath.Dir(name), 0o755)
		if err := os.Symlink(target, name); err != nil {
			t.Fatal(err)
		}
	}
	other, docs := "is in the workspace "+a, "docs/lnk is leased by row 1 in "+a
	for _, c := range []struct{ tool, arg, want string }{
		{"Bash", "cd sub/e/../web && touch page.ts", other},
		{"Bash", "cd -P sub/e/../web && echo x > new.ts", other},
		{"Bash", "rm docs/lnk", docs},
		{"Bash", "git rm docs/lnk", "is leased by row 1 in " + a},
		{"Bash", "mv docs/lnk web/z.ts", docs},
		{"Bash", "ln -sf ../web/page.ts docs/lnk", docs},
		{"Edit", b + "/out/../web/page.ts", "web/page.ts is leased by row 1 in " + a},
		{"Bash", "rm p/../lnk", docs},
		{"Bash", "git rm out/../web/page.ts", "`out/../web/page.ts` cannot be placed"},
		{"Bash", "cd -P p/.. && rm lnk", docs},
	} {
		if got := hook(t, f, c.tool, b, c.arg); !containsFold(got, c.want) {
			t.Errorf("%s %s: hook = %q; want %q", c.tool, c.arg, got, c.want)
		}
	}
	if got := hook(t, f, "Bash", b+"/web", "cd q && rm ../lnk"); !containsFold(got, docs) {
		t.Errorf("cd q && rm ../lnk from web: hook = %q; want %q", got, docs)
	}
	// Audit 3, N11: a shell never cleans `..` by text, so this writes outside every worktree.
	if got := hook(t, f, "Bash", b, "echo x > out/../web/page.ts"); got != "" {
		t.Errorf("a Bash write through a link to outside: hook = %q; want an allow", got)
	}
	if got := hook(t, f, "Edit", b, b+"/out/x.ts"); got != "" {
		t.Errorf("an edit through a link to outside every worktree: hook = %q; want an allow", got)
	}
}
