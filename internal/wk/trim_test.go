package wk

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

func TestTrimAllowedOnlyInAWorkspace(t *testing.T) {
	f := testrepo.New(t)
	add := func(rel, marker string) string {
		dir := filepath.Join(f.Repo, filepath.FromSlash(rel))
		f.Git(t, "worktree", "add", "--quiet", "--detach", dir)
		if marker != "" {
			gd := testrepo.Git(t, dir, "rev-parse", "--absolute-git-dir")
			if err := os.WriteFile(filepath.Join(gd, marker), []byte("{}\n"), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		return dir
	}
	work := add(".claude/worktrees/wk-12", "")
	for dir, want := range map[string]bool{
		work:                                     true,
		filepath.Join(work, "web"):               true,
		f.Repo:                                   false,
		filepath.Join(f.Repo, "web"):             false,
		add(".claude/worktrees/wk-template", ""): false,
		add(".claude/worktrees/wk-pool-1", PoolMarkerName): false,
		add(".claude/worktrees/old-template", MarkerName):  false,
		t.TempDir(): false,
	} {
		if got := TrimAllowed(f.Home, dir); got != want {
			t.Errorf("TrimAllowed(%s) = %v, want %v", dir, got, want)
		}
	}
	if err := os.WriteFile(filepath.Join(f.Home, "demo.toml"), []byte("[template]\npaht = \"x\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if TrimAllowed(f.Home, work) {
		t.Error("a config wk cannot read still allowed the trim")
	}
}
