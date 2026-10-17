package config

import (
	"bytes"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestLeasesDefaultsAreOff(t *testing.T) {
	l := Defaults().Leases
	if l.Enabled || l.Board != "PASSOFF.md" || !l.BashGuard || len(l.SingleWriter) != 0 {
		t.Fatalf("leases defaults = %+v", l)
	}
}

func TestLeasesSectionIsAdditive(t *testing.T) {
	cfg, _, err := Load(write(t, `
[pool]
size = 2

[leases]
enabled = true
single_writer = ["backend/main.go", "docs/"]
`))
	if err != nil {
		t.Fatal(err)
	}
	l := cfg.Leases
	if !l.Enabled || l.Board != "PASSOFF.md" || !l.BashGuard || !slices.Equal(l.SingleWriter, []string{"backend/main.go", "docs/"}) || cfg.Pool.Size != 2 {
		t.Fatalf("leases = %+v, pool = %+v", l, cfg.Pool)
	}
}

func TestLeasesValidation(t *testing.T) {
	for _, body := range []string{
		"[leases]\nboard = \"\"",
		"[leases]\nboard = \"/abs/PASSOFF.md\"",
		"[leases]\nboard = \"../PASSOFF.md\"",
		"[leases]\nsingle_writer = [\"./main.go\"]",
		"[leases]\nsingle_writer = [\"a//b\"]",
		"[leases]\nsingle_writer = [\"\"]",
		"[leases]\nsingle_writer = [\".\"]",
		"[leases]\nsingle_writer = [\"/etc/hosts\"]",
		"[leases]\nsingle_writer = [\"x/../y\"]",
		"[leases]\nrequire_claim = true",
	} {
		if _, _, err := Load(write(t, body)); err == nil {
			t.Errorf("%q loaded; want a refusal", body)
		}
	}
}

func TestShowPrintsTheEffectiveConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "none.toml")
	var out bytes.Buffer
	if err := Show(&out, Defaults(), path, false); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	for _, want := range []string{"# " + path + " (not found; these are the defaults)", "[leases]", `board = "PASSOFF.md"`, "bash_guard = true", "[template]"} {
		if !strings.Contains(got, want) {
			t.Errorf("show lacks %q:\n%s", want, got)
		}
	}
	back := write(t, got)
	cfg, found, err := Load(back)
	if err != nil || !found || cfg.Leases.Board != "PASSOFF.md" || cfg.Workspace.Base != "main" {
		t.Fatalf("show's output does not load back: %v %+v", err, cfg.Leases)
	}
}
