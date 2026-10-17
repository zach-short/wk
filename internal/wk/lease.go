package wk

import (
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/board"
	"github.com/zach-short/wk/internal/daemon"
	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/lease"
)

// ownsColumn is the board column `wk claim` reads, found by its header (D10).
const ownsColumn = "Files it owns"

// noneRow is the row a `wk claim --none` records: a claim that holds no path.
const noneRow = "none"

// leaseFile is leases.json: the store, the worktree roots the hook places paths in, and the
// repository it belongs to. `wk claim`, `wk lease add` and `wk release` rewrite the roots from
// git, so the hook never runs git (PD-2).
type leaseFile struct {
	Primary   string   `json:"primary"`
	CommonDir string   `json:"common_dir"`
	Roots     []string `json:"roots"`
	lease.Store
}

func leasesPath(state string) string { return filepath.Join(state, "leases.json") }

// loadLeases reads the store without a lock (files decision: readers take none, because every
// write is a rename) and drops every claim whose worktree root is gone.
func loadLeases(state string) (leaseFile, error) {
	var f leaseFile
	if _, err := daemon.LoadJSON(leasesPath(state), &f); err != nil {
		return leaseFile{}, err
	}
	f.reap()
	return f, nil
}

// reap drops the claims and roots of worktrees that no longer exist, so a removed workspace
// never holds a lease.
func (f *leaseFile) reap() {
	for root := range f.Workspaces {
		if !exists(root) {
			f.Release(root)
		}
	}
	f.Roots = slices.DeleteFunc(f.Roots, func(r string) bool { return !exists(r) })
}

// Claim takes the leases of row's "Files it owns" cell for the workspace holding dir. Every
// token it cannot read is printed, and only what it read is leased (D10, G3). A claim of the
// same row with the same or fewer leases succeeds and prints the claim it found.
func (e *Env) Claim(dir, row string, out io.Writer) error {
	root, err := workspaceRoot(dir)
	if err != nil {
		return err
	}
	leases, unread, err := e.rowLeases(root, row)
	if err != nil {
		return err
	}
	for _, u := range unread {
		_, _ = fmt.Fprintf(out, "unread: %s\n", u)
	}
	return e.claimLeases(root, row, leases, out)
}

// ClaimNone records a claim with no leases, for a session that reads and does not edit.
func (e *Env) ClaimNone(dir string, out io.Writer) error {
	root, err := workspaceRoot(dir)
	if err != nil {
		return err
	}
	return e.claimLeases(root, noneRow, nil, out)
}

func (e *Env) claimLeases(root, row string, leases []string, out io.Writer) error {
	return e.changeLeases(func(f *leaseFile) error {
		if ws, ok := f.Workspaces[root]; ok && ws.Row == row && subset(leases, ws.Leases) {
			_, _ = fmt.Fprintf(out, "already claimed: row %s in %s\n", row, root)
			printLeases(out, ws.Leases)
			return nil
		}
		if err := f.Claim(root, row, "", leases); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(out, "claimed: row %s in %s\n", row, root)
		printLeases(out, f.Workspaces[root].Leases)
		return nil
	})
}

func subset(some, all []string) bool {
	for _, s := range some {
		if !slices.Contains(all, s) {
			return false
		}
	}
	return true
}

func printLeases(out io.Writer, leases []string) {
	for _, l := range leases {
		_, _ = fmt.Fprintf(out, "lease: %s\n", l)
	}
}

// LeaseAdd widens the claim of the workspace holding dir by paths, each read against dir. A
// trailing "/" leases a directory.
func (e *Env) LeaseAdd(dir string, paths []string, out io.Writer) error {
	root, err := workspaceRoot(dir)
	if err != nil {
		return err
	}
	var rels []string
	for _, p := range paths {
		_, rel, ok := lease.Resolve([]string{root}, canonical(dir), p)
		if !ok || rel == "." {
			return fmt.Errorf("%s is not a path inside %s", p, root)
		}
		if strings.HasSuffix(p, "/") {
			rel += "/"
		}
		rels = append(rels, rel)
	}
	return e.changeLeases(func(f *leaseFile) error {
		if err := f.Add(root, rels); err != nil {
			return err
		}
		printLeases(out, rels)
		return nil
	})
}

// Release drops the claim of the workspace holding dir and every lease it held.
func (e *Env) Release(dir string, out io.Writer) error {
	root, err := workspaceRoot(dir)
	if err != nil {
		return err
	}
	return e.changeLeases(func(f *leaseFile) error {
		f.Release(root)
		_, _ = fmt.Fprintf(out, "released: %s\n", root)
		return nil
	})
}

// LeaseStatus prints every claim with its row and leases, oldest first.
func (e *Env) LeaseStatus(out io.Writer) error {
	f, err := loadLeases(e.State)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "leases.enabled = %v\n", e.Cfg.Leases.Enabled)
	roots := slices.Collect(maps.Keys(f.Workspaces))
	slices.SortFunc(roots, func(a, b string) int { return f.Workspaces[a].ClaimedAt.Compare(f.Workspaces[b].ClaimedAt) })
	for _, r := range roots {
		ws := f.Workspaces[r]
		_, _ = fmt.Fprintf(out, "%s  row %s  since %s  %d leases\n", r, ws.Row, ws.ClaimedAt.Local().Format(time.DateTime), len(ws.Leases))
		printLeases(out, ws.Leases)
	}
	if len(roots) == 0 {
		_, _ = fmt.Fprintln(out, "no claims")
	}
	return nil
}

// changeLeases runs change on the store under an exclusive lock held from the read to the
// rename (files decision, KILL-TEST.md), after the repository check, the reap and a refresh of
// the roots from git. A change that fails writes nothing.
func (e *Env) changeLeases(change func(*leaseFile) error) error {
	lock, err := flock.Take(filepath.Join(e.State, "leases.lock"), true, func() { e.say("waiting for leases.lock") })
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	f, err := loadLeases(e.State)
	if err != nil {
		return err
	}
	if f.CommonDir != "" && f.CommonDir != e.Repo.CommonDir {
		return fmt.Errorf("%s belongs to the repository at %s, not %s; two repositories share the name %s", leasesPath(e.State), f.CommonDir, e.Repo.CommonDir, e.Repo.Name)
	}
	if f.Roots, err = worktreeRoots(e.Repo.Primary); err != nil {
		return err
	}
	f.Primary, f.CommonDir = e.Repo.Primary, e.Repo.CommonDir
	if err := change(&f); err != nil {
		return err
	}
	reapTemps(e.State)
	return daemon.SaveJSON(leasesPath(e.State), f)
}

// staleTemp is the age after which a temp file of SaveJSON is a crashed writer's, not a live one.
const staleTemp = time.Minute

// reapTemps removes the temp files a writer killed between write and rename left behind.
func reapTemps(state string) {
	temps, _ := filepath.Glob(filepath.Join(state, ".leases.json.*.tmp"))
	for _, t := range temps {
		if info, err := os.Stat(t); err == nil && time.Since(info.ModTime()) > staleTemp {
			_ = os.Remove(t)
		}
	}
}

// rowLeases reads row's cell from the board and turns it into leases: exact paths and directory
// prefixes as they are, globs expanded against the files git tracks in root. unread holds every
// token board.Parse could not read and every glob that matched nothing.
func (e *Env) rowLeases(root, row string) (leases, unread []string, err error) {
	path := e.inPrimary(e.Cfg.Leases.Board)
	text, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("leases.board: %w", err)
	}
	cell, err := board.Column(string(text), row, ownsColumn)
	if err != nil {
		return nil, nil, fmt.Errorf("%s: %w", path, err)
	}
	paths, unread := board.Parse(cell)
	var globs []string
	for _, p := range paths {
		if strings.ContainsAny(p, "*?[") {
			globs = append(globs, p)
		} else {
			leases = append(leases, p)
		}
	}
	if len(globs) == 0 {
		return leases, unread, nil
	}
	files, err := gitx.Run(root, "ls-files", "-z")
	if err != nil {
		return nil, nil, err
	}
	matched, unmatched := board.Expand(globs, strings.Split(strings.TrimRight(files, "\x00"), "\x00"))
	for _, g := range unmatched {
		unread = append(unread, g+" (matches no tracked file)")
	}
	return append(leases, matched...), unread, nil
}

// workspaceRoot is the canonical top level of the worktree holding dir, the key a claim is held
// under (option H).
func workspaceRoot(dir string) (string, error) {
	top, err := gitx.Run(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return canonical(top), nil
}

// worktreeRoots are the canonical roots of every worktree of the repository.
func worktreeRoots(primary string) ([]string, error) {
	list, err := gitx.Worktrees(primary)
	if err != nil {
		return nil, err
	}
	var roots []string
	for _, w := range list {
		if !w.Bare && w.Path != "" {
			roots = append(roots, canonical(w.Path))
		}
	}
	return roots, nil
}

// canonical resolves the symlinks of the longest prefix of p that exists and keeps the rest as
// written, so a file not yet made is placed where its directory really is (macOS /tmp is
// /private/tmp). Resolve reads no disk, so every path it compares must come through here. A
// relative path comes back as it is: it has no place until the judgment reads it against a cwd.
func canonical(p string) string {
	if !filepath.IsAbs(p) {
		return p
	}
	p = filepath.Clean(p)
	rest := ""
	for {
		real, err := filepath.EvalSymlinks(p)
		if err == nil {
			return filepath.Join(real, rest)
		}
		parent := filepath.Dir(p)
		if parent == p || !errors.Is(err, os.ErrNotExist) {
			return filepath.Join(p, rest)
		}
		rest = filepath.Join(filepath.Base(p), rest)
		p = parent
	}
}
