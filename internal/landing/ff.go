package landing

import (
	"fmt"
	"strings"

	"github.com/zach-short/wk/internal/gitx"
)

// Blockers lists the paths dirty in primary that the move from branch to `to` changes, sorted.
// `git merge --ff-only` refuses to overwrite them (DESIGN.md section 6), so `wk land` prints
// them and FastForward refuses while any exist. It is lib.sh blocking_paths.
func Blockers(primary, branch, to string) ([]string, error) {
	changed, err := gitx.Run(primary, "diff", "--name-only", "--no-renames", "-z", "refs/heads/"+branch, to)
	if err != nil {
		return nil, err
	}
	dirty, err := gitx.DirtyPaths(primary)
	if err != nil {
		return nil, err
	}
	touched := map[string]bool{}
	for _, p := range strings.Split(changed, "\x00") {
		touched[p] = true
	}
	var blockers []string
	for _, p := range dirty {
		if touched[p] {
			blockers = append(blockers, p)
		}
	}
	return blockers, nil
}

// FastForward moves branch, checked out in primary, to `to` by a fast-forward merge, and only
// while `to` still resolves to expect, the hash the dry run printed: a landing in between is
// refused. It refuses while dirty paths in primary block the move, returning them, and while
// primary has another branch checked out, because merge moves whatever HEAD is. Afterwards
// branch holds expect, or the error says where it went.
func FastForward(primary, branch, to, expect string) ([]string, error) {
	now, err := gitx.Run(primary, "rev-parse", "--verify", to+"^{commit}")
	if err != nil {
		return nil, err
	}
	if now != expect {
		return nil, fmt.Errorf("%s is at %s, not the expected %s; run the dry run again and read it", to, now, expect)
	}
	if err := checkedOut(primary, branch); err != nil {
		return nil, err
	}
	blockers, err := Blockers(primary, branch, expect)
	if err != nil {
		return nil, err
	}
	if len(blockers) > 0 {
		return blockers, fmt.Errorf("%d dirty paths in %s block the fast-forward of %s; their owner commits them first", len(blockers), primary, branch)
	}
	if _, err := gitx.Run(primary, "merge", "--ff-only", "--no-autostash", expect); err != nil {
		return nil, err
	}
	got, err := gitx.Run(primary, "rev-parse", "--verify", "refs/heads/"+branch+"^{commit}")
	if err != nil {
		return nil, err
	}
	if got != expect {
		return nil, fmt.Errorf("%s moved to %s, not %s; stop and look before anything else runs", branch, got, expect)
	}
	return nil, nil
}

// checkedOut refuses a primary whose HEAD is not branch.
func checkedOut(primary, branch string) error {
	head, err := gitx.Run(primary, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil || head != branch {
		return fmt.Errorf("%s has %q checked out, not %s, and the fast-forward moves what is checked out", primary, head, branch)
	}
	return nil
}

// Push sends branch to remote from primary; `wk land --apply` runs it after FastForward when
// landing.push is set (DESIGN.md D20).
func Push(primary, remote, branch string) error {
	_, err := gitx.Run(primary, "push", remote, branch)
	return err
}
