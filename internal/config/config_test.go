package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "repo.toml")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingFileGivesDefaults(t *testing.T) {
	cfg, found, err := Load(filepath.Join(t.TempDir(), "none.toml"))
	if err != nil || found {
		t.Fatalf("found = %v, err = %v; want the defaults and no error", found, err)
	}
	if cfg.Workspace.Base != "main" || cfg.Pool.Size != 2 || cfg.New.MaxSecondsFull != 8 {
		t.Fatalf("defaults = %+v", cfg)
	}
}

func TestFileOverridesDefaultsAndKeepsTheRest(t *testing.T) {
	cfg, found, err := Load(write(t, `
[template]
clone_dirs = ["node_modules", "web/node_modules"]

[[template.post_clone]]
dir = "web"
run = ["bunx", "next", "typegen"]

[gates.env]
API_URL = "https://ci.invalid"
`))
	if err != nil || !found {
		t.Fatalf("found = %v, err = %v", found, err)
	}
	if got := strings.Join(cfg.Template.CloneDirs, ","); got != "node_modules,web/node_modules" {
		t.Fatalf("clone_dirs = %q", got)
	}
	if cfg.Template.PostClone[0].Dir != "web" || cfg.Template.PostClone[0].Run[2] != "typegen" {
		t.Fatalf("post_clone = %+v", cfg.Template.PostClone)
	}
	if cfg.Gates.Env["API_URL"] != "https://ci.invalid" {
		t.Fatalf("gates.env = %v", cfg.Gates.Env)
	}
	if cfg.Template.Path != ".claude/worktrees/wk-template" || cfg.GC.QuietMinutes != 60 {
		t.Fatalf("a dial the file does not name lost its default: %+v", cfg)
	}
}

func TestRefusals(t *testing.T) {
	for name, tc := range map[string]struct{ body, want string }{
		"unknown key":      {"[template]\nclone_dir = [\"x\"]\n", "unknown keys: template.clone_dir"},
		"unknown table":    {"[lease]\nttl = 1\n", "unknown keys: lease"},
		"absolute path":    {"[template]\npath = \"/tmp/t\"\n", "is absolute"},
		"climbing path":    {"[template]\ncopy_files = [\"../x/.env\"]\n", "leaves the main checkout"},
		"empty command":    {"[[template.build]]\ndir = \"web\"\nrun = []\n", "template.build[0].run is empty"},
		"row placeholder":  {"[workspace]\ndir = \"ws\"\n", "must both hold {row}"},
		"name placeholder": {"[hook]\nbranch = \"hook\"\n", "must both hold {name}"},
		"empty base":       {"[workspace]\nbase = \"\"\n", "workspace.base is empty"},
		"not toml":         {"[template\n", "repo.toml"},
		"wrong value type": {"[pool]\nsize = \"two\"\n", "repo.toml"},
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := Load(write(t, tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}
