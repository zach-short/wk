package wk

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

const hooksTestExe = "/opt/wk/bin/wk"

const hooksTestSlots = `[[slots.match]]
spelling = "bun run build"
class = "heavy"
trim_after = ["web/.next/cache"]

[[slots.match]]
spelling = "bunx tsc"
class = "check"

[[slots.match]]
spelling = "go test"
class = "check"
`

// hooksTestSetup gives the install a user settings file under a temporary HOME, so no test reads
// the owner's ~/.claude/settings.json, plus a tracked and a local settings file and a slots file.
func hooksTestSetup(t *testing.T, slotsTOML string) (*Env, testrepo.Fixture, map[string]string) {
	t.Helper()
	f := testrepo.New(t)
	t.Setenv("HOME", t.TempDir())
	files := map[string]string{
		filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"): `{"permissions": {"allow": ["Bash(bun *)"]}}`,
		filepath.Join(f.Repo, ".claude", "settings.json"):            `{"permissions": {"deny": ["Bash(go test *)"]}}`,
		filepath.Join(f.Repo, ".claude", "settings.local.json"):      `{"permissions": {"ask": ["Bash(bunx tsc *)"]}}`,
		filepath.Join(f.Home, "demo.slots.toml"):                     slotsTOML,
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return open(t, f, io.Discard), f, files
}

func TestHooksInstallWritesTheTwinsOnceBesideTheHook(t *testing.T) {
	e, f, files := hooksTestSetup(t, hooksTestSlots)
	local := filepath.Join(f.Repo, ".claude", "settings.local.json")
	if err := e.HooksInstall(hooksTestExe, io.Discard); err != nil {
		t.Fatal(err)
	}
	first := read(t, local)
	for _, want := range []string{
		`"Bash(/opt/wk/bin/wk run --slot heavy --trim web/.next/cache -- bun *)"`,
		`"Bash(/opt/wk/bin/wk run --slot check -- bunx tsc *)"`,
		`"Bash(/opt/wk/bin/wk run --slot check -- go test *)"`,
		`"command": "/opt/wk/bin/wk hook pre-tool-use || true"`,
		`"matcher": "Bash"`,
	} {
		if n := strings.Count(first, want); n != 1 {
			t.Errorf("%s appears %d times, want once:\n%s", want, n, first)
		}
	}
	if n := strings.Count(first, "run --slot"); n != 3 {
		t.Errorf("%d twins, want 3:\n%s", n, first)
	}
	var out bytes.Buffer
	if err := e.HooksInstall(hooksTestExe, &out); err != nil {
		t.Fatal(err)
	}
	if read(t, local) != first || !strings.Contains(out.String(), local+": unchanged") {
		t.Errorf("a second install changed the file; it printed %q", out.String())
	}
	for path, content := range files {
		if path != local && read(t, path) != content {
			t.Errorf("%s was written; wk only reads it", path)
		}
	}
}

// With no slots file the hook wraps nothing, so the twins of an earlier install go and the hook
// entries stay.
func TestHooksInstallWithoutASlotsFileRemovesTheTwins(t *testing.T) {
	e, f, _ := hooksTestSetup(t, hooksTestSlots)
	if err := e.HooksInstall(hooksTestExe, io.Discard); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(f.Home, "demo.slots.toml")); err != nil {
		t.Fatal(err)
	}
	if err := e.HooksInstall(hooksTestExe, io.Discard); err != nil {
		t.Fatal(err)
	}
	got := read(t, filepath.Join(f.Repo, ".claude", "settings.local.json"))
	if strings.Contains(got, "run --slot") || !strings.Contains(got, "hook pre-tool-use || true") {
		t.Errorf("want the hook and no twin:\n%s", got)
	}
}

// A broken slots file or settings source stops the install before the file changes: a hook
// written without its deny twins would let a refused gate run with no sign.
func TestHooksInstallRefusesABrokenSourceAndWritesNothing(t *testing.T) {
	for name, broken := range map[string]func(f testrepo.Fixture) (string, string){
		"slots file": func(f testrepo.Fixture) (string, string) {
			return filepath.Join(f.Home, "demo.slots.toml"), "[[slots.match]]\nspeling = \"go test\"\n"
		},
		"user settings": func(testrepo.Fixture) (string, string) {
			return filepath.Join(os.Getenv("HOME"), ".claude", "settings.json"), "{"
		},
		"local settings": func(f testrepo.Fixture) (string, string) {
			return filepath.Join(f.Repo, ".claude", "settings.local.json"), "[]"
		},
	} {
		t.Run(name, func(t *testing.T) {
			e, f, _ := hooksTestSetup(t, hooksTestSlots)
			path, content := broken(f)
			if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
				t.Fatal(err)
			}
			local := filepath.Join(f.Repo, ".claude", "settings.local.json")
			before := read(t, local)
			if err := e.HooksInstall(hooksTestExe, io.Discard); err == nil {
				t.Fatal("the install went through")
			}
			if read(t, local) != before {
				t.Errorf("the refused install changed %s", local)
			}
		})
	}
}
