package slot

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zach-short/wk/internal/gitx"
)

// checkTrim refuses a trim path that could reach outside the workspace. --trim deletes, so a
// bad path is a bad argument list with exit 2, never a skip the caller would not see.
func checkTrim(paths []string) error {
	for _, p := range paths {
		if err := checkTrimPath(p); err != nil {
			return fmt.Errorf("--trim %q: %w", p, err)
		}
	}
	return nil
}

func checkTrimPath(p string) error {
	switch {
	case filepath.IsAbs(p):
		return errors.New("must be relative to the git top level")
	case slices.Contains(strings.Split(filepath.ToSlash(p), "/"), ".."):
		return errors.New(`must not hold a ".." element`)
	case filepath.Clean(p) == ".":
		return errors.New("would remove the whole workspace")
	}
	return nil
}

// trim removes the --trim paths after the command ends with any status, while the place is
// still held (SB-12), so the disk a build filled is back before the next one starts. The
// caller says where trimming is not allowed: the main checkout is Zach's, and the template's
// cache is what every workspace clones.
func (r *runner) trim() {
	if len(r.opts.Trim) == 0 || r.opts.Keep || !r.opts.TrimAllowed {
		return
	}
	top, err := gitx.Run(r.dir, "rev-parse", "--show-toplevel")
	if err != nil {
		r.warn("trim skipped: " + err.Error())
		return
	}
	var removed []string
	for _, rel := range r.opts.Trim {
		ok, err := removeInside(top, rel)
		if err != nil {
			r.warn(fmt.Sprintf("trim of %s failed: %v", rel, err))
		} else if ok {
			removed = append(removed, rel)
		}
	}
	if len(removed) > 0 {
		r.say("trimmed %s", strings.Join(removed, ", "))
	}
}

// removeInside removes top/rel, unless a symlink on the way leads out of top: a workspace's
// trim must never delete what another checkout uses.
func removeInside(top, rel string) (bool, error) {
	target := filepath.Join(top, rel)
	if _, err := os.Lstat(target); errors.Is(err, fs.ErrNotExist) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	realTop, err := filepath.EvalSymlinks(top)
	if err != nil {
		return false, err
	}
	realParent, err := filepath.EvalSymlinks(filepath.Dir(target))
	if err != nil {
		return false, err
	}
	if !within(realTop, realParent) {
		return false, fmt.Errorf("it leads out of %s through a symlink", top)
	}
	return true, os.RemoveAll(target)
}

func within(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
