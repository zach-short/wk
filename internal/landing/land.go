package landing

import (
	"errors"
	"fmt"

	"github.com/zach-short/wk/internal/gitx"
)

var (
	// ErrMoved is Land's refusal: the landing ref no longer holds the tip the prediction was
	// made against, so the loop predicts again on its next pass.
	ErrMoved = errors.New("the landing ref moved")
	// ErrDiverged is Ensure's refusal: main and the landing ref each hold a commit the other
	// does not, and only a person merges them or resets the ref.
	ErrDiverged = errors.New("main and the landing ref diverged")
)

// Land commits result onto tip with message and moves ref from tip to the new commit by
// compare-and-swap (DESIGN.md D14). When the ref is no longer at tip the error wraps ErrMoved
// and names where the ref is; the commit then exists in the object store and no ref holds it.
func Land(repo, ref, tip, result, message string) (string, error) {
	commit, err := gitx.Run(repo, "commit-tree", result, "-p", tip, "-m", message)
	if err != nil {
		return "", err
	}
	if _, err := gitx.Run(repo, "update-ref", ref, commit, tip); err != nil {
		if now := current(repo, ref); now != tip {
			if now == "" {
				now = "missing"
			}
			return "", fmt.Errorf("%w: %s is %s, not %s", ErrMoved, ref, now, tip)
		}
		return "", err
	}
	return commit, nil
}

// Ensure puts ref where the loop can land on it and returns its tip. main is the hash the
// main branch holds: the ref is created there when missing, moved there when main has moved
// ahead of it, and left alone when it is at or ahead of main. When the two diverged the error
// wraps ErrDiverged and names both hashes. Every write carries the old value; a creation
// carries the empty string, so a ref that appeared in between is refused too.
func Ensure(repo, ref, main string) (string, error) {
	cur := current(repo, ref)
	switch {
	case cur == "":
		if _, err := gitx.Run(repo, "update-ref", ref, main, ""); err != nil {
			return "", err
		}
		return main, nil
	case cur == main || gitx.IsAncestor(repo, main, cur):
		return cur, nil
	case gitx.IsAncestor(repo, cur, main):
		if _, err := gitx.Run(repo, "update-ref", ref, main, cur); err != nil {
			return "", err
		}
		return main, nil
	}
	return "", fmt.Errorf("%w: main is at %s, %s is at %s", ErrDiverged, main, ref, cur)
}

// current is the hash ref holds, or "" when ref does not exist.
func current(repo, ref string) string {
	out, err := gitx.Run(repo, "rev-parse", "--verify", "--quiet", ref)
	if err != nil {
		return ""
	}
	return out
}
