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

## Close-out, 2026-10-16

Built `internal/gate/` (`run.go`, `paths.go`, `memory.go` and a test file for each). The package imports only the standard library. Nothing outside `internal/gate/` changed except this section.

**Exported surface.**

- `Gate{Name, Dir string; Run, NeedsEnv []string}`: `Run` is the argument vector, `Dir` is relative to the root.
- `Result{Name, Status, Reason string; Output []byte; Seconds float64}`, with `Status` one of the constants `Pass`, `Fail`, `Skip` (`"pass"`, `"fail"`, `"skip"`).
- `Run(root, gates, env, caches, log) ([]Result, error)`: runs the gates in order in `root/Dir`, with `env` added to the process environment, and stops after the first `fail`. A gate whose `NeedsEnv` names a variable unset in the process environment is `skip` with "<VAR> is unset" and is not run. Each `caches` variable points at one fresh empty `os.MkdirTemp` directory for the whole run, shared by its gates, removed afterwards; it wins over the same name in `env`. Stdout and stderr go together into `Output`; a status line and the last 20 lines go to `log` (PD-13). A non-zero exit is `fail` with `exit status N` as the reason. The error is only for a gate that could not start (missing binary or directory, empty `Run`): that says nothing about the tree, so it must not be blamed on a changeset.
- `Paths(root, output) []string`: the root-relative paths of existing files the output names, deduplicated in first-seen order. It strips ANSI colours, then reads each unindented line as `path:line[:col]`, `path(line,col)`, a madge `a > b` chain, or a lone path (gofmt `-l`, eslint stylish headers). Absolute paths under `root` (or under its resolved form) become relative; anything outside `root`, missing, or a directory is dropped. Indented lines are never read, because tsc, golangci-lint and eslint indent quoted source there.
- `FreeMB() (int, error)`: free plus inactive plus speculative pages from `vm_stat`, times the page size in its first line, in MiB.
- `WaitForMemory(min int, sample time.Duration, log io.Writer) error`: returns when `FreeMB() >= min`, logging one line when it starts to wait and one when it stops, nothing when no wait was needed.

**Output samples the path table covers** (`paths_test.go`). Each one was captured on 2026-10-16 in a throwaway probe project in the builder's scratchpad, not in ezhomesteading, and not from `SLOTS-SCOPE.md` 1.4, which records timings and no output. Only the absolute prefix is replaced: `go build` and `go vet` (go1.24.1), `gofmt -l`, `golangci-lint` v2.12.2, `tsc` 5.8.3 with and without `--pretty`, eslint 9.39.5 stylish, and madge 8.0.0 run from the root and from a subdirectory. The table also covers a path outside the root, a missing path, a directory, a `..` escape and a duplicate. A separate test covers a root reached through a symlink. `memory_test.go` reads a `vm_stat` captured the same day (5905 MiB).

**Build-level calls, each with its reversal.**

- `WaitForMemory` returns `error`; the prompt gave no return type. Without one, an unreadable `vm_stat` either blocks gating for ever or passes silently. Reversal: drop the return and log the error.
- `Run` returns an error, not a `fail`, for a gate it cannot start. Reversal: make it `fail` with the start error as the reason.
- `Paths` also accepts `path:line` with no column. No sample here needs it; the existence check keeps it safe. Reversal: make the column required in `position`.
- MB means MiB (`1 << 20`).

**Findings for row 5.**

- Tools print paths relative to their own working directory: `go build` prints `./main.go` relative to the gate's `dir`, `tsc` relative to its project, and madge relative to the directory or file it was given (`madge ... src/b.ts` printed `b.ts > c.ts`). `Paths(root, out)` drops all of these when the gate runs in a subdirectory. Recommendation: call `Paths(filepath.Join(trial, g.Dir), r.Output)` and join `g.Dir` onto each result. Give a madge gate the `dir` of its base directory, or its paths stay unreadable.
- eslint 9 (9.39.5 at ezhomesteading's root, 9.20.1 in `web/`) no longer ships the compact formatter, and the `lint` scripts use the default stylish one. The prompt's "eslint compact form" has no real sample here, so it is not covered.
- `go test` failures print `    x_test.go:12:` indented and relative to the package, so they give no path; a red `go test` falls back to T1.
- `NeedsEnv` reads the process environment, as the prompt says, not `gates.env`. A variable set only in `gates.env` still skips the gate.
- `Run` has no timeout, so a hung gate holds the trial. If row 5 wants one, add it to this package (a context or a per-gate limit) rather than wrap it.
- `Result` does not carry `Dir`, so row 5 keeps its own `[]Gate` to map a result back to its directory.

**Questions answered:** none were open. **Left owed:** the wiring, the config keys and the live proof, which are all row 5's.
