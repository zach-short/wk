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

	// Row 25: the time utility and `command`. The insertion goes after the wrapper's words, as it
	// does after env's. A gate behind timeout stays a miss (matchTestWrapperUntouched).
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
	{"a chain of wrappers", `command env -u X /usr/bin/time -p go test ./...`,
		`command env -u X /usr/bin/time -p /x/wk run --slot check -- go test ./...`, nil},
	{"/usr/bin/time in a loop under the time keyword", `time (for p in a b; do /usr/bin/time -p go test ./$p; done)`,
		`time (for p in a b; do /usr/bin/time -p /x/wk run --slot check -- go test ./$p; done)`, nil},

	// Row 27: a time option's argument is read when the shell passes it on as one word whatever
	// the variables and files hold; an env value stays one word unless an unquoted expansion
	// splits it.
	{"-o with a quoted file name", `/usr/bin/time -o "t 1.txt" go test ./...`,
		`/usr/bin/time -o "t 1.txt" /x/wk run --slot check -- go test ./...`, nil},
	{"-o with a tilde", `/usr/bin/time -o ~/t.txt go test ./...`,
		`/usr/bin/time -o ~/t.txt /x/wk run --slot check -- go test ./...`, nil},
	{"env with a quoted expansion", `env A="$X" go test ./...`,
		`env A="$X" /x/wk run --slot check -- go test ./...`, nil},
	{"env with an empty value", `env A= go test ./...`,
		`env A= /x/wk run --slot check -- go test ./...`, nil},
	{"env with a glob in the value, which only makes more A= words", `env A=x* go test ./...`,
		`env A=x* /x/wk run --slot check -- go test ./...`, nil},

	// Row 28, from the row 27 audit N1: a wrapper is read as a bare name or an absolute path.
	{"gtime by an absolute path", `/opt/homebrew/bin/gtime -p go test ./...`,
		`/opt/homebrew/bin/gtime -p /x/wk run --slot check -- go test ./...`, nil},
	{"env with a relative path in a value", `env A=./time go test ./...`,
		`env A=./time /x/wk run --slot check -- go test ./...`, nil},

	// Row 28, from the row 27 audit N2: these quoted expansions stay one word in real bash and
	// zsh, so the gate does not move.
	{`env with "$*"`, `env A="$*" go test ./...`,
		`env A="$*" /x/wk run --slot check -- go test ./...`, nil},
	{`env with "${arr[*]}"`, `env A="${arr[*]}" go test ./...`,
		`env A="${arr[*]}" /x/wk run --slot check -- go test ./...`, nil},
	{`env with "${#arr[@]}"`, `env A="${#arr[@]}" go test ./...`,
		`env A="${#arr[@]}" /x/wk run --slot check -- go test ./...`, nil},
	{`env with "$@" inside a quoted $( )`, `env A="$(echo "$@")" go test ./...`,
		`env A="$(echo "$@")" /x/wk run --slot check -- go test ./...`, nil},

	// Row 27: wk run waits for its place before the shell starts, so timeout's clock inside the
	// string starts after the wait; the string is wrapped whole as any other.
	{"sh -c around timeout", `sh -c 'timeout 300 go test ./...'`,
		`/x/wk run --slot check -- sh -c 'timeout 300 go test ./...'`, nil},

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

	// Row 25, decided by the coordinating session by risk, 2026-10-26: a gate behind timeout
	// stays a miss, because timeout's clock would also count the wait for a place, and waits over 10 minutes are common. The
	// first two are the four "after the wrapper timeout" lines of ezhomesteading's missed.log
	// from 2026-10-17 to 2026-10-26, shortened.
	{"timeout, from missed.log", `cd web && timeout 300 bun run lint -- --file a.tsx 2>&1 | tail -20`, `cd web && timeout 300 bun run lint -- --file a.tsx 2>&1 | tail -20`,
		[]Missed{{"timeout 300 bun run lint -- --file a.tsx", matchTimeoutReason}}},
	{"timeout twice on one line, from missed.log", `timeout 280 bunx eslint a.tsx | tail -15; cd ../native && timeout 200 bunx eslint b.tsx`, `timeout 280 bunx eslint a.tsx | tail -15; cd ../native && timeout 200 bunx eslint b.tsx`,
		[]Missed{{"timeout 280 bunx eslint a.tsx", matchTimeoutReason}, {"timeout 200 bunx eslint b.tsx", matchTimeoutReason}}},
	{"timeout -s -k with units", `timeout -s KILL -k 5s 2m go test ./...`, `timeout -s KILL -k 5s 2m go test ./...`,
		[]Missed{{"timeout -s KILL -k 5s 2m go test ./...", matchTimeoutReason}}},
	{"gtimeout", `gtimeout 30 go test ./...`, `gtimeout 30 go test ./...`,
		[]Missed{{"gtimeout 30 go test ./...", matchTimeoutReason}}},
	{"timeout --foreground", `timeout --foreground 300 go test ./...`, `timeout --foreground 300 go test ./...`,
		[]Missed{{"timeout --foreground 300 go test ./...", matchTimeoutReason}}},
	{"timeout behind a wrapper wk reads", `command env -u X timeout 300 go test ./...`, `command env -u X timeout 300 go test ./...`,
		[]Missed{{"command env -u X timeout 300 go test ./...", matchTimeoutReason}}},
	{"wrappers behind timeout", `timeout 600 env -u X /usr/bin/time -p go test ./...`, `timeout 600 env -u X /usr/bin/time -p go test ./...`,
		[]Missed{{"timeout 600 env -u X /usr/bin/time -p go test ./...", matchTimeoutReason}}},
	{"the time keyword over timeout", `time timeout 300 go test ./...`, `time timeout 300 go test ./...`,
		[]Missed{{"timeout 300 go test ./...", matchTimeoutReason}}},
	{"timeout in $( ) that leaves stderr out", `n=$(timeout 60 go test ./... | wc -l)`, `n=$(timeout 60 go test ./... | wc -l)`,
		[]Missed{{"timeout 60 go test ./...", matchTimeoutReason}}},

	// Row 25: other wrapper words wk does not read stay a miss.
	{"timeout with no gate", `timeout 5 curl -s localhost`, `timeout 5 curl -s localhost`, nil},
	{"/usr/bin/time with an option wk does not read", `/usr/bin/time -x go test ./...`, `/usr/bin/time -x go test ./...`,
		[]Missed{{"/usr/bin/time -x go test ./...", "after the wrapper time"}}},
	{"/usr/bin/time with a quoted spelling", `/usr/bin/time -p "go" test ./...`, `/usr/bin/time -p "go" test ./...`,
		[]Missed{{`/usr/bin/time -p "go" test ./...`, "quoted spelling"}}},
	{"command -p looks the gate up in another PATH", `command -p go test ./...`, `command -p go test ./...`,
		[]Missed{{"command -p go test ./...", "after the wrapper command"}}},
	{"/usr/bin/time in $( ) that captures stderr", `v=$(/usr/bin/time -p go test ./... 2>&1)`, `v=$(/usr/bin/time -p go test ./... 2>&1)`,
		[]Missed{{"/usr/bin/time -p go test ./...", "inside $( ) that captures stderr"}}},
	{"command time on a line that holds wk run", `command time go test ./...; wk run -- bun run lint`, `command time go test ./...; wk run -- bun run lint`,
		[]Missed{{"command time go test ./...", "the line already holds wk run"}}},

	// Row 27, from the row 25 audit N2: an option argument that an empty variable drops, or a
	// glob or brace splits, would move the gate, so the line stays a miss.
	{"-o $F", `/usr/bin/time -o $F go test ./...`, `/usr/bin/time -o $F go test ./...`,
		[]Missed{{"/usr/bin/time -o $F go test ./...", "after the wrapper time"}}},
	{"-f $FMT", `gtime -f $FMT go test ./...`, `gtime -f $FMT go test ./...`,
		[]Missed{{"gtime -f $FMT go test ./...", "after the wrapper gtime"}}},
	{"-o with a glob", `/usr/bin/time -o t*.txt go test ./...`, `/usr/bin/time -o t*.txt go test ./...`,
		[]Missed{{"/usr/bin/time -o t*.txt go test ./...", "after the wrapper time"}}},
	{"-o with a brace", `/usr/bin/time -o {a,b} go test ./...`, `/usr/bin/time -o {a,b} go test ./...`,
		[]Missed{{"/usr/bin/time -o {a,b} go test ./...", "after the wrapper time"}}},
	{"-o with a glob attached", `/usr/bin/time -ot?.txt go test ./...`, `/usr/bin/time -ot?.txt go test ./...`,
		[]Missed{{"/usr/bin/time -ot?.txt go test ./...", "after the wrapper time"}}},
	{"-o with an expansion attached", `/usr/bin/time -o$F go test ./...`, `/usr/bin/time -o$F go test ./...`,
		[]Missed{{"/usr/bin/time -o$F go test ./...", "after the wrapper time"}}},
	{"--output $F", `/usr/bin/time --output $F go test ./...`, `/usr/bin/time --output $F go test ./...`,
		[]Missed{{"/usr/bin/time --output $F go test ./...", "after the wrapper time"}}},
	{"--output=$F", `/usr/bin/time --output=$F go test ./...`, `/usr/bin/time --output=$F go test ./...`,
		[]Missed{{"/usr/bin/time --output=$F go test ./...", "after the wrapper time"}}},
	{"--format= with a brace", `gtime --format={a,b} go test ./...`, `gtime --format={a,b} go test ./...`,
		[]Missed{{"gtime --format={a,b} go test ./...", "after the wrapper gtime"}}},
	{"-o with a quoted expansion is refused too", `/usr/bin/time -o "$F" go test ./...`, `/usr/bin/time -o "$F" go test ./...`,
		[]Missed{{`/usr/bin/time -o "$F" go test ./...`, "after the wrapper time"}}},
	{"-o with no argument before the gate", `/usr/bin/time -o`, `/usr/bin/time -o`, nil},
	{"env with an unquoted expansion that may split", `env A=$X go test ./...`, `env A=$X go test ./...`,
		[]Missed{{"env A=$X go test ./...", "after the wrapper env"}}},
	{"env with a substitution in the value", `env A=$(date) go test ./...`, `env A=$(date) go test ./...`,
		[]Missed{{"env A=$(date) go test ./...", "after the wrapper env"}}},
	{"env with an expansion after text", `env A=x${Y} go test ./...`, `env A=x${Y} go test ./...`,
		[]Missed{{"env A=x${Y} go test ./...", "after the wrapper env"}}},

	// Row 27, from the row 25 audit N1: a shell string behind timeout runs its gate after the
	// wait would start timeout's clock, so it is logged with timeout's reason, as a gate is.
	{"timeout over sh -c", `cd web && timeout 300 sh -c 'bun run lint' | tail -5`, `cd web && timeout 300 sh -c 'bun run lint' | tail -5`,
		[]Missed{{"timeout 300 sh -c 'bun run lint'", matchTimeoutReason}}},
	{"timeout over bash -c in double quotes", `timeout 300 bash -c "cd web && go test ./..."`, `timeout 300 bash -c "cd web && go test ./..."`,
		[]Missed{{`timeout 300 bash -c "cd web && go test ./..."`, matchTimeoutReason}}},
	{"gtimeout and env over sh -c", `gtimeout 30 env -u X sh -c 'go test ./...'`, `gtimeout 30 env -u X sh -c 'go test ./...'`,
		[]Missed{{"gtimeout 30 env -u X sh -c 'go test ./...'", matchTimeoutReason}}},
	{"timeout over sh -c with no gate", `timeout 300 sh -c 'echo hi'`, `timeout 300 sh -c 'echo hi'`, nil},
	{"another wrapper over sh -c is logged under its name", `nohup sh -c 'go test ./...'`, `nohup sh -c 'go test ./...'`,
		[]Missed{{"nohup sh -c 'go test ./...'", "after the wrapper nohup"}}},

	// Row 27, from the row 25 audit N6: a quoted wrapper word is logged as a quoted spelling, and
	// a quoted timeout keeps timeout's reason.
	{"a quoted /usr/bin/time", `"/usr/bin/time" -p go test ./...`, `"/usr/bin/time" -p go test ./...`,
		[]Missed{{`"/usr/bin/time" -p go test ./...`, "quoted spelling"}}},
	{"a quoted env", `'env' -u X go test ./...`, `'env' -u X go test ./...`,
		[]Missed{{`'env' -u X go test ./...`, "quoted spelling"}}},
	{"an escaped time", `\time -p go test ./...`, `\time -p go test ./...`,
		[]Missed{{`\time -p go test ./...`, "quoted spelling"}}},
	{"a quoted timeout", `"timeout" 300 go test ./...`, `"timeout" 300 go test ./...`,
		[]Missed{{`"timeout" 300 go test ./...`, matchTimeoutReason}}},
	{"a quoted timeout behind env", `env -u X "timeout" 300 go test ./...`, `env -u X "timeout" 300 go test ./...`,
		[]Missed{{`env -u X "timeout" 300 go test ./...`, matchTimeoutReason}}},

	// Row 28, from the row 27 audit N1: a wrapper name with a relative path may be a project
	// script with arguments of its own, so wk run is not inserted among them.
	{"./time", `./time -p go test ./...`, `./time -p go test ./...`,
		[]Missed{{"./time -p go test ./...", matchRelativeWrapperReason}}},
	{"bin/env", `bin/env A=1 go test ./...`, `bin/env A=1 go test ./...`,
		[]Missed{{"bin/env A=1 go test ./...", matchRelativeWrapperReason}}},
	{"../tools/command", `../tools/command go test ./...`, `../tools/command go test ./...`,
		[]Missed{{"../tools/command go test ./...", matchRelativeWrapperReason}}},
	{"./time behind env", `env -u X ./time -p go test ./...`, `env -u X ./time -p go test ./...`,
		[]Missed{{"env -u X ./time -p go test ./...", matchRelativeWrapperReason}}},
	{"./timeout keeps the timeout reason", `./timeout 300 go test ./...`, `./timeout 300 go test ./...`,
		[]Missed{{"./timeout 300 go test ./...", matchTimeoutReason}}},
	{"a relative path in an env value is no wrapper", `env A=./time -C x go test ./...`, `env A=./time -C x go test ./...`,
		[]Missed{{"env A=./time -C x go test ./...", "after the wrapper env"}}},

	// Row 28, from the row 27 audit N2: a quoted expansion that makes one word per element moves
	// the gate (real bash and zsh: env ran the second element), and so does zsh's $=X.
	{`env with "$@"`, `env A="$@" go test ./...`, `env A="$@" go test ./...`,
		[]Missed{{`env A="$@" go test ./...`, "after the wrapper env"}}},
	{`env with "${arr[@]}"`, `env A="${arr[@]}" go test ./...`, `env A="${arr[@]}" go test ./...`,
		[]Missed{{`env A="${arr[@]}" go test ./...`, "after the wrapper env"}}},
	{`env with "${@:1}"`, `env A="${@:1}" go test ./...`, `env A="${@:1}" go test ./...`,
		[]Missed{{`env A="${@:1}" go test ./...`, "after the wrapper env"}}},
	{`env with "${X:-$@}"`, `env A="${X:-$@}" go test ./...`, `env A="${X:-$@}" go test ./...`,
		[]Missed{{`env A="${X:-$@}" go test ./...`, "after the wrapper env"}}},
	{`env with "${!p@}"`, `env A="${!p@}" go test ./...`, `env A="${!p@}" go test ./...`,
		[]Missed{{`env A="${!p@}" go test ./...`, "after the wrapper env"}}},
	{`env with "${arr[@]/x/y}"`, `env A="${arr[@]/x/y}" go test ./...`, `env A="${arr[@]/x/y}" go test ./...`,
		[]Missed{{`env A="${arr[@]/x/y}" go test ./...`, "after the wrapper env"}}},
	{`env with zsh's "$arr[@]"`, `env A="$arr[@]" go test ./...`, `env A="$arr[@]" go test ./...`,
		[]Missed{{`env A="$arr[@]" go test ./...`, "after the wrapper env"}}},
	{"env with zsh's $=X", `env A=$=X go test ./...`, `env A=$=X go test ./...`,
		[]Missed{{"env A=$=X go test ./...", "after the wrapper env"}}},
	{"time -o with zsh's $=F", `/usr/bin/time -o $=F go test ./...`, `/usr/bin/time -o $=F go test ./...`,
		[]Missed{{"/usr/bin/time -o $=F go test ./...", "after the wrapper time"}}},

	// Row 28, from the row 27 audit N3: the string behind a wrapper is parsed, so a spelling that
	// is only an argument or a comment there is no gate. A string wk cannot parse is searched as
	// text, which errs toward logging.
	{"timeout over sh -c that echoes a spelling", `timeout 30 sh -c 'echo go test ./...'`, `timeout 30 sh -c 'echo go test ./...'`, nil},
	{"timeout over eval that echoes a spelling", `timeout 30 eval 'echo go test'`, `timeout 30 eval 'echo go test'`, nil},
	{"nohup over sh -c that greps a spelling", `nohup sh -c 'grep "go test" log.txt'`, `nohup sh -c 'grep "go test" log.txt'`, nil},
	{"timeout over sh -c with a spelling in a comment", `timeout 30 sh -c 'echo hi # go test'`, `timeout 30 sh -c 'echo hi # go test'`, nil},
	{"timeout over sh -c with a gate in $( )", `timeout 30 sh -c 'echo $(go test ./...)'`, `timeout 30 sh -c 'echo $(go test ./...)'`,
		[]Missed{{`timeout 30 sh -c 'echo $(go test ./...)'`, matchTimeoutReason}}},
	{"timeout over sh -c with env before the gate", `timeout 30 sh -c 'env -u X go test ./...'`, `timeout 30 sh -c 'env -u X go test ./...'`,
		[]Missed{{`timeout 30 sh -c 'env -u X go test ./...'`, matchTimeoutReason}}},
	{"timeout over sh -c over timeout", `timeout 30 sh -c 'timeout 5 go test ./...'`, `timeout 30 sh -c 'timeout 5 go test ./...'`,
		[]Missed{{`timeout 30 sh -c 'timeout 5 go test ./...'`, matchTimeoutReason}}},
	{"timeout over eval of a gate", `timeout 30 eval go test ./...`, `timeout 30 eval go test ./...`,
		[]Missed{{"timeout 30 eval go test ./...", matchTimeoutReason}}},
	{"timeout over sh -c with an expansion is searched as text", `timeout 30 sh -c "echo go test $PKG"`, `timeout 30 sh -c "echo go test $PKG"`,
		[]Missed{{`timeout 30 sh -c "echo go test $PKG"`, matchTimeoutReason}}},
	{"timeout over sh -c that does not parse is searched as text", `timeout 30 sh -c 'echo go test ('`, `timeout 30 sh -c 'echo go test ('`,
		[]Missed{{`timeout 30 sh -c 'echo go test ('`, matchTimeoutReason}}},

	// Row 28, from the row 27 audit N4: inside a substitution that captures stderr, the capture is
	// the reason logged, whatever else holds the gate back. Neither form is rewritten.
	{"timeout over sh -c in $( ) that captures stderr", `x=$(timeout 300 sh -c 'go test ./...' 2>&1)`, `x=$(timeout 300 sh -c 'go test ./...' 2>&1)`,
		[]Missed{{"timeout 300 sh -c 'go test ./...'", "inside $( ) that captures stderr"}}},
	{"timeout in $( ) that captures stderr", `x=$(timeout 300 go test ./... 2>&1)`, `x=$(timeout 300 go test ./... 2>&1)`,
		[]Missed{{"timeout 300 go test ./...", "inside $( ) that captures stderr"}}},
	{"timeout in backticks that capture stderr", "x=`timeout 300 go test ./... 2>&1`", "x=`timeout 300 go test ./... 2>&1`",
		[]Missed{{"timeout 300 go test ./...", "inside backticks that capture stderr"}}},
	{"env with an option wk does not read in $( ) that captures stderr", `x=$(env -C d go test ./... 2>&1)`, `x=$(env -C d go test ./... 2>&1)`,
		[]Missed{{"env -C d go test ./...", "inside $( ) that captures stderr"}}},
	{"a quoted spelling in $( ) that captures stderr", `x=$("go" test ./... 2>&1)`, `x=$("go" test ./... 2>&1)`,
		[]Missed{{`"go" test ./...`, "inside $( ) that captures stderr"}}},

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

// Row 29, from the row 28 audit N-A to N-D and the row 28 builder's open item. The reasons are
// written out, not named by their constants, so the table runs on the code before row 29 too.
var matchTestRow29 = []matchTestCase{
	// N-A: zsh may split $=X inside double quotes as it does outside them. zsh could not be run
	// in the row 29 session (its guard refused it), so a $ followed by = in the literal text of a
	// double-quoted env value or time option argument is refused on the safe side.
	{`env with zsh's "$=X"`, `env A="$=X" go test ./...`, `env A="$=X" go test ./...`,
		[]Missed{{`env A="$=X" go test ./...`, "after the wrapper env"}}},
	{`env with zsh's $=X after text in quotes`, `env A="x $=X" go test ./...`, `env A="x $=X" go test ./...`,
		[]Missed{{`env A="x $=X" go test ./...`, "after the wrapper env"}}},
	{`time -o with zsh's "$=F"`, `/usr/bin/time -o "$=F" go test ./...`, `/usr/bin/time -o "$=F" go test ./...`,
		[]Missed{{`/usr/bin/time -o "$=F" go test ./...`, "after the wrapper time"}}},
	{`time --output= with zsh's "$=F"`, `/usr/bin/time --output="$=F" go test ./...`, `/usr/bin/time --output="$=F" go test ./...`,
		[]Missed{{`/usr/bin/time --output="$=F" go test ./...`, "after the wrapper time"}}},
	{`env with $= in single quotes, which no shell expands`, `env A='$=X' go test ./...`,
		`env A='$=X' /x/wk run --slot check -- go test ./...`, nil},
	{`env with = and no $ in quotes`, `env A="x=1" go test ./...`,
		`env A="x=1" /x/wk run --slot check -- go test ./...`, nil},

	// N-B: a wk run in the string of a shell under timeout is read as the bare form is.
	{"timeout over wk run", `timeout 30 wk run -- go test ./...`, `timeout 30 wk run -- go test ./...`,
		[]Missed{{"timeout 30 wk run -- go test ./...", matchTimeoutReason}}},
	{"timeout over sh -c over wk run", `timeout 30 sh -c 'wk run -- go test ./...'`, `timeout 30 sh -c 'wk run -- go test ./...'`,
		[]Missed{{`timeout 30 sh -c 'wk run -- go test ./...'`, matchTimeoutReason}}},
	{"timeout over sh -c over wk run --slot", `timeout 30 sh -c 'cd web && wk run --slot check -- bun run lint'`, `timeout 30 sh -c 'cd web && wk run --slot check -- bun run lint'`,
		[]Missed{{`timeout 30 sh -c 'cd web && wk run --slot check -- bun run lint'`, matchTimeoutReason}}},
	{"timeout over sh -c over a wk run with no gate", `timeout 30 sh -c 'wk run -- echo hi'`, `timeout 30 sh -c 'wk run -- echo hi'`, nil},

	// N-C: a tilde path is absolute once expanded, but a home script named time is still no time
	// utility, so the line stays a miss under its own reason.
	{"~/bin/time", `~/bin/time -p go test ./...`, `~/bin/time -p go test ./...`,
		[]Missed{{"~/bin/time -p go test ./...", "after a wrapper name under a home directory: it may be a personal script"}}},
	{"~/bin/env behind env", `env -u X ~/bin/env go test ./...`, `env -u X ~/bin/env go test ./...`,
		[]Missed{{"env -u X ~/bin/env go test ./...", "after a wrapper name under a home directory: it may be a personal script"}}},
	{"~user/bin/time", `~ci/bin/time go test ./...`, `~ci/bin/time go test ./...`,
		[]Missed{{"~ci/bin/time go test ./...", "after a wrapper name under a home directory: it may be a personal script"}}},

	// N-D: inside a substitution that captures stderr the capture is the reason logged, also for a
	// function body, a process substitution and a line that holds wk run; a quoted wrapper word
	// anywhere before the gate is a quoted spelling; an assignment is no timeout.
	{"a function body in $( ) that captures stderr", `x=$(f() { go test ./...; }; f 2>&1)`, `x=$(f() { go test ./...; }; f 2>&1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"a process substitution in $( ) that captures stderr", `x=$(diff <(go test ./...) a.txt 2>&1)`, `x=$(diff <(go test ./...) a.txt 2>&1)`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"a capture of stderr on a line that holds wk run", `x=$(go test ./... 2>&1); wk run -- bun run lint`, `x=$(go test ./... 2>&1); wk run -- bun run lint`,
		[]Missed{{"go test ./...", "inside $( ) that captures stderr"}}},
	{"a quoted time behind env", `env -u X "time" -p go test ./...`, `env -u X "time" -p go test ./...`,
		[]Missed{{`env -u X "time" -p go test ./...`, "quoted spelling"}}},
	{"a value that ends in /timeout is no timeout", `env -C d X=/a/timeout go test ./...`, `env -C d X=/a/timeout go test ./...`,
		[]Missed{{"env -C d X=/a/timeout go test ./...", "after the wrapper env"}}},

	// Row 28 builder's open item: the string of a shell or an eval is parsed, and only a gate it
	// runs as a command is wrapped or logged.
	{"sh -c that echoes a spelling", `sh -c 'echo go test ./...'`, `sh -c 'echo go test ./...'`, nil},
	{"bash -c that greps a spelling", `bash -c 'grep "go test" log.txt'`, `bash -c 'grep "go test" log.txt'`, nil},
	{"sh -c with a spelling in a comment", `sh -c 'echo hi # go test'`, `sh -c 'echo hi # go test'`, nil},
	{"eval that echoes a spelling", `eval 'echo go test'`, `eval 'echo go test'`, nil},
	{"sh -c in a function body that echoes a spelling", `f() { sh -c 'echo go test'; }`, `f() { sh -c 'echo go test'; }`, nil},
	// Row 29 audit F1, decided by the row 29 session by risk: the class still comes from every
	// spelling the string mentions, since a string can run a mentioned spelling in ways the walk
	// does not see. Each of these stays refused and logged, as on the code before row 29.
	{"sh -c that runs one class and echoes another", `sh -c 'go test ./... && echo bun run build'`, `sh -c 'go test ./... && echo bun run build'`,
		[]Missed{{`sh -c 'go test ./... && echo bun run build'`, "inside sh -c: spellings of more than one class"}}},
	{"sh -c that pipes another class into sh", `sh -c 'go test ./... && echo bun run build | sh'`, `sh -c 'go test ./... && echo bun run build | sh'`,
		[]Missed{{`sh -c 'go test ./... && echo bun run build | sh'`, "inside sh -c: spellings of more than one class"}}},
	{"sh -c that runs another class from a variable", `sh -c 'go test ./...; x="bun run build"; $x'`, `sh -c 'go test ./...; x="bun run build"; $x'`,
		[]Missed{{`sh -c 'go test ./...; x="bun run build"; $x'`, "inside sh -c: spellings of more than one class"}}},
	{"sh -c that feeds another class to bash", `sh -c 'go test ./...; bash <<< "bun run build"'`, `sh -c 'go test ./...; bash <<< "bun run build"'`,
		[]Missed{{`sh -c 'go test ./...; bash <<< "bun run build"'`, "inside sh -c: spellings of more than one class"}}},
	{"sh -c that sources another class", `sh -c 'go test ./...; . <(echo bun run build)'`, `sh -c 'go test ./...; . <(echo bun run build)'`,
		[]Missed{{`sh -c 'go test ./...; . <(echo bun run build)'`, "inside sh -c: spellings of more than one class"}}},
	{"sh -c that runs a heavy gate and echoes a check spelling", `sh -c 'bun run build && echo tsc'`, `sh -c 'bun run build && echo tsc'`,
		[]Missed{{`sh -c 'bun run build && echo tsc'`, "inside sh -c: spellings of more than one class"}}},
	{"sh -c with a gate in $( ) is still wrapped", `sh -c 'echo $(go test ./...)'`,
		`/x/wk run --slot check -- sh -c 'echo $(go test ./...)'`, nil},
	{"sh -c with a gate behind timeout is still wrapped", `sh -c 'timeout 300 go test ./...'`,
		`/x/wk run --slot check -- sh -c 'timeout 300 go test ./...'`, nil},
	{"eval of a gate is still logged", `eval 'go test ./...'`, `eval 'go test ./...'`,
		[]Missed{{`eval 'go test ./...'`, "inside eval"}}},
	{"sh -c with an expansion is still searched as text", `sh -c "echo go test $PKG"`, `sh -c "echo go test $PKG"`,
		[]Missed{{`sh -c "echo go test $PKG"`, "inside sh -c: the string is not plain text"}}},
}

func TestMatchRewriteRow29(t *testing.T) {
	matchTestRun(t, matchTestRow29)
}

func TestMatchRewriteWrappers(t *testing.T) {
	matchTestRun(t, append(append([]matchTestCase{}, matchTestWrapperRewrites...), matchTestWrapperUntouched...))
}

func matchTestRun(t *testing.T, cases []matchTestCase) {
	t.Helper()
	sf := matchTestSlots(t)
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
