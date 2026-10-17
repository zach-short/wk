package batch

import (
	"path"
	"strings"
)

// Blame names the one changeset whose diff touches a path a red gate printed (T3). diffs maps
// each changeset id of the batch to the paths of `git diff-tree -r --name-only <base> <tree>`;
// without -r git prints only top-level entries, and nothing matches. A failed path matches a
// diff path equal to it or under it, so a gate that names a directory blames the changesets
// that changed a file inside it. Both sides are repository-relative: an absolute or ../ path
// never matches. unique is true only when exactly one changeset matches; with none or several,
// id is "" and the caller falls back to Shrink (T1).
func Blame(failedPaths []string, diffs map[string][]string) (id string, unique bool) {
	failed := clean(failedPaths)
	var hits []string
	for cs, paths := range diffs {
		if touches(paths, failed) {
			hits = append(hits, cs)
		}
	}
	// Several hits return "" rather than one of them: map order would make the pick random.
	if len(hits) != 1 {
		return "", false
	}
	return hits[0], true
}

// clean normalises the failed paths and drops the ones that name no changeset: an empty path,
// and the repository root, which every diff path is under.
func clean(paths []string) []string {
	var out []string
	for _, p := range paths {
		if p = path.Clean(p); p != "." && p != "/" {
			out = append(out, p)
		}
	}
	return out
}

func touches(diffPaths, failed []string) bool {
	for _, d := range diffPaths {
		d = path.Clean(d)
		for _, f := range failed {
			if d == f || strings.HasPrefix(d, f+"/") {
				return true
			}
		}
	}
	return false
}
