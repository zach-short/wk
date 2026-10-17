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

## Close-out, 2026-10-17

Builder: Opus 5.5, one agent, in `.claude/worktrees/dl-row-15-build` on `dl/row-15-build`, from `main` at 7851ae9 (`git merge --ff-only main` said "Already up to date" before each gate). Build commit b361308; the review fixes and this section are in the commit after it.

### What changed, by step

1. N2, config. `Gates.validate` refuses a `trial_dir` whose `filepath.Clean` is `.` (`.` and `./`; `a/..` was already refused by `checkRelative`, which refuses any `..` part). `validateRuns` now calls `GateRun.validate` per entry and refuses a second gate of a name already used, naming both indexes. `GateRun.validate` refuses a `needs_env` entry that is empty or holds `=` or a space (the `fresh_caches` rule, now one function, `variableName`) and a negative `timeout_seconds`. Each refusal names the entry.
2. N4. `runOne` reads `needs_env` from the environment slice the gate runs with (`unmet`; a name present anywhere in the slice is set), so `[gates.env]` counts. `gate.Skips(g, env)` gives the same answer from the env map, and `canStart` calls it in place of the old `skips`, which read the worker's environment. The comments on `GateRun` and `gate.Gate` and the README sentence say "the environment the gate runs with".
3. N-a. `wrapperFailed` returns a start error for exit 75, 126 or 127 only when every non-empty line of the output is `wk run`'s (or the shell's line about the `wk` binary, below) and one of them is the line `wk run` writes as it gives up: `wk run: gave up after` for 75, `wk run: <argv0>: ` for 126 and 127. A slotted command's own 127, with output of its own, is now a fail and is blamed or bisected. The README sentences that said a slotted command's own 126 or 127 stops the queue are rewritten.
   The `/bin/sh: ...: not found` line: it means the `wk` binary the gate is routed through is gone (removed, or mid-replacement), so the gate never ran. It is read as a start error when the line starts `/bin/sh: ` and holds the binary's path followed by `: ` (bash prints `/bin/sh: <wk>: No such file or directory`, dash `/bin/sh: 1: <wk>: not found`; the bash form was checked on this machine, exit 127). `canStart` does not check the binary first, because `WKExe` is the running worker's own binary, so it is missing only while it is being replaced.
4. N-c. `executable` in `gatestart.go` uses `unix.Access(p, unix.X_OK)`, as `wk run` does.
5. The landing log. `runGates` calls the new `runGate`, which gives `gate.Run` a buffer as its log, copies it to the log only when `wrapperFailed` returns nil, and otherwise writes the start error and then the output's tail (the new `gate.Tail`, the same 20 lines `gate.Run` echoes). `gate.Run`'s signature is unchanged.
6. The `vet:` prefix. `candidates` strips a leading `vet: ` before the position match.
7. A per-gate timeout. `timeout_seconds` on `[[gates.run]]` (0 is no limit) becomes `gate.Gate.Timeout` in `gateList`. `execute` calls `runFor`, which, with a limit, starts the gate with `Setpgid` and sends SIGKILL to the whole group when the limit runs out; the result is `fail`, reason `timed out after <n>s`. Without a limit the gate stays in the worker's process group, as before.
   A killed `wk run --slot` frees its place: places and tickets are flocks (`internal/slot/queue.go`), which the kernel drops when the process dies. Seen live on 2026-10-17 15:36 EDT with a binary built from this branch and `WK_HOME` in the scratchpad (`[slots.check] capacity = 1`, `max_wait_seconds = 3`): a holder `wk run --slot check -- sh -c 'sleep 60 & sleep 60'` led its own group; a second run printed `waiting for a check place: 1 of 1 held by pid 26737 ...` and `gave up after 3s ...`, exit 75; after `kill -9 -26737` no `sleep 60` was left, and the next `wk run --slot check -- echo took` took the place at once (`real 0.00`), exit 0.
   Proposed values for wk's four gates (`~/.wk/wk.toml`, not written): `gofmt` 60, `vet` 300, `test` 900 (`internal/wk` alone took 133 s at load 15 and 209 s at load 103 to 131 in this row's gates), `lint` 600. wk has no slots file, so none of the four waits for a place.
8. N3. `ensureTrial` now makes the trial through `makeTrial`, which writes `~/.wk/<repo>/trial.making` (the trial's path) before `Env.create` and removes it after the marker. A trial directory without the marker that the record names is removed with `git worktree remove --force` (run in the main checkout, so only a worktree of this repository can go) and made again; one on a branch is refused, because the trial is always detached. Without the record, an unmarked directory is refused as before. A marked trial with a leftover record (a stop between the marker and the record's removal) only has the record removed.
9. The trial's dependencies. `refreshDeps` (`trial.go`), called first in `gatePlan`'s gate function, so after the `read-tree` and still under `trial.lock`, runs `template.post_clone` in the trial (through `gate.Run`, one gate named `post_clone` per command, no `[gates.env]`, as `Env.create` runs them) when the tree's `refresh_on` blob differs from the recorded one. The record is `wk-trial-deps` beside the marker; before it exists, the blob at the trial's HEAD, which is the commit the trial was made from (`read-tree` never moves HEAD). The record says `unknown` while the commands run, so a stop or a failure makes the next batch run them again. A failure is a red verdict named `post_clone` with `refresh_on` as its path when the batch changes the file; see review B1 for the other case. `gitx.Blob` is not used: `blobAt` reads `rev-parse <tree>:<path>` directly. Brief step 9 says "in `withTrial`"; the call is in `gatePlan`'s closure, which `withTrial` runs right after the `read-tree`, because the result must be a verdict and `withTrial`'s callback returns only an error.
10. The gate, below. 11. The review, below.

### What is now fixed

Row 5's N2, N3, N4, N-a and N-c; the landing log's `fail, exit status 75` line before a start error (audit 3's observation); and from "Left owed": the `vet:` prefix, the missing per-gate timeout, and the trial's dependencies after a lockfile change.

### Questions answered

- What a `/bin/sh` "not found" line in front of `wk run` means: the `wk` binary is gone; a start error (step 3).
- Does a killed `wk run --slot` free its place: yes, seen live (step 7).

### Build-level calls, each with its reversal

- The 75, 126 and 127 rule also asks for `wk run`'s give-up line, not only that every line is `wk run`'s, because `wk run` also writes `took a ... place` and `running unslotted: ...` before a command that may then exit 127 silently. Reversal: drop `givesUp` and keep only the all-lines rule.
- `/bin/sh`'s line about the `wk` binary counts as the wrapper's own (step 3). Reversal: drop `shellLine`; a missing `wk` binary is then a fail blamed on a changeset.
- `Setpgid` only for a gate with a limit (step 7). Reversal: always, which gives every gate the same process shape but takes every gate out of the worker's group (see review N3).
- A slotted gate that waited for a place and then timed out is a start error (review B2, below). Reversal: drop the timeout case in `neverRan`; the wait then counts as the tree's time, and a busy slot fails a changeset.
- A failing `post_clone` for a batch that does not change `refresh_on` is a start error (review B1). Reversal, as the brief names it: every `post_clone` failure a start error; or every one red.
- `TestGatesValidation`'s new rows check that the error names the entry, not only that `Load` refuses.

### Tests, each with whether it fails with its fix turned off

Each was run with only its fix disabled, then the file restored (`scratchpad/row-15/fixoff.py`, `fixoff2.py`, `fixoff3.py`, output in `fixoff.out`).

- `TestGatesValidation` new rows: fail with the `trial_dir` check off (`.`, `./`), the `needs_env` check off (three rows), the duplicate check off, the timeout check off. The `a/..` row passes either way, because `checkRelative` refused it before this row.
- `TestRunNeedsEnvReadsGatesEnv`: fails with `runOne` on the worker's environment.
- `TestSlottedGateNeedsEnvMetByGatesEnvIsChecked`: fails with `canStart` on the worker's environment.
- `TestSlottedCommandExit127IsAFail`: fails with the old rule (any 75, 126 or 127 a start error).
- `TestSlottedGateExit127IsAStartError` (rows now print `wk run`'s real lines): passes; its new row (a line after the give-up line) fails with the give-up line read from the last line only.
- `TestSlottedGateWithoutTheWkBinaryIsAStartError`: fails with `shellLine` off.
- `TestCanStartUsesAccessLikeWkRun`: fails with the mode-bit rule. Skipped as root, with the reason in the skip message.
- `TestSlottedStartErrorIsNotLoggedAsAFail`: fails with the echo written straight to the log.
- `TestPaths` "go vet type error": fails with the prefix kept; its "not a file" row names nothing either way, as it should.
- `TestRunKillsAGateThatRunsPastItsTimeout`: fails with only the leader killed (8 s, the background `sleep` alive).
- `TestRunTimeoutAfterTheLeaderExitedIsNotATimeout`: fails without the signal check.
- `TestSlottedGateTimeoutWhileWaitingIsAStartError`: fails with the timeout case off.
- `TestTrialInterruptedBeforeItsMarkerIsMadeAgain`: fails with the intent record ignored. `TestTrialRefusesAnUnmarkedDirectory` still passes.
- `TestTrialIntentNeverTakesABranch`: fails with the branch check off.
- `TestTrialRunsPostCloneWhenTheBatchChangesRefreshOn` and `TestTrialPostCloneFailureBlamesTheLockfileChange`: fail with `refreshDeps` off.
- `TestTrialPostCloneFailureWithoutALockfileChangeBlamesNothing`: fails with the tip-blob check off.

### Review

One Opus 5.5 reviewer in the detached worktree `.claude/worktrees/dl-row-15-review` at b361308, told to edit nothing. `git status --porcelain` printed nothing and `git diff --stat b361308` was empty afterwards; the worktree was removed with `git worktree remove`. Findings and outcomes:

- B1 (blocking): after one failing `post_clone`, the record says `unknown`, so every later batch runs it, and an environment failure (a registry down) was red with path `bun.lock` for batches that never touched it; bisection then failed each one. Fixed: `refreshDeps` takes the tip's tree and is red only when the batch changes `refresh_on`, else a start error. Test `TestTrialPostCloneFailureWithoutALockfileChangeBlamesNothing`. A batch of one that changes the lockfile while the registry is down is still red and failed; nothing can tell that from a bad lockfile.
- B2 (blocking as defined; the reviewer left it to the owner): the slot wait counts against `timeout_seconds`, so a long wait failed a changeset. Decided here, under the owner's delegation of judgment calls: a slotted gate that timed out after a `wk run: waiting for a` or `wk run: took a` line is a start error, because the time went to another workspace and a stop is loud and clears on the next worker; a slotted gate that never waited and times out is still a fail. Test rows in `TestSlottedGateTimeoutWhileWaitingIsAStartError`; README updated.
- N1: the give-up line need not be last (`cannot write ... runs.log` can follow it). Fixed: it is looked for on any line. Test row added.
- N2: a leader that exits by itself near the limit while a background child holds the pipe read as timed out. Fixed: a timeout needs a death by signal. Test `TestRunTimeoutAfterTheLeaderExitedIsNotATimeout`.
- N3: `Setpgid` takes a timed gate out of the worker's group, so a Ctrl-C or a group kill of the worker leaves the gate (and a slotted gate's place) running, and the next worker could `read-tree` under it. Not fixed: left owed. No live gate sets `timeout_seconds` yet. The fix is to catch SIGINT, SIGTERM and SIGHUP in `runFor` and kill the group before exiting.
- N4 (from row 11 and row 5's B1, not this row): a changeset that deletes or breaks a gate's own script reads as a start error and stops the queue. Left owed; out of scope.
- N5: a create that fails before the directory exists left `trial.making` behind, which could later name a worktree a person makes there. Fixed: `makeTrial` removes the record when the create fails and nothing is at the path. No test (a create that fails before `worktree add` needs a broken template or pool).

### Left owed

- Review N3 (signals to a worker while a timed gate runs) and N4 (a changeset that breaks a gate's own command is a start error).
- The owner's action on the plist `PATH`, unchanged.
- The items "Not in scope" names: the re-gate that refuses a member clean only on top of a left-out changeset; rows 11 and 12's notes.
- `timeout_seconds` values for wk's gates (proposed in step 7) are the owner's to write.

### Gate

`GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'` in the worktree:

- 2026-10-17 15:38:37 to 15:41:07 EDT at b361308: exit 0, every package `ok` (`internal/wk` 132.9 s, `internal/gate` 5.7 s), `0 issues.`, 150 s, load 15.09 at the start and 25.66 at the end.
- 2026-10-17 15:57:48 to 16:01:50 EDT on the final tree (review fixes): exit 0, every package `ok` (`internal/wk` 209.0 s, `internal/gate` 7.9 s, `internal/slot` 7.1 s), `0 issues.`, 241 s, load 102.95 at the start and 130.72 at the end. `TestHookPreToolUseTiming` passed at that load.

### Sign-off, 2026-10-17

- Builder: Opus 5.5, worktree `.claude/worktrees/dl-row-15-build`, branch `dl/row-15-build`, one pass, no relay. The worktree was made by the lead with `git worktree add`, because the Agent tool's isolation could not make a wk worktree from the lead's session.
- Auditor: Opus 5.5, worktree `.claude/worktrees/dl-row-15-audit`, branch `dl/row-15-audit`. It reproduced the build by fast-forward to e528cc9 (12 files, 838 insertions, 67 deletions), ran the gate (exit 0, 190 s, load 28.06 to 55.04), and turned each fix off: every new test failed with its fix off and passed after the restore.
- Verdict: non-blocking findings. Commit state: landed on `main` by fast-forward after this commit.
- F1, open. `gatestart.go` lines 112 to 113: a slotted gate that waited for its place, took it (the `wk run: took a ` line), and then ran past its limit reads as a start error, though its command ran. A changeset that deadlocks then stops the queue while the slot is busy, and nobody is blamed. Latent on 2026-10-17: no live gate sets `timeout_seconds`, and wk has no slots file. Fix: a start error only when no `took a` line is present, with its own wording; the README sentence at line 277 changes with it.
- F2, open. `trial.go` lines 79 to 83 with `new.go` line 107: a create that fails after its directory exists says the directory is left for inspection, but `trial.making` stays, so the next worker removes that directory with `git worktree remove --force`. Fix: the wording, or remove the record when the error is returned.
- F3, open. `trial.go` line 150: `post_clone` in the trial runs with no timeout, under `trial.lock`, so a hung install holds the worker. A limit needs a key, which this row's "Not in scope" forbids.
- F4, open. Style (conventions L1): `refreshDeps` is about 30 lines; `ensureTrial` and `makeTrial` are near 20.
- Review findings N3 (a Ctrl-C or group kill of the worker leaves a timed gate running) and N4 (from row 11: a changeset that breaks a gate's own script reads as a start error) stay open as the close-out says.
