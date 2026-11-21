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
	f.Write(t, "docs/audit/finding-x.md", "a finding\n")
	f.Write(t, "docs/audit/run-plain.sh", "#!/bin/sh\n")
	f.Write(t, "docs/audit/run-group-x.sh", "#!/bin/sh\n")
	f.Write(t, "docs/audit/finding-other-x.md", "a finding\n")
	if err := os.Chmod(filepath.Join(f.Repo, "docs/audit/run-plain.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(f.Repo, "docs/audit/run.sh"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(f.Repo, "docs/audit/run-group-x.sh"), 0o755); err != nil {
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
	if err := os.Chmod(p, mode); err != nil {
		t.Fatal(err)
	}
}

func TestHeldBlobHoldsOnlyAByteEqualRegularFile(t *testing.T) {
	f, wt, base := heldFixture(t)
	put(t, wt, "docs/audit/finding.md", "a finding\n", 0o644)
	put(t, wt, "docs/audit/run.sh", "#!/bin/sh\n", 0o755)
	put(t, wt, "docs/audit/finding-x.md", "a finding\n", 0o755)
	put(t, wt, "docs/audit/run-plain.sh", "#!/bin/sh\n", 0o644)
	put(t, wt, "docs/audit/run-group-x.sh", "#!/bin/sh\n", 0o654)
	put(t, wt, "docs/audit/finding-other-x.md", "a finding\n", 0o645)
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
		{"executable copy of a 100644 file", base, "docs/audit/finding-x.md", false},
		{"plain copy of a 100755 file", base, "docs/audit/run-plain.sh", false},
		{"group and other x bits only, against 100755", base, "docs/audit/run-group-x.sh", false},
		{"other x bit only, against 100644", base, "docs/audit/finding-other-x.md", true},
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

// A tree can name a blob the store lacks: a corrupt store, or a partial clone whose blob is only
// at a remote. The bytes on disk would then be the only copy, so the file is not held.
func TestHeldBlobNeedsTheBlobInTheStore(t *testing.T) {
	f, wt, base := heldFixture(t)
	put(t, wt, "docs/audit/finding.md", "a finding\n", 0o644)
	blob := testrepo.Git(t, f.Repo, "rev-parse", base+":docs/audit/finding.md")
	if held, err := HeldBlob(wt, base, "docs/audit/finding.md"); err != nil || held != blob {
		t.Fatalf("with the blob in the store: HeldBlob = %q, %v; want %q", held, err, blob)
	}
	common := testrepo.Git(t, f.Repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	loose := filepath.Join(common, "objects", blob[:2], blob[2:])
	if err := os.Remove(loose); err != nil {
		t.Fatalf("the probe needs the blob as a loose object: %v", err)
	}
	if out, err := Run(wt, "cat-file", "-e", blob); err == nil {
		t.Fatalf("the blob is still in the store after the delete: %q", out)
	}
	if held, err := HeldBlob(wt, base, "docs/audit/finding.md"); err != nil || held != "" {
		t.Fatalf("with the blob gone from the store: HeldBlob = %q, %v; want it not held", held, err)
	}
}

func TestHeldBlobHoldsAPackedBlob(t *testing.T) {
	f, wt, base := heldFixture(t)
	put(t, wt, "docs/audit/finding.md", "a finding\n", 0o644)
	testrepo.Git(t, f.Repo, "repack", "-a", "-d", "-q")
	testrepo.Git(t, f.Repo, "prune-packed")
	blob := testrepo.Git(t, f.Repo, "rev-parse", base+":docs/audit/finding.md")
	common := testrepo.Git(t, f.Repo, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if _, err := os.Stat(filepath.Join(common, "objects", blob[:2], blob[2:])); !os.IsNotExist(err) {
		t.Fatalf("the probe needs the blob only in a pack: %v", err)
	}
	if held, err := HeldBlob(wt, base, "docs/audit/finding.md"); err != nil || held != blob {
		t.Fatalf("HeldBlob = %q, %v; want the packed blob %q", held, err, blob)
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

func TestOnlyUntrackedSeesAChangeThatIsOnlyStaged(t *testing.T) {
	f := testrepo.New(t)
	put(t, f.Repo, "notes/draft.md", "a draft\n", 0o644)
	if only, err := OnlyUntracked(f.Repo); err != nil || !only {
		t.Fatalf("only untracked files: OnlyUntracked = %v, %v; want true", only, err)
	}
	put(t, f.Repo, "README.md", "staged\n", 0o644)
	testrepo.Git(t, f.Repo, "add", "README.md")
	put(t, f.Repo, "README.md", "demo\n", 0o644)
	if out := testrepo.Git(t, f.Repo, "diff", "--name-only", "HEAD"); out != "" {
		t.Fatalf("the probe needs git diff HEAD to see nothing, got %q", out)
	}
	if only, err := OnlyUntracked(f.Repo); err != nil || only {
		t.Fatalf("a staged edit undone on disk: OnlyUntracked = %v, %v; want false", only, err)
	}
}
