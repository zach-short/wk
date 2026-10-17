// Package lease holds path leases for parallel workspaces and judges each edit against them
// (DESIGN.md D10 to D13, PLAN-P2-P4.md section 2). It touches no disk and runs no git: the
// daemon owns the store and saves it, and the hook asks the daemon for a Decision.
package lease

import (
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Workspace is one worktree's claim. Leases are repository-relative paths; a trailing `/` makes
// a lease a directory prefix. A claim with no leases is `wk claim --none`.
type Workspace struct {
	Row       string    `json:"row"`
	Session   string    `json:"session"`
	Leases    []string  `json:"leases"`
	ClaimedAt time.Time `json:"claimed_at"`
}

// Store holds every claim, keyed by worktree root (option H: a builder and its auditor in one
// worktree share a holder). The zero value is empty and ready. A Store is not safe for
// concurrent use; the daemon holds one lock around each operation and the save that follows it.
type Store struct {
	Workspaces map[string]Workspace `json:"workspaces"`
}

// Claim records root's claim on row. It refuses a root that has claimed already, because a
// builder and its auditor share one root (option H) and a new claim would drop leases the other
// relies on; it refuses a lease that is malformed or overlaps a lease another root holds, and
// names that root and its row.
func (s *Store) Claim(root, row, session string, leases []string) error {
	root = filepath.Clean(root)
	if ws, ok := s.Workspaces[root]; ok {
		return fmt.Errorf("%s has already claimed row %s; run `wk release` there first", root, ws.Row)
	}
	if err := s.check(root, leases); err != nil {
		return err
	}
	if s.Workspaces == nil {
		s.Workspaces = map[string]Workspace{}
	}
	s.Workspaces[root] = Workspace{
		Row:       row,
		Session:   session,
		Leases:    merge(nil, leases),
		ClaimedAt: time.Now().UTC().Round(0),
	}
	return nil
}

// Add widens root's claim by leases, under the same rule as Claim. A root with no claim is
// refused, because a lease outside a claim would let the unclaimed workspace edit (G3).
func (s *Store) Add(root string, leases []string) error {
	root = filepath.Clean(root)
	ws, ok := s.Workspaces[root]
	if !ok {
		return fmt.Errorf("%s has not claimed; run `wk claim <row>` there first", root)
	}
	if err := s.check(root, leases); err != nil {
		return err
	}
	ws.Leases = merge(ws.Leases, leases)
	s.Workspaces[root] = ws
	return nil
}

// Release drops root's claim and every lease it held.
func (s *Store) Release(root string) {
	delete(s.Workspaces, filepath.Clean(root))
}

// Claimed reports whether root has claimed, with or without leases.
func (s *Store) Claimed(root string) bool {
	_, ok := s.Workspaces[filepath.Clean(root)]
	return ok
}

// Holder names the first root, in sorted order, holding a lease that overlaps the
// repository-relative path. The path counts as a possible directory, so a lease inside it is
// found too.
func (s *Store) Holder(rel string) (root, row string, ok bool) {
	return s.heldElsewhere("", rel)
}

// heldElsewhere is Holder skipping root. A directory target may meet leases of several roots;
// skipping only the caller's own makes sure another holder is never hidden behind it.
func (s *Store) heldElsewhere(root, rel string) (holder, row string, ok bool) {
	for _, r := range slices.Sorted(maps.Keys(s.Workspaces)) {
		if r == root {
			continue
		}
		for _, l := range s.Workspaces[r].Leases {
			if touches(l, rel) {
				return r, s.Workspaces[r].Row, true
			}
		}
	}
	return "", "", false
}

// leases reports whether root holds a lease that covers the whole of rel.
func (s *Store) leases(root, rel string) bool {
	for _, l := range s.Workspaces[root].Leases {
		if covers(l, rel) {
			return true
		}
	}
	return false
}

func (s *Store) check(root string, leases []string) error {
	for _, l := range leases {
		if !valid(l) {
			return fmt.Errorf("lease %q is not a clean repository-relative path", l)
		}
		if r, h, ok := s.overlapping(root, l); ok {
			return fmt.Errorf("lease %s overlaps %s, held by row %s in %s", l, h, s.Workspaces[r].Row, r)
		}
	}
	return nil
}

func (s *Store) overlapping(root, l string) (holder, lease string, ok bool) {
	for _, r := range slices.Sorted(maps.Keys(s.Workspaces)) {
		if r == root {
			continue
		}
		for _, h := range s.Workspaces[r].Leases {
			if overlap(l, h) {
				return r, h, true
			}
		}
	}
	return "", "", false
}

// overlap is the conflict rule (option K): two leases overlap when they name the same path or
// one is a directory prefix of the other. `web/` and `webapp/x` do not overlap.
func overlap(a, b string) bool {
	ab, bb := strings.TrimSuffix(a, "/"), strings.TrimSuffix(b, "/")
	return ab == bb || isDir(a) && strings.HasPrefix(bb, ab+"/") || isDir(b) && strings.HasPrefix(ab, bb+"/")
}

// touches reports whether a write to rel can change what lease l names. rel may be a directory
// (`rm -r dir`), and "." is the whole tree.
func touches(l, rel string) bool {
	return rel == "." || overlap(l, rel+"/")
}

// covers reports whether lease l takes in all of rel.
func covers(l, rel string) bool {
	b := strings.TrimSuffix(l, "/")
	return b == rel || isDir(l) && strings.HasPrefix(rel, b+"/")
}

func isDir(l string) bool { return strings.HasSuffix(l, "/") }

func valid(l string) bool {
	b := strings.TrimSuffix(l, "/")
	return b != "" && b != "." && b != ".." && !strings.HasPrefix(b, "/") &&
		!strings.HasPrefix(b, "../") && path.Clean(b) == b
}

func merge(have, add []string) []string {
	out := append(slices.Clone(have), add...)
	slices.Sort(out)
	return append([]string{}, slices.Compact(out)...)
}
