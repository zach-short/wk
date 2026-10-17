// Package gate runs a repository's configured gate commands over a working
// tree, reads the file paths a failing gate prints, and waits for free memory
// before a gate starts. It decides nothing about batches or attribution.
package gate

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// The three values of Result.Status. A skipped gate never counts as passed.
const (
	Pass = "pass"
	Fail = "fail"
	Skip = "skip"
)

// tailLines is how much of each gate's output reaches the log (PD-13).
const tailLines = 20

// Gate is one configured command: Run is the argument vector, Dir is relative
// to the root, and NeedsEnv names process variables without which it is skipped.
type Gate struct {
	Name, Dir     string
	Run, NeedsEnv []string
}

// Result is one gate's outcome. Output holds stdout and stderr together.
type Result struct {
	Name, Status, Reason string
	Output               []byte
	Seconds              float64
}

// Run runs gates in order in root and stops after the first fail. env is added
// to the process environment; each variable in caches points at a fresh empty
// directory for the whole run, removed afterwards. The error reports a gate the
// runner could not start, which says nothing about the tree.
func Run(root string, gates []Gate, env map[string]string, caches []string, log io.Writer) ([]Result, error) {
	cacheEnv, err := freshCaches(caches)
	defer removeCaches(cacheEnv, log)
	if err != nil {
		return nil, err
	}
	base := environ(env, cacheEnv)
	var results []Result
	for _, g := range gates {
		r, err := runOne(root, g, base)
		if err != nil {
			return results, err
		}
		echo(log, r)
		results = append(results, r)
		if r.Status == Fail {
			break
		}
	}
	return results, nil
}

func runOne(root string, g Gate, env []string) (Result, error) {
	for _, v := range g.NeedsEnv {
		if _, ok := os.LookupEnv(v); !ok {
			return Result{Name: g.Name, Status: Skip, Reason: v + " is unset"}, nil
		}
	}
	if len(g.Run) == 0 {
		return Result{}, fmt.Errorf("gate %s has no command", g.Name)
	}
	var out bytes.Buffer
	cmd := exec.Command(g.Run[0], g.Run[1:]...)
	cmd.Dir = filepath.Join(root, filepath.FromSlash(g.Dir))
	cmd.Env = env
	cmd.Stdout = &out
	cmd.Stderr = &out
	start := time.Now()
	err := cmd.Run()
	r := Result{Name: g.Name, Status: Pass, Output: out.Bytes(), Seconds: time.Since(start).Seconds()}
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		r.Status, r.Reason = Fail, exit.Error()
	case err != nil:
		return Result{}, fmt.Errorf("gate %s: %w", g.Name, err)
	}
	return r, nil
}

// freshCaches makes one directory per variable. On error it still returns the
// pairs made so far, so the caller can remove them.
func freshCaches(vars []string) ([][2]string, error) {
	var made [][2]string
	for _, v := range vars {
		dir, err := os.MkdirTemp("", "wk-gate-"+v+"-")
		if err != nil {
			return made, fmt.Errorf("fresh cache for %s: %w", v, err)
		}
		made = append(made, [2]string{v, dir})
	}
	return made, nil
}

func removeCaches(caches [][2]string, log io.Writer) {
	for _, c := range caches {
		if err := os.RemoveAll(c[1]); err != nil {
			_, _ = fmt.Fprintf(log, "gate: could not remove the %s cache: %v\n", c[0], err)
		}
	}
}

// environ appends env in sorted order, then the caches, so a cache variable
// wins over the same name in env: exec keeps the last value of a duplicate.
func environ(env map[string]string, caches [][2]string) []string {
	out := os.Environ()
	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out = append(out, k+"="+env[k])
	}
	for _, c := range caches {
		out = append(out, c[0]+"="+c[1])
	}
	return out
}

func echo(log io.Writer, r Result) {
	head := fmt.Sprintf("gate %s: %s", r.Name, r.Status)
	if r.Reason != "" {
		head += ", " + r.Reason
	}
	_, _ = fmt.Fprintf(log, "%s (%.1fs)\n", head, r.Seconds)
	for _, line := range tail(r.Output, tailLines) {
		_, _ = fmt.Fprintf(log, "  %s\n", line)
	}
}

func tail(out []byte, n int) []string {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		return nil
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return lines
}
