// Package wk makes warm workspaces: git worktrees whose dependency trees are copy-on-write
// clones of a template's.
package wk

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/gitx"
)

type Env struct {
	Repo  gitx.Repo
	Cfg   config.Config
	State string
	// Refill starts a pool fill that outlives this process. Open leaves it nil, so a library
	// caller or a test never forks; the command line sets it to a detached `wk pool fill`.
	Refill func()
	log    *syncWriter
	clock  func() time.Time
}

// Open resolves the repository holding dir and reads its config from home/<repo>.toml. All
// progress goes to log; stdout belongs to the caller, which prints one path there.
func Open(home, dir string, log io.Writer) (*Env, error) {
	if !filepath.IsAbs(home) {
		return nil, fmt.Errorf("%q is relative; wk takes absolute paths only", home)
	}
	repo, err := gitx.Resolve(dir)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, repo.Name+".toml")
	cfg, found, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	e := &Env{Repo: repo, Cfg: cfg, State: filepath.Join(home, repo.Name), log: &syncWriter{w: log}, clock: time.Now}
	if !found {
		e.say("no %s; using the defaults", path)
	}
	if err := os.MkdirAll(e.State, 0o755); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Env) say(format string, args ...any) {
	_, _ = fmt.Fprintf(e.log, "wk: "+format+"\n", args...)
}

func (e *Env) timed(step string, f func() error) error {
	start := time.Now()
	err := f()
	if err == nil {
		e.say("%-32s %6.2fs", step, time.Since(start).Seconds())
	}
	return err
}

func (e *Env) inPrimary(rel string) string {
	return filepath.Join(e.Repo.Primary, filepath.FromSlash(rel))
}

// copyFiles copies each listed file from the main checkout into root. The files are secrets
// (.env) and generated declarations; they are read and written inside this process and never
// printed, because a session that reads a .env file is refused by the auto-mode classifier.
func (e *Env) copyFiles(root string) error {
	for _, rel := range e.Cfg.Template.CopyFiles {
		src := e.inPrimary(rel)
		info, err := os.Stat(src)
		if os.IsNotExist(err) {
			e.say("skip copy of %s: the main checkout has none", rel)
			continue
		}
		if err != nil {
			return err
		}
		dst := filepath.Join(root, filepath.FromSlash(rel))
		if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
			return fmt.Errorf("copy %s: %w", rel, err)
		}
	}
	return nil
}

func copyFile(src, dst string, perm os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		return err
	}
	return out.Close()
}

func (e *Env) runCommands(root string, cmds []config.Command, extraEnv map[string]string) error {
	env := os.Environ()
	keys := make([]string, 0, len(extraEnv))
	for k := range extraEnv {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		env = append(env, k+"="+extraEnv[k])
	}
	for _, c := range cmds {
		cmd := exec.Command(c.Run[0], c.Run[1:]...)
		cmd.Dir = filepath.Join(root, filepath.FromSlash(c.Dir))
		cmd.Env = env
		cmd.Stdout = e.log
		cmd.Stderr = e.log
		step := fmt.Sprintf("%v in %s", c.Run, displayDir(c.Dir))
		if err := e.timed(step, cmd.Run); err != nil {
			return fmt.Errorf("%s: %w", step, err)
		}
	}
	return nil
}

func displayDir(rel string) string {
	if rel == "" {
		return "."
	}
	return rel
}

// syncWriter serialises writes: clones report from several goroutines at once.
type syncWriter struct {
	mu sync.Mutex
	w  io.Writer
}

func (s *syncWriter) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.w.Write(p)
}
