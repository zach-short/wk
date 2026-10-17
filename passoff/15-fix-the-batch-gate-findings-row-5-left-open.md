# 15. Fix the batch gate findings row 5 left open

> **Claimed `IN FLIGHT` 2026-10-17.**

*Opened 2026-10-17, from the open findings of row 5's sign-off (P4, the batch gate, landed on `main` at 8bea094): N2, N3 and N4 of audit 1, N-a and N-c of audit 2, audit 3's observation on the landing log, and the "Left owed" list of row 5's close-out. Sources: `docs/incomplete/autopilot-2026-10-16/reports/row-5-audit-1.md`, `row-5-audit-2.md`, `row-5-audit-3.md`, and the archived item file `~/Projects/archive/wk/passoff/5-build-p4-one-gate-run-per-batch.md` (close-out, "Left owed", fix rounds 1 and 2, sign-off).*

**Model: Opus 5.5. Lane A. Waits on nothing (row 5 landed). Work in your own worktree. Files it owns: `internal/config/config.go`, `internal/config/gates_test.go`, `internal/wk/trial.go`, `internal/wk/gatestart.go`, `internal/wk/gateplan.go`, `internal/wk/gatekeeper_test.go`, `internal/gate/run.go`, `internal/gate/run_test.go`, `internal/gate/paths.go`, `internal/gate/paths_test.go`, `README.md`.**

## Orientation

You pick up the batch gate that row 5 built: the landing worker (`wk land --loop`) forms a batch, gates its final tree once in a detached trial worktree, lands it on green, fast-forwards `main` and pushes it; on red it blames by path, then bisects. Row 5 closed with nine findings for a later row and one action for the owner. This row fixes the nine. Each is small and local; none changes the batch policy.

Read, in this order: `docs/conventions-go.md` (the Go code standard); the close-out, fix rounds and sign-off of `~/Projects/archive/wk/passoff/5-build-p4-one-gate-run-per-batch.md`; the three audit reports above; then each file this row owns, at the functions named below. `README.md` "Gate each batch before it lands" (from line 269) says what the gate promises a user.

Session rules, restated here because this prompt is read alone:

- Work in your own worktree branch, made from `main` at its tip. Commit on that branch only; the lead merges, pushes and removes the worktree after the audit. Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Add no `Co-Authored-By:` trailer and no "Generated with" line.
- Never submit, land, fast-forward or push in this repository (`~/Projects/wk`) through `wk` itself, and write nothing under `~/.wk` or `~/Library/LaunchAgents`. A live proof, if you run one, goes in a throwaway repository in the session scratchpad with a `git init --bare` directory as `origin`, as row 5's proof did.
- Written text (the close-out, the README, comments) uses absolute dates, no contractions and no em dashes.

## Why this exists

The batch gate is the one thing between a submitted changeset and wk's `main`. Every finding here is a way it stops when it should keep going, keeps going on a wrong reading, or tells the person watching it something false: a queue that waits for a person after a red `bun run` gate, a trial a crash leaves unusable, a hung gate that holds the worker for ever, a lockfile change gated over the old dependencies, a config the worker accepts and then trips on. None of them lands a wrong tree. Each costs a person a trip to the landing log, and the gate is only worth running if people stop doing that.

## What is fixed

Read these before changing anything; do not relitigate them.

- The batch policy, blame and bisection, the `gated` crash rule and the green rule (at least one `pass`, no `fail`; a `skip` is never a pass) are as row 5 built and three audits passed them. This row changes none of them.
- A gate that cannot start is an error, not a fail: it stops the pass and blames nobody (row 11; row 5 fix round 1, B1). `runGates` in `internal/wk/gateplan.go` (about lines 190 to 222) holds that rule: `canStart` before the run, `wrapperFailed` after it.
- `wk run` exits 75 when `slots.max_wait_seconds` runs out, 126 for a command it cannot run, 127 for one it cannot find, and in each case prints one line through `runner.say` that starts `wk run: ` (`internal/slot/run.go`: `say` about line 93, `resolve` about lines 106 to 121, `pause` about line 324, `finish` about line 442). It decides executability with `unix.Access(path, unix.X_OK)` (`executable`, about line 165).
- `gate.Run` (`internal/gate/run.go`, line 47) echoes each result to the log inside `runAll` (line 63) before it returns, so `runGates` reads `wrapperFailed` only after the line `gate <name>: fail, exit status 75` is already in the landing log.
- `ensureTrial` (`internal/wk/trial.go`, lines 40 to 57) calls `Env.create` (line 49), then writes the `wk-trial` marker into the private git directory (line 56). The order cannot be swapped: the private git directory does not exist until `worktree add` runs inside `Env.create` (row 5, fix round 1, N3). A failed create leaves its directory "for inspection" (`internal/wk/new.go`, line 107).
- `needs_env` is read from the worker's own environment in two places that must agree: `runOne` in `internal/gate/run.go` (lines 73 to 77) and `skips` in `internal/wk/gatestart.go` (lines 65 to 72). On 2026-10-17 neither `~/.wk/ezhomesteading.toml` nor `~/.wk/wk.toml` sets `EZH_TEST_DB` in `[gates.env]` (grep of both files), so reading `[gates.env]` changes no live gate's outcome.
- The trial's dependencies are made once, by `Env.create`'s post-clone step (`postClone`, `internal/wk/new.go` lines 124 to 130), and `read-tree -u --reset` (`trial.go`, line 31) changes tracked files only. `template.refresh_on` and `template.post_clone` already exist (`internal/config/config.go`, lines 27 to 36), and the template uses the same pair (`internal/wk/template.go`, `refreshBlob` about line 94, the post-clone run about line 83). `internal/testrepo/testrepo.go` sets `refresh_on = "bun.lock"` and a stand-in `post_clone` (lines 46 to 52) that the tests can count. `gitx.Blob` (`internal/gitx`, about line 136) falls back to a commit check when `rev:path` does not resolve, so for a tree read the blob with `rev-parse` directly.
- `gate.Paths` (`internal/gate/paths.go`) reads a position at the start of a line with the `position` pattern (line 17) in `candidates` (lines 48 to 59). `go vet` prints a type error as `vet: ./b.go:3:23: ...`; the lazy path group takes `vet: ./b.go`, which is not a file, so the line names nothing and its batch is bisected (row 5 proof, first run).
- `[[gates.run]]` has `name`, `dir`, `run` and `needs_env` (`GateRun`, `config.go` lines 55 to 63), validated in `Gates.validate` and `validateRuns` (lines 339 to 375). `fresh_caches` entries are already checked as variable names (lines 352 to 356); copy that rule.
- No landing timer is installed on 2026-10-17: `~/Library/LaunchAgents` holds `com.github.zach-short.wk.gc.ezhomesteading.plist` only.

## Do these, in order

1. **N2, config.** In `Gates.validate` and `validateRuns` (`internal/config/config.go`), refuse: a `needs_env` entry that is empty or holds `=` or a space (the `fresh_caches` rule), because `[""]` makes the gate skip for ever with the reason " is unset"; two gates with the same `name`, because the log, `gates.json`, `wk status` and a failed changeset's `gate:` line name a gate by its name alone; and a `trial_dir` whose `filepath.Clean` is `.`, because the trial would be the main checkout, and at 8bea094 only `checkTrialMarker` refuses it, at gate time. Each refusal names the entry, as the existing ones do. Rows in `TestGatesValidation` (`gates_test.go`, line 58) for each, plus `trial_dir = "./"` and `"a/.."`.
2. **N4, `needs_env` reads `[gates.env]`.** In `runOne` (`internal/gate/run.go`), look a `needs_env` variable up in the environment the gate runs with (the slice `environ` builds, lines 131 to 145, where the last value of a name wins), so a variable set in `[gates.env]` counts. Give `skips` in `gatestart.go` the same answer (pass it the gate's env), or `canStart` and the run disagree. Update the doc comments that say "the worker's environment" (`config.go` line 56, `run.go` line 30) and the README sentence at line 275. Tests: `TestRunNeedsEnvReadsGatesEnv` in `run_test.go` beside `TestRunSkipsOnUnsetEnv` (set only in the env map: the gate runs; set nowhere: `skip`), and a row in `gatekeeper_test.go` where a slotted gate's `needs_env` is met only by `[gates.env]` and `canStart` checks it.
3. **N-a, a slotted command's own 126 or 127 is a fail.** In `wrapperFailed` (`gatestart.go`, lines 56 to 63), return a start error only when the exit status is 75, 126 or 127 and the output is `wk run`'s own: every non-empty line starts `wk run: `, because the command never ran and only `wk run` wrote. A slotted `bun run lint` that exits 127 for a package script naming a missing command (bun 1.2.9, audit 2's `bun127.out`) then reads as a fail and is blamed or bisected like any red gate, and the queue keeps moving. Decide what a `/bin/sh: ...: not found` line from the shell that wraps `wk run` means (the `wk` binary itself is missing) and say it in the close-out. Rewrite the README sentences at line 275 that say a slotted command's own 126 or 127 stops the queue. Tests: the existing rows of `TestSlottedGateExit127IsAStartError` (`gatekeeper_test.go`, line 394) print `wk run`'s real lines for 126 and 127 and stay start errors; a new test, `TestSlottedCommandExit127IsAFail`, where the stand-in exits 127 with its own output and no `wk run: ` line, and the changeset ends `failed`.
4. **N-c, one executability rule.** Make `executable` in `gatestart.go` (lines 81 to 84) use `unix.Access(p, unix.X_OK)`, as `wk run` does, in place of the mode bits (`golang.org/x/sys/unix` is already a dependency of `internal/slot`). Test: `TestCanStartUsesAccessLikeWkRun`, a file of mode `0o010` owned by the test's user, which the mode bits call executable and `access` does not; `canStart` refuses it. Skip the test when it runs as root, and say why in the skip message.
5. **The landing log names a start error as a fail.** In `runGates` (`gateplan.go`, lines 207 to 214), give `gate.Run` a buffer as its log for a slotted gate and copy the buffer to `out` only when `wrapperFailed` returns nil; when it returns a start error, write the output's tail and the start error, never `fail, exit status 75`. Keep `gate.Run`'s signature. Test: `TestSlottedStartErrorIsNotLoggedAsAFail`, for exit 75 with `wk run`'s give-up line: the log holds the start error and no `gate <name>: fail` line.
6. **The `vet:` prefix.** In `candidates` (`paths.go`, lines 48 to 59), strip a leading `vet: ` before the position match, so `vet: ./b.go:3:23: undefined: x` names `b.go`. Nothing else in the pattern changes. Test: a `vet:` row in `TestPaths` (`paths_test.go`, line 139), and a row where `vet: ` is followed by a name that is not a file, which still names nothing.
7. **A per-gate timeout.** Add `timeout_seconds` to `[[gates.run]]` (`GateRun`, default 0, which means no limit; a negative value refused in `validateRuns`) and a `Timeout time.Duration` to `gate.Gate`, set in `gateList` (`gateplan.go`, about line 268). In `execute` (`run.go`, lines 89 to 105), start the gate in its own process group (`SysProcAttr.Setpgid`) and, when the time runs out, kill the group, because `go test` and `bun` leave children that hold the CPU and the pipe past `pipeGrace`. The result is `fail` with the reason `timed out after <n>s`, so blame and bisection treat it as a red gate. Check that a killed `wk run --slot` frees its slot place (its places are held by a lock the process owns) and say what you saw. Write no value into any `~/.wk` file; propose values for wk's four gates in the close-out. Tests: `TestRunKillsAGateThatRunsPastItsTimeout` in `run_test.go` (`sh -c 'sleep 60 & sleep 60'` with a 1 s limit returns `fail` within a few seconds, and the background `sleep` is gone afterwards), and a `timeout_seconds = -1` row in `TestGatesValidation`.
8. **N3, a trial left between the create and its marker.** In `ensureTrial` (`trial.go`), write an intent record before `Env.create` (for example `~/.wk/<repo>/trial.making`, holding the trial's path; `withTrial` already holds `trial.lock`, so no second worker races it) and remove it after the marker is written. When the trial directory exists without the marker and the intent record names it, the directory is this worker's interrupted trial: remove it with `git worktree remove --force` and make the trial again, because a create that stopped partway may have no files or no dependencies. Without the intent record, refuse it as now. Test: `TestTrialInterruptedBeforeItsMarkerIsMadeAgain` (an unmarked worktree at `trial_dir` and an intent record naming it: the batch is gated and the trial is marked afterwards), and `TestTrialRefusesAnUnmarkedDirectory` (line 323) still passes with no intent record.
9. **The trial's dependencies after a lockfile change.** In `withTrial` (`trial.go`, after the `read-tree` at line 31), when `template.refresh_on` is set and the blob at `<tree>:<refresh_on>` differs from the blob the trial's dependencies were last made at, run `template.post_clone` in the trial (the call `internal/wk/new.go` line 129 makes) before the first gate, still under `trial.lock`, then record the new blob (beside the marker in the trial's private git directory, or in `~/.wk/<repo>/`). The first recorded blob is the one at the commit the trial was made from. A failing post-clone step is a red verdict named `post_clone` with `refresh_on` as its path, so blame finds the changeset that changed the lockfile; the reversal is to stop the pass as a start error instead. Tests: `TestTrialRunsPostCloneWhenTheBatchChangesRefreshOn` in `gatekeeper_test.go` (the testrepo's stand-in counts its runs: a batch that changes `bun.lock` runs it once more, the next batch with the same `bun.lock` does not, and a batch that changes it back runs it again).
10. Run the gate below. Write a `## Close-out, <date>` section at the end of this file: what changed, by step; each build-level call with its reversal; each test with whether it fails with its fix turned off; the gate output with its time and load.
11. Review: spawn one Opus 5.5 subagent in its own worktree with your branch checked out there, briefed to read `gatestart.go`, `gateplan.go`'s `runGates`, `trial.go` and `run.go`'s `execute` for any path where a start error is now blamed on a changeset, a red tree now reads as a start error, a timeout leaves a process running, or a recovered trial is someone else's directory. It edits nothing and returns findings with file and line. Check its diffstat, and record each finding's outcome in the close-out.

## Ask before building

Nothing is the owner's call. Steps 3, 7, 8 and 9 each name a default and its reversal; take the default unless a fact you find contradicts it, then say so in the close-out with the fact.

## Owner's action, not the builder's

- The landing timer's `PATH`. A worker started by launchd gets the plist's `PATH`, and the README's plist text under "Land work through the queue" (`README.md`, lines 231 to 266, the `PATH` string at line 251) is `/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`. It lacks `/usr/local/go/bin` (`go`, `gofmt`) and `/Users/zachshort/go/bin` (`golangci-lint`), and a gate's first word is found on the worker's `PATH`, not `[gates.env]`'s. When the owner installs the timer, the plist's `PATH` needs both directories, and the README's plist text should carry them (as `/usr/local/go/bin` and `/Users/USER/go/bin`). The README says the timer is the owner's to install, so the builder changes neither the plist block nor any plist.

## Not in scope, whoever asks

- Any change to the batch policy (`batch.Pick`, `batch_max`, the idle wait), to blame or to bisection (`batch.Blame`, `batch.Halves`, `tryBlame`, `bisect`), or to the green rule.
- Any new configuration key beyond `timeout_seconds` on `[[gates.run]]`. No global timeout, no retry count, no new `[gates]` dial.
- The README's plist block and any launchd plist (the owner's action above), and any write under `~/.wk`.
- The re-gate that refuses a member clean only on top of a left-out changeset ("Left owed", third item); rows 11 and 12's notes on a background child after `pipeGrace` and `Seconds` including `pipeGrace`.
- `internal/batch`, `internal/slot`, `internal/landing`, `gatekeeper.go`, `status.go` and `gc.go`. If a fix seems to need one of them, stop and say why in the close-out.

## Done when

- N2: the new rows of `TestGatesValidation` pass (`needs_env = [""]`, a duplicate name, `trial_dir` of `.`, `./` and `a/..`, `timeout_seconds = -1`).
- N3: `TestTrialInterruptedBeforeItsMarkerIsMadeAgain` passes, and `TestTrialRefusesAnUnmarkedDirectory` still does.
- N4: `TestRunNeedsEnvReadsGatesEnv` and the slotted `needs_env` row in `gatekeeper_test.go` pass.
- N-a: `TestSlottedCommandExit127IsAFail` passes, and `TestSlottedGateExit127IsAStartError` still does with `wk run`'s real lines.
- N-c: `TestCanStartUsesAccessLikeWkRun` passes.
- The landing log: `TestSlottedStartErrorIsNotLoggedAsAFail` passes.
- The `vet:` prefix: the `vet:` rows of `TestPaths` pass.
- The timeout: `TestRunKillsAGateThatRunsPastItsTimeout` passes.
- The trial's dependencies: `TestTrialRunsPostCloneWhenTheBatchChangesRefreshOn` passes.
- Each new test fails with its fix turned off, and the close-out says so.
- The gate below exits 0 in your worktree.

## Hand back

Gate, run in your worktree:

```bash
GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'
```

If it fails only on `TestHookPreToolUseTiming` in `internal/slot` under a load average above about 35, run it again when the load is lower, and report every run with its load.

Hand back the gate output, the diffstat, each new test with its result, the review's findings and their outcomes, and the close-out section in this file. The owner then installs the timer with the `PATH` above when they choose to; after the lead lands this row, `GOTOOLCHAIN=local go install ./cmd/wk` from `main` puts the new binary where the worker runs it. Commit on your worktree branch only, with no `Co-Authored-By:` trailer and no "Generated with" line.
