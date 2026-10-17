package wk

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
)

// PoolMarkerName is the file in a pool slot's private git directory that holds the slot's
// state. Its presence alone makes gc skip the slot; only `wk pool fill` removes one.
const PoolMarkerName = "wk-pool"

const (
	slotFilling = "filling"
	slotReady   = "ready"
)

// poolSlot is a slot's marker. Shape records the sparse patterns it was checked out with, so a
// change to workspace.sparse_exclude never hands out a slot of the old shape.
type poolSlot struct {
	State       string    `json:"state"`
	Sparse      bool      `json:"sparse"`
	Shape       string    `json:"shape"`
	Commit      string    `json:"commit"`
	RefreshBlob string    `json:"refresh_blob"`
	MadeAt      time.Time `json:"made_at"`
}

func (e *Env) shape(sparse bool) string {
	if !sparse {
		return "full"
	}
	return "sparse " + strings.Join(sparseExcludes(e.Cfg.Workspace.SparseExclude), " ")
}

type slotEntry struct {
	path   string
	marker string
	slot   poolSlot
}

func (e *Env) poolLockPath() string { return filepath.Join(e.State, "pool.lock") }

func (e *Env) fillLockPath() string { return filepath.Join(e.State, "pool-fill.lock") }

func (e *Env) PoolLogPath() string { return filepath.Join(e.State, "pool.log") }

func (e *Env) poolWants(sparse bool) int {
	if sparse {
		return e.Cfg.Pool.SparseSize
	}
	return e.Cfg.Pool.Size
}

func kind(sparse bool) string {
	if sparse {
		return "sparse"
	}
	return "full"
}

func (e *Env) refill() {
	if e.Refill != nil && e.Cfg.Pool.Size+e.Cfg.Pool.SparseSize > 0 {
		e.Refill()
	}
}

// claim turns a ready pool slot into the workspace at dir. claimed is false when no slot was
// taken, and the caller then makes the workspace from the template; once a slot has moved to
// dir, any later failure is the workspace's own and is returned with claimed true.
func (e *Env) claim(dir, branch, commit string, sparse bool) (path string, claimed bool, err error) {
	if e.poolWants(sparse) == 0 {
		return "", false, nil
	}
	start := time.Now()
	slot, err := e.takeSlot(dir, sparse)
	if err != nil || slot == nil {
		return "", false, err
	}
	e.say("%-32s %6.2fs", "claim pool slot "+filepath.Base(slot.path), time.Since(start).Seconds())
	if err := e.finishClaim(dir, branch, commit, slot.slot); err != nil {
		return "", true, fmt.Errorf("%w; the workspace is left at %s for inspection", err, dir)
	}
	return dir, true, nil
}

// takeSlot moves the oldest ready slot of the wanted kind to dir. It holds the pool lock, so two
// claims never take one slot and a fill never removes the slot being taken. The marker goes
// before the move: a move that then fails leaves a plain clean worktree, which gc removes, and
// never a slot marker on someone's workspace, which gc would skip and a fill would delete.
func (e *Env) takeSlot(dir string, sparse bool) (*slotEntry, error) {
	lock, err := flock.Take(e.poolLockPath(), true, nil)
	if err != nil {
		return nil, err
	}
	defer func() { _ = lock.Release() }()
	state, found, err := e.readState()
	if err != nil || !found {
		return nil, err
	}
	entries, err := e.slots()
	if err != nil {
		return nil, err
	}
	var pick *slotEntry
	for i, s := range entries {
		if s.slot.State != slotReady || s.slot.Sparse != sparse || s.slot.Shape != e.shape(sparse) || s.slot.RefreshBlob != state.RefreshBlob {
			continue
		}
		if pick == nil || s.slot.MadeAt.Before(pick.slot.MadeAt) {
			pick = &entries[i]
		}
	}
	if pick == nil {
		e.say("pool: no ready %s slot", kind(sparse))
		return nil, nil
	}
	if err := os.Remove(pick.marker); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
		return nil, err
	}
	if _, err := gitx.Run(e.Repo.Primary, "worktree", "move", pick.path, dir); err != nil {
		return nil, err
	}
	return pick, nil
}

// finishClaim puts the moved slot on its branch at commit, then runs the post-clone commands
// only when the tree changed, because an install or a typegen over an unchanged tree changes
// nothing.
func (e *Env) finishClaim(dir, branch, commit string, slot poolSlot) error {
	if err := e.timed("branch "+branch, func() error { return attachBranch(dir, branch, commit, slot.Commit) }); err != nil {
		return err
	}
	if err := e.copyFiles(dir); err != nil {
		return err
	}
	return e.postClone(dir, commit, slot.Commit)
}

// attachBranch puts the detached slot on a new branch at commit. At the slot's own commit the
// tree and index are already right, so it only writes the branch ref and points HEAD at it:
// `git checkout -b` would re-stat every file, which in a burst waits behind other workspaces'
// clones on the same volume (about 5 s each on 2026-10-16). At any other commit, git writes
// the files that differ. With branch "" the slot stays detached, moved to commit.
func attachBranch(dir, branch, commit, at string) error {
	if branch == "" {
		return detachAt(dir, commit, at)
	}
	if commit != at {
		_, err := gitx.Run(dir, "checkout", "--quiet", "-b", branch, commit)
		return err
	}
	if _, err := gitx.Run(dir, "branch", branch, commit); err != nil {
		return err
	}
	_, err := gitx.Run(dir, "symbolic-ref", "HEAD", "refs/heads/"+branch)
	return err
}

func detachAt(dir, commit, at string) error {
	if commit == at {
		return nil
	}
	_, err := gitx.Run(dir, "checkout", "--quiet", "--detach", commit)
	return err
}

// FillPool makes slots until each kind holds its pool size, then returns. One fill runs at a
// time; a second waits for the first, then finds the pool full and returns.
func (e *Env) FillPool() error {
	if e.Cfg.Pool.Size+e.Cfg.Pool.SparseSize == 0 {
		return nil
	}
	lock, err := flock.Take(e.fillLockPath(), true, func() { e.say("waiting: another pool fill is running") })
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	limit := 2*(e.Cfg.Pool.Size+e.Cfg.Pool.SparseSize) + 2
	for range limit {
		sparse, need, err := e.prunePool()
		if err != nil || !need {
			return err
		}
		if err := e.makeSlot(sparse); err != nil {
			return err
		}
	}
	return fmt.Errorf("the pool is still short after %d slots were made", limit)
}

// prunePool removes every slot that can never be claimed, and every ready slot beyond the pool
// size, and says which kind to make next. It runs under the fill lock, so a slot still marked
// filling belongs to a fill that died.
func (e *Env) prunePool() (sparse, need bool, err error) {
	lock, err := flock.Take(e.poolLockPath(), true, nil)
	if err != nil {
		return false, false, err
	}
	defer func() { _ = lock.Release() }()
	state, found, err := e.readState()
	if err != nil {
		return false, false, err
	}
	entries, err := e.slots()
	if err != nil {
		return false, false, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].slot.MadeAt.After(entries[j].slot.MadeAt) })
	ready := map[bool]int{}
	for _, s := range entries {
		var why string
		switch {
		case s.slot.State != slotReady:
			why = "a fill stopped while making it"
		case s.slot.Shape != e.shape(s.slot.Sparse):
			why = "it was checked out with other sparse patterns"
		case !found || s.slot.RefreshBlob != state.RefreshBlob:
			why = "the template was refreshed after it was made"
		case ready[s.slot.Sparse] >= e.poolWants(s.slot.Sparse):
			why = "the pool already holds enough " + kind(s.slot.Sparse) + " slots"
		default:
			ready[s.slot.Sparse]++
			continue
		}
		e.say("pool: removing %s: %s", e.display(s.path), why)
		if err := e.removeSlot(s); err != nil {
			return false, false, err
		}
	}
	switch {
	case ready[false] < e.Cfg.Pool.Size:
		return false, true, nil
	case ready[true] < e.Cfg.Pool.SparseSize:
		return true, true, nil
	}
	return false, false, nil
}

// removeSlot forces the removal: a slot is wk's own, never anyone's work, and one whose fill
// stopped halfway holds a partial checkout that a plain remove refuses.
func (e *Env) removeSlot(s slotEntry) error {
	if _, err := gitx.Run(e.Repo.Primary, "worktree", "remove", "--force", s.path); err != nil {
		return err
	}
	if err := os.Remove(s.marker); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func (e *Env) makeSlot(sparse bool) error {
	id, err := slotID()
	if err != nil {
		return err
	}
	dir := e.inPrimary(strings.ReplaceAll(e.Cfg.Pool.Dir, config.SlotPlaceholder, id))
	createSlot, err := e.takeCreateSlot()
	if err != nil {
		return err
	}
	lock, state, err := e.takeFreshTemplate()
	if err != nil {
		_ = createSlot.Release()
		return err
	}
	slot, marker, err := e.startSlot(dir, sparse)
	if err == nil {
		err = e.fill(dir, sparse)
	}
	err = errors.Join(err, lock.Release(), createSlot.Release())
	if err == nil {
		err = e.copyFiles(dir)
	}
	if err == nil {
		err = e.postClone(dir, slot.Commit, state.Commit)
	}
	if err != nil {
		return fmt.Errorf("pool slot %s: %w; the next fill removes it", e.display(dir), err)
	}
	slot.State = slotReady
	if err := writeSlot(marker, slot); err != nil {
		return err
	}
	e.say("pool: %s slot %s ready", kind(sparse), e.display(dir))
	return nil
}

// startSlot adds the slot's worktree and marks it filling at once, so gc skips it from its first
// moment. It runs under the shared template lock, so the recorded blob names the tree cloned.
func (e *Env) startSlot(dir string, sparse bool) (poolSlot, string, error) {
	state, _, err := e.readState()
	if err != nil {
		return poolSlot{}, "", err
	}
	commit, err := gitx.Commit(e.Repo.Primary, e.Cfg.Workspace.Base)
	if err != nil {
		return poolSlot{}, "", fmt.Errorf("base %q: %w", e.Cfg.Workspace.Base, err)
	}
	err = e.timed("pool worktree add --no-checkout", func() error {
		if err := os.MkdirAll(filepath.Dir(dir), 0o755); err != nil {
			return err
		}
		_, err := gitx.Run(e.Repo.Primary, "worktree", "add", "--quiet", "--no-checkout", "--detach", dir, commit)
		return err
	})
	if err != nil {
		return poolSlot{}, "", err
	}
	gd, err := privateGitDir(dir)
	if err != nil {
		return poolSlot{}, "", err
	}
	slot := poolSlot{State: slotFilling, Sparse: sparse, Shape: e.shape(sparse), Commit: commit, RefreshBlob: state.RefreshBlob, MadeAt: e.clock().UTC()}
	marker := filepath.Join(gd, PoolMarkerName)
	return slot, marker, writeSlot(marker, slot)
}

// slots lists every worktree that carries a pool marker.
func (e *Env) slots() ([]slotEntry, error) {
	list, err := gitx.Worktrees(e.Repo.Primary)
	if err != nil {
		return nil, err
	}
	var out []slotEntry
	for _, w := range list {
		if w.Bare || w.Prunable {
			continue
		}
		gd, err := privateGitDir(w.Path)
		if err != nil {
			continue
		}
		marker := filepath.Join(gd, PoolMarkerName)
		data, err := os.ReadFile(marker)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		var s poolSlot
		if json.Unmarshal(data, &s) != nil {
			s = poolSlot{State: "unreadable"}
		}
		out = append(out, slotEntry{path: w.Path, marker: marker, slot: s})
	}
	return out, nil
}

// privateGitDir reads a linked worktree's .git file instead of asking git, because a claim reads
// it for every worktree while it holds the pool lock.
func privateGitDir(wt string) (string, error) {
	data, err := os.ReadFile(filepath.Join(wt, ".git"))
	if err == nil {
		if gd, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: "); ok {
			if !filepath.IsAbs(gd) {
				gd = filepath.Join(wt, gd)
			}
			return gd, nil
		}
	}
	return gitx.GitDir(wt)
}

func writeSlot(marker string, s poolSlot) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp := marker + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, marker)
}

func slotID() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// PoolStatus prints one line per slot and the count of ready slots against each pool size.
func (e *Env) PoolStatus(out io.Writer) error {
	entries, err := e.slots()
	if err != nil {
		return err
	}
	ready := map[bool]int{}
	for _, s := range entries {
		if s.slot.State == slotReady {
			ready[s.slot.Sparse]++
		}
		_, _ = fmt.Fprintf(out, "%-8s %-7s %-44s %.10s made %s\n", s.slot.State, kind(s.slot.Sparse), e.display(s.path), s.slot.Commit, s.slot.MadeAt.Local().Format(time.RFC3339))
	}
	_, _ = fmt.Fprintf(out, "ready: %d of %d full, %d of %d sparse\n", ready[false], e.Cfg.Pool.Size, ready[true], e.Cfg.Pool.SparseSize)
	return nil
}
