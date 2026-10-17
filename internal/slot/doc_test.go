package slot

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/testrepo"
)

// The self-test runs `wk run` as a separate process: this test binary, started again with
// runAsEnv set, goes through TestMain into Run. A run that dies of a signal then kills that
// process, never the test runner.
const runAsEnv = "WK_SLOT_SELFTEST_RUN"

type spec struct {
	Class       string   `json:"class"`
	Trim        []string `json:"trim"`
	Keep        bool     `json:"keep"`
	TrimAllowed bool     `json:"trim_allowed"`
	Dir         string   `json:"dir"`
	Home        string   `json:"home"`
	MemFile     string   `json:"mem_file"`
}

func TestMain(m *testing.M) {
	if encoded := os.Getenv(runAsEnv); encoded != "" {
		os.Exit(runAsWK(encoded, os.Args[1:]))
	}
	testrepo.Isolate()
	os.Exit(m.Run())
}

func runAsWK(encoded string, argv []string) int {
	var s spec
	if err := json.Unmarshal([]byte(encoded), &s); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 99
	}
	opts := s.options(os.Stdin, os.Stdout, os.Stderr)
	opts.Env = os.Environ()
	return Run(opts, argv)
}

func (s spec) options(stdin, stdout, stderr *os.File) RunOptions {
	opts := RunOptions{Class: s.Class, Trim: s.Trim, Keep: s.Keep, TrimAllowed: s.TrimAllowed,
		Dir: s.Dir, Home: s.Home, Stdin: stdin, Stdout: stdout, Stderr: stderr}
	if s.MemFile != "" {
		opts.MemFree = func() (int, error) {
			data, err := os.ReadFile(s.MemFile)
			if err != nil {
				return 0, err
			}
			return strconv.Atoi(strings.TrimSpace(string(data)))
		}
	}
	return opts
}

// fixture is a temporary WK_HOME and an empty working directory; no test reads a real one.
type fixture struct {
	t     *testing.T
	home  string
	work  string
	slots string
}

func newFixture(t *testing.T, machineTOML string) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := fixture{t: t, home: filepath.Join(root, "home"), work: filepath.Join(root, "work")}
	f.slots = filepath.Join(f.home, "slots")
	for _, d := range []string{f.home, f.work} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if machineTOML != "" {
		writeFile(t, filepath.Join(f.home, machineFile), machineTOML)
	}
	return f
}

func (f fixture) spec(class string) spec { return spec{Class: class, Dir: f.work, Home: f.home} }

func (f fixture) path(name string) string { return filepath.Join(f.work, name) }

func (f fixture) tickets(class string) []string {
	f.t.Helper()
	entries, err := os.ReadDir(filepath.Join(f.slots, class, "queue"))
	if err != nil && !os.IsNotExist(err) {
		f.t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func (f fixture) logLines(name string) []string {
	f.t.Helper()
	lines, err := readLines(filepath.Join(f.slots, name))
	if err != nil {
		f.t.Fatal(err)
	}
	return lines
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// proc is one `wk run` process under test.
type proc struct {
	t      *testing.T
	cmd    *exec.Cmd
	stdout string
	stderr string
	done   chan struct{}
}

// start runs `wk run` in a process group of the test's own making, so every signal the test
// sends goes to that group or to a pid in it, never to the group of the test runner.
func start(t *testing.T, s spec, argv ...string) *proc {
	t.Helper()
	encoded, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	p := &proc{t: t, stdout: filepath.Join(dir, "stdout"), stderr: filepath.Join(dir, "stderr"), done: make(chan struct{})}
	p.cmd = exec.Command(os.Args[0], argv...)
	p.cmd.Env = append(os.Environ(), runAsEnv+"="+string(encoded))
	p.cmd.Dir = s.Dir
	p.cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	p.cmd.Stdout, p.cmd.Stderr = createFile(t, p.stdout), createFile(t, p.stderr)
	if err := p.cmd.Start(); err != nil {
		t.Fatal(err)
	}
	go func() { _ = p.cmd.Wait(); close(p.done) }()
	t.Cleanup(p.killGroup)
	return p
}

func createFile(t *testing.T, path string) *os.File {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func (p *proc) pid() int { return p.cmd.Process.Pid }

// killGroup ends whatever is left in the group the test made, such as the command of a run the
// test killed with SIGKILL. The pid is checked, since kill(-0) would signal the test's own group.
func (p *proc) killGroup() {
	if pid := p.pid(); pid > 1 {
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}
	<-p.done
}

// wait returns the exit code, or the signal the run died of.
func (p *proc) wait() (code int, sig syscall.Signal) {
	p.t.Helper()
	select {
	case <-p.done:
	case <-time.After(60 * time.Second):
		p.t.Fatalf("wk run pid %d did not end", p.pid())
	}
	ws := p.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if ws.Signaled() {
		return -1, ws.Signal()
	}
	return ws.ExitStatus(), 0
}

func (p *proc) waitCode(want int) {
	p.t.Helper()
	code, sig := p.wait()
	if code != want || sig != 0 {
		p.t.Fatalf("wk run: exit %d, signal %v; want exit %d\nstderr:\n%s", code, sig, want, readFile(p.t, p.stderr))
	}
}

func (p *proc) stderrText() string { return readFile(p.t, p.stderr) }

func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// holdCmd marks that it started, then runs until release exists, so a test decides when a
// holder lets go of its place.
func holdCmd(started, release string) []string {
	return []string{"sh", "-c", `touch "$1"; while [ ! -e "$2" ]; do sleep 0.05; done`, "sh", started, release}
}

// holder starts a run that holds a place of class until the returned function is called.
func (f fixture) holder(class, name string) (*proc, func()) {
	f.t.Helper()
	started, release := f.path(name+".started"), f.path(name+".release")
	p := start(f.t, f.spec(class), holdCmd(started, release)...)
	eventually(f.t, name+" to take its place", func() bool { return exists(started) })
	return p, func() { writeFile(f.t, release, "") }
}

const capacityOne = "[slots.heavy]\ncapacity = 1\n"
