// Package landing moves the landing ref and, on the owner's command, main: it predicts a
// changeset's merge onto the tip, lands the predicted tree by compare-and-swap, syncs a
// workspace to the tip, and fast-forwards main (DESIGN.md D14, D20; PLAN-P2-P4.md section 3).
// It takes and returns hashes; what the ref, the branch and the remote are called is the
// caller's config. Every ref write carries the old value, so a ref that moved under the caller
// is refused, never overwritten.
package landing

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
)

// Predict merges tree onto tip with base as the merge base, as the landing commit would, and
// returns the merged tree. A conflict is not an error: result is then the tree merge-tree
// wrote, with git's markers in the conflicted files, and conflicts names those paths; Land
// takes only a result that came back with no conflicts, Sync takes either. Any other git
// failure is an error carrying git's stderr.
func Predict(repo, base, tip, tree string) (result string, conflicts []string, err error) {
	// core.quotePath=false keeps a non-ASCII path readable, as lib.sh merge_step does.
	out, code, err := run(repo, nil, "-c", "core.quotePath=false", "merge-tree", "--write-tree",
		"--name-only", "--merge-base="+base, tip, tree)
	if err != nil && code != 1 {
		return "", nil, err
	}
	first, rest, _ := strings.Cut(out, "\n")
	if first == "" {
		return "", nil, errors.New("git merge-tree printed no tree")
	}
	if code == 0 {
		return first, nil, nil
	}
	return first, conflictPaths(rest), nil
}

// conflictPaths reads merge-tree's conflicted-file section: the lines after the tree, up to
// the blank line before its informational messages.
func conflictPaths(section string) []string {
	var paths []string
	for _, line := range strings.Split(section, "\n") {
		if line == "" {
			break
		}
		paths = append(paths, line)
	}
	sort.Strings(paths)
	return paths
}

// run is gitx.Run with the exit status and extra environment, for the two calls gitx.Run
// cannot make: merge-tree, whose exit 1 is a conflict and not a failure, and the private-index
// sequence, which needs GIT_INDEX_FILE. The error has gitx.Run's shape.
func run(dir string, env []string, args ...string) (stdout string, code int, err error) {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(append(os.Environ(), "GIT_OPTIONAL_LOCKS=0"), env...)
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			code = exit.ExitCode()
		}
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		return strings.TrimRight(out.String(), "\n"), code, fmt.Errorf("git %s: %s", strings.Join(args, " "), msg)
	}
	return strings.TrimRight(out.String(), "\n"), 0, nil
}
