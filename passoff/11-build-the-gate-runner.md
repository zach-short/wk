# 11. Build the gate runner, the path extractor and the memory floor in `internal/gate`

> **Claimed `IN FLIGHT` 2026-10-16.**

*Opened 2026-10-16 by row 2, as a P4 sibling of row 5 (`docs/incomplete/landing-layer/PLAN-P2-P4.md` section 4).*

**Model: Opus 5.5. Lane H. Waits on 2** (this package imports nothing from P2 or P3; its tests are shell scripts and a captured `vm_stat`, so it starts with the P2 siblings).

You are building one package of P4 of the landing layer, `wk`. Read first: `PLAN-P2-P4.md` sections 1 (PD-10) and 4, the row 11 line of its sub-row table (binding), `DESIGN.md` option S, T3 and section 6 (`skip` is never `pass`; fresh caches; every gate reads the working tree), `docs/conventions-go.md`, and `internal/wk/env.go` `runCommands` (the precedent for running a configured command with extra env). You own only `internal/gate/`; row 5 wires it. Nothing in your package imports `internal/wk`, `internal/config`, `internal/daemon` or `internal/batch`.

Do these, in order:
1. `internal/gate/run.go`: `Gate{Name, Dir string, Run, NeedsEnv []string}`; `Result{Name, Status, Reason string, Output []byte, Seconds float64}` with `Status` one of `pass`, `fail`, `skip`; `Run(root string, gates []Gate, env map[string]string, caches []string, log io.Writer) ([]Result, error)`: in order; a gate whose `NeedsEnv` names a variable unset in the process environment is `skip` with "<VAR> is unset" and is not run; each `caches` variable is set to a fresh directory under `os.MkdirTemp` for the whole run and removed after; stdout and stderr captured together and the last 20 lines echoed to `log` (PD-13); a non-zero exit is `fail` with the exit status as the reason; the run stops at the first `fail`.
2. `internal/gate/paths.go`: `Paths(root string, output []byte) []string`: the repository-relative paths a failure prints, deduplicated in first-seen order: `path:line:col` and `path(line,col)` forms (`tsc`, `go build`, `go vet`, eslint's compact form), eslint's stylish header lines (a path alone on a line that is followed by indented `line:col` lines), `gofmt -l` (a path alone per line), `golangci-lint` (`path:line:col: message`) and madge (`path > path` chains). Absolute paths under `root` are made relative; paths outside `root` and paths that do not exist under `root` are dropped.
3. `internal/gate/memory.go`: `FreeMB() (int, error)` from `vm_stat`: page size from its first line, free plus inactive plus speculative pages; `WaitForMemory(min int, sample time.Duration, log io.Writer)` returns when `FreeMB() >= min`, logging one line when it starts to wait and one when it stops.
4. Tests: `run_test.go` with gates that are small shell scripts written into `t.TempDir()` (a pass, a fail with exit 3, a skip by `NeedsEnv`, a gate that reads its cache variable and proves it is a fresh empty directory, the stop at the first fail); `paths_test.go` as a table over real output samples written into the test for each tool above (take them from a run in this repository or from `SLOTS-SCOPE.md` 1.4, and cite where each came from); `memory_test.go` over a captured `vm_stat` text.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`; the package tests above green.

Ask before building: nothing.
Not in scope, whoever asks: the trial workspace, the batch, attribution, the slots (row 200), any subcommand or config key. A need outside your package is a finding for row 5 in your close-out, never an edit.

Hand back: the gate output, the exported surface with each function's one-line contract, the output samples the path table covers, the findings for row 5, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.
