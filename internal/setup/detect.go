// Package setup is what a bare `wk` opens: a guided first setup that writes the repository's
// config and warms its template, or, once the config exists, a home screen over it.
package setup

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/zach-short/wk/internal/gitx"
)

// Option is one choice the wizard offers, on or off.
type Option struct {
	Value string // what goes in the config
	Note  string // why wk offers it
	On    bool
}

// Install is one install command the template runs after checkout, and the lockfile whose
// change means the template must be rebuilt.
type Install struct {
	Dir      string
	Run      []string
	Lockfile string
}

// Found is what wk read from the main checkout. Reading it changes nothing.
type Found struct {
	Branches  []string // local branches, the current one first
	CloneDirs []Option
	CopyFiles []Option
	Installs  []Option // Value is the command line; installs[i] holds its parts
	installs  []Install
}

// maxDepth bounds the walk: manifests and .env files of a monorepo sit at most two folders down.
const maxDepth = 2

// skipDirs are never walked into: git's own data, other worktrees, and dependency trees, whose
// own manifests are not the repository's.
var skipDirs = map[string]bool{
	".git": true, ".claude": true, "node_modules": true, ".venv": true, "venv": true,
	"target": true, "Pods": true, "vendor": true, "dist": true, "build": true, ".next": true,
}

// manifests name the dependency folder that sits next to each manifest. Cloning that folder from
// the template is what makes a new workspace warm.
var manifests = []struct{ file, dir, note string }{
	{"package.json", "node_modules", "JavaScript packages"},
	{"pyproject.toml", ".venv", "Python virtualenv"},
	{"requirements.txt", ".venv", "Python virtualenv"},
	{"Cargo.toml", "target", "Rust build output"},
	{"Podfile", "Pods", "CocoaPods"},
}

// lockfiles map a root lockfile to the install that reads it, in order of preference.
var lockfiles = []struct {
	file string
	run  []string
}{
	{"bun.lock", []string{"bun", "install", "--frozen-lockfile"}},
	{"bun.lockb", []string{"bun", "install", "--frozen-lockfile"}},
	{"pnpm-lock.yaml", []string{"pnpm", "install", "--frozen-lockfile"}},
	{"yarn.lock", []string{"yarn", "install", "--frozen-lockfile"}},
	{"package-lock.json", []string{"npm", "ci"}},
	{"uv.lock", []string{"uv", "sync", "--frozen"}},
	{"poetry.lock", []string{"poetry", "install", "--no-root"}},
	{"Cargo.lock", []string{"cargo", "fetch", "--locked"}},
	{"go.sum", []string{"go", "mod", "download"}},
}

// Detect reads the main checkout for the choices the wizard offers. It looks at names only:
// a .env file is listed, never opened.
func Detect(primary string) Found {
	var f Found
	f.Branches = branches(primary)

	seenDir := map[string]bool{}
	_ = filepath.WalkDir(primary, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, _ := filepath.Rel(primary, path)
		rel = filepath.ToSlash(rel)
		depth := strings.Count(rel, "/")
		if d.IsDir() {
			if rel != "." && (skipDirs[d.Name()] || depth >= maxDepth) {
				return filepath.SkipDir
			}
			return nil
		}
		dir := filepath.ToSlash(filepath.Dir(rel))
		for _, m := range manifests {
			if d.Name() != m.file {
				continue
			}
			clone := join(dir, m.dir)
			if !seenDir[clone] {
				seenDir[clone] = true
				f.CloneDirs = append(f.CloneDirs, Option{Value: clone, Note: m.note, On: true})
			}
		}
		if isEnvFile(d.Name()) {
			o := Option{Value: rel, Note: "not in git, so a worktree lacks it", On: true}
			if tracked(primary, rel) {
				o = Option{Value: rel, Note: "tracked by git, so a worktree has it"}
			}
			f.CopyFiles = append(f.CopyFiles, o)
		}
		return nil
	})
	if _, err := os.Stat(filepath.Join(primary, ".claude", "settings.local.json")); err == nil {
		f.CopyFiles = append(f.CopyFiles, Option{Value: ".claude/settings.local.json", Note: "local Claude settings", On: true})
	}
	sort.SliceStable(f.CloneDirs, func(i, j int) bool { return f.CloneDirs[i].Value < f.CloneDirs[j].Value })
	sort.SliceStable(f.CopyFiles, func(i, j int) bool { return f.CopyFiles[i].Value < f.CopyFiles[j].Value })

	taken := map[string]bool{}
	for _, l := range lockfiles {
		if _, err := os.Stat(filepath.Join(primary, l.file)); err != nil || taken[l.run[0]] {
			continue
		}
		taken[l.run[0]] = true
		f.installs = append(f.installs, Install{Dir: ".", Run: l.run, Lockfile: l.file})
		f.Installs = append(f.Installs, Option{Value: strings.Join(l.run, " "), Note: "from " + l.file, On: len(f.installs) == 1})
	}
	return f
}

func join(dir, name string) string {
	if dir == "." {
		return name
	}
	return dir + "/" + name
}

// isEnvFile matches .env and .env.<anything>, but not the example files a repository commits.
func isEnvFile(name string) bool {
	if name != ".env" && !strings.HasPrefix(name, ".env.") {
		return false
	}
	for _, s := range []string{".example", ".sample", ".template", ".dist"} {
		if strings.HasSuffix(name, s) {
			return false
		}
	}
	return true
}

func tracked(primary, rel string) bool {
	_, err := gitx.Run(primary, "ls-files", "--error-unmatch", "--", rel)
	return err == nil
}

// branches lists the local branches with the checked-out one first, since that is the base the
// main checkout follows.
func branches(primary string) []string {
	current, _ := gitx.Run(primary, "symbolic-ref", "--quiet", "--short", "HEAD")
	out, _ := gitx.Run(primary, "for-each-ref", "--format=%(refname:short)", "refs/heads")
	var list []string
	if current != "" {
		list = append(list, current)
	}
	for b := range strings.SplitSeq(out, "\n") {
		if b != "" && b != current && !strings.HasPrefix(b, "wk/") && !strings.HasPrefix(b, "worktree-") {
			list = append(list, b)
		}
	}
	if len(list) == 0 {
		list = []string{"main"}
	}
	return list
}
