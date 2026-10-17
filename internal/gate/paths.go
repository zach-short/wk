package gate

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	ansi = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	// A position at the start of a line: path:line:col (go, golangci-lint,
	// tsc --pretty) or path(line,col) (tsc without --pretty). The column is
	// optional in the colon form; the existence check drops a false match.
	position = regexp.MustCompile(`^(\S+?)(?::\d+(?::\d+)?|\(\d+,\d+\))`)
	// madge numbers each cycle: "1) a.ts > b.ts".
	madgeNumber = regexp.MustCompile(`^\d+\)\s+`)
)

// Paths returns the root-relative paths of existing files that a gate's output
// names, deduplicated in first-seen order. Paths outside root are dropped, and
// so is any name that is not a file under root, which is what keeps the loose
// "a path alone on a line" form (gofmt -l, eslint's stylish headers) honest.
func Paths(root string, output []byte) []string {
	roots := []string{root}
	if real, err := filepath.EvalSymlinks(root); err == nil && real != root {
		roots = append(roots, real)
	}
	var paths []string
	seen := map[string]bool{}
	for _, line := range strings.Split(ansi.ReplaceAllString(string(output), ""), "\n") {
		for _, c := range candidates(strings.TrimRight(line, "\r")) {
			p, ok := underRoot(roots, c)
			if ok && !seen[p] {
				seen[p] = true
				paths = append(paths, p)
			}
		}
	}
	return paths
}

// candidates reads the names one line may hold. An indented line is never a
// name of its own: tsc, golangci-lint and eslint indent quoted source and
// per-problem lines.
func candidates(line string) []string {
	if line == "" || line[0] == ' ' || line[0] == '\t' {
		return nil
	}
	if m := position.FindStringSubmatch(line); m != nil {
		return []string{m[1]}
	}
	if strings.Contains(line, " > ") {
		return strings.Split(madgeNumber.ReplaceAllString(line, ""), " > ")
	}
	return []string{strings.TrimSpace(line)}
}

// underRoot makes a candidate root-relative and keeps it only when it names a
// file that exists under one of roots (root and its resolved form).
func underRoot(roots []string, name string) (string, bool) {
	for _, r := range roots {
		rel := filepath.Clean(name)
		if filepath.IsAbs(name) {
			var err error
			if rel, err = filepath.Rel(r, name); err != nil {
				continue
			}
		}
		if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			continue
		}
		if info, err := os.Stat(filepath.Join(r, rel)); err == nil && !info.IsDir() {
			return filepath.ToSlash(rel), true
		}
	}
	return "", false
}
