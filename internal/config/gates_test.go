package config

import (
	"bytes"
	"strings"
	"testing"
)

func TestGatesDefaults(t *testing.T) {
	g := Defaults().Gates
	if g.TrialDir != ".claude/worktrees/wk-trial" || g.BatchMax != 1 || g.BatchIdleSeconds != 120 || g.MinFreeMB != 2048 {
		t.Fatalf("gates defaults = %+v", g)
	}
	if strings.Join(g.FreshCaches, ",") != "GOCACHE,GOLANGCI_LINT_CACHE" || len(g.Run) != 0 {
		t.Fatalf("gates defaults = %+v", g)
	}
}

// The live ezhomesteading file opens [gates.env] before any other gates key, so the [gates]
// table and the [[gates.run]] entries come after it; the load must take that order.
func TestGatesSectionIsAdditive(t *testing.T) {
	cfg, _, err := Load(write(t, `
[gates.env]
API_URL = "https://ci.invalid"

[landing]
enabled = true
ff_main = false

[gates]
batch_max = 2
batch_idle_seconds = 0
min_free_mb = 0
fresh_caches = ["GOCACHE"]

[[gates.run]]
name = "vet"
dir = "backend"
run = ["go", "vet", "./..."]

[[gates.run]]
name = "sqlgate"
run = ["go", "run", "./cmd/sqlgate"]
needs_env = ["EZH_TEST_DB"]
`))
	if err != nil {
		t.Fatal(err)
	}
	g := cfg.Gates
	if g.Env["API_URL"] != "https://ci.invalid" || g.BatchMax != 2 || g.BatchIdleSeconds != 0 || g.MinFreeMB != 0 || cfg.Landing.FFMain {
		t.Fatalf("gates = %+v, landing = %+v", g, cfg.Landing)
	}
	if g.TrialDir != ".claude/worktrees/wk-trial" || len(g.Run) != 2 || g.Run[0].Dir != "backend" || g.Run[1].NeedsEnv[0] != "EZH_TEST_DB" {
		t.Fatalf("gates = %+v", g)
	}
}

func TestGatesValidation(t *testing.T) {
	for _, body := range []string{
		"[gates]\ntrial_dir = \"\"",
		"[gates]\ntrial_dir = \"/tmp/trial\"",
		"[gates]\ntrial_dir = \"../trial\"",
		"[gates]\nbatch_max = 0",
		"[gates]\nbatch_idle_seconds = -1",
		"[gates]\nmin_free_mb = -1",
		"[gates]\nfresh_caches = [\"\"]",
		"[[gates.run]]\nname = \"vet\"\nrun = []",
		"[[gates.run]]\nname = \"vet\"",
		"[[gates.run]]\nrun = [\"go\", \"vet\"]",
		"[[gates.run]]\nname = \"vet\"\ndir = \"../x\"\nrun = [\"go\", \"vet\"]",
		"[[gates.run]]\nname = \"vet\"\nrun = [\"go\"]\nshell = true",
	} {
		if _, _, err := Load(write(t, body)); err == nil {
			t.Errorf("%q loaded; want a refusal", body)
		}
	}
	// Row 15 (row 5's N2 and the timeout): each refusal names the entry it refuses.
	const vet = "[[gates.run]]\nname = \"vet\"\nrun = [\"go\", \"vet\"]\n"
	for _, tc := range []struct{ body, want string }{
		{vet + "needs_env = [\"\"]", "gates.run[0].needs_env[0] (vet)"},
		{vet + "needs_env = [\"A=B\"]", "gates.run[0].needs_env[0] (vet)"},
		{vet + "needs_env = [\"EZH_TEST_DB\", \"A B\"]", "gates.run[0].needs_env[1] (vet)"},
		{vet + vet, "gates.run[1]: the name \"vet\" is gates.run[0]'s too"},
		{"[gates]\ntrial_dir = \".\"", "gates.trial_dir \".\" is the main checkout itself"},
		{"[gates]\ntrial_dir = \"./\"", "gates.trial_dir \"./\" is the main checkout itself"},
		{"[gates]\ntrial_dir = \"a/..\"", "gates.trial_dir"},
		{vet + "timeout_seconds = -1", "gates.run[0].timeout_seconds (vet) is -1"},
	} {
		if _, _, err := Load(write(t, tc.body)); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q: Load = %v; want a refusal naming %q", tc.body, err, tc.want)
		}
	}
	if _, _, err := Load(write(t, vet+"timeout_seconds = 600\nneeds_env = [\"EZH_TEST_DB\"]")); err != nil {
		t.Errorf("a gate with a timeout and a needs_env variable was refused: %v", err)
	}
}

func TestShowPrintsTheGatesSection(t *testing.T) {
	cfg := Defaults()
	cfg.Gates.Run = []GateRun{{Name: "vet", Run: []string{"go", "vet", "./..."}}}
	var out bytes.Buffer
	if err := Show(&out, cfg, "/x/demo.toml", true); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"trial_dir = \".claude/worktrees/wk-trial\"", "batch_max = 1", "[[gates.run]]", "name = \"vet\"", "ff_main = true"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("show lacks %q:\n%s", want, out.String())
		}
	}
}
