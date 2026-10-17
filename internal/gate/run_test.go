package gate

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// script writes a shell script under dir and returns the gate that runs it.
func script(t *testing.T, dir, name, body string) Gate {
	t.Helper()
	writeFile(t, filepath.Join(dir, name+".sh"), "set -e\n"+body)
	return Gate{Name: name, Run: []string{"sh", filepath.Join(dir, name+".sh")}}
}

func TestRunPassEnvAndDir(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	g := script(t, t.TempDir(), "pass", `echo "$WK_GATE_EXTRA"; pwd -P; echo oops >&2`)
	g.Dir = "sub"
	var log strings.Builder
	res, err := Run(root, []Gate{g}, map[string]string{"WK_GATE_EXTRA": "extra"}, nil, &log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != Pass || res[0].Reason != "" {
		t.Fatalf("results = %+v, want one pass", res)
	}
	sub, _ := filepath.EvalSymlinks(filepath.Join(root, "sub"))
	if want := "extra\n" + sub + "\noops\n"; string(res[0].Output) != want {
		t.Errorf("output = %q, want %q (stdout and stderr together, in sub)", res[0].Output, want)
	}
	if !strings.Contains(log.String(), "gate pass: pass") {
		t.Errorf("log = %q, want the gate's status line", log.String())
	}
}

func TestRunFailEchoesTailAndStops(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	gates := []Gate{
		script(t, dir, "first", "true"),
		script(t, dir, "broken", `i=1; while [ $i -le 25 ]; do echo "line $i"; i=$((i+1)); done; exit 3`),
		script(t, dir, "after", "touch "+marker),
	}
	var log strings.Builder
	res, err := Run(t.TempDir(), gates, nil, nil, &log)
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Status != Pass || res[1].Status != Fail || res[1].Reason != "exit status 3" {
		t.Fatalf("results = %+v, want pass then fail with exit status 3", res)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("the gate after the fail ran")
	}
	if l := log.String(); !strings.Contains(l, "  line 6\n") || !strings.Contains(l, "  line 25\n") || strings.Contains(l, "  line 5\n") {
		t.Errorf("log = %q, want exactly lines 6 to 25 of the failing gate", l)
	}
	if !strings.Contains(string(res[1].Output), "line 1\n") {
		t.Error("Result.Output lost the head of the output")
	}
}

func TestRunSkipsOnUnsetEnv(t *testing.T) {
	const v = "WK_GATE_TEST_DB"
	t.Setenv(v, "")
	if err := os.Unsetenv(v); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	marker := filepath.Join(dir, "ran")
	g := script(t, dir, "db", "touch "+marker)
	g.NeedsEnv = []string{v}
	res, err := Run(t.TempDir(), []Gate{g, script(t, dir, "next", "true")}, nil, nil, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 2 || res[0].Status != Skip || res[0].Reason != v+" is unset" || res[1].Status != Pass {
		t.Fatalf("results = %+v, want skip with %q, then the next gate run", res, v+" is unset")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Error("the skipped gate ran")
	}
}

// Row 5's N4: a needs_env variable set only in the env Run is given (the
// worker's [gates.env]) counts, so the gate runs; set nowhere, it skips. Skips,
// which the worker's start check reads, says the same.
func TestRunNeedsEnvReadsGatesEnv(t *testing.T) {
	const v = "WK_GATE_TEST_DB"
	t.Setenv(v, "")
	if err := os.Unsetenv(v); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	g := script(t, dir, "db", `test "$`+v+`" = gates-env`)
	g.NeedsEnv = []string{v}
	for _, tc := range []struct {
		env  map[string]string
		want string
	}{
		{map[string]string{v: "gates-env"}, Pass},
		{nil, Skip},
		{map[string]string{"OTHER": "x"}, Skip},
	} {
		res, err := Run(t.TempDir(), []Gate{g}, tc.env, nil, &strings.Builder{})
		if err != nil {
			t.Fatal(err)
		}
		if len(res) != 1 || res[0].Status != tc.want {
			t.Errorf("env %v: results = %+v; want %s", tc.env, res, tc.want)
		}
		if Skips(g, tc.env) != (tc.want == Skip) {
			t.Errorf("env %v: Skips = %v; want %v", tc.env, Skips(g, tc.env), tc.want == Skip)
		}
	}
}

// A gate that runs past its timeout is killed with its whole process group: the
// background child is gone too, and the result is a fail that says why.
func TestRunKillsAGateThatRunsPastItsTimeout(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "bg.pid")
	g := Gate{Name: "hang", Run: []string{"sh", "-c", "sleep 60 & echo $! > " + pidFile + "; sleep 60"}, Timeout: time.Second}
	start := time.Now()
	res, err := Run(t.TempDir(), []Gate{g}, nil, nil, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > 10*time.Second {
		t.Fatalf("Run took %s; want the kill after about 1s", took)
	}
	if len(res) != 1 || res[0].Status != Fail || res[0].Reason != "timed out after 1s" {
		t.Fatalf("results = %+v; want a fail timed out after 1s", res)
	}
	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			t.Fatalf("the background sleep %d still runs after the timeout", pid)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// The review's N2: a gate whose leader exits by itself is not timed out, even when the limit
// runs out while Wait still reads the pipe a background child holds. The kill still clears the
// child.
func TestRunTimeoutAfterTheLeaderExitedIsNotATimeout(t *testing.T) {
	g := Gate{Name: "bg", Run: []string{"sh", "-c", "sleep 5 & exit 0"}, Timeout: time.Second}
	res, err := Run(t.TempDir(), []Gate{g}, nil, nil, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if len(res) != 1 || res[0].Status != Pass {
		t.Fatalf("results = %+v; want a pass", res)
	}
}

// Each run gets one fresh empty directory per cache variable, shared by its
// gates and removed afterwards; the next run gets another.
func TestRunFreshCaches(t *testing.T) {
	t.Setenv("WK_GATE_CACHE", "/inherited")
	dir := t.TempDir()
	record := filepath.Join(dir, "seen")
	gates := []Gate{
		script(t, dir, "fill", `test -d "$WK_GATE_CACHE"; test -z "$(ls -A "$WK_GATE_CACHE")"; echo "$WK_GATE_CACHE" >> `+record+`; touch "$WK_GATE_CACHE/x"`),
		script(t, dir, "read", `test -f "$WK_GATE_CACHE/x"`),
	}
	env := map[string]string{"WK_GATE_CACHE": "/from-env"}
	for i := 0; i < 2; i++ {
		res, err := Run(t.TempDir(), gates, env, []string{"WK_GATE_CACHE"}, &strings.Builder{})
		if err != nil {
			t.Fatal(err)
		}
		if len(res) != 2 || res[0].Status != Pass || res[1].Status != Pass {
			t.Fatalf("run %d: results = %+v, want two passes", i, res)
		}
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	seen := strings.Fields(string(data))
	if len(seen) != 2 || seen[0] == seen[1] || seen[0] == "/inherited" || seen[0] == "/from-env" {
		t.Fatalf("cache dirs = %q, want two distinct fresh directories", seen)
	}
	for _, d := range seen {
		if _, err := os.Stat(d); !os.IsNotExist(err) {
			t.Errorf("cache %s was not removed", d)
		}
	}
}

func TestRunRefusesUnstartableGate(t *testing.T) {
	gates := []Gate{{Name: "missing", Run: []string{filepath.Join(t.TempDir(), "no-such-binary")}}}
	if _, err := Run(t.TempDir(), gates, nil, nil, &strings.Builder{}); err == nil {
		t.Error("Run gave no error for a command it could not start")
	}
}

// A child the gate leaves in the background keeps the output pipe open; Run
// returns once the gate itself exits, after pipeGrace at most.
func TestRunDoesNotWaitForBackgroundChild(t *testing.T) {
	g := script(t, t.TempDir(), "bg", "sleep 5 &\necho started\nexit 0\n")
	start := time.Now()
	res, err := Run(t.TempDir(), []Gate{g}, nil, nil, &strings.Builder{})
	if err != nil {
		t.Fatal(err)
	}
	if took := time.Since(start); took > pipeGrace+time.Second {
		t.Errorf("Run took %s, want under %s", took, pipeGrace+time.Second)
	}
	if len(res) != 1 || res[0].Status != Pass || !strings.Contains(string(res[0].Output), "started") {
		t.Errorf("results = %+v, want one pass with its output", res)
	}
}
