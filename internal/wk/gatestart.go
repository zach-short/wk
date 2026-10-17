package wk

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zach-short/wk/internal/gate"
)

// gateSpec is one gate as the worker runs it: g, possibly routed through wk run --slot, and the
// first word of the command as configured, which canStart resolves.
type gateSpec struct {
	g       gate.Gate
	argv0   string
	slotted bool
}

// canStart refuses a slotted gate whose command cannot be found where /bin/sh and wk run will
// look for it: on the gate's PATH ([gates.env] over the worker's own) and in the gate's
// directory. A gate run as it is needs no check, because gate.Run already reports a command it
// cannot start as an error. A gate that will skip is not checked.
func (s gateSpec) canStart(root string, env map[string]string) error {
	if !s.slotted || skips(s.g) {
		return nil
	}
	dir := filepath.Join(root, filepath.FromSlash(s.g.Dir))
	if strings.Contains(s.argv0, "/") {
		if executable(inDir(dir, s.argv0)) {
			return nil
		}
		return fmt.Errorf("gate %s: %s does not exist or is not executable, so the gate cannot start", s.g.Name, s.argv0)
	}
	path, ok := env["PATH"]
	if !ok {
		path = os.Getenv("PATH")
	}
	for _, d := range filepath.SplitList(path) {
		if d == "" {
			d = "."
		}
		if executable(inDir(dir, filepath.Join(d, s.argv0))) {
			return nil
		}
	}
	return fmt.Errorf("gate %s: %s is not on the gate's PATH (%s), so the gate cannot start", s.g.Name, s.argv0, path)
}

// wrapperFailed turns the three exit statuses wk run uses when the gate's command never ran
// into a start error (internal/slot/run.go): 75, exitTempFail, when slots.max_wait_seconds ran
// out before a place was free; 126, exitNotExecutable, and 127, exitNotFound, for a command it
// cannot run or find, as /bin/sh does too. A slotted command that exits 75, 126 or 127 by itself
// is then a start error too, which stops the pass and blames nobody. Exit 2, wk run's usage
// error, is left a fail, because tsc and many other gates exit 2 for a red tree.
func (s gateSpec) wrapperFailed(r gate.Result) error {
	if !s.slotted || r.Status != gate.Fail || !notRun[r.Reason] {
		return nil
	}
	return fmt.Errorf("gate %s: %s through wk run --slot, which means its command could not run: %s", s.g.Name, r.Reason, strings.TrimSpace(lastLine(r.Output)))
}

var notRun = map[string]bool{"exit status 75": true, "exit status 126": true, "exit status 127": true}

func skips(g gate.Gate) bool {
	for _, v := range g.NeedsEnv {
		if _, ok := os.LookupEnv(v); !ok {
			return true
		}
	}
	return false
}

func inDir(dir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

func executable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && fi.Mode()&0o111 != 0
}

func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	return lines[len(lines)-1]
}
