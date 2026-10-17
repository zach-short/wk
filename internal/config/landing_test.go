package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestLandingDefaultsAreOff(t *testing.T) {
	l := Defaults().Landing
	if l.Enabled || l.Ref != "refs/heads/wk/landing" || l.Remote != "origin" || !l.Push || l.RequireReview {
		t.Fatalf("landing defaults = %+v", l)
	}
}

func TestLandingSectionIsAdditive(t *testing.T) {
	cfg, _, err := Load(write(t, `
[leases]
enabled = true

[landing]
enabled = true
ref = "refs/heads/queue"
push = false
require_review = true
`))
	if err != nil {
		t.Fatal(err)
	}
	l := cfg.Landing
	if !l.Enabled || l.Ref != "refs/heads/queue" || l.Remote != "origin" || l.Push || !l.RequireReview || !cfg.Leases.Enabled {
		t.Fatalf("landing = %+v, leases = %+v", l, cfg.Leases)
	}
}

func TestLandingValidation(t *testing.T) {
	for _, body := range []string{
		"[landing]\nref = \"wk/landing\"",
		"[landing]\nref = \"refs/heads/\"",
		"[landing]\nref = \"refs/tags/v1\"",
		"[landing]\nremote = \"\"",
		"[landing]\nff_main = true",
	} {
		if _, _, err := Load(write(t, body)); err == nil {
			t.Errorf("%q loaded; want a refusal", body)
		}
	}
}

func TestShowPrintsTheLandingSection(t *testing.T) {
	var out bytes.Buffer
	if err := Show(&out, Defaults(), "/x/demo.toml", false); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"[landing]", "ref = \"refs/heads/wk/landing\"", "require_review = false"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("show lacks %q:\n%s", want, out.String())
		}
	}
}
