package gate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
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
