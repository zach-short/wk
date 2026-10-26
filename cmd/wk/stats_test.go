package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStatsRefusesBadArguments(t *testing.T) {
	isolated(t)
	for name, tc := range map[string]struct {
		args []string
		why  string
	}{
		"unknown flag":     {[]string{"stats", "--yaml"}, `unexpected argument to wk stats: "--yaml"`},
		"--since no value": {[]string{"stats", "--since"}, `unexpected argument to wk stats: "--since"`},
		"--json twice":     {[]string{"stats", "--json", "--json"}, `unexpected argument to wk stats: "--json"`},
		"--until twice":    {[]string{"stats", "--until", "2026-10-26", "--until", "2026-10-27"}, `"--until"`},
		"a bad date":       {[]string{"stats", "--since", "yesterday"}, `--since "yesterday" is not a YYYY-MM-DD date`},
	} {
		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := run(tc.args, &stdout, &stderr); code != exitUsage {
				t.Fatalf("exit %d, want %d\n%s", code, exitUsage, stderr.String())
			}
			if stdout.Len() != 0 || !strings.Contains(stderr.String(), tc.why) || !strings.Contains(stderr.String(), "wk stats [--since") {
				t.Fatalf("stdout %q, stderr %q: want nothing, then %q and the usage", stdout.String(), stderr.String(), tc.why)
			}
		})
	}
}

// With a run in the fixture's runs.log and no machine file, wk stats reads the default slots
// directory of WK_HOME and prints one JSON document.
func TestStatsPrintsJSONFromWKHome(t *testing.T) {
	isolated(t)
	slots := filepath.Join(os.Getenv("WK_HOME"), "slots")
	if err := os.MkdirAll(slots, 0o755); err != nil {
		t.Fatal(err)
	}
	line := "2026-10-20T10:00:00-04:00\tclass=check\tpid=1\texit=SIGTERM\twait=5.0s\trun=1.0s\tdir=/w/backend\tcmd=go test ./...\n"
	if err := os.WriteFile(filepath.Join(slots, "runs.log"), []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"stats", "--since", "2026-10-17", "--json"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("exit %d\n%s", code, stderr.String())
	}
	var doc struct {
		Slots struct {
			Since   string
			Runs    int
			Classes []struct {
				Name   string
				Failed int
			}
		}
		Warm []any
	}
	if err := json.Unmarshal(stdout.Bytes(), &doc); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if doc.Slots.Since != "2026-10-17" || doc.Slots.Runs != 1 || len(doc.Slots.Classes) != 1 || doc.Slots.Classes[0].Failed != 1 || doc.Warm == nil {
		t.Fatalf("document %+v, want one failed check run and an empty warm list", doc)
	}
}
