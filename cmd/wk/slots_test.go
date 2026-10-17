package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSlotVerbsRefuseBadArguments(t *testing.T) {
	isolated(t)
	for name, tc := range map[string]struct {
		args       []string
		why, usage string
	}{
		"run without --":       {[]string{"run", "--keep"}, "needs -- before the command", "wk run [--slot <class>]"},
		"run a bare command":   {[]string{"run", "true"}, `unexpected argument to wk run: "true"`, "wk run [--slot <class>]"},
		"run with no command":  {[]string{"run", "--slot", "heavy", "--"}, "needs a command after --", "wk run [--slot <class>]"},
		"run unknown flag":     {[]string{"run", "--bogus", "--", "true"}, `unexpected argument to wk run: "--bogus"`, "wk run [--slot <class>]"},
		"run --slot no value":  {[]string{"run", "--slot", "--", "true"}, `--slot needs a value, not "--"`, "wk run [--slot <class>]"},
		"run --slot last":      {[]string{"run", "--slot"}, `unexpected argument to wk run: "--slot"`, "wk run [--slot <class>]"},
		"run --slot twice":     {[]string{"run", "--slot", "a", "--slot", "b", "--", "true"}, "--slot is given twice", "wk run [--slot <class>]"},
		"run --trim no value":  {[]string{"run", "--trim", "--keep", "--", "true"}, `--trim needs a value, not "--keep"`, "wk run [--slot <class>]"},
		"run behind --repo":    {[]string{"--repo", "rel", "run", "sh"}, `unexpected argument to wk run: "sh"`, "wk run [--slot <class>]"},
		"slots unknown flag":   {[]string{"slots", "--yaml"}, `unexpected arguments to wk slots: "--yaml"`, "wk slots [--json]"},
		"slots two arguments":  {[]string{"slots", "--json", "--json"}, "unexpected arguments to wk slots", "wk slots [--json]"},
		"hook unknown subverb": {[]string{"hook", "pre-tool-use-x"}, "usage:", "|pre-tool-use < <Claude Code hook input>"},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != exitUsage {
				t.Fatalf("exit %d, want %d\n%s", code, exitUsage, stderr.String())
			}
			if stdout.Len() != 0 {
				t.Fatalf("stdout = %q, want nothing", stdout.String())
			}
			for _, want := range []string{tc.why, tc.usage} {
				if !strings.Contains(stderr.String(), want) {
					t.Errorf("stderr lacks %q:\n%s", want, stderr.String())
				}
			}
		})
	}
}

// The command's own status comes back, and the run went through the place of the machine file
// in WK_HOME, so the wiring hands slot.Run the home and the class.
func TestRunPassesTheExitStatusThroughAPlace(t *testing.T) {
	isolated(t)
	home := os.Getenv("WK_HOME")
	machine := "[slots]\ndir = \"" + filepath.Join(home, "slots") + "\"\n\n[slots.heavy]\ncapacity = 1\n"
	if err := os.WriteFile(filepath.Join(home, "machine.toml"), []byte(machine), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"run", "--slot", "heavy", "--", "sh", "-c", "exit 7"}, &stdout, &stderr); code != 7 {
		t.Fatalf("exit %d, want 7\n%s", code, stderr.String())
	}
	runs, err := os.ReadFile(filepath.Join(home, "slots", "runs.log"))
	if err != nil || !strings.Contains(string(runs), "exit=7") || strings.Count(string(runs), "\n") != 1 {
		t.Fatalf("runs.log %q (%v), want one line with exit=7", runs, err)
	}
}

func TestSlotsPrintsTheStatus(t *testing.T) {
	isolated(t)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"slots"}, &stdout, &stderr); code != exitOK || !strings.Contains(stdout.String(), "no machine file at ") {
		t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
	}
	stdout.Reset()
	if code := run([]string{"slots", "--json"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d, stderr %q", code, stderr.String())
	}
	var doc struct {
		Found   *bool `json:"found"`
		Classes []any `json:"classes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil || doc.Found == nil || *doc.Found || doc.Classes == nil {
		t.Fatalf("stdout %q is not the empty status (%v)", stdout.String(), err)
	}
}

// Exit 2 from a PreToolUse hook blocks the Bash call, so no input, home or flag may make the hook
// exit non-zero or print anything but a decision.
func TestHookPreToolUseAlwaysExitsZero(t *testing.T) {
	isolated(t)
	for name, tc := range map[string]struct {
		args          []string
		home, input   string
		wantLogInHome bool
	}{
		"input that is not JSON": {[]string{"hook", "pre-tool-use"}, "", "{", true},
		"empty input":            {[]string{"hook", "pre-tool-use"}, "", "", true},
		"a relative WK_HOME":     {[]string{"hook", "pre-tool-use"}, "rel", "{", false},
		"a bad --repo":           {[]string{"--repo", "rel", "hook", "pre-tool-use"}, "", `{"tool_name":"Read"}`, false},
	} {
		t.Run(name, func(t *testing.T) {
			t.Setenv("WK_HOME", t.TempDir())
			if tc.home != "" {
				t.Setenv("WK_HOME", tc.home)
			}
			feed(t, tc.input)
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != exitOK || stdout.Len() != 0 {
				t.Fatalf("exit %d, stdout %q, stderr %q", code, stdout.String(), stderr.String())
			}
			if _, err := os.Stat("rel"); !os.IsNotExist(err) {
				t.Error("a relative WK_HOME made a directory in the working directory")
			}
			_, err := os.Stat(filepath.Join(os.Getenv("WK_HOME"), "slots", "errors.log"))
			if logged := err == nil; logged != tc.wantLogInHome {
				t.Errorf("errors.log in WK_HOME: %v, want %v", logged, tc.wantLogInHome)
			}
		})
	}
}
