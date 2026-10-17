package landing

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

const ref = "refs/heads/wk/landing"

// shimLog holds every git call the package made under logged, one tab-separated argument
// list per line, after a "# <name>" line naming the call. predicted holds every result
// Predict returned, for TestShimLog's commit-tree check.
var (
	shimLog   string
	predicted []string
)

func TestMain(m *testing.M) {
	testrepo.Isolate()
	dir, err := os.MkdirTemp("", "wk-landing-")
	if err != nil {
		panic(err)
	}
	if err := writeShim(dir); err != nil {
		panic(err)
	}
	shimLog = filepath.Join(dir, "git.log")
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// realGit is the git the shim runs, for a test that moves a ref behind the package's back.
var realGit string

// writeShim puts a git first on PATH that appends its arguments to $WK_GIT_LOG, when that is
// set, and then runs the real git. The arguments are tab-separated, so the empty old value
// of a creation shows as two tabs. After a write-tree it runs $WK_GIT_AFTER_WRITE_TREE, when
// set, unlogged: the one seam a test has between Sync's tree and its CAS.
func writeShim(dir string) error {
	real, err := exec.LookPath("git")
	if err != nil {
		return err
	}
	realGit = real
	script := "#!/bin/bash\n" +
		"[ -z \"$WK_GIT_LOG\" ] || { printf '%s\\t' \"$@\" >>\"$WK_GIT_LOG\"; printf '\\n' >>\"$WK_GIT_LOG\"; }\n" +
		"'" + real + "' \"$@\"\n" +
		"status=$?\n" +
		"case \" $* \" in *' write-tree '*) [ -z \"$WK_GIT_AFTER_WRITE_TREE\" ] || env -u WK_GIT_LOG -u WK_GIT_AFTER_WRITE_TREE sh -c \"$WK_GIT_AFTER_WRITE_TREE\" >/dev/null 2>&1 ;; esac\n" +
		"exit $status\n"
	if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0o755); err != nil {
		return err
	}
	return os.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

// logged runs fn with every git call the package makes written to the shim log under name.
// The tests' own fixture calls run with the variable unset and are not logged.
func logged(t *testing.T, name string, fn func()) {
	t.Helper()
	appendLog(t, "# "+name+"\n")
	if err := os.Setenv("WK_GIT_LOG", shimLog); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Unsetenv("WK_GIT_LOG") }()
	fn()
}

func appendLog(t *testing.T, line string) {
	t.Helper()
	f, err := os.OpenFile(shimLog, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(line); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

type repo struct {
	testrepo.Fixture
	c0 string // the first commit on main, every changeset's base
}

func newRepo(t *testing.T) repo {
	t.Helper()
	f := testrepo.New(t)
	return repo{Fixture: f, c0: f.Git(t, "rev-parse", "HEAD")}
}

// worktree adds a worktree at c0: on a new branch, as `wk new` makes a workspace, or detached
// when branch is empty, for a scratch tree.
func (r repo) worktree(t *testing.T, branch string) string {
	t.Helper()
	dir := t.TempDir()
	if branch == "" {
		r.Git(t, "worktree", "add", "--quiet", "--detach", dir, r.c0)
	} else {
		r.Git(t, "worktree", "add", "--quiet", "-b", branch, dir, r.c0)
	}
	return dir
}

// treeWith is c0's tree with rel replaced by content: a changeset's tree, built in a detached
// scratch worktree so no branch and no index the tests look at moves.
func (r repo) treeWith(t *testing.T, rel, content string) string {
	t.Helper()
	dir := r.worktree(t, "")
	write(t, dir, rel, content)
	testrepo.Git(t, dir, "add", "-A")
	return testrepo.Git(t, dir, "write-tree")
}

// land predicts tree, based on c0, against tip and lands it through the package.
func (r repo) land(t *testing.T, tip, tree, message string) string {
	t.Helper()
	var commit string
	logged(t, "Land", func() {
		result, conflicts, err := Predict(r.Repo, r.c0, tip, tree)
		if err != nil || len(conflicts) > 0 {
			t.Fatalf("Predict = %q, %v, %v", result, conflicts, err)
		}
		predicted = append(predicted, result)
		if commit, err = Land(r.Repo, ref, tip, result, message); err != nil {
			t.Fatal(err)
		}
	})
	return commit
}

// landed creates the landing ref at c0 and lands README.md replaced by content: the tip ahead
// of main that most tests start from.
func (r repo) landed(t *testing.T, content string) string {
	t.Helper()
	var tip string
	logged(t, "Ensure", func() {
		var err error
		if tip, err = Ensure(r.Repo, ref, r.c0); err != nil {
			t.Fatal(err)
		}
	})
	return r.land(t, tip, r.treeWith(t, "README.md", content), "row 1: readme")
}

func write(t *testing.T, dir, rel, content string) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func read(t *testing.T, dir, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestLandPutsThePredictedTreeOnTheRef(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	if got := r.Git(t, "rev-parse", ref); got != first {
		t.Fatalf("ref = %s, want %s", got, first)
	}
	// A second changeset, still based on c0, touches another path: it merges onto the moved tip.
	tree := r.treeWith(t, "packages/core/index.ts", "export const core = 2;\n")
	var result, commit string
	var conflicts []string
	var err error
	logged(t, "Predict", func() { result, conflicts, err = Predict(r.Repo, r.c0, first, tree) })
	if err != nil || conflicts != nil {
		t.Fatalf("Predict = %q, %v, %v", result, conflicts, err)
	}
	predicted = append(predicted, result)
	logged(t, "Land", func() { commit, err = Land(r.Repo, ref, first, result, "row 2: core") })
	if err != nil {
		t.Fatal(err)
	}
	if got := r.Git(t, "rev-parse", commit+"^{tree}"); got != result {
		t.Fatalf("landed tree = %s, want the predicted %s", got, result)
	}
	if got := r.Git(t, "rev-parse", ref); got != commit {
		t.Fatalf("ref = %s, want %s", got, commit)
	}
	if got := r.Git(t, "rev-parse", commit+"^"); got != first {
		t.Fatalf("parent = %s, want the tip %s", got, first)
	}
	if got := r.Git(t, "log", "-1", "--format=%s", commit); got != "row 2: core" {
		t.Fatalf("message = %q", got)
	}
	for _, p := range []string{"README.md", "packages/core/index.ts"} {
		want := r.Git(t, "rev-parse", first+":"+p)
		if p != "README.md" {
			want = r.Git(t, "rev-parse", tree+":"+p)
		}
		if got := r.Git(t, "rev-parse", result+":"+p); got != want {
			t.Fatalf("%s in the result = %s, want %s", p, got, want)
		}
	}
}

func TestPredictConflictWritesNothing(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	tree := r.treeWith(t, "README.md", "other\n")
	before := r.Git(t, "for-each-ref")
	var result string
	var conflicts []string
	var err error
	logged(t, "Predict", func() { result, conflicts, err = Predict(r.Repo, r.c0, first, tree) })
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(conflicts, []string{"README.md"}) {
		t.Fatalf("conflicts = %v", conflicts)
	}
	if got := r.Git(t, "cat-file", "-p", result+":README.md"); !strings.Contains(got, "<<<<<<<") {
		t.Fatalf("the result holds no markers:\n%s", got)
	}
	if after := r.Git(t, "for-each-ref"); after != before {
		t.Fatalf("a ref moved:\n%s\nwas\n%s", after, before)
	}
}

func TestLandRefusesAMovedRef(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	tree := r.treeWith(t, "packages/core/index.ts", "export const core = 2;\n")
	// Predicted against c0 as the tip, as a loop pass that read the ref before the landing did.
	var result string
	var err error
	logged(t, "Predict", func() { result, _, err = Predict(r.Repo, r.c0, r.c0, tree) })
	if err != nil {
		t.Fatal(err)
	}
	predicted = append(predicted, result)
	logged(t, "Land", func() { _, err = Land(r.Repo, ref, r.c0, result, "row 2: core") })
	if !errors.Is(err, ErrMoved) || !strings.Contains(err.Error(), first) {
		t.Fatalf("err = %v, want ErrMoved naming %s", err, first)
	}
	if got := r.Git(t, "rev-parse", ref); got != first {
		t.Fatalf("ref = %s, want %s unchanged", got, first)
	}
}

func TestEnsureFourCases(t *testing.T) {
	r := newRepo(t)
	ensure := func(name, main string) (string, error) {
		var tip string
		var err error
		logged(t, name, func() { tip, err = Ensure(r.Repo, ref, main) })
		return tip, err
	}
	check := func(what, tip string, err error, want string) {
		t.Helper()
		if err != nil || tip != want {
			t.Fatalf("%s: Ensure = %s, %v, want %s", what, tip, err, want)
		}
		if got := r.Git(t, "rev-parse", ref); got != want {
			t.Fatalf("%s: ref = %s, want %s", what, got, want)
		}
	}
	tip, err := ensure("Ensure-missing", r.c0)
	check("missing", tip, err, r.c0)

	r.Write(t, "web/page.ts", "export const page = 2;\n")
	r.Commit(t, "page")
	m1 := r.Git(t, "rev-parse", "HEAD")
	tip, err = ensure("Ensure-behind", m1)
	check("behind", tip, err, m1)

	ahead := r.land(t, m1, r.treeWith(t, "packages/core/index.ts", "export const core = 2;\n"), "row 2: core")
	tip, err = ensure("Ensure-ahead", m1)
	check("ahead", tip, err, ahead)

	r.Write(t, "web/page.ts", "export const page = 3;\n")
	r.Commit(t, "page again")
	m2 := r.Git(t, "rev-parse", "HEAD")
	_, err = ensure("Ensure-diverged", m2)
	if !errors.Is(err, ErrDiverged) || !strings.Contains(err.Error(), m2) || !strings.Contains(err.Error(), ahead) {
		t.Fatalf("err = %v, want ErrDiverged naming %s and %s", err, m2, ahead)
	}
	if got := r.Git(t, "rev-parse", ref); got != ahead {
		t.Fatalf("ref = %s, want %s unchanged", got, ahead)
	}
}

func TestSyncMovesTheBranchAndKeepsTheWork(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	ws := r.worktree(t, "wk/1")
	write(t, ws, "web/page.ts", "export const page = 2;\n")
	write(t, ws, "notes.txt", "todo\n")
	var conflicts []string
	var err error
	logged(t, "Sync", func() { conflicts, err = Sync(ws, r.c0, first) })
	if err != nil || conflicts != nil {
		t.Fatalf("Sync = %v, %v", conflicts, err)
	}
	if got := testrepo.Git(t, ws, "rev-parse", "refs/heads/wk/1"); got != first {
		t.Fatalf("branch = %s, want %s", got, first)
	}
	if got := testrepo.Git(t, ws, "symbolic-ref", "HEAD"); got != "refs/heads/wk/1" {
		t.Fatalf("HEAD = %s", got)
	}
	if got := read(t, ws, "README.md"); got != "first\n" {
		t.Fatalf("README.md = %q, want the tip's", got)
	}
	if got := read(t, ws, "web/page.ts"); got != "export const page = 2;\n" {
		t.Fatalf("web/page.ts = %q, want the edit kept", got)
	}
	if got := testrepo.Git(t, ws, "status", "--porcelain"); got != " M web/page.ts\n?? notes.txt" {
		t.Fatalf("status:\n%s", got)
	}
	if got := testrepo.Git(t, ws, "diff", "--cached", "--name-only"); got != "" {
		t.Fatalf("staged: %s", got)
	}
	noLeftoverIndex(t, ws)
}

func TestSyncLeavesTheMarkers(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	ws := r.worktree(t, "wk/2")
	write(t, ws, "README.md", "mine\n")
	var conflicts []string
	var err error
	logged(t, "Sync-conflict", func() { conflicts, err = Sync(ws, r.c0, first) })
	if err != nil || !slices.Equal(conflicts, []string{"README.md"}) {
		t.Fatalf("Sync = %v, %v", conflicts, err)
	}
	got := read(t, ws, "README.md")
	if !strings.Contains(got, "<<<<<<<") || !strings.Contains(got, "mine\n") || !strings.Contains(got, "first\n") {
		t.Fatalf("README.md:\n%s", got)
	}
	if got := testrepo.Git(t, ws, "rev-parse", "refs/heads/wk/2"); got != first {
		t.Fatalf("branch = %s, want %s", got, first)
	}
	if got := testrepo.Git(t, ws, "status", "--porcelain"); got != " M README.md" {
		t.Fatalf("status:\n%s", got)
	}
	noLeftoverIndex(t, ws)
}

func TestSyncRefusesABranchWithCommits(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	ws := r.worktree(t, "wk/3")
	write(t, ws, "web/page.ts", "export const page = 2;\n")
	testrepo.Git(t, ws, "add", "-A")
	testrepo.Git(t, ws, "commit", "--quiet", "--no-verify", "-m", "own")
	own := testrepo.Git(t, ws, "rev-parse", "HEAD")
	write(t, ws, "notes.txt", "todo\n")
	var err error
	logged(t, "Sync-refused", func() { _, err = Sync(ws, r.c0, first) })
	if err == nil || !strings.Contains(err.Error(), own) {
		t.Fatalf("err = %v, want a refusal naming %s", err, own)
	}
	if got := testrepo.Git(t, ws, "rev-parse", "refs/heads/wk/3"); got != own {
		t.Fatalf("branch = %s, want %s unchanged", got, own)
	}
	if got := testrepo.Git(t, ws, "status", "--porcelain"); got != "?? notes.txt" {
		t.Fatalf("status:\n%s", got)
	}
	if got := read(t, ws, "README.md"); got != "demo\n" {
		t.Fatalf("README.md = %q, want untouched", got)
	}
}

func TestSyncRefusesThePrimary(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	r.Write(t, "web/page.ts", "export const page = 2;\n")
	var err error
	logged(t, "Sync-primary", func() { _, err = Sync(r.Repo, r.c0, first) })
	if err == nil || !strings.Contains(err.Error(), r.Repo) {
		t.Fatalf("err = %v, want a refusal naming %s", err, r.Repo)
	}
	if got := r.Git(t, "rev-parse", "refs/heads/main"); got != r.c0 {
		t.Fatalf("main = %s, want %s unchanged", got, r.c0)
	}
	if got := r.Git(t, "status", "--porcelain"); got != " M web/page.ts" {
		t.Fatalf("status:\n%s", got)
	}
}

func TestSyncSaysWhatAMovedBranchLeft(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	ws := r.worktree(t, "wk/4")
	write(t, ws, "web/page.ts", "export const page = 2;\n")
	// Another process moves the branch after Sync built its tree and before its CAS.
	other := r.Git(t, "commit-tree", r.treeWith(t, "notes.txt", "other\n"), "-p", r.c0, "-m", "other")
	t.Setenv("WK_GIT_AFTER_WRITE_TREE", realGit+" -C "+ws+" update-ref refs/heads/wk/4 "+other+" "+r.c0)
	var err error
	logged(t, "Sync-moved", func() { _, err = Sync(ws, r.c0, first) })
	if err == nil || !strings.Contains(err.Error(), "run wk sync again") || !strings.Contains(err.Error(), r.c0) {
		t.Fatalf("err = %v, want the state it left and what to do", err)
	}
	if got := testrepo.Git(t, ws, "rev-parse", "refs/heads/wk/4"); got != other {
		t.Fatalf("branch = %s, want %s, where the other process put it", got, other)
	}
	if got := read(t, ws, "README.md"); got != "first\n" {
		t.Fatalf("README.md = %q, want the merge the error names", got)
	}
	noLeftoverIndex(t, ws)
}

func noLeftoverIndex(t *testing.T, ws string) {
	t.Helper()
	entries, err := os.ReadDir(testrepo.Git(t, ws, "rev-parse", "--absolute-git-dir"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "wk-sync-") {
			t.Fatalf("the private index %s was left behind", e.Name())
		}
	}
}

func TestFastForward(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	ff := func(name, expect string) ([]string, error) {
		var blockers []string
		var err error
		logged(t, name, func() { blockers, err = FastForward(r.Repo, "main", ref, expect) })
		return blockers, err
	}
	unmoved := func(what string) {
		t.Helper()
		if got := r.Git(t, "rev-parse", "main"); got != r.c0 {
			t.Fatalf("%s: main = %s, want %s unchanged", what, got, r.c0)
		}
	}
	if _, err := ff("FastForward-stale", r.c0); err == nil {
		t.Fatal("a stale expect was not refused")
	}
	unmoved("stale")

	r.Git(t, "checkout", "--quiet", "-b", "other")
	if _, err := ff("FastForward-other", first); err == nil {
		t.Fatal("another branch checked out was not refused")
	}
	r.Git(t, "checkout", "--quiet", "main")
	unmoved("other")

	r.Write(t, "README.md", "edited in the checkout\n")
	blockers, err := ff("FastForward-blocked", first)
	if err == nil || !slices.Equal(blockers, []string{"README.md"}) {
		t.Fatalf("FastForward = %v, %v, want the blocker refused", blockers, err)
	}
	unmoved("blocked")
	if got := read(t, r.Repo, "README.md"); got != "edited in the checkout\n" {
		t.Fatalf("README.md = %q, want the edit kept", got)
	}
	r.Git(t, "checkout", "--quiet", "--", "README.md")

	if blockers, err := ff("FastForward", first); err != nil || blockers != nil {
		t.Fatalf("FastForward = %v, %v", blockers, err)
	}
	if got := r.Git(t, "rev-parse", "main"); got != first {
		t.Fatalf("main = %s, want %s", got, first)
	}
	if got := read(t, r.Repo, "README.md"); got != "first\n" {
		t.Fatalf("README.md = %q, want the landing's", got)
	}
	if got := r.Git(t, "status", "--porcelain"); got != "" {
		t.Fatalf("status:\n%s", got)
	}
}

func TestPushToABareRemote(t *testing.T) {
	r := newRepo(t)
	first := r.landed(t, "first\n")
	bare := filepath.Join(r.Root, "origin.git")
	testrepo.Git(t, r.Root, "init", "--quiet", "--bare", bare)
	r.Git(t, "remote", "add", "origin", bare)
	var err error
	logged(t, "FastForward", func() { _, err = FastForward(r.Repo, "main", ref, first) })
	if err != nil {
		t.Fatal(err)
	}
	logged(t, "Push", func() { err = Push(r.Repo, "origin", "main") })
	if err != nil {
		t.Fatal(err)
	}
	if got := testrepo.Git(t, bare, "rev-parse", "refs/heads/main"); got != first {
		t.Fatalf("origin main = %s, want %s", got, first)
	}
}

// readOnly is every git call the package may make in any section. merge-tree writes objects
// and never a ref; symbolic-ref and worktree are read-only in the forms allowed below.
var readOnly = map[string]bool{"rev-parse": true, "merge-base": true, "merge-tree": true, "diff": true, "ls-files": true}

// writes is every writing call a section may make; anything else that writes fails the test.
var writes = map[string][]string{
	"Land":        {"commit-tree", "update-ref"},
	"Ensure":      {"update-ref"},
	"Sync":        {"read-tree", "add", "write-tree", "update-ref", "reset"},
	"FastForward": {"merge"},
	"Push":        {"push"},
}

// mayRun says whether a section may make this call: a read, or one of its own writes.
func mayRun(section, cmd string, rest []string) bool {
	switch {
	case readOnly[cmd]:
		return true
	case cmd == "symbolic-ref":
		// The read form is options then HEAD; a write names a ref before its target.
		if len(rest) == 0 || rest[len(rest)-1] != "HEAD" {
			return false
		}
		for _, a := range rest[:len(rest)-1] {
			if !strings.HasPrefix(a, "--") {
				return false
			}
		}
		return true
	case cmd == "worktree":
		return len(rest) > 0 && rest[0] == "list"
	}
	prefix, _, _ := strings.Cut(section, "-")
	return slices.Contains(writes[prefix], cmd)
}

// TestShimLog reads every call the tests above made through the package: each section makes
// only reads and its own writes; the only ref writes are the CAS on the landing ref and the
// branch move in Sync, each with its old value; commit-tree is given exactly a predicted tree;
// main is named only by FastForward and Push. It runs last, in source order.
func TestShimLog(t *testing.T) {
	data, err := os.ReadFile(shimLog)
	if err != nil {
		t.Fatal(err)
	}
	section := ""
	var updates, commits int
	for _, line := range strings.Split(string(data), "\n") {
		if name, ok := strings.CutPrefix(line, "# "); ok {
			section = name
			continue
		}
		if line == "" {
			continue
		}
		args := strings.Split(strings.TrimSuffix(line, "\t"), "\t")
		cmd, rest := subcommand(args)
		if !mayRun(section, cmd, rest) {
			t.Errorf("%s runs %q, which it may not", section, args)
		}
		switch cmd {
		case "update-ref":
			updates++
			if len(rest) != 3 {
				t.Errorf("%s: update-ref %q, want ref, new and old", section, rest)
				continue
			}
			inSync := strings.HasPrefix(section, "Sync") && strings.HasPrefix(rest[0], "refs/heads/wk/")
			if rest[0] != ref && !inSync {
				t.Errorf("%s: update-ref %q, want the landing ref or Sync's branch", section, rest)
			}
			if rest[0] == "refs/heads/main" {
				t.Errorf("%s: update-ref writes main", section)
			}
		case "commit-tree":
			commits++
			if !slices.Contains(predicted, rest[0]) {
				t.Errorf("%s: commit-tree %q, want a predicted tree", section, rest)
			}
		}
		if strings.HasPrefix(section, "FastForward") || section == "Push" {
			continue
		}
		for _, a := range args {
			if strings.Contains(a, "main") {
				t.Errorf("%s names main: %q", section, args)
			}
		}
	}
	if updates == 0 || commits == 0 || len(predicted) == 0 {
		t.Fatalf("the log saw %d ref writes and %d commits; the shim is not on PATH", updates, commits)
	}
	t.Logf("shim log, %d ref writes, %d commits:\n%s", updates, commits, data)
}

// subcommand skips git's -C and -c options to the subcommand and its arguments.
func subcommand(args []string) (string, []string) {
	for len(args) >= 2 && (args[0] == "-C" || args[0] == "-c") {
		args = args[2:]
	}
	if len(args) == 0 {
		return "", nil
	}
	return args[0], args[1:]
}
