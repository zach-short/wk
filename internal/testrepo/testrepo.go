// Package testrepo builds throwaway repositories shaped like a bun monorepo for the self-test,
// in the style of the land-worktrees selftest.sh.
package testrepo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Isolate points git at no user or system config and a fixed identity, so a test commit in a
// throwaway repository never reads the owner's hooks, signing or aliases. Call it from TestMain.
func Isolate() {
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL":   os.DevNull,
		"GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME":     "wk selftest",
		"GIT_AUTHOR_EMAIL":    "selftest@wk.invalid",
		"GIT_COMMITTER_NAME":  "wk selftest",
		"GIT_COMMITTER_EMAIL": "selftest@wk.invalid",
	} {
		_ = os.Setenv(k, v)
	}
}

type Fixture struct {
	Root string
	Repo string
	Home string
}

// The fake install does what `bun install` does to the tree that matters here: a workspace
// package linked by a relative symlink, a nested node_modules, and a record of which lockfile
// it installed from.
const fakeInstall = `mkdir -p node_modules/@ezh web/node_modules/react &&
ln -sfn ../../packages/core node_modules/@ezh/core &&
cp bun.lock node_modules/.installed-from &&
echo react > web/node_modules/react/index.js`

const fakeBuild = `mkdir -p .next/cache && echo "$WK_TEST_GATE" > .next/cache/built-with`

const configTOML = `[template]
path = ".claude/worktrees/wk-template"
refresh_on = "bun.lock"
clone_dirs = ["node_modules", "web/node_modules", "web/.next/cache"]
copy_files = [".env", "web/.env", "missing/.env"]

[[template.post_clone]]
dir = "."
run = ["sh", "-c", '''` + fakeInstall + `''']

[[template.build]]
dir = "web"
run = ["sh", "-c", '''` + fakeBuild + `''']

[gates.env]
WK_TEST_GATE = "placeholder"

[workspace]
sparse_exclude = ["docs/incomplete"]
`

// New builds the repository under a fresh temporary directory, commits a tracked tree on main,
// leaves ignored secrets in the main checkout and writes the wk config for it.
func New(t *testing.T) Fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := Fixture{Root: root, Repo: filepath.Join(root, "demo"), Home: filepath.Join(root, "home")}
	f.Write(t, "README.md", "demo\n")
	f.Write(t, ".gitignore", "node_modules/\n.env\n.next/\n.claude/worktrees/\n")
	f.Write(t, "bun.lock", "lock v1\n")
	f.Write(t, "packages/core/index.ts", "export const core = 1;\n")
	f.Write(t, "web/page.ts", "export const page = 1;\n")
	f.Write(t, "docs/incomplete/plan/PLAN.md", "# plan\n")
	f.Git(t, "init", "--quiet", "--initial-branch=main")
	f.Commit(t, "init")
	f.Write(t, ".env", "SECRET=main-checkout\n")
	f.Write(t, "web/.env", "WEB_SECRET=main-checkout\n")
	if err := os.MkdirAll(f.Home, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(f.Home, "demo.toml"), []byte(configTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	return f
}

func (f Fixture) Write(t *testing.T, rel, content string) {
	t.Helper()
	p := filepath.Join(f.Repo, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func (f Fixture) Git(t *testing.T, args ...string) string {
	t.Helper()
	return Git(t, f.Repo, args...)
}

func (f Fixture) Commit(t *testing.T, message string) {
	t.Helper()
	f.Git(t, "add", "--all")
	f.Git(t, "commit", "--quiet", "--no-verify", "-m", message)
}

func Git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimRight(string(out), "\n")
}
