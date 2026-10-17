package wk

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/zach-short/wk/internal/gate"
)

// gateSpec is one gate as the worker runs it: g, possibly routed through wk run --slot by the
// wk binary at wrapper, and the first word of the command as configured, which canStart resolves.
type gateSpec struct {
	g       gate.Gate
	argv0   string
	slotted bool
	wrapper string
}

// canStart refuses a slotted gate whose command cannot be found where /bin/sh and wk run will
// look for it: on the gate's PATH ([gates.env] over the worker's own) and in the gate's
// directory. A gate run as it is needs no check, because gate.Run already reports a command it
// cannot start as an error. A gate that will skip is not checked; env is the gate's own, so a
// needs_env variable set in [gates.env] counts here as it does in gate.Run.
func (s gateSpec) canStart(root string, env map[string]string) error {
	if !s.slotted || gate.Skips(s.g, env) {
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

// wrapperFailed turns a slotted gate's fail into a start error when the fail says nothing about
// the tree (internal/slot/run.go has the lines and codes it reads):
//
//   - Exit 75, exitTempFail, 126, exitNotExecutable, or 127, exitNotFound, when wk run never ran
//     the command: the output is the wrapper's alone (every non-empty line is wk run's, or the
//     line /bin/sh prints when the wk binary itself is missing) and holds the line wk run writes
//     as it gives up: "gave up after" for 75, one naming the command for 126 and 127. A slotted
//     command that exits 75, 126 or 127 itself writes its own output, so it stays a fail and is
//     blamed like any red gate (row 5's N-a).
//   - A timeout while wk run was still waiting for a place: the wait counts against
//     timeout_seconds, and the command never ran, so the time went to another workspace's run.
//     The next worker gates the batch again. Once wk run has the place (or gave up on the wait
//     and ran the command unslotted), the command ran, and a timeout is a fail like any hang
//     (row 15's audit F1).
//
// Exit 2, wk run's usage error, is left a fail, because tsc and many other gates exit 2 for a
// red tree.
func (s gateSpec) wrapperFailed(r gate.Result) error {
	if !s.slotted || r.Status != gate.Fail {
		return nil
	}
	lines := nonEmptyLines(r.Output)
	if !s.neverRan(r.Reason, lines) {
		return nil
	}
	if timedOut(r.Reason) {
		return fmt.Errorf("gate %s: %s while still waiting for a place through wk run --slot, so its command never ran; the next worker gates the batch again", s.g.Name, r.Reason)
	}
	return fmt.Errorf("gate %s: %s through wk run --slot, which means its command could not run: %s", s.g.Name, r.Reason, lines[len(lines)-1])
}

const wkRun = "wk run: "

func (s gateSpec) neverRan(reason string, lines []string) bool {
	if timedOut(reason) {
		return stillWaiting(lines)
	}
	if !slices.ContainsFunc(lines, func(l string) bool { return s.givesUp(reason, l) }) {
		return false
	}
	for _, l := range lines {
		if !strings.HasPrefix(l, wkRun) && !s.shellLine(l) {
			return false
		}
	}
	return true
}

// givesUp is whether l is the line wk run (or /bin/sh in front of it) writes when it exits with
// reason's status without running the command. It need not be the last line: wk run can still
// say it could not write runs.log afterwards.
func (s gateSpec) givesUp(reason, l string) bool {
	switch reason {
	case "exit status 75":
		return strings.HasPrefix(l, wkRun+"gave up after ")
	case "exit status 126", "exit status 127":
		return strings.HasPrefix(l, wkRun+s.argv0+": ") || s.shellLine(l)
	}
	return false
}

func timedOut(reason string) bool { return strings.HasPrefix(reason, "timed out after ") }

// stillWaiting is whether wk run was still waiting for a place when the gate's time ran out: it
// wrote the line it writes as a wait starts, and neither line it writes before it runs the
// command after a wait.
func stillWaiting(lines []string) bool {
	return slices.ContainsFunc(lines, waitStarted) && !slices.ContainsFunc(lines, waitEnded)
}

func waitStarted(l string) bool { return strings.HasPrefix(l, wkRun+"waiting for a ") }

// waitEnded is "took a <class> place after <duration>", written once the place is taken, or
// "running unslotted: ...", written when the wait itself fails; the command runs after either.
func waitEnded(l string) bool {
	return strings.HasPrefix(l, wkRun+"took a ") || strings.HasPrefix(l, wkRun+"running unslotted: ")
}

// shellLine is the line /bin/sh prints when it cannot run the wk binary the gate is routed
// through ("/bin/sh: <wk>: No such file or directory", or dash's "/bin/sh: 1: <wk>: not
// found"): the binary is gone, so the gate never ran, which says nothing about the tree.
func (s gateSpec) shellLine(l string) bool {
	return s.wrapper != "" && strings.HasPrefix(l, "/bin/sh: ") && strings.Contains(l, s.wrapper+": ")
}

func inDir(dir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(dir, p)
}

// executable is wk run's own rule (internal/slot/run.go): a file, not a directory, that
// access(2) lets this process execute. The mode bits alone would pass a file whose only execute
// bit belongs to a group or other users, which wk run then refuses with exit 126.
func executable(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir() && unix.Access(p, unix.X_OK) == nil
}

func nonEmptyLines(out []byte) []string {
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if l = strings.TrimRight(l, " \t\r"); l != "" {
			lines = append(lines, l)
		}
	}
	return lines
}
