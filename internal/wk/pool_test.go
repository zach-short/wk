package wk

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

func openPool(t *testing.T, f testrepo.Fixture, log io.Writer, full, sparse int) *Env {
	t.Helper()
	e := open(t, f, log)
	e.Cfg.Pool.Size, e.Cfg.Pool.SparseSize = full, sparse
	return e
}

func mustFill(t *testing.T, e *Env) {
	t.Helper()
	if err := e.FillPool(); err != nil {
		t.Fatal(err)
	}
}

func slotsOf(t *testing.T, e *Env) []slotEntry {
	t.Helper()
	s, err := e.slots()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func countReady(t *testing.T, e *Env, sparse bool) int {
	t.Helper()
	n := 0
	for _, s := range slotsOf(t, e) {
		if s.slot.State == slotReady && s.slot.Sparse == sparse {
			n++
		}
	}
	return n
}

func TestFillMakesEachKindAndGCSkipsThem(t *testing.T) {
	f := testrepo.New(t)
	e := openPool(t, f, io.Discard, 2, 1)
	mustFill(t, e)
	if full, sparse := countReady(t, e, false), countReady(t, e, true); full != 2 || sparse != 1 {
		t.Fatalf("ready = %d full, %d sparse; want 2 and 1", full, sparse)
	}
	for _, s := range slotsOf(t, e) {
		assertClean(t, s.path)
		read(t, filepath.Join(s.path, "node_modules/.installed-from"))
		read(t, filepath.Join(s.path, "web/.env"))
		_, docsErr := os.Stat(filepath.Join(s.path, "docs/incomplete"))
		if s.slot.Sparse != os.IsNotExist(docsErr) {
			t.Fatalf("%s: sparse = %v but docs/incomplete stat err = %v", s.path, s.slot.Sparse, docsErr)
		}
	}
	judgments, err := e.judgeAll()
	if err != nil {
		t.Fatal(err)
	}
	skipped := 0
	for _, j := range judgments {
		if j.Verdict == Skipped && strings.Contains(strings.Join(j.Reasons, ""), "pool slot") {
			skipped++
		}
	}
	if skipped != 3 {
		t.Fatalf("gc skipped %d pool slots, want 3: %+v", skipped, judgments)
	}

	before := len(slotsOf(t, e))
	mustFill(t, e)
	if after := len(slotsOf(t, e)); after != before {
		t.Fatalf("a fill over a full pool made %d slots", after-before)
	}
}

func TestNewClaimsASlot(t *testing.T) {
	f := testrepo.New(t)
	e := openPool(t, f, io.Discard, 1, 0)
	mustFill(t, e)
	slot := slotsOf(t, e)[0]

	var log bytes.Buffer
	e = openPool(t, f, &log, 1, 0)
	refills := 0
	e.Refill = func() { refills++ }
	dir := mustNew(t, e, "21", false)

	if !strings.Contains(log.String(), "claim pool slot") || strings.Contains(log.String(), "clone node_modules") {
		t.Fatalf("wk new did not take the slot:\n%s", log.String())
	}
	if !strings.Contains(log.String(), "skip the post-clone commands") {
		t.Fatalf("a claim at the slot's own commit ran the post-clone commands:\n%s", log.String())
	}
	if _, err := os.Stat(slot.path); !os.IsNotExist(err) {
		t.Fatal("the slot directory is still there after the claim")
	}
	if got := testrepo.Git(t, dir, "branch", "--show-current"); got != "wk/21" {
		t.Fatalf("branch = %q, want wk/21", got)
	}
	assertClean(t, dir)
	read(t, filepath.Join(dir, "node_modules/.installed-from"))
	if _, err := os.Stat(slot.marker); !os.IsNotExist(err) {
		t.Fatal("the claimed workspace still carries the pool marker")
	}
	if len(slotsOf(t, e)) != 0 {
		t.Fatal("the claimed workspace still counts as a slot")
	}
	if refills != 1 {
		t.Fatalf("Refill ran %d times, want once", refills)
	}
	resolved, err := filepath.EvalSymlinks(filepath.Join(dir, "node_modules/@ezh/core"))
	if err != nil || resolved != filepath.Join(dir, "packages/core") {
		t.Fatalf("node_modules/@ezh/core resolves to %q (%v) after the move", resolved, err)
	}
}

func TestClaimAfterMainMovedChecksOutTheDelta(t *testing.T) {
	f := testrepo.New(t)
	e := openPool(t, f, io.Discard, 1, 0)
	mustFill(t, e)
	f.Write(t, "web/new-page.ts", "export const fresh = 1;\n")
	f.Commit(t, "a page lands on main")

	var log bytes.Buffer
	e = openPool(t, f, &log, 1, 0)
	dir := mustNew(t, e, "22", false)
	if !strings.Contains(log.String(), "claim pool slot") {
		t.Fatalf("wk new did not take the slot:\n%s", log.String())
	}
	if got, want := testrepo.Git(t, dir, "rev-parse", "HEAD"), f.Git(t, "rev-parse", "main"); got != want {
		t.Fatalf("HEAD = %s, want main %s", got, want)
	}
	read(t, filepath.Join(dir, "web/new-page.ts"))
	if strings.Contains(log.String(), "skip the post-clone commands") {
		t.Fatal("the post-clone commands were skipped over a moved commit")
	}
	assertClean(t, dir)
}

func TestStaleSlotsAreNeverClaimedAndFillReplacesThem(t *testing.T) {
	f := testrepo.New(t)
	e := openPool(t, f, io.Discard, 1, 0)
	mustFill(t, e)
	old := slotsOf(t, e)[0]
	f.Write(t, "bun.lock", "lock v2\n")
	f.Commit(t, "bump the lockfile")
	if err := e.RefreshTemplate(); err != nil {
		t.Fatal(err)
	}

	var log bytes.Buffer
	e = openPool(t, f, &log, 1, 0)
	dir := mustNew(t, e, "23", false)
	if strings.Contains(log.String(), "claim pool slot") {
		t.Fatalf("wk new took a slot cloned from the old template:\n%s", log.String())
	}
	if got := read(t, filepath.Join(dir, "node_modules/.installed-from")); got != "lock v2\n" {
		t.Fatalf("workspace installed from %q", got)
	}
	mustFill(t, e)
	if _, err := os.Stat(old.path); !os.IsNotExist(err) {
		t.Fatal("fill left the stale slot on disk")
	}
	now := slotsOf(t, e)
	if len(now) != 1 || now[0].slot.State != slotReady {
		t.Fatalf("slots after fill = %+v, want one ready", now)
	}
}

func TestSparseAndFullNeverTakeEachOthersSlot(t *testing.T) {
	f := testrepo.New(t)
	e := openPool(t, f, io.Discard, 0, 1)
	mustFill(t, e)

	var log bytes.Buffer
	e = openPool(t, f, &log, 1, 1)
	mustNew(t, e, "24", false)
	if strings.Contains(log.String(), "claim pool slot") {
		t.Fatalf("a full wk new took the sparse slot:\n%s", log.String())
	}
	log.Reset()
	dir := mustNew(t, e, "25", true)
	if !strings.Contains(log.String(), "claim pool slot") {
		t.Fatalf("a sparse wk new did not take the sparse slot:\n%s", log.String())
	}
	if _, err := os.Stat(filepath.Join(dir, "docs/incomplete")); !os.IsNotExist(err) {
		t.Fatal("the sparse claim has docs/incomplete")
	}
	assertClean(t, dir)
}

func TestFillRemovesASlotWhoseFillStopped(t *testing.T) {
	f := testrepo.New(t)
	e := openPool(t, f, io.Discard, 1, 0)
	mustFill(t, e)
	dead := slotsOf(t, e)[0]
	dead.slot.State = slotFilling
	if err := writeSlot(dead.marker, dead.slot); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(dead.path, "README.md")); err != nil {
		t.Fatal(err)
	}
	mustFill(t, e)
	if _, err := os.Stat(dead.path); !os.IsNotExist(err) {
		t.Fatal("fill kept a slot whose fill had stopped")
	}
	if countReady(t, e, false) != 1 {
		t.Fatal("fill did not make a replacement")
	}
}

func TestFillTrimsAPoolLargerThanItsSize(t *testing.T) {
	f := testrepo.New(t)
	mustFill(t, openPool(t, f, io.Discard, 2, 0))
	e := openPool(t, f, io.Discard, 1, 0)
	mustFill(t, e)
	if n := len(slotsOf(t, e)); n != 1 {
		t.Fatalf("slots = %d after lowering pool.size to 1", n)
	}
}

func TestTwoClaimsAtOnceTakeTwoSlots(t *testing.T) {
	f := testrepo.New(t)
	mustFill(t, openPool(t, f, io.Discard, 2, 0))
	var wg sync.WaitGroup
	logs := make([]bytes.Buffer, 2)
	errs := make([]error, 2)
	for i, row := range []string{"31", "32"} {
		e := openPool(t, f, &logs[i], 2, 0)
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.New(row, DefaultCheckout)
		}()
	}
	wg.Wait()
	for i := range errs {
		if errs[i] != nil {
			t.Fatal(errs[i])
		}
		if !strings.Contains(logs[i].String(), "claim pool slot") {
			t.Fatalf("claim %d did not get a slot:\n%s", i, logs[i].String())
		}
	}
}

func TestPoolStatus(t *testing.T) {
	f := testrepo.New(t)
	e := openPool(t, f, io.Discard, 1, 0)
	mustFill(t, e)
	var out bytes.Buffer
	if err := e.PoolStatus(&out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "ready: 1 of 1 full, 0 of 0 sparse") {
		t.Fatalf("status:\n%s", out.String())
	}
}

// A change to the sparse patterns makes every sparse slot of the old shape unclaimable, and the
// next fill replaces it.
func TestSlotsOfAnOldSparseShapeAreReplaced(t *testing.T) {
	f := testrepo.New(t)
	e := openPool(t, f, io.Discard, 0, 1)
	mustFill(t, e)
	old := slotsOf(t, e)[0]

	var log bytes.Buffer
	e = openPool(t, f, &log, 0, 1)
	e.Cfg.Workspace.SparseExclude = []string{"docs/incomplete/**/*.png"}
	dir, err := e.New("26", SparseCheckout)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(log.String(), "claim pool slot") {
		t.Fatalf("a slot of the old shape was claimed:\n%s", log.String())
	}
	read(t, filepath.Join(dir, "docs/incomplete/plan/PLAN.md"))
	mustFill(t, e)
	if _, err := os.Stat(old.path); !os.IsNotExist(err) {
		t.Fatal("the old-shape slot is still on disk")
	}
}
