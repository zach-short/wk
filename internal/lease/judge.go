package lease

import (
	"fmt"
	"path/filepath"
	"strings"
)

// Rules are the `[leases]` keys the judgment reads. SingleWriter holds repository-relative
// paths (D13 L1); BashGuard turns the `Bash` word walk on (D11).
type Rules struct {
	SingleWriter []string
	BashGuard    bool
}

// Request is one tool call as the hook sees it. Path is the edit tools' file path, raw as the
// hook input gives it; Command is the `Bash` command.
type Request struct {
	Tool    string
	Cwd     string
	Path    string
	Command string
}

// Decision is the judgment. Reason is empty on an allow and, on a deny, says what to do next.
type Decision struct {
	Allow  bool
	Reason string
}

var allow = Decision{Allow: true}

// Resolve finds which root holds path, read against cwd when relative and cleaned of `..`, by
// longest-prefix match, so a worktree nested under the main checkout wins over it. rel is
// slash-separated and "." for the root itself. ok is false for a path in no root. It reads no
// disk, so a symlink is judged where it is written, not where it points.
func Resolve(roots []string, cwd, path string) (root, rel string, ok bool) {
	if !filepath.IsAbs(path) {
		path = filepath.Join(cwd, path)
	}
	path = filepath.Clean(path)
	for _, r := range roots {
		r = filepath.Clean(r)
		if len(r) > len(root) && (path == r || strings.HasPrefix(path, r+"/")) {
			root = r
		}
	}
	if root == "" {
		return "", "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", "", false
	}
	return root, filepath.ToSlash(rel), true
}

// editTools are the tools that carry a file path, named exactly as Claude Code sends them.
var editTools = map[string]bool{"Edit": true, "Write": true, "MultiEdit": true, "NotebookEdit": true}

// Judge decides one tool call, in this order: a path the judgment cannot place is denied; a path
// in no root is allowed; a root that has not claimed is denied (D10); a path another root leases
// is denied with its row and root; a single-writer path this root does not lease is denied
// (D13 L1); anything else is allowed. A `Bash` call is judged on every file WriteTargets finds,
// and one denial denies the command.
//
// Tool names match exactly (`Bash`, `Edit`, `Write`, `MultiEdit`, `NotebookEdit`). To fail
// closed, any other tool that carries a path is judged as an edit, and any other tool without
// one is denied, even with a command: `bash` in lower case is not `Bash`.
func Judge(store *Store, cfg Rules, roots []string, req Request) Decision {
	switch {
	case req.Tool == "Bash":
		return judgeBash(store, cfg, roots, req)
	case editTools[req.Tool] || req.Path != "":
		return judgePath(store, cfg, roots, req.Cwd, req.Path)
	}
	return deny("wk: cannot tell which file the tool %q writes; give the path in full", req.Tool)
}

func judgeBash(store *Store, cfg Rules, roots []string, req Request) Decision {
	if !cfg.BashGuard {
		return allow
	}
	for _, t := range WriteTargets(req.Cwd, req.Command) {
		if t == openQuote {
			return deny("wk: cannot tell which file this writes: the command has an open quote, and bash runs the lines before it; close the quote")
		}
		if d := judgePath(store, cfg, roots, "", t); !d.Allow {
			d.Reason += fmt.Sprintf(" (the command writes %s)", t)
			return d
		}
	}
	return allow
}

func judgePath(store *Store, cfg Rules, roots []string, cwd, path string) Decision {
	if path == "" || !filepath.IsAbs(path) && !filepath.IsAbs(cwd) {
		return deny("wk: cannot tell which file this writes (%q); give the path in full", path)
	}
	root, rel, ok := Resolve(roots, cwd, path)
	if !ok {
		return allow
	}
	if !store.Claimed(root) {
		return deny("wk: this workspace has not claimed a row; run `wk claim <row>` or `wk claim --none` in %s", root)
	}
	if holder, row, held := store.heldElsewhere(root, rel); held {
		return deny("wk: %s is leased by row %s in %s; leave it to that workspace or wait until it releases", rel, row, holder)
	}
	if sw, hit := unleasedSingleWriter(store, root, rel, cfg.SingleWriter); hit {
		return deny("wk: %s is single-writer (leases.single_writer) and this workspace does not lease it; run `wk lease add %s` in %s", sw, sw, root)
	}
	return allow
}

// unleasedSingleWriter finds a single-writer path the write to rel reaches that root does not
// lease. When rel is a directory holding the single-writer path, the lease must cover that path;
// otherwise it must cover rel.
func unleasedSingleWriter(store *Store, root, rel string, single []string) (string, bool) {
	for _, sw := range single {
		if !touches(sw, rel) {
			continue
		}
		need, base := rel, strings.TrimSuffix(sw, "/")
		if rel == "." || strings.HasPrefix(base, rel+"/") {
			need = base
		}
		if !store.leases(root, need) {
			return sw, true
		}
	}
	return "", false
}

func deny(format string, args ...any) Decision {
	return Decision{Reason: fmt.Sprintf(format, args...)}
}
