# 18. Fix row 16's open findings

*Opened 2026-10-18, from the sign-off of row 16 (`passoff/16-fix-the-batch-gate-findings-row-15-left-open.md`, its last section): A1, A2, and the "left owed" gap about the wait's length in a failed reason. The owner named the three in chat. F3 and row 15's review N3 and N4 stay owner's calls and are not in this row.*

**Model: Opus 5.5 (rows 15 and 16 ran on Opus 5.5, and the session that built this one did too). Lane A. Waits on nothing. Files it owns: `internal/wk/new.go`, `internal/wk/trial.go`, `internal/wk/gatestart.go`, `internal/wk/gateplan.go`, `internal/wk/gatekeeper_test.go`, `internal/slot/run.go`, `internal/slot/run_test.go`, `README.md`.**

## What is fixed

- A1. A directory that appears at `gates.trial_dir` between the `Lstat` in `ensureTrial` and the exists check in `create` kept `trial.making`, and the error promised a removal that `removeInterruptedTrial` may refuse.
- A2. A slotted gate whose command never waited, but printed a line that starts `wk run: waiting for a ` and then timed out, read as a start error, so the changeset was never blamed.
- The wait's length was in the landing log only, inside the last 20 lines, so a gate with a long output lost it from the failed record.

## Close-out, 2026-10-18

Built on `main` in the main checkout by one Opus 5.5 session, with no subagent and no audit.

1. A1. `create` in `new.go` now returns the sentinel `errExists` for a directory that is already there. `failedTrialCreate` in `trial.go` reads it first: create made nothing, so the directory is someone else's. The record is removed, and the error says `it appeared after the worker looked, so it is not the trial; move it`. The next worker then refuses the directory as unmarked and never removes it. Test: `TestTrialDirThatAppearsBeforeTheCreateIsNotTaken`, which calls `makeTrial` on a directory that is already there (the same window).
2. A2. The output alone cannot tell wk run's waiting line from a command's copy of it, so wk run now says when the command starts. `slot.SayStartEnv` (`WK_RUN_SAY_START`), set to any value, makes `wk run` write `wk run: starting <command>` just before the command starts, on every path that starts it (a free place, after a wait, unslotted, nested). wk run removes the variable from the command's environment, so a nested wk run stays quiet (SB-10). `runGate` sets it on every slotted gate. A timeout is now a start error only when that line is missing; `stillWaiting`, `waitStarted` and `waitEnded` are gone. A command can print any line of wk run's but cannot remove the start line, so the only error left is in the safe direction: a faked start line makes a fail. Tests: `TestRunSaysStartWhenAsked` (a free place, and no machine file; the command sees the variable unset), and two new rows in `TestSlottedGateTimeoutWhileWaitingIsAStartError` (a timeout with no output is a start error; the faked waiting line after the start line is a fail). The fake wrapper in the existing rows writes the start line only when `WK_RUN_SAY_START` is set, so the rows also prove that `runGate` sets it.
3. The wait's length. `withWait` in `gateplan.go` adds `(wk run took a <class> place after <duration>)` to a slotted gate's timeout reason when wk run waited. Test: a new row in `TestSlottedGateTimeoutAfterTakingAPlaceIsAFail` with 30 lines of output after the wait.

The README's two paragraphs on the gate timeout and the trial say each of these.

### Not changed

- Without `WK_RUN_SAY_START`, `wk run` writes exactly what it wrote before (`TestRunWithoutAMachineFile` checks the whole stderr), so agents' Bash calls through the hook see no new line.
- The start error's wording for a timeout still says "while still waiting for a place", which is also the message for a timeout before wk run wrote anything at all.
- F3, row 15's review N3 and N4: owner's calls, unchanged.

### Gate

`GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l cmd internal)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'` in the main checkout (`gofmt -l cmd internal`, because `gofmt -l .` there reads the nested worktrees): 2026-10-18 12:00:31 to 12:01:38 EDT on the final tree over 325ab08, exit 0, every package `ok` (`internal/wk` 61.4 s, `internal/slot` 5.3 s), `0 issues.`, 67 s. The first run failed at gofmt on the alignment of the new constant in `run.go`; `gofmt -w` fixed it.

### Not verified

No audit ran. No live gate sets `timeout_seconds`, so the timeout paths ran only in throwaway repositories. `~/go/bin/wk` is not reinstalled, so the installed worker does not have these fixes until `GOTOOLCHAIN=local go install ./cmd/wk` runs from `main`.
