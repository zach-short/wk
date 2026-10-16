package wk

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/testrepo"
)

func TestMain(m *testing.M) {
	testrepo.Isolate()
	os.Exit(m.Run())
}

func open(t *testing.T, f testrepo.Fixture, log io.Writer) *Env {
	t.Helper()
	e, err := Open(f.Home, f.Repo, log)
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func mustNew(t *testing.T, e *Env, row string, sparse bool) string {
	t.Helper()
	c := DefaultCheckout
	if sparse {
		c = SparseCheckout
	}
	dir, err := e.New(row, c)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func assertClean(t *testing.T, dir string) {
	t.Helper()
	dirty, err := gitx.DirtyPaths(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(dirty) > 0 {
		t.Fatalf("%s is not clean: %v", dir, dirty)
	}
}

func TestNewFull(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	dir := mustNew(t, e, "7", false)

	if want := filepath.Join(f.Repo, ".claude/worktrees/wk-7"); dir != want {
		t.Fatalf("path = %q, want %q", dir, want)
	}
	if got, want := testrepo.Git(t, dir, "rev-parse", "HEAD"), f.Git(t, "rev-parse", "main"); got != want {
		t.Fatalf("HEAD = %s, want main's %s", got, want)
	}
	if got := testrepo.Git(t, dir, "branch", "--show-current"); got != "wk/7" {
		t.Fatalf("branch = %q, want wk/7", got)
	}
	read(t, filepath.Join(dir, "docs/incomplete/plan/PLAN.md"))
	read(t, filepath.Join(dir, "web/node_modules/react/index.js"))
	if got := read(t, filepath.Join(dir, "web/.next/cache/built-with")); got != "placeholder\n" {
		t.Fatalf("template build ran with %q, want the gates.env placeholder", got)
	}
	if got := read(t, filepath.Join(dir, ".env")); got != "SECRET=main-checkout\n" {
		t.Fatal("the root .env was not copied from the main checkout")
	}
	if got := read(t, filepath.Join(dir, "web/.env")); got != "WEB_SECRET=main-checkout\n" {
		t.Fatal("web/.env was not copied from the main checkout")
	}
	assertClean(t, dir)
}

// A cloned node_modules keeps its relative workspace links, so they resolve to the new
// workspace's own packages and never into the template (DESIGN.md 1.2).
func TestClonedWorkspaceLinkResolvesInsideWorkspace(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	dir := mustNew(t, e, "8", false)

	link := filepath.Join(dir, "node_modules/@ezh/core")
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if target != "../../packages/core" {
		t.Fatalf("link target = %q, want the relative ../../packages/core", target)
	}
	resolved, err := filepath.EvalSymlinks(link)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "packages/core"); resolved != want {
		t.Fatalf("node_modules/@ezh/core resolves to %s, want %s", resolved, want)
	}
	if strings.HasPrefix(resolved, e.TemplateDir()) {
		t.Fatalf("the link resolves into the template: %s", resolved)
	}
}

func TestNewSparse(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	dir := mustNew(t, e, "9", true)

	if _, err := os.Stat(filepath.Join(dir, "docs/incomplete")); !os.IsNotExist(err) {
		t.Fatalf("docs/incomplete exists in a sparse workspace (err %v)", err)
	}
	read(t, filepath.Join(dir, "web/page.ts"))
	read(t, filepath.Join(dir, "node_modules/.installed-from"))
	if ls := testrepo.Git(t, dir, "ls-files", "docs/incomplete"); ls == "" {
		t.Fatal("the index lost docs/incomplete; sparse must only skip writing it")
	}
	assertClean(t, dir)
}

func TestNewRefusesBadRows(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	for _, row := range []string{"", "abc", "0", "-3", "07", "1/2", "../x", "1234567890"} {
		if _, err := e.New(row, DefaultCheckout); err == nil || !strings.Contains(err.Error(), "not a positive integer") {
			t.Errorf("row %q: err = %v, want a refusal", row, err)
		}
	}
}

func TestOpenRefusesRelativePaths(t *testing.T) {
	f := testrepo.New(t)
	if _, err := Open(f.Home, "demo", io.Discard); err == nil || !strings.Contains(err.Error(), "relative") {
		t.Fatalf("relative repo: err = %v, want a refusal", err)
	}
	if _, err := Open("home", f.Repo, io.Discard); err == nil || !strings.Contains(err.Error(), "relative") {
		t.Fatalf("relative home: err = %v, want a refusal", err)
	}
}

func TestNewRefusesExistingRowAndBranch(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	mustNew(t, e, "3", false)
	if _, err := e.New("3", DefaultCheckout); err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("second wk new 3: err = %v, want a refusal", err)
	}
	f.Git(t, "branch", "wk/4", "main")
	if _, err := e.New("4", DefaultCheckout); err == nil || !strings.Contains(err.Error(), "branch wk/4 already exists") {
		t.Fatalf("wk new 4 over an existing branch: err = %v, want a refusal", err)
	}
	if _, err := os.Stat(filepath.Join(f.Repo, ".claude/worktrees/wk-4")); !os.IsNotExist(err) {
		t.Fatal("a refused wk new left a directory behind")
	}
}

func TestNewRefreshesWhenLockfileChanges(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	mustNew(t, e, "1", false)
	before, _, err := e.readState()
	if err != nil {
		t.Fatal(err)
	}

	f.Write(t, "bun.lock", "lock v2\n")
	f.Commit(t, "bump the lockfile")
	var log bytes.Buffer
	e = open(t, f, &log)
	dir := mustNew(t, e, "2", false)

	after, _, err := e.readState()
	if err != nil {
		t.Fatal(err)
	}
	if after.RefreshBlob == before.RefreshBlob {
		t.Fatal("the template state still records the old lockfile blob")
	}
	if want := f.Git(t, "rev-parse", "main:bun.lock"); after.RefreshBlob != want {
		t.Fatalf("recorded blob %s, want main's %s", after.RefreshBlob, want)
	}
	if got, want := testrepo.Git(t, e.TemplateDir(), "rev-parse", "HEAD"), f.Git(t, "rev-parse", "main"); got != want {
		t.Fatalf("template HEAD = %s, want main %s", got, want)
	}
	if got := read(t, filepath.Join(dir, "node_modules/.installed-from")); got != "lock v2\n" {
		t.Fatalf("workspace installed from %q, want the new lockfile", got)
	}
	if !strings.Contains(log.String(), "bun.lock changed on main") {
		t.Fatalf("log does not say why it refreshed:\n%s", log.String())
	}
}

func TestNewDoesNotRefreshAnUnchangedTemplate(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	mustNew(t, e, "1", false)
	var log bytes.Buffer
	e = open(t, f, &log)
	mustNew(t, e, "2", false)
	if strings.Contains(log.String(), "refreshing") {
		t.Fatalf("a second wk new refreshed an unchanged template:\n%s", log.String())
	}
}

// waitWriter closes seen the first time a write contains want.
type waitWriter struct {
	mu   sync.Mutex
	want string
	seen chan struct{}
	done bool
}

func (w *waitWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if !w.done && bytes.Contains(p, []byte(w.want)) {
		w.done = true
		close(w.seen)
	}
	return len(p), nil
}

func TestNewWaitsForAHeldRefreshLock(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	if err := e.RefreshTemplate(); err != nil {
		t.Fatal(err)
	}
	held, err := flock.Take(e.lockPath(), true, nil)
	if err != nil {
		t.Fatal(err)
	}

	w := &waitWriter{want: "waiting: the template is being refreshed", seen: make(chan struct{})}
	e = open(t, f, w)
	type result struct {
		dir string
		err error
	}
	done := make(chan result, 1)
	go func() {
		dir, err := e.New("5", DefaultCheckout)
		done <- result{dir, err}
	}()

	select {
	case <-w.seen:
	case r := <-done:
		t.Fatalf("wk new finished while the refresh lock was held: %+v", r)
	case <-time.After(10 * time.Second):
		t.Fatal("wk new never said it was waiting")
	}
	select {
	case r := <-done:
		t.Fatalf("wk new finished while the refresh lock was held: %+v", r)
	case <-time.After(200 * time.Millisecond):
	}
	if _, err := os.Stat(filepath.Join(f.Repo, ".claude/worktrees/wk-5")); !os.IsNotExist(err) {
		t.Fatal("wk new made the worktree before it had the lock")
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case r := <-done:
		if r.err != nil {
			t.Fatal(r.err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("wk new did not finish after the lock was released")
	}
}

func TestRefreshTwiceLeavesOneTemplateAndOneMarker(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	for range 2 {
		if err := e.RefreshTemplate(); err != nil {
			t.Fatal(err)
		}
	}
	list, err := gitx.Worktrees(f.Repo)
	if err != nil {
		t.Fatal(err)
	}
	templates := 0
	for _, w := range list {
		if w.Path == e.TemplateDir() {
			templates++
			if !w.Detached {
				t.Fatal("the template is on a branch; it must be detached at main's commit")
			}
		}
	}
	if templates != 1 || len(list) != 2 {
		t.Fatalf("worktrees = %+v, want the main checkout and one template", list)
	}
	markers, err := filepath.Glob(filepath.Join(f.Repo, ".git/worktrees/*", MarkerName))
	if err != nil {
		t.Fatal(err)
	}
	if len(markers) != 1 {
		t.Fatalf("markers = %v, want exactly one", markers)
	}
	assertClean(t, e.TemplateDir())
}

func TestRefreshRefusesADirtyTemplate(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	if err := e.RefreshTemplate(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.TemplateDir(), "README.md"), []byte("edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := e.RefreshTemplate(); err == nil || !strings.Contains(err.Error(), "never used for work") {
		t.Fatalf("refresh over an edited template: err = %v, want a refusal", err)
	}
}

func TestRefreshRefusesAForeignDirectory(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	if err := os.MkdirAll(e.TemplateDir(), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := e.RefreshTemplate(); err == nil || !strings.Contains(err.Error(), "will not take it over") {
		t.Fatalf("refresh over a plain directory: err = %v, want a refusal", err)
	}
}

func TestOpenSaysOnceWhenConfigIsMissing(t *testing.T) {
	f := testrepo.New(t)
	if err := os.Remove(filepath.Join(f.Home, "demo.toml")); err != nil {
		t.Fatal(err)
	}
	var log bytes.Buffer
	e := open(t, f, &log)
	if n := strings.Count(log.String(), "using the defaults"); n != 1 {
		t.Fatalf("log said %d times that it uses the defaults:\n%s", n, log.String())
	}
	if e.Cfg.Template.Path != ".claude/worktrees/wk-template" {
		t.Fatalf("defaults not used: %+v", e.Cfg.Template)
	}
}

// The pattern form of sparse_exclude leaves out the images under docs/incomplete and keeps
// every markdown, so a sparse workspace can still grep the design documents (D8 as amended).
func TestPatternSparseKeepsTextAndDropsImages(t *testing.T) {
	f := testrepo.New(t)
	f.Write(t, "docs/incomplete/plan/shots/a.png", "png\n")
	f.Commit(t, "a screenshot")
	e := open(t, f, io.Discard)
	e.Cfg.Workspace.Sparse = true
	e.Cfg.Workspace.SparseExclude = []string{"docs/incomplete/**/*.png"}
	dir := mustNew(t, e, "51", false)

	read(t, filepath.Join(dir, "docs/incomplete/plan/PLAN.md"))
	if _, err := os.Stat(filepath.Join(dir, "docs/incomplete/plan/shots/a.png")); !os.IsNotExist(err) {
		t.Fatalf("the screenshot is in the sparse workspace (err %v)", err)
	}
	assertClean(t, dir)

	full, err := e.New("52", FullCheckout)
	if err != nil {
		t.Fatal(err)
	}
	read(t, filepath.Join(full, "docs/incomplete/plan/shots/a.png"))
}

func TestSparseExcludesTurnsDirectoriesAndPatternsIntoNegations(t *testing.T) {
	got := strings.Join(sparseExcludes([]string{"docs/incomplete", "/docs/x/**/*.svg/"}), " ")
	if want := "!/docs/incomplete/ !/docs/x/**/*.svg"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

// A base that moved without a lockfile change gets a light refresh, with no build, and the
// workspace made at the new head then skips its own post-clone commands.
func TestNewFollowsTheBaseWithALightRefresh(t *testing.T) {
	f := testrepo.New(t)
	mustNew(t, open(t, f, io.Discard), "53", false)
	f.Write(t, "web/next.ts", "export const next = 1;\n")
	f.Commit(t, "main moves")

	var log bytes.Buffer
	e := open(t, f, &log)
	dir := mustNew(t, e, "54", false)
	if !strings.Contains(log.String(), "updating the template first, without a build: main moved") {
		t.Fatalf("no light refresh:\n%s", log.String())
	}
	if strings.Contains(log.String(), "in web") {
		t.Fatalf("the light refresh ran the build:\n%s", log.String())
	}
	if !strings.Contains(log.String(), "skip the post-clone commands") {
		t.Fatalf("the workspace at the template's commit ran its post-clone commands:\n%s", log.String())
	}
	main := f.Git(t, "rev-parse", "main")
	if got := testrepo.Git(t, e.TemplateDir(), "rev-parse", "HEAD"); got != main {
		t.Fatalf("template HEAD = %s, want main %s", got, main)
	}
	read(t, filepath.Join(dir, "web/next.ts"))
	assertClean(t, dir)
}

func TestNewWithoutFollowBaseRunsThePostCloneCommands(t *testing.T) {
	f := testrepo.New(t)
	mustNew(t, open(t, f, io.Discard), "55", false)
	f.Write(t, "web/next.ts", "export const next = 1;\n")
	f.Commit(t, "main moves")

	var log bytes.Buffer
	e := open(t, f, &log)
	e.Cfg.Template.FollowBase = false
	mustNew(t, e, "56", false)
	if strings.Contains(log.String(), "updating the template") || strings.Contains(log.String(), "skip the post-clone") {
		t.Fatalf("follow_base = false still followed the base:\n%s", log.String())
	}
}

// With every create slot taken, a `wk new` waits and says so, and goes on once one frees.
func TestNewWaitsForACreateSlot(t *testing.T) {
	f := testrepo.New(t)
	e := open(t, f, io.Discard)
	if err := e.RefreshTemplate(); err != nil {
		t.Fatal(err)
	}
	e.Cfg.New.MaxParallel = 1
	held, err := flock.Take(filepath.Join(e.State, "create-0.lock"), true, nil)
	if err != nil {
		t.Fatal(err)
	}
	w := &waitWriter{want: "workspaces are already cloning", seen: make(chan struct{})}
	e.log = &syncWriter{w: w}
	done := make(chan error, 1)
	go func() {
		_, err := e.New("57", DefaultCheckout)
		done <- err
	}()
	select {
	case <-w.seen:
	case err := <-done:
		t.Fatalf("wk new finished while every create slot was held: %v", err)
	case <-time.After(10 * time.Second):
		t.Fatal("wk new never said it was waiting for a slot")
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("wk new did not finish after the slot freed")
	}
}
