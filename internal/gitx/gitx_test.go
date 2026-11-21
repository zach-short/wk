package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

// heldFixture commits the files HeldBlob compares against, then leaves the working tree at the
// commit before them, as an /audit agent's worktree is: made before the lead committed its
// finding files on main.
func heldFixture(t *testing.T) (testrepo.Fixture, string, string) {
	t.Helper()
	f := testrepo.New(t)
	old := f.Git(t, "rev-parse", "HEAD")
	f.Write(t, "docs/audit/finding.md", "a finding\n")
	f.Write(t, "docs/audit/run.sh", "#!/bin/sh\n")
	if err := os.Chmod(filepath.Join(f.Repo, "docs/audit/run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("finding.md", filepath.Join(f.Repo, "docs/audit/link")); err != nil {
		t.Fatal(err)
	}
	f.Commit(t, "the lead commits the findings")
	base := f.Git(t, "rev-parse", "HEAD")
	wt := filepath.Join(f.Root, "agent")
	f.Git(t, "worktree", "add", "--quiet", "--detach", wt, old)
	return f, wt, base
}

func put(t *testing.T, dir, rel, content string, mode os.FileMode) {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), mode); err != nil {
		t.Fatal(err)
	}
}

func TestHeldBlobHoldsOnlyAByteEqualRegularFile(t *testing.T) {
	f, wt, base := heldFixture(t)
	put(t, wt, "docs/audit/finding.md", "a finding\n", 0o644)
	put(t, wt, "docs/audit/run.sh", "#!/bin/sh\n", 0o755)
	put(t, wt, "docs/audit/differs.md", "x\n", 0o644)
	put(t, wt, "docs/audit/new.md", "not on the base\n", 0o644)
	put(t, wt, "docs/audit/link", "finding.md", 0o644)
	outside := filepath.Join(f.Root, "equal-copy")
	put(t, f.Root, "equal-copy", "a finding\n", 0o644)

	for _, tc := range []struct {
		name, rev, rel string
		held           bool
	}{
		{"equal file", base, "docs/audit/finding.md", true},
		{"equal executable", base, "docs/audit/run.sh", true},
		{"different bytes", base, "docs/audit/differs.md", false},
		{"path the base lacks", base, "docs/audit/new.md", false},
		{"regular file over the base's symbolic link", base, "docs/audit/link", false},
		{"directory", base, "docs/audit", false},
		{"rev before the file", "HEAD", "docs/audit/finding.md", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			blob, err := HeldBlob(wt, tc.rev, tc.rel)
			if err != nil {
				t.Fatal(err)
			}
			want := ""
			if tc.held {
				want = testrepo.Git(t, f.Repo, "rev-parse", tc.rev+":"+tc.rel)
			}
			if blob != want {
				t.Fatalf("HeldBlob = %q, want %q", blob, want)
			}
		})
	}

	if err := os.Remove(filepath.Join(wt, "docs/audit/finding.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(wt, "docs/audit/finding.md")); err != nil {
		t.Fatal(err)
	}
	if blob, err := HeldBlob(wt, base, "docs/audit/finding.md"); err != nil || blob != "" {
		t.Fatalf("a symbolic link to an equal file: HeldBlob = %q, %v; want it not held", blob, err)
	}
}

func TestHeldBlobReadsAGlobCharacterLiterally(t *testing.T) {
	f := testrepo.New(t)
	old := f.Git(t, "rev-parse", "HEAD")
	f.Write(t, "docs/a1.md", "one\n")
	f.Commit(t, "a1")
	base := f.Git(t, "rev-parse", "HEAD")
	wt := filepath.Join(f.Root, "agent")
	f.Git(t, "worktree", "add", "--quiet", "--detach", wt, old)
	put(t, wt, "docs/a?.md", "one\n", 0o644)
	if blob, err := HeldBlob(wt, base, "docs/a?.md"); err != nil || blob != "" {
		t.Fatalf("HeldBlob = %q, %v; a glob in the name must not match docs/a1.md", blob, err)
	}
}

func TestUntrackedIsTheUntrackedHalfOfDirtyPaths(t *testing.T) {
	f := testrepo.New(t)
	put(t, f.Repo, "README.md", "edited\n", 0o644)
	put(t, f.Repo, "notes/draft.md", "a draft\n", 0o644)
	put(t, f.Repo, "node_modules/x.js", "ignored\n", 0o644)
	got, err := Untracked(f.Repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "notes/draft.md" {
		t.Fatalf("Untracked = %q, want [notes/draft.md]", got)
	}
}

func TestTrackedIsTheTrackedHalfOfDirtyPaths(t *testing.T) {
	f := testrepo.New(t)
	put(t, f.Repo, "web/page.ts", "edited\n", 0o644)
	put(t, f.Repo, "notes/draft.md", "a draft\n", 0o644)
	testrepo.Git(t, f.Repo, "rm", "--cached", "--quiet", "README.md")
	got, err := Tracked(f.Repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "README.md" || got[1] != "web/page.ts" {
		t.Fatalf("Tracked = %q, want [README.md web/page.ts]", got)
	}
}
