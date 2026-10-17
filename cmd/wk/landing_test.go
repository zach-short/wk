package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/testrepo"
)

// wk submit prints the id alone on stdout; the landing commands refuse a bad argument list
// with a usage error, never a move.
func TestSubmitPrintsOnlyTheIDAndLandParsesItsFlags(t *testing.T) {
	f := testrepo.New(t)
	t.Setenv("WK_HOME", f.Home)
	cfg := filepath.Join(f.Home, "demo.toml")
	data, _ := os.ReadFile(cfg)
	if err := os.WriteFile(cfg, append(data, "\n[landing]\nenabled = true\n"...), 0o644); err != nil {
		t.Fatal(err)
	}
	ws := filepath.Join(f.Root, "ws")
	f.Git(t, "worktree", "add", "--quiet", "-b", "ws", ws)
	if err := os.WriteFile(filepath.Join(ws, "README.md"), []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"--repo", ws, "submit", "-m", "Change The Readme", "--row", "7"}, &stdout, &stderr); code != exitOK {
		t.Fatalf("submit: exit %d\n%s", code, stderr.String())
	}
	id := strings.TrimSuffix(stdout.String(), "\n")
	if len(id) != 12 || strings.Contains(id, "\n") {
		t.Fatalf("stdout = %q; want the id alone", stdout.String())
	}
	if !strings.Contains(stderr.String(), "wk: includes README.md") {
		t.Fatalf("stderr lacks the included paths:\n%s", stderr.String())
	}
	stdout.Reset()
	for _, args := range [][]string{{"land", "--loop"}, {"status"}, {"land"}} {
		if code := run(append([]string{"--repo", f.Repo}, args...), &stdout, &stderr); code != exitOK {
			t.Fatalf("%v: exit %d\n%s", args, code, stderr.String())
		}
	}
	for _, want := range []string{"landed " + id, id + "  landed", "row 7: change the readme", "fast-forward possible: wk land --apply --expect "} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("output lacks %q:\n%s", want, stdout.String())
		}
	}
	for _, args := range [][]string{
		{"submit"}, {"submit", "-m"}, {"submit", "-m", "--row"}, {"submit", "x"},
		{"land", "--apply"}, {"land", "--apply", "--expect"}, {"land", "--apply", "--expect", "-x"}, {"land", "--go"}, {"sync", "now"},
	} {
		if code := run(append([]string{"--repo", ws}, args...), &stdout, &stderr); code != exitUsage {
			t.Errorf("%v: exit %d; want usage", args, code)
		}
	}
}
