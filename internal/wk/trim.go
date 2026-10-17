package wk

import (
	"path/filepath"

	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/gitx"
)

// TrimAllowed says whether `wk run --trim` may delete inside the worktree that holds dir
// (SLOTS-PLAN.md SB-12). The main checkout is the owner's, the template's cache is what every
// workspace clones, and a pool slot becomes someone's workspace. Any doubt answers false: a
// trim cannot be undone, and a skipped one costs only disk.
func TrimAllowed(home, dir string) bool {
	top, err := gitx.Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return false
	}
	repo, err := gitx.Resolve(dir)
	if err != nil || physical(top) == physical(repo.Primary) || ownedByWK(top) {
		return false
	}
	cfg, _, err := config.Load(filepath.Join(home, repo.Name+".toml"))
	return err == nil && physical(top) != physical(filepath.Join(repo.Primary, filepath.FromSlash(cfg.Template.Path)))
}

// ownedByWK reads the template's and the pool's markers, so the answer holds even when the
// config names another template path than the one this worktree was made at.
func ownedByWK(top string) bool {
	gd, err := privateGitDir(top)
	return err != nil || exists(filepath.Join(gd, MarkerName)) || exists(filepath.Join(gd, PoolMarkerName))
}
