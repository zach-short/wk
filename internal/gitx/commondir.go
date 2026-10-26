package gitx

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// CommonDir finds what `git rev-parse --path-format=absolute --git-common-dir` prints for dir by
// reading the files git reads, with no child process. wk's PreToolUse hook runs before every
// Bash call, and on a machine full of slotted suites its one git run took it past its 200 ms
// deadline (ezhomesteading errors.log, 42 lines from 2026-10-17 to 2026-10-26), so the gate it
// stood in front of ran unslotted.
//
// It walks up from the real path of dir to the first .git: a directory is the git directory, and
// a file names one on its "gitdir:" line. A git directory with a commondir file (a linked
// worktree's) points at the common one. ok is false wherever the files leave the answer open: a
// relative dir, a GIT_ variable that moves or bounds the repository, a bare repository, no .git
// up to the root, or a .git that does not read as git writes it. The caller then asks git.
// Unlike git, the walk does not stop at a file system boundary; no repository wk serves sits
// above a mount point.
func CommonDir(dir string) (string, bool) {
	if !filepath.IsAbs(dir) || gitEnvMovesRepo() {
		return "", false
	}
	d, err := filepath.EvalSymlinks(dir)
	if err != nil {
		return "", false
	}
	for {
		if common, found, ok := commonDirAt(d); found {
			return common, ok
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", false
		}
		d = parent
	}
}

func gitEnvMovesRepo() bool {
	for _, name := range []string{"GIT_DIR", "GIT_COMMON_DIR", "GIT_WORK_TREE", "GIT_CEILING_DIRECTORIES", "GIT_DISCOVERY_ACROSS_FILESYSTEM"} {
		if os.Getenv(name) != "" {
			return true
		}
	}
	return false
}

// commonDirAt reads d/.git. found is false only when there is no .git in d, so the walk goes on.
func commonDirAt(d string) (common string, found, ok bool) {
	dotgit := filepath.Join(d, ".git")
	info, err := os.Stat(dotgit)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, false
	}
	if err != nil {
		return "", true, false
	}
	gitdir := dotgit
	if !info.IsDir() {
		if gitdir, ok = readGitFile(dotgit); !ok {
			return "", true, false
		}
	}
	common, ok = commonOf(gitdir)
	return common, true, ok
}

// readGitFile reads the one "gitdir: <path>" line git writes in a worktree's .git file; a
// relative path is relative to the file's directory.
func readGitFile(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	target, ok := strings.CutPrefix(strings.TrimRight(string(data), "\r\n"), "gitdir: ")
	if !ok || target == "" || strings.ContainsAny(target, "\r\n") {
		return "", false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(filepath.Dir(path), target)
	}
	return target, true
}

// commonOf is the common directory of a git directory: the one its commondir file names, or
// itself. git prints it with symbolic links resolved.
func commonOf(gitdir string) (string, bool) {
	if _, err := os.Stat(filepath.Join(gitdir, "HEAD")); err != nil {
		return "", false
	}
	common := gitdir
	data, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	switch {
	case err == nil:
		common = strings.TrimRight(string(data), "\r\n")
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitdir, common)
		}
	case !errors.Is(err, fs.ErrNotExist):
		return "", false
	}
	real, err := filepath.EvalSymlinks(common)
	return real, err == nil
}
