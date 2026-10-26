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
