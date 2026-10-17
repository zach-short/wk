package slot

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

const hookTestGateLine = `cd web && bunx tsc --noEmit && bun run lint && bun run build`

// The fields of a real PreToolUse input (SLOTS-PLAN.md 0.2), from a subagent's call.
func hookTestInput(toolName, toolInput string) string {
	return `{"session_id":"s-1","transcript_path":"/t/s-1.jsonl","cwd":"/r/ezhomesteading","scratchpad_dir":"/t/scratch",` +
		`"prompt_id":"p-1","permission_mode":"auto","effort":"high","hook_event_name":"PreToolUse",` +
		`"tool_name":"` + toolName + `","tool_input":` + toolInput + `,"tool_use_id":"tu-1",` +
		`"agent_id":"a-1","agent_type":"general-purpose"}`
}

type hookTestEnv struct {
	deps HookDeps
	home string
	cwds []string
}

func hookTestSetup(t *testing.T, slotsTOML string) *hookTestEnv {
	t.Helper()
	home := t.TempDir()
	if slotsTOML != "" {
		if err := os.WriteFile(filepath.Join(home, "ezhomesteading.slots.toml"), []byte(slotsTOML), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	env := &hookTestEnv{home: home}
	env.deps = HookDeps{
		Exe:    matchTestExe,
		Home:   home,
		LogDir: filepath.Join(home, "slots"),
		Resolve: func(cwd string) (string, error) {
			env.cwds = append(env.cwds, cwd)
			return "ezhomesteading", nil
		},
	}
	return env
}

func (e *hookTestEnv) run(t *testing.T, input string) string {
	t.Helper()
	var out bytes.Buffer
	if code := HookPreToolUse(strings.NewReader(input), &out, e.deps); code != 0 {
		t.Fatalf("HookPreToolUse returned %d; it must always return 0", code)
	}
	return out.String()
}

// logLines drops each line's RFC 3339 time, checking it is there.
func (e *hookTestEnv) logLines(t *testing.T, name string) []string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(e.deps.LogDir, name))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var lines []string
	for _, line := range strings.Split(strings.TrimSuffix(string(data), "\n"), "\n") {
		stamp, rest, _ := strings.Cut(line, "\t")
		if _, err := time.Parse(time.RFC3339, stamp); err != nil {
			t.Errorf("%s line %q does not start with an RFC 3339 time", name, line)
		}
		lines = append(lines, rest)
	}
	return lines
}

func TestHookPreToolUseRewritesTheGateLine(t *testing.T) {
	env := hookTestSetup(t, matchTestSlotsTOML)
	got := env.run(t, hookTestInput("Bash", `{"command":"`+hookTestGateLine+`","description":"Run the web gates","run_in_background":false}`))
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{` +
		`"command":"cd web && /x/wk run --slot check -- bunx tsc --noEmit && /x/wk run --slot check -- bun run lint && /x/wk run --slot heavy --trim web/.next/cache -- bun run build",` +
		`"description":"Run the web gates","run_in_background":false,"timeout":600000}}}` + "\n"
	if got != want {
		t.Fatalf("stdout\n got %s\nwant %s", got, want)
	}
	if len(env.cwds) != 1 || env.cwds[0] != "/r/ezhomesteading" {
		t.Errorf("resolved %v; want the input's cwd once", env.cwds)
	}
	if lines := env.logLines(t, "errors.log"); lines != nil {
		t.Errorf("errors.log: %q", lines)
	}
}

func TestHookPreToolUseTimeout(t *testing.T) {
	for _, tc := range []struct{ name, toolInput, want string }{
		{"a lower timeout is raised, other fields kept byte for byte",
			`{"command":"go test ./...","timeout":120000,"dangerouslyDisableSandbox":true,"extra":{"a":[1, 2.50]}}`,
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"command":"/x/wk run --slot check -- go test ./...","dangerouslyDisableSandbox":true,"extra":{"a":[1,2.50]},"timeout":600000}}}` + "\n"},
		{"the maximum is kept as written",
			`{"command":"go test ./...","timeout":600000}`,
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"command":"/x/wk run --slot check -- go test ./...","timeout":600000}}}` + "\n"},
		{"a null timeout is raised",
			`{"command":"go test ./...","timeout":null}`,
			`{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{"command":"/x/wk run --slot check -- go test ./...","timeout":600000}}}` + "\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := hookTestSetup(t, matchTestSlotsTOML)
			if got := env.run(t, hookTestInput("Bash", tc.toolInput)); got != tc.want {
				t.Errorf("stdout\n got %s\nwant %s", got, tc.want)
			}
		})
	}
}

func TestHookPreToolUsePrintsNothing(t *testing.T) {
	for _, tc := range []struct{ name, slots, input string }{
		{"a non-Bash tool", matchTestSlotsTOML, hookTestInput("Read", `{"file_path":"/r/ezhomesteading/bun run build"}`)},
		{"no slots file", "", hookTestInput("Bash", `{"command":"bun run build"}`)},
		{"no match", matchTestSlotsTOML, hookTestInput("Bash", `{"command":"ls -la && git status"}`)},
		{"no command", matchTestSlotsTOML, hookTestInput("Bash", `{"description":"nothing"}`)},
		{"a slots file with no entries", "[hook]\ntimeout_ms = 100\n", hookTestInput("Bash", `{"command":"bun run build"}`)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := hookTestSetup(t, tc.slots)
			if got := env.run(t, tc.input); got != "" {
				t.Errorf("stdout %q; want nothing", got)
			}
			if _, err := os.Stat(env.deps.LogDir); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("the log directory was made (%v); nothing should be logged", err)
			}
		})
	}
}

func TestHookPreToolUseBrokenFileFailsOpen(t *testing.T) {
	env := hookTestSetup(t, matchTestSlotsTOML+"\nfail_closed = true\n")
	if got := env.run(t, hookTestInput("Bash", `{"command":"bun run build"}`)); got != "" {
		t.Errorf("stdout %q; want nothing", got)
	}
	path := filepath.Join(env.home, "ezhomesteading.slots.toml")
	want := []string{"hook pre-tool-use ezhomesteading: " + path + ": unknown keys: hook.fail_closed"}
	if got := env.logLines(t, "errors.log"); !slices.Equal(got, want) {
		t.Errorf("errors.log\n got %q\nwant %q", got, want)
	}
}

func TestHookPreToolUseBrokenFileDeniesWhenFailOpenIsFalse(t *testing.T) {
	env := hookTestSetup(t, "[hook]\nfail_open = false\nfail_closed = true\n")
	got := env.run(t, hookTestInput("Bash", `{"command":"ls"}`))
	path := filepath.Join(env.home, "ezhomesteading.slots.toml")
	want := fmt.Sprintf(`{"hookSpecificOutput":{"hookEventName":"PreToolUse","permissionDecision":"deny","permissionDecisionReason":`+
		`"wk hook pre-tool-use: %s: unknown keys: hook.fail_closed; hook.fail_open is false, so every Bash call is refused until the slots file is fixed"}}`+"\n", path)
	if got != want {
		t.Errorf("stdout\n got %s\nwant %s", got, want)
	}
	if lines := env.logLines(t, "errors.log"); len(lines) != 1 {
		t.Errorf("errors.log %q; want one line", lines)
	}
}

func TestHookPreToolUseDeadline(t *testing.T) {
	env := hookTestSetup(t, matchTestSlotsTOML)
	env.deps.Resolve = func(string) (string, error) {
		time.Sleep(250 * time.Millisecond)
		return "ezhomesteading", nil
	}
	if got := env.run(t, hookTestInput("Bash", `{"command":"bun run build"}`)); got != "" {
		t.Errorf("stdout %q; want nothing past the deadline", got)
	}
	lines := env.logLines(t, "errors.log")
	pattern := regexp.MustCompile(`^hook pre-tool-use ezhomesteading: deadline of 200 ms passed after 2[5-9]\d ms; the command runs unslotted$`)
	if len(lines) != 1 || !pattern.MatchString(lines[0]) {
		t.Errorf("errors.log %q; want one deadline line", lines)
	}
}

func TestHookPreToolUseInputNotJSON(t *testing.T) {
	env := hookTestSetup(t, matchTestSlotsTOML)
	if got := env.run(t, "bun run build\n"); got != "" {
		t.Errorf("stdout %q; want nothing", got)
	}
	want := []string{"hook pre-tool-use : PreToolUse input: invalid character 'b' looking for beginning of value"}
	if got := env.logLines(t, "errors.log"); !slices.Equal(got, want) {
		t.Errorf("errors.log\n got %q\nwant %q", got, want)
	}
}

func TestHookPreToolUseLogsMisses(t *testing.T) {
	env := hookTestSetup(t, matchTestSlotsTOML)
	if got := env.run(t, hookTestInput("Bash", `{"command":"echo $(bun run build) && go test ./..."}`)); !strings.Contains(got, `"command":"echo $(bun run build) && /x/wk run --slot check -- go test ./..."`) {
		t.Errorf("stdout %q; want the go test rewritten", got)
	}
	env.run(t, hookTestInput("Bash", `{"command":"bun run build && ("}`))
	want := []string{
		`ezhomesteading inside $( ): "bun run build" in "echo $(bun run build) && go test ./..."`,
		`ezhomesteading does not parse: 1:18: reached EOF without matching ( with ): "bun run build && (" in "bun run build && ("`,
	}
	if got := env.logLines(t, "missed.log"); !slices.Equal(got, want) {
		t.Errorf("missed.log\n got %q\nwant %q", got, want)
	}
}

// The hook writes through appendLog, so wk slots counts its lines beside wk run's own; a line in
// another stamp format would be read as having no age and never counted.
func TestHookPreToolUseLinesCountInStatus(t *testing.T) {
	env := hookTestSetup(t, matchTestSlotsTOML)
	env.run(t, hookTestInput("Bash", `{"command":"echo $(bun run build)"}`))
	env.deps.Resolve = func(string) (string, error) { return "", errors.New("not a git repository") }
	env.run(t, hookTestInput("Bash", `{"command":"bun run build"}`))
	st, err := ReadStatus(env.home)
	if err != nil {
		t.Fatal(err)
	}
	if st.Missed.Last24h != 1 || st.Errors.Last24h != 1 {
		t.Errorf("wk slots counts missed %+v and errors %+v; want one line each", st.Missed, st.Errors)
	}
}

func TestHookPreToolUsePrefersToolInputCwd(t *testing.T) {
	env := hookTestSetup(t, matchTestSlotsTOML)
	env.run(t, hookTestInput("Bash", `{"command":"go test ./...","cwd":"/r/ezhomesteading/backend"}`))
	if len(env.cwds) != 1 || env.cwds[0] != "/r/ezhomesteading/backend" {
		t.Errorf("resolved %v; want tool_input's cwd", env.cwds)
	}
}

func TestHookPreToolUseFailsOpenOnResolveErrorAndPanic(t *testing.T) {
	for _, tc := range []struct {
		name    string
		resolve func(string) (string, error)
		want    string
	}{
		{"resolve error", func(string) (string, error) { return "", errors.New("not a git repository") },
			"hook pre-tool-use : repository of /r/ezhomesteading: not a git repository"},
		{"panic", func(string) (string, error) { panic("boom") }, "hook pre-tool-use : panic: boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := hookTestSetup(t, matchTestSlotsTOML)
			env.deps.Resolve = tc.resolve
			if got := env.run(t, hookTestInput("Bash", `{"command":"bun run build"}`)); got != "" {
				t.Errorf("stdout %q; want nothing", got)
			}
			if got := env.logLines(t, "errors.log"); !slices.Equal(got, []string{tc.want}) {
				t.Errorf("errors.log %q; want %q", got, tc.want)
			}
		})
	}
}

// The done-when measurement of row 212: the real git resolver against a throwaway repository,
// 20 runs, with the timing printed. Git reads no user or system config (testrepo.Isolate's
// variables, set per test because TestMain is not this file's to define).
func TestHookPreToolUseTiming(t *testing.T) {
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	repo := filepath.Join(t.TempDir(), "ezhomesteading")
	if out, err := exec.Command("git", "init", "-q", repo).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v: %s", err, out)
	}
	web := filepath.Join(repo, "web")
	if err := os.MkdirAll(web, 0o755); err != nil {
		t.Fatal(err)
	}
	// A 5 s deadline, so the stdout check proves the real-git rewrite at any machine load; the
	// 200 ms deadline is TestHookPreToolUseDeadline's, with a stub resolver.
	env := hookTestSetup(t, strings.Replace(matchTestSlotsTOML, "timeout_ms = 200", "timeout_ms = 5000", 1))
	env.deps.Resolve = nil
	input := strings.Replace(hookTestInput("Bash", `{"command":"`+hookTestGateLine+`","description":"Run the web gates"}`), "/r/ezhomesteading", web, 1)
	want := `{"hookSpecificOutput":{"hookEventName":"PreToolUse","updatedInput":{` +
		`"command":"cd web && /x/wk run --slot check -- bunx tsc --noEmit && /x/wk run --slot check -- bun run lint && /x/wk run --slot heavy --trim web/.next/cache -- bun run build",` +
		`"description":"Run the web gates","timeout":600000}}}` + "\n"
	var total, longest time.Duration
	runs := make([]string, 0, 20)
	cpuStart := hookTestCPU(t)
	for range 20 {
		start := time.Now()
		got := env.run(t, input)
		took := time.Since(start)
		if got != want {
			t.Fatalf("stdout\n got %s\nwant %s", got, want)
		}
		total, longest = total+took, max(longest, took)
		runs = append(runs, fmt.Sprintf("%.1f", float64(took.Microseconds())/1000))
	}
	cpu, mean := (hookTestCPU(t)-cpuStart)/20, total/20
	t.Logf("HookPreToolUse, 20 runs, wall ms: %s; wall mean %.1f ms, longest %.1f ms; CPU mean %.1f ms", strings.Join(runs, " "),
		float64(mean.Microseconds())/1000, float64(longest.Microseconds())/1000, float64(cpu.Microseconds())/1000)
	if cpu >= 50*time.Millisecond {
		t.Errorf("mean CPU time %v per run over 20 runs; the bound is 50 ms", cpu)
	}
}

// The bound is CPU time because wall time on a loaded machine measures the wait for a core, not
// the hook (ezhomesteading row 234). The children's share counts the resolver's git run, which
// gitx.Run reaps, and nothing else runs meanwhile: no parallel test runs beside a sequential one.
func hookTestCPU(t *testing.T) time.Duration {
	t.Helper()
	var total time.Duration
	for _, who := range []int{syscall.RUSAGE_SELF, syscall.RUSAGE_CHILDREN} {
		var ru syscall.Rusage
		if err := syscall.Getrusage(who, &ru); err != nil {
			t.Fatal(err)
		}
		total += time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
	}
	return total
}
