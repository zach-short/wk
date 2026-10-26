package slot

import (
	"reflect"
	"testing"
)

// The four forms missed.log held most from 2026-10-17 to 2026-10-25 in ezhomesteading: 14 lines
// "inside time", 11 "inside $( )", 9 "after the wrapper env" and 5 "inside sh -c". Several cases
// are those lines, shortened.
var matchTestWrapperRewrites = []matchTestCase{
	// time: the gate runs as `time <wk> run ...`, so `time` also counts the wait for a place,
	// which wk run reports on stderr. Claude Code strips `time` before it matches a permission
	// rule, so the wrapped command still meets the gate's twin.
	{"time", `time bun run build`,
		`time /x/wk run --slot heavy --trim web/.next/cache -- bun run build`, nil},
	{"time -p", `time -p go test ./...`,
		`time -p /x/wk run --slot check -- go test ./...`, nil},
	{"time after cd, from missed.log", `cd web && time bunx tsc --noEmit; echo EXIT=$?`,
		`cd web && time /x/wk run --slot check -- bunx tsc --noEmit; echo EXIT=$?`, nil},
	{"time over a pipeline", `time bun run lint 2>&1 | tail -15`,
		`time /x/wk run --slot check -- bun run lint 2>&1 | tail -15`, nil},
	{"time over a loop, from missed.log", `time (for p in core api; do (cd packages/$p && bun run lint >/dev/null 2>&1); done)`,
		`time (for p in core api; do (cd packages/$p && /x/wk run --slot check -- bun run lint >/dev/null 2>&1); done)`, nil},
	{"time with an assignment, from missed.log", `time GOLANGCI_LINT_CACHE=$(mktemp -d) go run ./cmd/lintgate > $TMPDIR/lintgate.log 2>&1`,
		`time GOLANGCI_LINT_CACHE=$(mktemp -d) /x/wk run --slot heavy -- go run ./cmd/lintgate > $TMPDIR/lintgate.log 2>&1`, nil},

	// env: the insertion goes after env's options and assignments, so wk run sees the same
	// environment as the gate and runs.log names the gate.
	{"env -u, from missed.log", `cd backend && env -u EZH_TEST_DB go test -race -count=1 ./... 2>&1 | tail -20`,
		`cd backend && env -u EZH_TEST_DB /x/wk run --slot check -- go test -race -count=1 ./... 2>&1 | tail -20`, nil},
	{"env with an assignment", `env GOTOOLCHAIN=local go test ./...`,
		`env GOTOOLCHAIN=local /x/wk run --slot check -- go test ./...`, nil},
	{"env by path, -uNAME, --unset=, --, two assignments", `/usr/bin/env -uA --unset=B -- C=1 D="x y" bun run build`,
		`/usr/bin/env -uA --unset=B -- C=1 D="x y" /x/wk run --slot heavy --trim web/.next/cache -- bun run build`, nil},
	{"an assignment before env", `CI=1 env -u X go test ./...`,
		`CI=1 env -u X /x/wk run --slot check -- go test ./...`, nil},

	// Row 25: the time utility, `command`, and timeout. The insertion goes after the wrapper's
	// words, as it does after env's. The timeout cases are the four "after the wrapper timeout"
	// lines of ezhomesteading's missed.log from 2026-10-17 to 2026-10-26, shortened.
	{"/usr/bin/time -p, the form row 22 left open", `/usr/bin/time -p go test ./...`,
		`/usr/bin/time -p /x/wk run --slot check -- go test ./...`, nil},
	{"/usr/bin/time -l on macOS", `cd web && /usr/bin/time -l bun run build 2>time.txt`,
		`cd web && /usr/bin/time -l /x/wk run --slot heavy --trim web/.next/cache -- bun run build 2>time.txt`, nil},
	{"/usr/bin/time -o FILE -a", `/usr/bin/time -o /tmp/t.txt -a go test ./...`,
		`/usr/bin/time -o /tmp/t.txt -a /x/wk run --slot check -- go test ./...`, nil},
	{"time flags in one cluster, the last taking the next word", `/usr/bin/time -ao t.txt go test ./...`,
		`/usr/bin/time -ao t.txt /x/wk run --slot check -- go test ./...`, nil},
	{"-o with its file attached", `/usr/bin/time -ot.txt go test ./...`,
		`/usr/bin/time -ot.txt /x/wk run --slot check -- go test ./...`, nil},
	{"GNU time -v -f FORMAT", `gtime -v -f '%e %M' go test ./...`,
		`gtime -v -f '%e %M' /x/wk run --slot check -- go test ./...`, nil},
	{"GNU time long options and --", `/usr/bin/time --verbose --output=t.txt --format '%e' -- go test ./...`,
		`/usr/bin/time --verbose --output=t.txt --format '%e' -- /x/wk run --slot check -- go test ./...`, nil},
	{"command time", `command time -l go test ./...`,
		`command time -l /x/wk run --slot check -- go test ./...`, nil},
	{"command with --", `command -- go test ./...`,
		`command -- /x/wk run --slot check -- go test ./...`, nil},
	{"time after an assignment is the utility", `CI=1 time -p go test ./...`,
		`CI=1 time -p /x/wk run --slot check -- go test ./...`, nil},
	{"timeout, from missed.log", `cd web && timeout 300 bun run lint -- --file a.tsx 2>&1 | tail -20`,
		`cd web && timeout 300 /x/wk run --slot check -- bun run lint -- --file a.tsx 2>&1 | tail -20`, nil},
	{"timeout twice on one line, from missed.log", `timeout 280 bunx eslint a.tsx 2>&1 | tail -15; cd ../native && timeout 200 bunx eslint b.tsx`,
		`timeout 280 /x/wk run --slot check -- bunx eslint a.tsx 2>&1 | tail -15; cd ../native && timeout 200 /x/wk run --slot check -- bunx eslint b.tsx`, nil},
	{"timeout -s -k with units", `timeout -s KILL -k 5s 2m go test ./...`,
		`timeout -s KILL -k 5s 2m /x/wk run --slot check -- go test ./...`, nil},
	{"timeout long options and a fraction", `timeout --signal=INT --kill-after 10 --preserve-status 1.5m go test ./...`,
		`timeout --signal=INT --kill-after 10 --preserve-status 1.5m /x/wk run --slot check -- go test ./...`, nil},
	{"timeout -sINT -v -p and --", `gtimeout -sINT -v -p -- 30 go test ./...`,
		`gtimeout -sINT -v -p -- 30 /x/wk run --slot check -- go test ./...`, nil},
	{"a chain of wrappers", `timeout 600 env -u X /usr/bin/time -p go test ./...`,
		`timeout 600 env -u X /usr/bin/time -p /x/wk run --slot check -- go test ./...`, nil},
	{"the time keyword over timeout", `time timeout 300 go test ./...`,
		`time timeout 300 /x/wk run --slot check -- go test ./...`, nil},
	{"timeout in $( ) that leaves stderr out", `n=$(timeout 60 go test ./... | wc -l)`,
		`n=$(timeout 60 /x/wk run --slot check -- go test ./... | wc -l)`, nil},
	{"/usr/bin/time in a loop under the time keyword", `time (for p in a b; do /usr/bin/time -p go test ./$p; done)`,
		`time (for p in a b; do /usr/bin/time -p /x/wk run --slot check -- go test ./$p; done)`, nil},

	// $( ) and backticks: the insertion sits at the start of the substituted command. wk run
	// writes its waiting, place and trim lines to stderr and nothing to stdout, so the captured
	// text is the same while stderr stays out of the capture; a substitution that sends stderr
	// into it is a miss (matchTestWrapperUntouched).
	{"$( )", `echo $(bun run build)`,
		`echo $(/x/wk run --slot heavy --trim web/.next/cache -- bun run build)`, nil},
	{"backticks", "echo `bun run test`",
		"echo `/x/wk run --slot check -- bun run test`", nil},
	{"$( ) in an assignment", `OUT=$(go test ./...) && echo "$OUT"`,
		`OUT=$(/x/wk run --slot check -- go test ./...) && echo "$OUT"`, nil},
	{"$( ) in a parameter default", `echo ${X:-$(bun run lint)}`,
		`echo ${X:-$(/x/wk run --slot check -- bun run lint)}`, nil},
	{"$( ) in a test clause", `[[ -n $(tsc --version) ]]`,
		`[[ -n $(/x/wk run --slot check -- tsc --version) ]]`, nil},
	{"$( ) in a here-document", "cat <<EOF\n$(bun run build)\nEOF",
		"cat <<EOF\n$(/x/wk run --slot heavy --trim web/.next/cache -- bun run build)\nEOF", nil},
	{"$( ) in double quotes, from missed.log", `echo "after SKIP: $(go test -v ./services/ | grep -c -- '--- SKIP')"`,
		`echo "after SKIP: $(/x/wk run --slot check -- go test -v ./services/ | grep -c -- '--- SKIP')"`, nil},
	{"$( ) inside $( )", `echo $(echo $(cd web && bunx tsc --noEmit | wc -l))`,
		`echo $(echo $(cd web && /x/wk run --slot check -- bunx tsc --noEmit | wc -l))`, nil},
	{"$( ) beside a top-level gate", `bun run build && echo "lint: $(bun run lint | tail -1)"`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bun run build && echo "lint: $(/x/wk run --slot check -- bun run lint | tail -1)"`, nil},
	{"$( ) as a gate's argument", `go test $(go list ./... | grep -v vendor)`,
		`/x/wk run --slot check -- go test $(go list ./... | grep -v vendor)`, nil},
	{"env and time inside $( )", `n=$(time env -u X go test ./... | wc -l)`,
		`n=$(time env -u X /x/wk run --slot check -- go test ./... | wc -l)`, nil},
	{"$( ) with stderr sent elsewhere", `echo "$(go test ./... 2>/dev/null)"`,
		`echo "$(/x/wk run --slot check -- go test ./... 2>/dev/null)"`, nil},
	{"$( ) with stderr closed", `v=$(go test ./... 2>&-)`,
		`v=$(/x/wk run --slot check -- go test ./... 2>&-)`, nil},
	{"2>&1 on the outer statement, not in the capture", `OUT=$(go test ./...) 2>&1`,
		`OUT=$(/x/wk run --slot check -- go test ./...) 2>&1`, nil},

	// Row 22 audit 3: >&word with no fd number dups stdout only when word is an fd.
	{">&1 with no fd number dups stdout to itself", `v=$(go test ./... >&1)`,
		`v=$(/x/wk run --slot check -- go test ./... >&1)`, nil},
	{">&2 with no fd number sends stdout to stderr", `v=$(go test ./... >&2)`,
		`v=$(/x/wk run --slot check -- go test ./... >&2)`, nil},
	{">&2- moves stderr onto stdout's fd and closes 2", `v=$(go test ./... >&2-)`,
		`v=$(/x/wk run --slot check -- go test ./... >&2-)`, nil},
	{"stderr to /dev/null spelled with a dot", `v=$(go test ./... 2>/dev/./null)`,
		`v=$(/x/wk run --slot check -- go test ./... 2>/dev/./null)`, nil},
	{"1>&2 sends stdout out of the capture and leaves stderr", `v=$(go test ./... 1>&2)`,
		`v=$(/x/wk run --slot check -- go test ./... 1>&2)`, nil},
	{"01>&2 is fd 1 as 1>&2 is", `v=$(go test ./... 01>&2)`,
		`v=$(/x/wk run --slot check -- go test ./... 01>&2)`, nil},

	// sh -c: the whole call is wrapped, so one place holds every gate its string runs. Only when
	// the string is plain text, parses, and every spelling in it has one class.
	{"sh -c", `sh -c 'cd web && bun run build'`,
		`/x/wk run --slot heavy --trim web/.next/cache -- sh -c 'cd web && bun run build'`, nil},
	{"bash -c in double quotes", `bash -c "cd web && bun run build"`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bash -c "cd web && bun run build"`, nil},
	{"sh -ec by path", `/bin/sh -ec 'go test ./...'`,
		`/x/wk run --slot check -- /bin/sh -ec 'go test ./...'`, nil},
	{"bash -o pipefail -c", `bash -o pipefail -c 'go test ./... | tail -5'`,
		`/x/wk run --slot check -- bash -o pipefail -c 'go test ./... | tail -5'`, nil},
	{"wk's gate line, from missed.log", `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`,
		`GOTOOLCHAIN=local /x/wk run --slot check -- sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, nil},
	{"two spellings of one class and one trim", `bash -c 'bun run build && next build'`,
		`/x/wk run --slot heavy --trim web/.next/cache -- bash -c 'bun run build && next build'`, nil},
	{"sh -c with its own arguments", `sh -c 'go test "$@"' sh ./a ./b`,
		`/x/wk run --slot check -- sh -c 'go test "$@"' sh ./a ./b`, nil},
}

var matchTestWrapperUntouched = []matchTestCase{
	{"time with no gate", `time echo hi`, `time echo hi`, nil},
	{"coproc", `coproc bun run build`, `coproc bun run build`,
		[]Missed{{"bun run build", "inside coproc"}}},
	{"env -i would empty wk run's environment", `env -i PATH=/usr/bin go test ./...`, `env -i PATH=/usr/bin go test ./...`,
		[]Missed{{"env -i PATH=/usr/bin go test ./...", "after the wrapper env"}}},
	{"env HOME= would move wk's home", `env HOME=/tmp go test ./...`, `env HOME=/tmp go test ./...`,
		[]Missed{{"env HOME=/tmp go test ./...", "after the wrapper env"}}},
	{"env -u of a WK_ variable", `env -u WK_SLOT_HELD go test ./...`, `env -u WK_SLOT_HELD go test ./...`,
		[]Missed{{"env -u WK_SLOT_HELD go test ./...", "after the wrapper env"}}},
	{"env with an option wk does not read", `env -C backend go test ./...`, `env -C backend go test ./...`,
		[]Missed{{"env -C backend go test ./...", "after the wrapper env"}}},
	{"env with a quoted spelling", `env -u X "go" test ./...`, `env -u X "go" test ./...`,
		[]Missed{{`env -u X "go" test ./...`, "quoted spelling"}}},
	{"env alone", `env | grep GO`, `env | grep GO`, nil},
	{"env PATH= would change the PATH wk run sees", `env PATH=/usr/bin go test ./...`, `env PATH=/usr/bin go test ./...`,
		[]Missed{{"env PATH=/usr/bin go test ./...", "after the wrapper env"}}},
	{"env -u PATH", `env -u PATH go test ./...`, `env -u PATH go test ./...`,
		[]Missed{{"env -u PATH go test ./...", "after the wrapper env"}}},

	// Row 25: wrapper words wk does not read stay a miss.
	{"timeout --foreground signals wk run alone", `timeout --foreground -k 5 300 go test ./...`, `timeout --foreground -k 5 300 go test ./...`,
		[]Missed{{"timeout --foreground -k 5 300 go test ./...", "after the wrapper timeout"}}},
	{"timeout -f is --foreground", `timeout -f 300 go test ./...`, `timeout -f 300 go test ./...`,
		[]Missed{{"timeout -f 300 go test ./...", "after the wrapper timeout"}}},
	{"timeout with a duration wk does not read", `timeout "$T" go test ./...`, `timeout "$T" go test ./...`,
		[]Missed{{`timeout "$T" go test ./...`, "after the wrapper timeout"}}},
	{"timeout before a wrapper wk does not read", `timeout 300 nice go test ./...`, `timeout 300 nice go test ./...`,
		[]Missed{{"timeout 300 nice go test ./...", "after the wrapper timeout"}}},
	{"timeout with no gate", `timeout 5 curl -s localhost`, `timeout 5 curl -s localhost`, nil},
	{"/usr/bin/time with an option wk does not read", `/usr/bin/time -x go test ./...`, `/usr/bin/time -x go test ./...`,
		[]Missed{{"/usr/bin/time -x go test ./...", "after the wrapper time"}}},
	{"/usr/bin/time with a quoted spelling", `/usr/bin/time -p "go" test ./...`, `/usr/bin/time -p "go" test ./...`,
		[]Missed{{`/usr/bin/time -p "go" test ./...`, "quoted spelling"}}},
	{"command -p looks the gate up in another PATH", `command -p go test ./...`, `command -p go test ./...`,
		[]Missed{{"command -p go test ./...", "after the wrapper command"}}},
	{"/usr/bin/time in $( ) that captures stderr", `v=$(/usr/bin/time -p go test ./... 2>&1)`, `v=$(/usr/bin/time -p go test ./... 2>&1)`,
		[]Missed{{"/usr/bin/time -p go test ./...", "inside $( ) that captures stderr"}}},
	{"timeout on a line that holds wk run", `timeout 300 go test ./...; wk run -- bun run lint`, `timeout 300 go test ./...; wk run -- bun run lint`,
		[]Missed{{"timeout 300 go test ./...", "the line already holds wk run"}}},

	// Row 22 audit B2: wk run's stderr lines would enter a capture that takes stderr.
	{"2>&1 on each command in $( )", `test -z "$(go vet ./... 2>&1; go test ./... 2>&1)"`, `test -z "$(go vet ./... 2>&1; go test ./... 2>&1)"`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"2>&1 in backticks", "echo `go test ./... 2>&1`", "echo `go test ./... 2>&1`",
		[]Missed{{"go test ./...", "inside backticks that capture stderr"}}},
	{"a clean check on captured stderr", `[ -z "$(bun run lint 2>&1)" ] && echo clean`, `[ -z "$(bun run lint 2>&1)" ] && echo clean`,
		[]Missed{{"bun run lint", "inside $( ) that captures stderr"}}},
	{"2>&1 on a block in $( )", `x=$({ go test ./...; } 2>&1)`, `x=$({ go test ./...; } 2>&1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"|& in $( )", `x=$(go test ./... |& tail -3)`, `x=$(go test ./... |& tail -3)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"&> in $( )", `x=$(go test ./... &>/dev/stdout)`, `x=$(go test ./... &>/dev/stdout)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"an outer capture of stderr reaches an inner $( )", `x=$(echo $(go test ./...) 2>&1)`, `x=$(echo $(go test ./...) 2>&1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},

	// Row 22 second audit A: stderr reaches the capture through another fd, a dup the other way,
	// a move, a quoted target or a process substitution.
	{"stderr through fd 3 after the command", `v=$(go test ./... 3>&1 2>&3)`, `v=$(go test ./... 3>&1 2>&3)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"stderr through fd 3 before the command", `v=$(3>&1 2>&3 go test ./...)`, `v=$(3>&1 2>&3 go test ./...)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"exec dups fd 3 first", `v=$(exec 3>&1; go test ./... 2>&3)`, `v=$(exec 3>&1; go test ./... 2>&3)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"2<&1", `v=$(go test ./... 2<&1)`, `v=$(go test ./... 2<&1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"2>&1- moves the fd", `v=$(go test ./... 2>&1-)`, `v=$(go test ./... 2>&1-)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"a quoted dup target", `v=$(go test ./... 2>&"1")`, `v=$(go test ./... 2>&"1")`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"stderr into a process substitution", `v=$(go test ./... 2> >(cat))`, `v=$(go test ./... 2> >(cat))`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"stderr into tee", `v=$(go test ./... 2> >(tee err.log))`, `v=$(go test ./... 2> >(tee err.log))`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},

	// Row 22 audit 3: bash reads >&word with no fd number as &>word when word is not an fd, and
	// a target spelled with // or /./ is the same file.
	{">&/dev/stdout with no fd number", `v=$(go test ./... >&/dev/stdout)`, `v=$(go test ./... >&/dev/stdout)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{">&/dev/fd/1 with no fd number", `v=$(go test ./... >&/dev/fd/1)`, `v=$(go test ./... >&/dev/fd/1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{">&/dev/stderr counts as &> to anything does", `v=$(go test ./... >&/dev/stderr)`, `v=$(go test ./... >&/dev/stderr)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{">& to a word that is not plain text", `v=$(go test ./... >&"$out")`, `v=$(go test ./... >&"$out")`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{">&/dev/stdout in backticks", "v=`go test ./... >&/dev/stdout`", "v=`go test ./... >&/dev/stdout`",
		[]Missed{{"go test ./...", "inside backticks that capture stderr"}}},
	{"&>> in $( )", `v=$(go test ./... &>>/dev/stdout)`, `v=$(go test ./... &>>/dev/stdout)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"2>//dev/stdout", `v=$(go test ./... 2>//dev/stdout)`, `v=$(go test ./... 2>//dev/stdout)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"2>/dev/./stdout", `v=$(go test ./... 2>/dev/./stdout)`, `v=$(go test ./... 2>/dev/./stdout)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"2>/dev//fd/1", `v=$(go test ./... 2>/dev//fd/1)`, `v=$(go test ./... 2>/dev//fd/1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},

	// Row 22 audit 4: bash reads an fd number with leading zeros by its value.
	{"02>&1", `v=$(go test ./... 02>&1)`, `v=$(go test ./... 02>&1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"002>&1", `v=$(go test ./... 002>&1)`, `v=$(go test ./... 002>&1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"02>/dev/stdout", `v=$(go test ./... 02>/dev/stdout)`, `v=$(go test ./... 02>/dev/stdout)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"2>&01", `v=$(go test ./... 2>&01)`, `v=$(go test ./... 2>&01)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"<( ) stays", `diff <(bun run lint) lint.txt`, `diff <(bun run lint) lint.txt`,
		[]Missed{{"bun run lint", "inside a process substitution"}}},
	{"$( ) in a function body", `f() { echo $(go test ./...); }; f`, `f() { echo $(go test ./...); }; f`,
		[]Missed{{"go test ./...", "inside a function body"}}},
	{"$( ) on a line that holds wk run", `echo $(go test ./...); wk run -- bun run lint`, `echo $(go test ./...); wk run -- bun run lint`,
		[]Missed{{"go test ./...", "the line already holds wk run"}}},
	{"time on a line that holds wk run", `time go test ./...; wk run -- bun run lint`, `time go test ./...; wk run -- bun run lint`,
		[]Missed{{"go test ./...", "the line already holds wk run"}}},
	{"env on a line that holds wk run", `env -u X go test ./...; wk run -- bun run lint`, `env -u X go test ./...; wk run -- bun run lint`,
		[]Missed{{"env -u X go test ./...", "the line already holds wk run"}}},
	{"sh -c around a wk run", `sh -c 'wk run -- go test ./...'`, `sh -c 'wk run -- go test ./...'`,
		[]Missed{{`sh -c 'wk run -- go test ./...'`, "the line already holds wk run"}}},
	{"sh -c with two classes", `sh -c 'bun run lint && bun run build'`, `sh -c 'bun run lint && bun run build'`,
		[]Missed{{`sh -c 'bun run lint && bun run build'`, "inside sh -c: spellings of more than one class"}}},
	{"sh -c with an expansion", `sh -c "go test $PKG"`, `sh -c "go test $PKG"`,
		[]Missed{{`sh -c "go test $PKG"`, "inside sh -c: the string is not plain text"}}},
	{"sh -c that does not parse", `sh -c 'go test ./... && ('`, `sh -c 'go test ./... && ('`,
		[]Missed{{`sh -c 'go test ./... && ('`, "inside sh -c: the string does not parse"}}},
	{"bash with a long option", `bash --norc -c 'go test ./...'`, `bash --norc -c 'go test ./...'`,
		[]Missed{{`bash --norc -c 'go test ./...'`, "inside bash -c: an option wk does not read"}}},
	{"sh -c with no gate", `sh -c 'echo hi'`, `sh -c 'echo hi'`, nil},
	{"sh with a script", `sh build.sh go test`, `sh build.sh go test`, nil},
	{"eval stays", `eval "bun run lint"`, `eval "bun run lint"`,
		[]Missed{{`eval "bun run lint"`, "inside eval"}}},
	{"sh -c in a function body", `f() { sh -c 'go test ./...'; }`, `f() { sh -c 'go test ./...'; }`,
		[]Missed{{`sh -c 'go test ./...'`, "inside a function body"}}},
}

func TestMatchRewriteWrappers(t *testing.T) {
	sf := matchTestSlots(t)
	for _, tc := range append(append([]matchTestCase{}, matchTestWrapperRewrites...), matchTestWrapperUntouched...) {
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
			if again, changed, _, _ := Rewrite(out, matchTestExe, sf); changed {
				t.Errorf("a second Rewrite changed its own output: %q", again)
			}
		})
	}
}

// Inside backticks a backslash, a backtick or a dollar sign in the inserted path would be read
// again by the outer shell, so such a path is not inserted there; $( ) has no such second read.
func TestMatchRewriteBackticksRefuseAPathTheyWouldReread(t *testing.T) {
	sf := matchTestSlots(t)
	exe := "/x/a$b/wk"
	out, changed, missed, err := Rewrite("echo `go test ./...`", exe, sf)
	if err != nil || changed || out != "echo `go test ./...`" {
		t.Fatalf("got %q, changed=%v, err=%v; want the line back", out, changed, err)
	}
	if want := []Missed{{"go test ./...", "inside backticks"}}; !reflect.DeepEqual(missed, want) {
		t.Errorf("missed %#v; want %#v", missed, want)
	}
	out, changed, _, _ = Rewrite("echo $(go test ./...)", exe, sf)
	if want := "echo $('/x/a$b/wk' run --slot check -- go test ./...)"; !changed || out != want {
		t.Errorf("$( ): got %q; want %q", out, want)
	}
}
