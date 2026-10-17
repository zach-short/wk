package slot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

func TestCheckTrim(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		path string
		ok   bool
	}{
		{"web/.next/cache", true},
		{"./web/.next/cache", true},
		{"cache", true},
		{"/tmp/cache", false},
		{"../other/web/.next/cache", false},
		{"web/../../x", false},
		{"web/..", false},
		{".", false},
		{"", false},
		{"./", false},
	} {
		if err := checkTrim([]string{tc.path}); (err == nil) != tc.ok {
			t.Errorf("checkTrim(%q) = %v, want ok %v", tc.path, err, tc.ok)
		}
	}
}

// trimFixture is a throwaway repository with a build cache under web/, and a machine file in a
// temporary home; the command runs in web/, so the trim path is relative to the top level.
func trimFixture(t *testing.T) (fixture, spec, string) {
	f := newFixture(t, capacityOne)
	repo := testrepo.New(t)
	cache := filepath.Join(repo.Repo, "web", ".next", "cache")
	writeFile(t, filepath.Join(cache, "fetch", "entry"), "cached\n")
	s := f.spec("heavy")
	s.Dir = filepath.Join(repo.Repo, "web")
	s.Trim = []string{"web/.next/cache"}
	s.TrimAllowed = true
	return f, s, cache
}

func TestRunTrimsAfterAnyExit(t *testing.T) {
	t.Parallel()
	for _, status := range []string{"0", "1"} {
		t.Run("exit "+status, func(t *testing.T) {
			t.Parallel()
			_, s, cache := trimFixture(t)
			code, stderr := runHere(t, s, "sh", "-c", "exit "+status)
			if want := map[string]int{"0": 0, "1": 1}[status]; code != want {
				t.Fatalf("exit %d, want %d", code, want)
			}
			if exists(cache) || !exists(filepath.Dir(cache)) {
				t.Fatalf("cache there %v, web/.next there %v; want only the cache gone", exists(cache), exists(filepath.Dir(cache)))
			}
			if stderr != "wk run: trimmed web/.next/cache\n" {
				t.Fatalf("stderr %q, want the one trim line", stderr)
			}
		})
	}
}

func TestRunKeepsTheCache(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		apply func(*spec)
	}{
		{"--keep", func(s *spec) { s.Keep = true }},
		{"main checkout, template or pool slot", func(s *spec) { s.TrimAllowed = false }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, s, cache := trimFixture(t)
			tc.apply(&s)
			if code, stderr := runHere(t, s, "true"); code != 0 || stderr != "" || !exists(cache) {
				t.Fatalf("exit %d, stderr %q, cache there %v; want 0, nothing, kept", code, stderr, exists(cache))
			}
		})
	}
}

func TestRunRefusesABadTrimPathAndRunsNothing(t *testing.T) {
	t.Parallel()
	for _, bad := range []string{"../elsewhere", "/abs/cache"} {
		t.Run(bad, func(t *testing.T) {
			t.Parallel()
			f, s, cache := trimFixture(t)
			s.Trim = []string{bad}
			ran := f.path("ran")
			code, stderr := runHere(t, s, "touch", ran)
			if code != exitUsage || exists(ran) || !exists(cache) || !strings.HasPrefix(stderr, "wk run: --trim ") {
				t.Fatalf("exit %d, ran %v, stderr %q; want exit 2 before anything runs", code, exists(ran), stderr)
			}
		})
	}
}

func TestRemoveInsideStopsAtASymlinkOut(t *testing.T) {
	t.Parallel()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	top, outside := filepath.Join(root, "ws"), filepath.Join(root, "template")
	writeFile(t, filepath.Join(outside, ".next", "cache", "entry"), "shared\n")
	writeFile(t, filepath.Join(top, "web", "page.ts"), "page\n")
	if err := os.Symlink(filepath.Join(outside, ".next"), filepath.Join(top, "web", ".next")); err != nil {
		t.Fatal(err)
	}
	if removed, err := removeInside(top, "web/.next/cache"); err == nil || removed {
		t.Fatalf("removeInside through a symlink out: removed %v, %v", removed, err)
	}
	if !exists(filepath.Join(outside, ".next", "cache", "entry")) {
		t.Fatal("the trim deleted a cache outside the workspace")
	}
	if removed, err := removeInside(top, "web/missing"); err != nil || removed {
		t.Fatalf("removeInside of a missing path: removed %v, %v", removed, err)
	}
}
