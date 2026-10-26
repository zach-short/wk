package slot

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// The spellings are ezhomesteading's (SLOTS-PLAN.md section 3); they live here and in its
// slots file, never in wk's code (D19).
const matchTestSlotsTOML = `
[[slots.match]]
spelling = "next build"
class = "heavy"
trim_after = ["web/.next/cache"]

[[slots.match]]
spelling = "bun run build"
class = "heavy"
trim_after = ["web/.next/cache"]

[[slots.match]]
spelling = "bunx next build"
class = "heavy"
trim_after = ["web/.next/cache"]

[[slots.match]]
spelling = "bun run e2e"
class = "heavy"

[[slots.match]]
spelling = "playwright test"
class = "heavy"

[[slots.match]]
spelling = "bunx playwright test"
class = "heavy"

[[slots.match]]
spelling = "go run ./cmd/lintgate"
class = "heavy"

[[slots.match]]
spelling = "tsc"
class = "check"

[[slots.match]]
spelling = "bunx tsc"
class = "check"

[[slots.match]]
spelling = "eslint"
class = "check"

[[slots.match]]
spelling = "bunx eslint"
class = "check"

[[slots.match]]
spelling = "bun run lint"
class = "check"

[[slots.match]]
spelling = "vitest"
class = "check"

[[slots.match]]
spelling = "bunx vitest"
class = "check"

[[slots.match]]
spelling = "bun run test"
class = "check"

[[slots.match]]
spelling = "go test"
class = "check"

[hook]
fail_open = true
timeout_ms = 200
`

const matchTestExe = "/x/wk"

func matchTestSlots(t *testing.T) SlotsFile {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ezhomesteading.slots.toml")
	if err := os.WriteFile(path, []byte(matchTestSlotsTOML), 0o644); err != nil {
		t.Fatal(err)
	}
	sf, found, err := LoadSlotsFile(path)
	if err != nil || !found {
		t.Fatalf("LoadSlotsFile: found=%v err=%v", found, err)
	}
	return sf
}

type matchTestCase struct {
	name   string
	in     string
	want   string
	missed []Missed
}

var matchTestRewrites = []matchTestCase{
	{"next build", `next build`,
		`/x/wk run --slot heavy --trim web/.next/cache -- next build`, nil},
	{"bun run build", `bun run build`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bun run build`, nil},
	{"bunx next build", `bunx next build`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bunx next build`, nil},
	{"bun run e2e has no trim", `bun run e2e`,
		`/x/wk run --slot heavy -- bun run e2e`, nil},
	{"playwright test", `playwright test --project=chromium`,
		`/x/wk run --slot heavy -- playwright test --project=chromium`, nil},
	{"bunx playwright test", `bunx playwright test`,
		`/x/wk run --slot heavy -- bunx playwright test`, nil},
	{"lintgate", `go run ./cmd/lintgate`,
		`/x/wk run --slot heavy -- go run ./cmd/lintgate`, nil},
	{"tsc", `tsc --noEmit`,
		`/x/wk run --slot check -- tsc --noEmit`, nil},
	{"bunx tsc", `bunx tsc --noEmit`,
		`/x/wk run --slot check -- bunx tsc --noEmit`, nil},
	{"eslint", `eslint .`,
		`/x/wk run --slot check -- eslint .`, nil},
	{"bunx eslint", `bunx eslint . --max-warnings 41`,
		`/x/wk run --slot check -- bunx eslint . --max-warnings 41`, nil},
	{"bun run lint", `bun run lint`,
		`/x/wk run --slot check -- bun run lint`, nil},
	{"vitest", `vitest run`,
		`/x/wk run --slot check -- vitest run`, nil},
	{"bunx vitest", `bunx vitest run src/catalog`,
		`/x/wk run --slot check -- bunx vitest run src/catalog`, nil},
	{"bun run test", `bun run test`,
		`/x/wk run --slot check -- bun run test`, nil},
	{"go test with arguments", `go test -race -count=1 ./...`,
		`/x/wk run --slot check -- go test -race -count=1 ./...`, nil},
	{"arguments after a build", `bun run build --debug`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bun run build --debug`, nil},
	{"expansions in the arguments", `go test $PKGS "${FLAGS[@]}" -run 'Test.*'`,
		`/x/wk run --slot check -- go test $PKGS "${FLAGS[@]}" -run 'Test.*'`, nil},
	{"VAR=$(mktemp -d) before a spelling", `GOLANGCI_LINT_CACHE=$(mktemp -d) go run ./cmd/lintgate`,
		`GOLANGCI_LINT_CACHE=$(mktemp -d) /x/wk run --slot heavy -- go run ./cmd/lintgate`, nil},
	{"two assignments", `CI=1 GOTOOLCHAIN=local go test ./...`,
		`CI=1 GOTOOLCHAIN=local /x/wk run --slot check -- go test ./...`, nil},
	{"the web gate line, &&", `cd web && bunx tsc --noEmit && bun run lint && bun run build`,
		`cd web && /x/wk run --slot check -- bunx tsc --noEmit && /x/wk run --slot check -- bun run lint && /x/wk run --slot heavy --trim web/.next/cache -- bun run build`, nil},
	{"||", `bun run test || echo failed`,
		`/x/wk run --slot check -- bun run test || echo failed`, nil},
	{";", `cd backend; go test ./...`,
		`cd backend; /x/wk run --slot check -- go test ./...`, nil},
	{"|", `bun run build 2>&1 | tail -20`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bun run build 2>&1 | tail -20`, nil},
	{"a grep that drops wk's waiting lines", `cd web && bunx tsc --noEmit 2>&1 | grep -v '^wk run' | tail -5`,
		`cd web && /x/wk run --slot check -- bunx tsc --noEmit 2>&1 | grep -v '^wk run' | tail -5`, nil},
	{"an rg pattern naming wk run", `go test ./... 2>&1 | /usr/bin/rg -v "wk run:"`,
		`/x/wk run --slot check -- go test ./... 2>&1 | /usr/bin/rg -v "wk run:"`, nil},
	{"|&", `go test ./... |& tee test.log`,
		`/x/wk run --slot check -- go test ./... |& tee test.log`, nil},
	{"!", `! bun run lint`,
		`! /x/wk run --slot check -- bun run lint`, nil},
	{"&", `bun run build & wait`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bun run build & wait`, nil},
	{"newlines", "cd web\nbun run lint\n\tbun run build\n",
		"cd web\n/x/wk run --slot check -- bun run lint\n\t/x/wk run --slot heavy --trim web/.next/cache -- bun run build\n", nil},
	{"( )", `(cd web && bun run build)`,
		`(cd web && /x/wk run --slot heavy --trim web/.next/cache -- bun run build)`, nil},
	{"{ }", `{ bun run lint; bun run test; }`,
		`{ /x/wk run --slot check -- bun run lint; /x/wk run --slot check -- bun run test; }`, nil},
	{"if, elif and else", `if bun run test; then echo ok; elif go test ./...; then :; else bun run lint; fi`,
		`if /x/wk run --slot check -- bun run test; then echo ok; elif /x/wk run --slot check -- go test ./...; then :; else /x/wk run --slot check -- bun run lint; fi`, nil},
	{"while", `while ! bun run test; do sleep 1; done`,
		`while ! /x/wk run --slot check -- bun run test; do sleep 1; done`, nil},
	{"until", `until go test ./...; do sleep 1; done`,
		`until /x/wk run --slot check -- go test ./...; do sleep 1; done`, nil},
	{"for", `for d in web native; do (cd $d && bunx tsc --noEmit); done`,
		`for d in web native; do (cd $d && /x/wk run --slot check -- bunx tsc --noEmit); done`, nil},
	{"case", `case $X in web) bun run build ;; *) go test ./... ;; esac`,
		`case $X in web) /x/wk run --slot heavy --trim web/.next/cache -- bun run build ;; *) /x/wk run --slot check -- go test ./... ;; esac`, nil},
	{"nested positions", `if true; then (cd web && { bun run lint || bun run build; }); fi`,
		`if true; then (cd web && { /x/wk run --slot check -- bun run lint || /x/wk run --slot heavy --trim web/.next/cache -- bun run build; }); fi`, nil},
	{"a redirection before the command", `>build.log 2>&1 bun run build`,
		`>build.log 2>&1 /x/wk run --slot heavy --trim web/.next/cache -- bun run build`, nil},
	{"a comment is kept", `bun run build # then lint`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bun run build # then lint`, nil},
	{"a comment that mentions wk run", "bun run build # see wk run\ngo test ./...",
		"/x/wk run --slot heavy --trim web/.next/cache -- bun run build # see wk run\n/x/wk run --slot check -- go test ./...", nil},
	{"multi-byte characters before the insertion", `echo "café ☕ — ok" && bun run build`,
		`echo "café ☕ — ok" && /x/wk run --slot heavy --trim web/.next/cache -- bun run build`, nil},
	{"a here-document after the command", "cat <<EOF && bun run build\nhello\nEOF\ngo test ./...",
		"cat <<EOF && /x/wk run --slot heavy --trim web/.next/cache -- bun run build\nhello\nEOF\n/x/wk run --slot check -- go test ./...", nil},
	{"a here-document into the command", "bun run test <<'EOF'\nbun run build\nEOF",
		"/x/wk run --slot check -- bun run test <<'EOF'\nbun run build\nEOF", nil},
	{"a C-style for", `for ((i=0; i<2; i++)); do go test ./...; done`,
		`for ((i=0; i<2; i++)); do /x/wk run --slot check -- go test ./...; done`, nil},
	{"extra spaces between the words", "bun  run \tbuild",
		"/x/wk run --slot heavy --trim web/.next/cache -- bun  run \tbuild", nil},
}

// The cases for $( ), backticks, bash -c, sh -ec, time and env that stood here until
// 2026-10-26 encoded the refusals match_wrappers.go lifts; each moved to
// match_wrappers_test.go with its rewritten form.
var matchTestUntouched = []matchTestCase{
	{"<( )", `diff <(bun run lint) lint.txt`, `diff <(bun run lint) lint.txt`,
		[]Missed{{"bun run lint", "inside a process substitution"}}},
	{"eval", `eval "bun run lint"`, `eval "bun run lint"`,
		[]Missed{{`eval "bun run lint"`, "inside eval"}}},
	{"a function body", `f() { bun run build; }; f`, `f() { bun run build; }; f`,
		[]Missed{{"bun run build", "inside a function body"}}},
	{"a quoted first word", `"bun" run build`, `"bun" run build`,
		[]Missed{{`"bun" run build`, "quoted spelling"}}},
	{"a quoted later word", `bun run 'build'`, `bun run 'build'`,
		[]Missed{{`bun run 'build'`, "quoted spelling"}}},
	{"an escaped word", `bun run buil\d`, `bun run buil\d`,
		[]Missed{{`bun run buil\d`, "quoted spelling"}}},
	{"a line that already holds wk run", `/x/wk run --slot check -- bun run lint && bun run build`,
		`/x/wk run --slot check -- bun run lint && bun run build`,
		[]Missed{{"bun run build", "the line already holds wk run"}}},
	{"a hand-typed wk run", `wk run -- bun run build; go test ./...`, `wk run -- bun run build; go test ./...`,
		[]Missed{{"go test ./...", "the line already holds wk run"}}},
	{"Rewrite's own output", `cd web && /x/wk run --slot check -- bunx tsc --noEmit && /x/wk run --slot heavy --trim web/.next/cache -- bun run build`,
		`cd web && /x/wk run --slot check -- bunx tsc --noEmit && /x/wk run --slot heavy --trim web/.next/cache -- bun run build`, nil},
	{"not a spelling", `bun run build:prod && go testing`, `bun run build:prod && go testing`, nil},
	{"too few words", `bun run`, `bun run`, nil},
	{"a spelling that is not the first word", `echo bun run build; xargs go test`, `echo bun run build; xargs go test`,
		[]Missed{{"xargs go test", "after the wrapper xargs"}}},
	{"spellings behind wrapper words", `command -v tsc && exec go test ./...`, `command -v tsc && exec go test ./...`,
		[]Missed{{"command -v tsc", "after the wrapper command"}, {"exec go test ./...", "after the wrapper exec"}}},
	{"a spelling behind a wrapper with options", `nice -n 19 bun run build`, `nice -n 19 bun run build`,
		[]Missed{{"nice -n 19 bun run build", "after the wrapper nice"}}},
	{"wk run before ;", `wk run; bun run build`, `wk run; bun run build`,
		[]Missed{{"bun run build", "the line already holds wk run"}}},
	{"wk run as two words of another command", `ls ~/Projects/wk run; go test ./...`, `ls ~/Projects/wk run; go test ./...`,
		[]Missed{{"go test ./...", "the line already holds wk run"}}},
	{"wk run before a closing quote", `echo "use wk run" && bun run build`, `echo "use wk run" && bun run build`,
		[]Missed{{"bun run build", "the line already holds wk run"}}},
	{"a grep beside a real wk run", `grep -v '^wk run' log; wk run -- go test ./...; bun run build`,
		`grep -v '^wk run' log; wk run -- go test ./...; bun run build`,
		[]Missed{{"bun run build", "the line already holds wk run"}}},
	{"wk run quoted for a shell, not a filter", `awk '{ system("wk run x") }' f; bun run build`,
		`awk '{ system("wk run x") }' f; bun run build`,
		[]Missed{{"bun run build", "the line already holds wk run"}}},
	{"assignments only", `FOO=bar; echo $(X=1)`, `FOO=bar; echo $(X=1)`, nil},
}

func TestMatchRewrite(t *testing.T) {
	sf := matchTestSlots(t)
	cases := append(append([]matchTestCase{}, matchTestRewrites...), matchTestUntouched...)
	if len(cases) < 40 {
		t.Fatalf("the table holds %d command lines; phase 2 step 6 asks for at least 40", len(cases))
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, changed, missed, err := Rewrite(tc.in, matchTestExe, sf)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			if out != tc.want || changed != (tc.want != tc.in) {
				t.Errorf("Rewrite(%q)\n got %q changed=%v\nwant %q", tc.in, out, changed, tc.want)
			}
			if !reflect.DeepEqual(missed, tc.missed) {
				t.Errorf("missed\n got %#v\nwant %#v", missed, tc.missed)
			}
		})
	}
}

func TestMatchRewriteUnparsed(t *testing.T) {
	sf := matchTestSlots(t)
	for _, tc := range []matchTestCase{
		{"a spelling in text that does not parse", `bun run build && (`, `bun run build && (`,
			[]Missed{{`bun run build && (`, "does not parse: 1:18: reached EOF without matching ( with )"}}},
		{"no spelling in text that does not parse", `echo "unterminated`, `echo "unterminated`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, changed, missed, err := Rewrite(tc.in, matchTestExe, sf)
			if err == nil || out != tc.in || changed {
				t.Fatalf("Rewrite(%q) = %q, changed=%v, err=%v; want the text back and a parse error", tc.in, out, changed, err)
			}
			if !reflect.DeepEqual(missed, tc.missed) {
				t.Errorf("missed\n got %#v\nwant %#v", missed, tc.missed)
			}
		})
	}
}

func TestMatchRewriteMultiByteOffsets(t *testing.T) {
	sf := matchTestSlots(t)
	in := "echo 日本語 && go test ./..."
	want := "echo 日本語 && /x/wk run --slot check -- go test ./..."
	out, _, _, err := Rewrite(in, matchTestExe, sf)
	if err != nil || out != want {
		t.Fatalf("got %q, %v; want %q", out, err, want)
	}
	if out[:len("echo 日本語 && ")] != in[:len("echo 日本語 && ")] || out[len(out)-len("go test ./..."):] != "go test ./..." {
		t.Errorf("bytes outside the insertion changed: %q", out)
	}
}

func TestMatchRewriteQuotesTheWKPath(t *testing.T) {
	sf := matchTestSlots(t)
	out, changed, _, err := Rewrite("go test ./...", "/Users/a b/wk", sf)
	want := "'/Users/a b/wk' run --slot check -- go test ./..."
	if err != nil || !changed || out != want {
		t.Fatalf("got %q, changed=%v, err=%v; want %q", out, changed, err, want)
	}
	again, changed, _, _ := Rewrite(out, "/Users/a b/wk", sf)
	if changed || again != out {
		t.Errorf("a second Rewrite changed its own output: %q", again)
	}
}

func TestMatchRewriteNoEntries(t *testing.T) {
	out, changed, missed, err := Rewrite("bun run build", matchTestExe, SlotsFile{Hook: slotsFileHookDefaults()})
	if out != "bun run build" || changed || missed != nil || err != nil {
		t.Fatalf("got %q, %v, %v, %v; want the command back untouched", out, changed, missed, err)
	}
}
