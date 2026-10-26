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
// It walks up from the real path of dir, checking each level as git does: first d/.git, where a
// directory is the git directory and a file names one on its "gitdir:" line, then d itself. A
// git directory with a commondir file (a linked worktree's) points at the common one; a relative
// commondir or gitdir path is resolved on disk, through any symbolic link, as git resolves it,
// the git directory must hold a valid HEAD file, and the common directory objects and refs
// directories. ok is false wherever the files leave the answer open: a relative dir, a GIT_
// variable that moves or bounds the repository, a level that may itself be a git directory (a
// bare repository, a submodule's git directory, the inside of .git), no .git up to the root, or
// a .git, HEAD or commondir that fails those checks, where git would skip to an outer
// repository or fail. The caller then asks git. Unlike git, the walk does not stop at a file system
// boundary; no repository wk serves sits above a mount point.
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
		if isGitDir(d) {
			return "", false
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
	return onDisk(filepath.Dir(path), target)
}

// commonOf is the common directory of a git directory: the one its commondir file names, or
// itself. git prints it with symbolic links resolved.
func commonOf(gitdir string) (string, bool) {
	gitdir, ok := onDisk("", gitdir)
	if !ok || !validHead(filepath.Join(gitdir, "HEAD")) {
		return "", false
	}
	common := gitdir
	data, err := os.ReadFile(filepath.Join(gitdir, "commondir"))
	switch {
	case err == nil:
		if common, ok = onDisk(gitdir, strings.TrimRight(string(data), "\r\n")); !ok {
			return "", false
		}
	case !errors.Is(err, fs.ErrNotExist):
		return "", false
	}
	return common, isDir(filepath.Join(common, "objects")) && isDir(filepath.Join(common, "refs"))
}

// onDisk resolves p, relative to base when it is relative, the way git's real-path does: each
// component in turn, so a ".." after a symbolic link leaves the link's target, not the link.
// filepath.Join would clean ".." away as text first and could name a directory git never reads.
func onDisk(base, p string) (string, bool) {
	if p == "" {
		return "", false
	}
	if !filepath.IsAbs(p) {
		p = base + string(filepath.Separator) + p
	}
	real, err := filepath.EvalSymlinks(p)
	return real, err == nil
}

// isGitDir says whether d may be a git directory: HEAD, objects and refs exist. It is looser
// than git's own test on purpose. CommonDir uses it only to stop and leave the answer to git, so
// a level that merely looks like a git directory costs one git run, never a wrong answer.
func isGitDir(d string) bool {
	return exists(filepath.Join(d, "HEAD")) && exists(filepath.Join(d, "objects")) && exists(filepath.Join(d, "refs"))
}

// validHead is the part of git's test of a git directory that reads HEAD: a regular file
// holding "ref: refs/..." or an object name. Git skips a .git whose HEAD fails it and walks on
// to an outer repository (row 22 second audit B); CommonDir does not copy that skip, it leaves
// such a repository to git. A symbolic-link HEAD, which git also reads, goes to git as well.
func validHead(p string) bool {
	info, err := os.Lstat(p)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return false
	}
	head := strings.TrimRight(string(data), "\r\n")
	if ref, ok := strings.CutPrefix(head, "ref: "); ok {
		return strings.HasPrefix(ref, "refs/")
	}
	return isObjectName(head)
}

func isObjectName(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	return strings.Trim(s, "0123456789abcdef") == ""
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

func isDir(p string) bool {
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}
