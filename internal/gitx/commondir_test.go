package gitx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

func TestMain(m *testing.M) {
	testrepo.Isolate()
	os.Exit(m.Run())
}

// CommonDir must give git's own answer wherever it gives one, because the hook names the slots
// file by it; git is the oracle here.
func TestCommonDirMatchesGit(t *testing.T) {
	f := testrepo.New(t)
	wt := filepath.Join(f.Root, "wt")
	f.Git(t, "worktree", "add", "--quiet", "-b", "wk/1", wt)
	rel := filepath.Join(f.Root, "wt-rel")
	f.Git(t, "worktree", "add", "--quiet", "--relative-paths", "-b", "wk/2", rel)
	link := filepath.Join(f.Root, "link")
	if err := os.Symlink(filepath.Join(wt, "web"), link); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{
		f.Repo,
		filepath.Join(f.Repo, "web"),
		filepath.Join(f.Repo, "packages", "core"),
		wt,
		filepath.Join(wt, "web"),
		filepath.Join(rel, "web"),
		link,
	} {
		t.Run(dir, func(t *testing.T) {
			want, err := Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
			if err != nil {
				t.Fatal(err)
			}
			got, ok := CommonDir(dir)
			if !ok || got != want {
				t.Errorf("CommonDir = %q, %v; git says %q", got, ok, want)
			}
		})
	}
}

// Where the files leave the answer open, CommonDir says so and the caller asks git.
func TestCommonDirLeavesTheOpenCasesToGit(t *testing.T) {
	f := testrepo.New(t)
	outside := t.TempDir()
	bare := filepath.Join(t.TempDir(), "bare.git")
	testrepo.Git(t, f.Root, "init", "--quiet", "--bare", bare)
	broken := t.TempDir()
	if err := os.WriteFile(filepath.Join(broken, ".git"), []byte("not a gitdir line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, dir, env string }{
		{"no repository above", outside, ""},
		{"a bare repository", bare, ""},
		{"a .git file it cannot read", broken, ""},
		{"a relative path", "demo", ""},
		{"GIT_DIR set", f.Repo, "GIT_DIR"},
		{"GIT_CEILING_DIRECTORIES set", f.Repo, "GIT_CEILING_DIRECTORIES"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.env != "" {
				t.Setenv(tc.env, filepath.Join(f.Repo, ".git"))
			}
			if got, ok := CommonDir(tc.dir); ok {
				t.Errorf("CommonDir(%q) = %q, true; want it left to git", tc.dir, got)
			}
		})
	}
}

// Row 22 audit B1: git checks at each level whether the directory is itself a git directory, and
// resolves a relative commondir on disk. A git directory found that way goes to git; a commondir
// reached through a symbolic link must give git's answer or none. Each case is checked against
// git: where git answers, CommonDir gives the same answer or leaves it to git; where git fails,
// CommonDir must not answer.
func TestCommonDirAgreesWithGitOnGitDirectoriesAndLinks(t *testing.T) {
	f := testrepo.New(t)
	inner := filepath.Join(f.Repo, "inner.git")
	f.Git(t, "init", "--quiet", "--bare", inner)
	other := filepath.Join(f.Root, "other")
	testrepo.Git(t, f.Root, "init", "--quiet", "--initial-branch=main", other)
	testrepo.Git(t, other, "commit", "--quiet", "--allow-empty", "-m", "init")
	f.Git(t, "-c", "protocol.file.allow=always", "submodule", "--quiet", "add", other, "sub")

	linked := testrepo.New(t)
	wt := filepath.Join(linked.Root, "wt")
	linked.Git(t, "worktree", "add", "--quiet", "-b", "wk/1", wt)
	moved := filepath.Join(linked.Root, "elsewhere", "worktrees")
	if err := os.MkdirAll(filepath.Dir(moved), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(filepath.Join(linked.Repo, ".git", "worktrees"), moved); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(moved, filepath.Join(linked.Repo, ".git", "worktrees")); err != nil {
		t.Fatal(err)
	}

	for _, dir := range []string{
		inner,
		filepath.Join(inner, "objects"),
		filepath.Join(f.Repo, ".git", "modules", "sub"),
		filepath.Join(f.Repo, ".git", "refs"),
		filepath.Join(f.Repo, "sub"),
		wt,
	} {
		t.Run(dir, func(t *testing.T) {
			want, err := Run(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
			got, ok := CommonDir(dir)
			switch {
			case err != nil && ok:
				t.Errorf("CommonDir = %q, true; git fails: %v", got, err)
			case err == nil && ok && got != want:
				t.Errorf("CommonDir = %q; git says %q", got, want)
			}
		})
	}
}
