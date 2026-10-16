// Package procs finds the processes whose working directory is inside a directory, the way the
// land-worktrees lib.sh lsof_snapshot and live_in do.
package procs

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

type Proc struct {
	PID     int
	Command string
	Cwd     string
}

func (p Proc) String() string { return fmt.Sprintf("%d %s", p.PID, p.Command) }

// Snapshot lists every process's working directory with one `lsof -d cwd` call, which is slow,
// so a caller takes one and asks it many questions.
//
// lib.sh ignores an lsof failure. wk cannot: a snapshot that silently came back empty would
// make every live session look idle to `wk gc`. So the snapshot must contain this process
// itself, whose cwd always exists, or it is refused. lsof exits 1 when it could not read some
// other user's process, which is normal, so the exit code alone is not the test.
func Snapshot() ([]Proc, error) {
	bin, err := exec.LookPath("lsof")
	if err != nil {
		bin = "/usr/sbin/lsof"
	}
	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bin, "-d", "cwd", "-Fpcn")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	list := parse(stdout.String())
	self := os.Getpid()
	for _, p := range list {
		if p.PID == self {
			return list, nil
		}
	}
	return nil, errors.Join(fmt.Errorf("lsof listed %d processes and not this one (pid %d), so its snapshot cannot be trusted: %s",
		len(list), self, strings.TrimSpace(stderr.String())), runErr)
}

// parse reads lsof's field output: a `p` line starts a process, `c` names its command, and an
// `n` line names an open file, here only the cwd.
func parse(out string) []Proc {
	var list []Proc
	var pid int
	var command string
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		value := line[1:]
		switch line[0] {
		case 'p':
			pid, _ = strconv.Atoi(value)
			command = ""
		case 'c':
			command = value
		case 'n':
			list = append(list, Proc{PID: pid, Command: command, Cwd: value})
		}
	}
	return list
}

// In returns the processes standing in dir or below it. dir must be physical (symlinks
// resolved), because lsof prints physical paths.
func In(list []Proc, dir string) []Proc {
	var inside []Proc
	for _, p := range list {
		if p.Cwd == dir || strings.HasPrefix(p.Cwd, dir+"/") {
			inside = append(inside, p)
		}
	}
	return inside
}
