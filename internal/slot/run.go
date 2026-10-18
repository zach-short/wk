package slot

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// RunOptions is one `wk run`. The three descriptors are *os.File so os/exec hands them to the
// command as they are: any other io type makes it copy through a pipe, which changes buffering
// and can lose bytes at exit.
type RunOptions struct {
	Class                 string
	Trim                  []string // relative to the git top level of Dir; refused when absolute or with ".."
	Keep                  bool
	TrimAllowed           bool   // false in the main checkout, the template and a pool slot (SB-12); the caller decides
	Dir                   string // the working directory of the command
	Home                  string // WK_HOME
	Stdin, Stdout, Stderr *os.File
	Env                   []string
	MemFree               func() (mb int, err error) // nil means read vm_stat (SB-19)
}

const (
	exitUsage         = 2
	exitTempFail      = 75 // EX_TEMPFAIL, when slots.max_wait_seconds runs out (SB-19)
	exitNotExecutable = 126
	exitNotFound      = 127
	heldEnv           = "WK_SLOT_HELD"
	// SayStartEnv, set to any value, makes wk run write "starting <command>" just before the
	// command starts, on every path that starts it. The landing worker sets it on a slotted gate,
	// because a gate's own output can print any line wk run writes, but cannot remove one: a
	// timeout without the line is a command that never ran (row 16's A2). The command never sees
	// it, so a wk run inside the command stays quiet (SB-10).
	SayStartEnv = "WK_RUN_SAY_START"
	defaultPath = "/usr/bin:/bin"
)

type runner struct {
	opts      RunOptions
	argv      []string
	env       []string
	dir       string
	stderr    io.Writer
	slotsDir  string
	held      *place
	waited    time.Duration
	memWarned bool
	sayStart  bool
}

// Run executes argv under a place of opts.Class and returns the exit status for os.Exit.
// On a child death by SIGINT, SIGTERM, SIGHUP or SIGKILL it resets its handler and kills itself
// with that signal, so it does not return (SB-8). It loads the machine file from opts.Home itself,
// so every fail-open path of SB-9 lives inside it.
func Run(opts RunOptions, argv []string) int {
	r := newRunner(opts, argv)
	if err := r.checkArgs(); err != nil {
		r.say("%v", err)
		return exitUsage
	}
	path, code := r.resolve()
	if code != 0 {
		return code
	}
	sigs := catchSignals()
	defer signal.Stop(sigs)
	if code, ended := r.admit(sigs); ended {
		return code
	}
	if r.sayStart {
		r.say("starting %s", formatCommand(r.argv))
	}
	return r.finish(r.runChild(path, sigs))
}

func newRunner(opts RunOptions, argv []string) *runner {
	r := &runner{opts: opts, argv: argv, env: opts.Env, dir: opts.Dir, stderr: io.Discard}
	if opts.Stderr != nil {
		r.stderr = opts.Stderr
	}
	if r.env == nil {
		r.env = os.Environ()
	}
	v, _ := envValue(r.env, SayStartEnv)
	r.sayStart = v != ""
	r.env = slices.DeleteFunc(slices.Clone(r.env), func(kv string) bool { return strings.HasPrefix(kv, SayStartEnv+"=") })
	if abs, err := filepath.Abs(r.dir); err == nil {
		r.dir = abs
	}
	return r
}

func (r *runner) say(format string, args ...any) {
	_, _ = fmt.Fprintf(r.stderr, "wk run: "+format+"\n", args...)
}

func (r *runner) checkArgs() error {
	if len(r.argv) == 0 {
		return errors.New("no command to run")
	}
	return checkTrim(r.opts.Trim)
}

// resolve finds the command as the shell would, in the PATH of the command's own environment and
// relative to its own directory, so a typo fails before the run waits for a place.
func (r *runner) resolve() (string, int) {
	name := r.argv[0]
	path, err := lookPath(name, r.dir, r.pathEnv())
	switch {
	case err == nil:
		return path, 0
	case errors.Is(err, fs.ErrNotExist) && !strings.Contains(name, "/"):
		r.say("%s: command not found", name)
		return "", exitNotFound
	case errors.Is(err, fs.ErrNotExist):
		r.say("%s: no such file or directory", name)
		return "", exitNotFound
	}
	r.say("%s: %v", name, err)
	return "", exitNotExecutable
}

func (r *runner) pathEnv() string {
	if p, ok := envValue(r.env, "PATH"); ok {
		return p
	}
	return defaultPath
}

// envValue reads key the way os/exec builds the child's environment: the last entry wins.
func envValue(env []string, key string) (string, bool) {
	for i := len(env) - 1; i >= 0; i-- {
		if v, ok := strings.CutPrefix(env[i], key+"="); ok {
			return v, true
		}
	}
	return "", false
}

func lookPath(name, dir, pathEnv string) (string, error) {
	if strings.Contains(name, "/") {
		if !filepath.IsAbs(name) {
			name = filepath.Join(dir, name)
		}
		return name, executable(name)
	}
	for _, d := range filepath.SplitList(pathEnv) {
		if !filepath.IsAbs(d) {
			d = filepath.Join(dir, d)
		}
		if p := filepath.Join(d, name); executable(p) == nil {
			return p, nil
		}
	}
	return "", fs.ErrNotExist
}

func executable(path string) error {
	fi, err := os.Stat(path)
	switch {
	case err != nil:
		return err
	case fi.IsDir():
		return errors.New("is a directory")
	case unix.Access(path, unix.X_OK) != nil:
		return errors.New("permission denied")
	}
	return nil
}

// caught are the signals wk run takes over so that a stop of the run reaches the command (SB-7).
// SIGTSTP, SIGCONT, SIGTTIN, SIGTTOU and SIGWINCH stay at their defaults, so job control stops
// and continues both processes together.
var caught = []os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT, syscall.SIGUSR1, syscall.SIGUSR2}

func catchSignals() chan os.Signal {
	c := make(chan os.Signal, 16)
	for _, s := range caught {
		// A signal the shell set to ignored stays ignored, so the command inherits it unchanged.
		if !signal.Ignored(s) {
			signal.Notify(c, s)
		}
	}
	// Caught and dropped: a write to a closed stderr then fails instead of killing wk run while
	// its command runs on, and the command still starts with the default disposition.
	signal.Notify(c, syscall.SIGPIPE)
	return c
}

// admit takes a place for the run, or decides it runs without one. ended is true when the run
// ends here, because the wait ran out or a signal came while it waited.
func (r *runner) admit(sigs <-chan os.Signal) (code int, ended bool) {
	if r.opts.Class == "" || r.nested() {
		return 0, false
	}
	m, found, err := LoadMachine(r.opts.Home)
	switch {
	case err != nil:
		r.unslotted(defaultSlotsDir(r.opts.Home), err.Error())
		return 0, false
	case !found:
		r.say("running unslotted: no machine file at %s", machinePath(r.opts.Home))
		return 0, false
	}
	if _, ok := m.Classes[r.opts.Class]; !ok {
		r.unslotted(m.Dir, fmt.Sprintf("no class %q in %s", r.opts.Class, machinePath(r.opts.Home)))
		return 0, false
	}
	return r.queueUp(m, sigs)
}

// nested is true under another wk run of any class: a heavy holder waiting for a check place
// while a check holder waits for heavy would deadlock (SB-11).
func (r *runner) nested() bool {
	v, _ := envValue(r.env, heldEnv)
	return v != ""
}

func (r *runner) queueUp(m Machine, sigs <-chan os.Signal) (int, bool) {
	q := newQueue(m, r.opts.Class, r.memFree())
	r.slotsDir = m.Dir
	p, end, err := r.wait(q, time.Duration(m.MaxWaitSeconds)*time.Second, sigs)
	switch {
	case err != nil:
		r.slotsDir = ""
		r.unslotted(m.Dir, err.Error())
		return 0, false
	case end != nil:
		r.logRun(*end, 0)
		return end.exit(), true
	}
	r.held = p
	return 0, false
}

func (r *runner) memFree() func() (int, error) {
	if r.opts.MemFree != nil {
		return r.opts.MemFree
	}
	return vmStatFreeMB
}

// wait queues the run and polls until it holds a place (SB-5). It returns an outcome instead
// when the run ends while it waits, and an error when the queue itself fails, so the run goes
// on unslotted. The ticket is gone by the time it returns.
func (r *runner) wait(q queue, maxWait time.Duration, sigs <-chan os.Signal) (*place, *outcome, error) {
	if err := q.prepare(); err != nil {
		return nil, nil, err
	}
	rec := r.record()
	t, err := q.enqueue(rec)
	if err != nil {
		return nil, nil, err
	}
	defer t.cancel()
	start := time.Now()
	for pass := 0; ; pass++ {
		p, st, err := q.tryAdmit(t, rec)
		r.waited = time.Since(start)
		if err != nil {
			return nil, nil, err
		}
		r.notePass(q, st, pass, p != nil)
		if p != nil {
			return p, nil, nil
		}
		if end := r.pause(q, st, start, maxWait, sigs); end != nil {
			return nil, end, nil
		}
	}
}

// notePass prints what a waiter owes (SB-10): nothing when the first pass takes a place, one
// line as the wait starts and one as the place is taken after it. A failed memory reading is
// reported once per run (SB-19).
func (r *runner) notePass(q queue, st waitState, pass int, admitted bool) {
	switch {
	case admitted && pass > 0:
		r.say("took a %s place after %s", q.class, r.waited.Round(100*time.Millisecond))
	case !admitted && pass == 0:
		r.sayWaiting(q, st)
	}
	if st.memErr != nil && !r.memWarned {
		r.warn("slots.min_free_mb not applied: " + st.memErr.Error())
		r.memWarned = true
	}
}

func (r *runner) record() record {
	return record{PID: os.Getpid(), Workspace: r.dir, Command: formatCommand(r.argv), Start: time.Now()}
}

func (r *runner) sayWaiting(q queue, st waitState) {
	if st.memLow {
		r.say("waiting for a %s place: %d MB free, under slots.min_free_mb %d", q.class, st.memMB, q.minFreeMB)
		return
	}
	by := ""
	if len(st.holders) > 0 {
		parts := make([]string, len(st.holders))
		for i, h := range st.holders {
			parts[i] = h.describe(time.Now())
		}
		by = " by " + strings.Join(parts, ", ")
	}
	r.say("waiting for a %s place: %d of %d held%s; %d ahead of this run", q.class, len(st.holders), q.capacity, by, st.ahead)
}

func (rec record) describe(now time.Time) string {
	if rec.PID == 0 {
		return "an unknown run"
	}
	return fmt.Sprintf("pid %d in %s (%s, %s)", rec.PID, rec.Workspace, rec.Command, now.Sub(rec.Start).Round(time.Second))
}

// pause sleeps until the next pass. It ends the wait when slots.max_wait_seconds runs out or a
// signal comes; the caller's deferred cancel then removes the ticket, and the places stay as
// they were (SB-7, SB-19).
func (r *runner) pause(q queue, st waitState, start time.Time, maxWait time.Duration, sigs <-chan os.Signal) *outcome {
	d := st.interval(q.capacity)
	if maxWait > 0 {
		left := maxWait - time.Since(start)
		if left <= 0 {
			r.say("gave up after %s waiting for a %s place (slots.max_wait_seconds)", maxWait, q.class)
			return &outcome{code: exitTempFail}
		}
		d = min(d, left)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		select {
		case <-timer.C:
			return nil
		case s := <-sigs:
			if s != syscall.SIGPIPE {
				o := signalOutcome(s.(syscall.Signal))
				return &o
			}
		}
	}
}

func (r *runner) unslotted(dir, reason string) {
	r.say("running unslotted: %s", reason)
	r.logError(dir, reason)
}

// warn reports a failure that does not change how the command runs.
func (r *runner) warn(reason string) {
	r.say("%s", reason)
	dir := r.slotsDir
	if dir == "" {
		dir = defaultSlotsDir(r.opts.Home)
	}
	r.logError(dir, reason)
}

func (r *runner) logError(dir, reason string) {
	if !filepath.IsAbs(dir) {
		return
	}
	line := fmt.Sprintf("run\tpid=%d\tclass=%s\t%s", os.Getpid(), r.opts.Class, reason)
	if err := appendLog(dir, errorsLog, line); err != nil {
		r.say("cannot write %s: %v", filepath.Join(dir, errorsLog), err)
	}
}

// runChild runs the command with wk run's own descriptors, in wk run's own process group, so a
// stop that signals the shell's group still reaches it (SB-6). The place's lock file is opened
// close-on-exec, as Go opens every file, so the place belongs to wk run alone and a server the
// command leaves running never holds it.
func (r *runner) runChild(path string, sigs <-chan os.Signal) (*os.ProcessState, time.Duration, error) {
	cmd := r.command(path)
	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, 0, err
	}
	done := make(chan struct{})
	go forward(cmd.Process, sigs, done, r.opts.Stdin)
	_ = cmd.Wait()
	close(done)
	return cmd.ProcessState, time.Since(start), nil
}

func (r *runner) command(path string) *exec.Cmd {
	cmd := &exec.Cmd{Path: path, Args: r.argv, Dir: r.dir, Env: r.env}
	if r.held != nil {
		cmd.Env = append(slices.Clip(r.env), heldEnv+"="+r.opts.Class)
	}
	// A nil *os.File stored in the io interface would not be nil to os/exec.
	if r.opts.Stdin != nil {
		cmd.Stdin = r.opts.Stdin
	}
	if r.opts.Stdout != nil {
		cmd.Stdout = r.opts.Stdout
	}
	if r.opts.Stderr != nil {
		cmd.Stderr = r.opts.Stderr
	}
	return cmd
}

// forward sends each caught signal to the command's pid, never to the group (SB-7).
func forward(p *os.Process, sigs <-chan os.Signal, done <-chan struct{}, stdin *os.File) {
	for {
		select {
		case <-done:
			return
		case s := <-sigs:
			if relays(s, stdin) {
				_ = p.Signal(s)
			}
		}
	}
}

// relays leaves out SIGPIPE, and SIGINT and SIGQUIT from a terminal: the terminal already sent
// those to its whole foreground group, and a second SIGINT means "force quit" to some tools.
func relays(s os.Signal, stdin *os.File) bool {
	switch s {
	case syscall.SIGPIPE:
		return false
	case syscall.SIGINT, syscall.SIGQUIT:
		return !inForeground(stdin)
	}
	return true
}

func inForeground(f *os.File) bool {
	if f == nil {
		return false
	}
	pgrp, err := unix.IoctlGetInt(int(f.Fd()), unix.TIOCGPGRP)
	return err == nil && pgrp == unix.Getpgrp()
}

func (r *runner) finish(ps *os.ProcessState, ran time.Duration, err error) int {
	if err != nil {
		r.release()
		r.say("%s: %v", r.argv[0], err)
		return exitNotExecutable
	}
	o := outcomeOf(ps)
	r.trim()
	r.release()
	r.logRun(o, ran)
	return o.exit()
}

func (r *runner) release() {
	if r.held != nil {
		_ = r.held.release()
		r.held = nil
	}
}

// logRun appends the run to runs.log (SB-10), which `wk slots` and the live proof read.
func (r *runner) logRun(o outcome, ran time.Duration) {
	if r.slotsDir == "" {
		return
	}
	line := fmt.Sprintf("class=%s\tpid=%d\texit=%s\twait=%.1fs\trun=%.1fs\tdir=%s\tcmd=%s",
		r.opts.Class, os.Getpid(), o, r.waited.Seconds(), ran.Seconds(), noTabs(r.dir), noTabs(formatCommand(r.argv)))
	if err := appendLog(r.slotsDir, runsLog, line); err != nil {
		r.say("cannot write %s: %v", filepath.Join(r.slotsDir, runsLog), err)
	}
}

func noTabs(s string) string { return strings.ReplaceAll(s, "\t", " ") }

// formatCommand quotes only the words a shell would need quoted, so a log line reads like the
// command that was typed.
func formatCommand(argv []string) string {
	words := make([]string, len(argv))
	for i, a := range argv {
		words[i] = a
		if a == "" || strings.ContainsAny(a, " \t\n'\"\\$`|&;<>()*?[]{}#~!") {
			words[i] = "'" + strings.ReplaceAll(a, "'", `'\''`) + "'"
		}
	}
	return strings.Join(words, " ")
}

// outcome is how the command ended, as the shell reports it.
type outcome struct {
	code   int            // the exit code, or 128 plus the signal for a death wk run cannot copy
	signal syscall.Signal // set when wk run must die of the same signal (SB-8)
}

func outcomeOf(ps *os.ProcessState) outcome {
	ws, ok := ps.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() {
		return outcome{code: ps.ExitCode()}
	}
	return signalOutcome(ws.Signal())
}

// signalOutcome copies only SIGINT, SIGTERM, SIGHUP and SIGKILL: Go cannot restore the default
// action of SIGQUIT or SIGSEGV without its own crash dump, and ignores SIGUSR1 and SIGUSR2 once
// they are reset, so those give 128 plus the number, the code the shell reports.
func signalOutcome(sig syscall.Signal) outcome {
	o := outcome{code: 128 + int(sig)}
	switch sig {
	case syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGKILL:
		o.signal = sig
	}
	return o
}

func (o outcome) String() string {
	if o.signal != 0 {
		return unix.SignalName(o.signal)
	}
	return strconv.Itoa(o.code)
}

// exit returns the code, or ends wk run by the same signal so the shell sees the same death:
// bash and zsh stop a script on a child's SIGINT death and do not on an exit of 130. A signal
// ignored since wk run started cannot kill it, so that one falls back to the code.
func (o outcome) exit() int {
	if o.signal == 0 || signal.Ignored(o.signal) {
		return o.code
	}
	signal.Reset(o.signal)
	_ = syscall.Kill(os.Getpid(), o.signal)
	// The signal can land on another thread; the process ends there, not here.
	time.Sleep(time.Second)
	return o.code
}
