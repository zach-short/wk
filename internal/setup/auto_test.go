package setup

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

func TestDefaultsTakeEachOptionsOn(t *testing.T) {
	dir := monorepo(t)
	// A second lockfile makes a second install, which the wizard offers but leaves off.
	if err := os.WriteFile(filepath.Join(dir, "go.sum"), []byte("sum"), 0o644); err != nil {
		t.Fatal(err)
	}
	found := Detect(dir)
	if len(found.Installs) != 2 {
		t.Fatalf("fixture has %d installs, want 2: %v", len(found.Installs), values(found.Installs))
	}
	got := Defaults(found)
	if want := []string{"node_modules", "web/node_modules"}; !reflect.DeepEqual(got.CloneDirs, want) {
		t.Errorf("clone dirs = %v, want %v", got.CloneDirs, want)
	}
	if want := []string{"web/.env"}; !reflect.DeepEqual(got.CopyFiles, want) {
		t.Errorf("copy files = %v, want %v (the tracked .env.production is off)", got.CopyFiles, want)
	}
	if len(got.Installs) != 1 || got.Installs[0].Lockfile != "bun.lock" {
		t.Errorf("installs = %v, want only the bun install", got.Installs)
	}
	if got.Schedule || got.Warm {
		t.Errorf("schedule %v, warm %v, want both off", got.Schedule, got.Warm)
	}
}

func TestDefaultsPickMainOverTheCurrentBranch(t *testing.T) {
	f := testrepo.New(t)
	f.Git(t, "switch", "--quiet", "-c", "feature")
	found := Detect(f.Repo)
	if found.Branches[0] != "feature" {
		t.Fatalf("branches = %v, want feature first", found.Branches)
	}
	if got := Defaults(found).Base; got != "main" {
		t.Errorf("base = %q, want main", got)
	}
}

func TestDefaultBaseFallsBack(t *testing.T) {
	for name, tc := range map[string]struct {
		branches []string
		want     string
	}{
		"master over current": {[]string{"feature", "master"}, "master"},
		"main over master":    {[]string{"feature", "master", "main"}, "main"},
		"current otherwise":   {[]string{"trunk", "dev"}, "trunk"},
		"empty":               {nil, "main"},
	} {
		t.Run(name, func(t *testing.T) {
			if got := defaultBase(tc.branches); got != tc.want {
				t.Errorf("base = %q, want %q", got, tc.want)
			}
		})
	}
}
