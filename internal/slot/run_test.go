package slot

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestRunExitCodesPassThrough(t *testing.T) {
	t.Parallel()
	for _, code := range []int{0, 1, 3, 42, 255} {
		t.Run(strconv.Itoa(code), func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, capacityOne)
			p := start(t, f.spec("heavy"), "sh", "-c", "exit "+strconv.Itoa(code))
			p.waitCode(code)
			if got := p.stderrText(); got != "" {
				t.Fatalf("stderr %q, want nothing when no wait happened", got)
			}
		})
	}
}

func skipIfIgnored(t *testing.T, sig syscall.Signal) {
	if signal.Ignored(sig) {
		t.Skipf("%v is ignored in this test process, so its command cannot die of it", sig)
	}
}

func TestRunDiesOfTheCommandsSignal(t *testing.T) {
	t.Parallel()
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGKILL} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()
			skipIfIgnored(t, sig)
			f := newFixture(t, capacityOne)
			p := start(t, f.spec("heavy"), "sh", "-c", "kill -"+strconv.Itoa(int(sig))+" $$")
			if code, got := p.wait(); got != sig {
				t.Fatalf("wk run ended with exit %d, signal %v; want death by %v\nstderr:\n%s", code, got, sig, p.stderrText())
			}
			if lines := f.logLines(runsLog); len(lines) != 1 || !strings.Contains(lines[0], "exit="+signalName(sig)) {
				t.Fatalf("runs.log %q, want one line with exit=%s", lines, signalName(sig))
			}
		})
	}
}

func signalName(sig syscall.Signal) string { return outcome{signal: sig}.String() }

func TestRunGives128PlusOtherSignals(t *testing.T) {
	t.Parallel()
	for _, sig := range []syscall.Signal{syscall.SIGQUIT, syscall.SIGSEGV} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, capacityOne)
			p := start(t, f.spec("heavy"), "sh", "-c", "kill -"+strconv.Itoa(int(sig))+" $$")
			p.waitCode(128 + int(sig))
		})
	}
}

// runHere calls Run in the test process for cases that cannot end in a death by signal.
func runHere(t *testing.T, s spec, argv ...string) (code int, stderr string) {
	t.Helper()
	dir := t.TempDir()
	out, errOut := createFile(t, filepath.Join(dir, "stdout")), createFile(t, filepath.Join(dir, "stderr"))
	code = Run(s.options(nil, out, errOut), argv)
	return code, readFile(t, errOut.Name())
}

func TestRunCommandNotFoundOrNotExecutable(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	writeFile(t, f.path("plain"), "#!/bin/sh\nexit 0\n")
	if err := os.Mkdir(f.path("adir"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		want int
		line string
	}{
		{"wk-selftest-no-such-command", 127, "wk run: wk-selftest-no-such-command: command not found\n"},
		{"./missing", 127, "wk run: ./missing: no such file or directory\n"},
		{"./plain", 126, "wk run: ./plain: permission denied\n"},
		{"./adir", 126, "wk run: ./adir: is a directory\n"},
	} {
		code, stderr := runHere(t, f.spec("heavy"), tc.name)
		if code != tc.want || stderr != tc.line {
			t.Errorf("%s: exit %d, stderr %q; want %d, %q", tc.name, code, stderr, tc.want, tc.line)
		}
	}
	if got := f.tickets("heavy"); len(got) != 0 {
		t.Fatalf("tickets %v left by commands that never ran", got)
	}
}

func TestRunBadArgumentsExit2(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	if code, stderr := runHere(t, f.spec("heavy")); code != exitUsage || stderr != "wk run: no command to run\n" {
		t.Fatalf("no command: exit %d, stderr %q", code, stderr)
	}
}

func TestRunPassesBytesThrough(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	in, errBytes := randomBytes(t), randomBytes(t)
	writeFile(t, f.path("err.bin"), string(errBytes))
	enc := mustJSON(t, f.spec("heavy"))
	cmd := exec.Command(os.Args[0], "sh", "-c", `cat; cat "$1" >&2`, "sh", f.path("err.bin"))
	cmd.Env = append(os.Environ(), runAsEnv+"="+enc)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stdin = bytes.NewReader(in)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("wk run: %v\nstderr %d bytes", err, stderr.Len())
	}
	if sha256.Sum256(stdout.Bytes()) != sha256.Sum256(in) {
		t.Fatalf("stdout: %d bytes, not the %d bytes of stdin", stdout.Len(), len(in))
	}
	if sha256.Sum256(stderr.Bytes()) != sha256.Sum256(errBytes) {
		t.Fatalf("stderr: %d bytes, not the command's %d bytes and nothing else", stderr.Len(), len(errBytes))
	}
}

func randomBytes(t *testing.T) []byte {
	b := make([]byte, 1<<20)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		t.Fatal(err)
	}
	return b
}

// trapCmd traps sig, counts each delivery in count, and exits 3 on the first.
func trapCmd(sig, count, ready string) []string {
	script := `trap 'echo got >> "$1"; exit 3' ` + sig + `; touch "$2"; while :; do sleep 0.05; done`
	return []string{"sh", "-c", script, "sh", count, ready}
}

func TestRunForwardsASignalToTheCommandOnce(t *testing.T) {
	t.Parallel()
	for _, sig := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		t.Run(sig.String(), func(t *testing.T) {
			t.Parallel()
			skipIfIgnored(t, sig)
			f := newFixture(t, capacityOne)
			count, ready := f.path("count"), f.path("ready")
			p := start(t, f.spec("heavy"), trapCmd(strings.TrimPrefix(signalName(sig), "SIG"), count, ready)...)
			eventually(t, "the trap", func() bool { return exists(ready) })
			if err := syscall.Kill(p.pid(), sig); err != nil {
				t.Fatal(err)
			}
			p.waitCode(3)
			if got := readFile(t, count); got != "got\n" {
				t.Fatalf("the command saw %q, want the signal once", got)
			}
		})
	}
}

func TestRunSignalToTheWholeGroup(t *testing.T) {
	t.Parallel()
	t.Run("trapped", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, capacityOne)
		count, ready := f.path("count"), f.path("ready")
		p := start(t, f.spec("heavy"), trapCmd("TERM", count, ready)...)
		eventually(t, "the trap", func() bool { return exists(ready) })
		if err := syscall.Kill(-p.pid(), syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		p.waitCode(3)
	})
	t.Run("default", func(t *testing.T) {
		t.Parallel()
		f := newFixture(t, capacityOne)
		ready := f.path("ready")
		p := start(t, f.spec("heavy"), "sh", "-c", `touch "$1"; sleep 30`, "sh", ready)
		eventually(t, "the command", func() bool { return exists(ready) })
		if err := syscall.Kill(-p.pid(), syscall.SIGTERM); err != nil {
			t.Fatal(err)
		}
		if code, sig := p.wait(); sig != syscall.SIGTERM {
			t.Fatalf("exit %d, signal %v; want death by SIGTERM", code, sig)
		}
	})
}

func TestRunSignalWhileWaitingRemovesTheTicket(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	h, release := f.holder("heavy", "holder")
	ran := f.path("waiter-ran")
	w := start(t, f.spec("heavy"), "touch", ran)
	eventually(t, "the waiter's ticket", func() bool { return len(f.tickets("heavy")) == 1 })
	if err := syscall.Kill(w.pid(), syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if code, sig := w.wait(); sig != syscall.SIGTERM {
		t.Fatalf("waiter: exit %d, signal %v; want death by SIGTERM", code, sig)
	}
	if got := f.tickets("heavy"); len(got) != 0 || exists(ran) {
		t.Fatalf("tickets %v, command ran %v; want no ticket and no run", got, exists(ran))
	}
	st, err := ReadStatus(f.home)
	if err != nil {
		t.Fatal(err)
	}
	if holder := st.Classes[0].Places[0].Holder; holder == nil || holder.PID != h.pid() {
		t.Fatalf("place 0 after the waiter left: %+v, want held by pid %d", holder, h.pid())
	}
	release()
	h.waitCode(0)
}

func TestRunNestedPassesThrough(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "[slots]\nmax_wait_seconds = 5\n\n[slots.heavy]\ncapacity = 1\n\n[slots.check]\ncapacity = 1\n")
	held := f.path("held")
	outer := start(t, f.spec("heavy"), os.Args[0], "sh", "-c", `printf %s "$WK_SLOT_HELD" > "$1"; exit 5`, "sh", held)
	outer.waitCode(5)
	if got := readFile(t, held); got != "heavy" {
		t.Fatalf("WK_SLOT_HELD in the nested command = %q, want the outer class", got)
	}
	if lines := f.logLines(runsLog); len(lines) != 1 {
		t.Fatalf("runs.log %q, want only the outer run", lines)
	}
}

func TestRunFailsOpen(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, machine, class, reason string
		blockClassDir                bool
	}{
		{name: "broken machine file", machine: "[slots\n", class: "heavy", reason: "machine.toml"},
		{name: "unknown key", machine: capacityOne + "colour = \"red\"\n", class: "heavy", reason: "unknown keys: slots.heavy.colour"},
		{name: "unknown class", machine: capacityOne, class: "nope", reason: `no class "nope"`},
		{name: "unwritable slot directory", machine: capacityOne, class: "heavy", reason: "not a directory", blockClassDir: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, tc.machine)
			if tc.blockClassDir {
				writeFile(t, filepath.Join(f.slots, "heavy"), "a file where the class directory goes\n")
			}
			code, stderr := runHere(t, f.spec(tc.class), "sh", "-c", "exit 4")
			lines := strings.Split(strings.TrimSuffix(stderr, "\n"), "\n")
			if code != 4 || len(lines) != 1 || !strings.HasPrefix(lines[0], "wk run: running unslotted: ") || !strings.Contains(lines[0], tc.reason) {
				t.Fatalf("exit %d, stderr %q; want exit 4 and one unslotted line naming %q", code, stderr, tc.reason)
			}
			if got := f.logLines(errorsLog); len(got) != 1 || !strings.Contains(got[0], tc.reason) {
				t.Fatalf("errors.log %q, want one line naming %q", got, tc.reason)
			}
		})
	}
}

func TestRunWithoutAMachineFile(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	code, stderr := runHere(t, f.spec("heavy"), "sh", "-c", "exit 6")
	want := "wk run: running unslotted: no machine file at " + filepath.Join(f.home, machineFile) + "\n"
	if code != 6 || stderr != want {
		t.Fatalf("exit %d, stderr %q; want 6, %q", code, stderr, want)
	}
	if exists(f.slots) {
		t.Fatalf("%s was made without a machine file", f.slots)
	}
}

func TestRunMaxWaitSeconds(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "[slots]\nmax_wait_seconds = 1\n\n"+capacityOne)
	h, release := f.holder("heavy", "holder")
	began := time.Now()
	w := start(t, f.spec("heavy"), "touch", f.path("waiter-ran"))
	w.waitCode(exitTempFail)
	if waited := time.Since(began); waited < time.Second {
		t.Fatalf("gave up after %s, before slots.max_wait_seconds", waited)
	}
	if !strings.Contains(w.stderrText(), "gave up after 1s waiting for a heavy place") || exists(f.path("waiter-ran")) {
		t.Fatalf("stderr %q; want the give-up line and no run", w.stderrText())
	}
	if got := f.tickets("heavy"); len(got) != 0 {
		t.Fatalf("tickets %v after the wait ran out", got)
	}
	release()
	h.waitCode(0)
	if lines := f.logLines(runsLog); len(lines) != 2 || !strings.Contains(strings.Join(lines, "\n"), "exit=75") {
		t.Fatalf("runs.log %q, want the holder's run and the waiter's exit=75", lines)
	}
}

func TestRunOrderAnyTakesNoTicket(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "[slots]\norder = \"any\"\n\n"+capacityOne)
	h, release := f.holder("heavy", "holder")
	w := start(t, f.spec("heavy"), "touch", f.path("waiter-ran"))
	eventually(t, "the waiting line", func() bool { return strings.Contains(w.stderrText(), "waiting for a heavy place") })
	if got := f.tickets("heavy"); len(got) != 0 {
		t.Fatalf("tickets %v under order = \"any\"", got)
	}
	release()
	h.waitCode(0)
	w.waitCode(0)
	if !exists(f.path("waiter-ran")) {
		t.Fatal("the waiter's command did not run")
	}
}

func TestRunMinFreeMB(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "[slots]\nmin_free_mb = 500\n\n"+capacityOne)
	mem := f.path("mem")
	writeFile(t, mem, "100\n")
	s := f.spec("heavy")
	s.MemFile = mem
	w := start(t, s, "touch", f.path("ran"))
	eventually(t, "the memory line", func() bool {
		return strings.Contains(w.stderrText(), "waiting for a heavy place: 100 MB free, under slots.min_free_mb 500")
	})
	if exists(f.path("ran")) {
		t.Fatal("the command ran with too little memory")
	}
	writeFile(t, mem, "900\n")
	w.waitCode(0)
	if !strings.Contains(w.stderrText(), "took a heavy place after") {
		t.Fatalf("stderr %q, want the took line", w.stderrText())
	}
}

func TestEnvValueLastWins(t *testing.T) {
	t.Parallel()
	env := []string{"PATH=/a", "X=1", "PATH=/b"}
	if v, ok := envValue(env, "PATH"); !ok || v != "/b" {
		t.Fatalf("PATH = %q, %v", v, ok)
	}
	if _, ok := envValue(env, "PAT"); ok {
		t.Fatal("PAT matched a PATH entry")
	}
}

func TestFormatCommand(t *testing.T) {
	t.Parallel()
	got := formatCommand([]string{"bun", "run", "build", "a b", "", "it's"})
	if want := `bun run build 'a b' '' 'it'\''s'`; got != want {
		t.Fatalf("formatCommand = %q, want %q", got, want)
	}
}

func mustJSON(t *testing.T, s spec) string {
	t.Helper()
	enc, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return string(enc)
}

func TestRunSurvivesAClosedStderr(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	h, release := f.holder("heavy", "holder")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	ran := f.path("waiter-ran")
	cmd := exec.Command(os.Args[0], "touch", ran)
	cmd.Env = append(os.Environ(), runAsEnv+"="+mustJSON(t, f.spec("heavy")))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Stderr = w
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	eventually(t, "the waiter's ticket", func() bool { return len(f.tickets("heavy")) == 1 })
	release()
	h.waitCode(0)
	err = cmd.Wait()
	if ws := cmd.ProcessState.Sys().(syscall.WaitStatus); err != nil || !exists(ran) {
		t.Fatalf("wk run with a closed stderr: %v (signaled %v %v), command ran %v", err, ws.Signaled(), ws.Signal(), exists(ran))
	}
}

func TestRunLeavesAnIgnoredSignalIgnored(t *testing.T) {
	t.Parallel()
	nohup, err := exec.LookPath("nohup")
	if err != nil {
		t.Skip("no nohup to start wk run with SIGHUP ignored")
	}
	f := newFixture(t, capacityOne)
	skipIfIgnored(t, syscall.SIGHUP)
	if _, sig := start(t, f.spec("heavy"), "sh", "-c", "kill -HUP $$; exit 7").wait(); sig != syscall.SIGHUP {
		t.Fatalf("without nohup: signal %v, want death by SIGHUP", sig)
	}
	cmd := exec.Command(nohup, os.Args[0], "sh", "-c", "kill -HUP $$; exit 7")
	cmd.Env = append(os.Environ(), runAsEnv+"="+mustJSON(t, f.spec("heavy")))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Dir, cmd.Stdout, cmd.Stderr = f.work, createFile(t, f.path("out")), createFile(t, f.path("err"))
	err = cmd.Run()
	if code := cmd.ProcessState.ExitCode(); code != 7 {
		t.Fatalf("under nohup: exit %d, %v; want 7, SIGHUP still ignored for the command", code, err)
	}
}
