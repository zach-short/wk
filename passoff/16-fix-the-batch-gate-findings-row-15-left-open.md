# 16. Fix the batch gate findings row 15 left open

> **Claimed `IN FLIGHT` 2026-10-17.**

*Opened 2026-10-17, from the open findings of row 15's audit (the batch gate fixes, landed on `main` at c77e630): F1, F2 and F4 of the "Sign-off, 2026-10-17" section, with F3 and the review's N3 and N4 from the "Close-out, 2026-10-17" section named below as out of scope. Source: `passoff/15-fix-the-batch-gate-findings-row-5-left-open.md`, its last two sections.*

**Model: Opus 5.5. Lane A. Waits on nothing (row 15 landed). Work in your own worktree. Files it owns: `internal/wk/gatestart.go`, `internal/wk/trial.go`, `internal/wk/gatekeeper_test.go`, `README.md`, and `internal/wk/new.go` only if step 2's reversal is taken.**

## Orientation

You pick up the batch gate after row 15: the landing worker (`wk land --loop`) gates each batch once in a detached trial worktree at `gates.trial_dir`, lands it on green, and on red blames by path, then bisects. Row 15 added a per-gate `timeout_seconds`, recovery of a trial interrupted before its marker, and `post_clone` in the trial after a lockfile change. Its audit left four findings open. This row fixes F1 and F2 and does the style split F4 asks for. Each is small and local; none changes the batch policy.

Read, in this order: `docs/conventions-go.md` (the Go code standard, L1 at line 38); the "Close-out, 2026-10-17" and "Sign-off, 2026-10-17" sections at the end of `passoff/15-fix-the-batch-gate-findings-row-5-left-open.md` (review B2, N3, N4, N5 and audit F1 to F4); `internal/wk/gatestart.go` (`wrapperFailed`, `neverRan`, `waited`, lines 55 to 114); `internal/wk/trial.go` (all of it); `internal/wk/new.go` lines 61 to 110 (`create`); `README.md` line 277.

Session rules, restated here because this prompt is read alone:

- Work in your own worktree branch, made from `main` at its tip. Commit on that branch only; the lead merges, pushes and removes the worktree after the audit. Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Add no `Co-Authored-By:` trailer and no "Generated with" line.
- Never submit, land, fast-forward or push in this repository (`~/Projects/wk`) through `wk` itself, and write nothing under `~/.wk` or `~/Library/LaunchAgents`. A live proof, if you run one, goes in a throwaway repository in the session scratchpad with a `git init --bare` directory as `origin`.
- Run git as `git -C <path>` and use absolute paths. Do not `cd` at the top level of a shell call; use a subshell, `(cd <dir> && ...)`, where a tool needs a working directory.
- Written text (the close-out, the README, comments) uses absolute dates, no contractions and no em dashes.

## Why this exists

The batch gate is the one thing between a submitted changeset and wk's `main`, and the worker runs unattended. F1 is a way it stops the queue and blames nobody when a changeset is at fault: once a live gate sets `timeout_seconds`, a slotted gate that waited for its place, took it, and then deadlocked reads as "the time went to another workspace", so every worker stops on the same batch until a person reads the log. F2 is a message that tells a person a directory is kept for them to inspect while the next worker is about to delete it. F4 keeps the trial code reviewable at a glance, which is how the next audit of it stays cheap. Both bugs are latent on 2026-10-17 (no live gate sets `timeout_seconds`, and the trial exists), which is why they are cheap to fix now and expensive after the first stuck queue.

## What is fixed

Read these before changing anything; do not relitigate them.

- The batch policy, blame and bisection, the green rule (at least one `pass`, no `fail`; a `skip` is never a pass), and the rule that a gate that cannot start is an error that stops the pass and blames nobody are as rows 5, 11 and 15 built them. This row changes none of them.
- `wrapperFailed` (`gatestart.go` lines 70 to 79) turns a slotted gate's `fail` into a start error when `neverRan` (lines 83 to 96) says so. For a timeout (`reason` starts `timed out after `) `neverRan` returns `slices.ContainsFunc(lines, waited)`, and `waited` (lines 112 to 114) matches both `wk run: waiting for a ` and `wk run: took a `. That second prefix is F1. The start error's wording (line 78) is one sentence for every case: `gate <name>: <reason> through wk run --slot, which means its command could not run: <last line>`.
- `wk run` writes `took a <class> place after <duration>` only after a wait, and only once the place is taken (`internal/slot/run.go` line 279), then runs the command. It writes `waiting for a <class> place: ...` while it waits (lines 295 and 306) and `gave up after ...` when `slots.max_wait_seconds` runs out (line 324). So a `took a` line means the command ran; a `waiting for a` line with no `took a` line after it means the command never ran.
- Row 15's review B2 made any timeout after a wait a start error, because the wait counts against `timeout_seconds` (the time starts with `wk run`, not with the command) and time lost to another workspace is not the tree's fault. Its audit F1 narrows that: once the place is taken, the command ran, and a hang is the tree's. `TestSlottedGateTimeoutWhileWaitingIsAStartError` (`gatekeeper_test.go` line 472) has three rows: still waiting (start error), waited then took a place then hung (start error today, a `fail` after this row), never waited (a `fail`).
- `makeTrial` (`trial.go` lines 69 to 93) writes `~/.wk/<repo>/trial.making` naming the trial before `Env.create`, and removes it after the marker. When the create fails and nothing is at the path (lines 78 to 82, row 15's N5), it removes the record too. When the create fails after the directory exists, it returns `create`'s error, which ends `; the workspace is left at <dir> for inspection` (`new.go` line 107), and leaves the record. The next worker's `ensureTrial` (lines 46 to 63) then finds the unmarked directory named by the record and removes it with `git worktree remove --force` (`removeInterruptedTrial`, lines 107 to 116). That is F2: the message says one thing and the next worker does another.
- `create` is shared by `wk new` and the trial. Its "left for inspection" wording is true for `wk new`, where nothing removes the workspace, so it does not change for this row.
- `refreshDeps` (`trial.go` lines 131 to 161) is about 30 lines; `ensureTrial` is 18 and `makeTrial` 25 (F4). Conventions L1 says roughly 6 to 15 lines, extracted by concept, never to relocate lines; a helper that needs three parameters to explain itself was the wrong cut. `refreshDeps` is called once, from `gatePlan`'s gate function (`gateplan.go` line 182), with signature `(dir, tree, tipTree string, out io.Writer) (verdict, error)`; keep that signature.
- The tests that pin the trial's behaviour today: `TestTrialRefusesAnUnmarkedDirectory` (line 325), `TestTrialInterruptedBeforeItsMarkerIsMadeAgain` (line 541), `TestTrialIntentNeverTakesABranch` (line 563), `TestTrialRunsPostCloneWhenTheBatchChangesRefreshOn` (line 598), `TestTrialPostCloneFailureBlamesTheLockfileChange` (line 627), `TestTrialPostCloneFailureWithoutALockfileChangeBlamesNothing` (line 650).

## Do these, in order

1. **F1, a timeout after the place was taken is a fail.** In `neverRan` (`gatestart.go`), a timeout is a start error only when the output has a `wk run: waiting for a ` line and no `wk run: took a ` line, because only then did the command never run. With a `took a` line, the command ran and the gate is a `fail`, blamed or bisected like any red gate. Give the still-waiting start error its own wording in `wrapperFailed`, in place of "which means its command could not run": for example `gate <name>: timed out after <n>s while still waiting for a place through wk run --slot, so its command never ran; the next worker gates the batch again`. The `fail`'s reason stays `timed out after <n>s`, set in `internal/gate`; the landing log already echoes the gate's output, `took a <class> place after <duration>` included, so a person can see how much of the time went to the wait. Build-level call: a long wait followed by a command that needs more than the time left is now a `fail` too (row 15's B2 case); bisection gates the batch again, usually without a wait, so only a batch of one is failed by it. Reversal: keep the start error when the `took a` duration is most of the limit, which needs a threshold; say in the close-out if you saw a reason to want it. Update `waited`'s comment and the bullet in `wrapperFailed`'s doc comment (lines 64 to 66), and rewrite README line 277's sentence about a slotted gate's timeout to say both cases. Tests: move the second row of `TestSlottedGateTimeoutWhileWaitingIsAStartError` out into a new test, `TestSlottedGateTimeoutAfterTakingAPlaceIsAFail`, where the changeset ends `failed` with the timeout reason (and, with the default, the wait); the remaining still-waiting row checks the new wording. The new test fails with the fix off (the old `waited`).
2. **F2, the failed trial create says what the next worker does.** Default: keep `trial.making` when the create fails after the directory exists, because that directory is exactly the unfinished trial the record exists for, and keeping it lets the queue recover without a person. Change the error `makeTrial` returns in that case so it no longer promises the directory is kept: wrap `create`'s error with a sentence saying the next worker removes this unfinished trial with `git worktree remove --force` and makes it again, so copy anything needed out of it first. Do not change `create`'s own wording, which `wk new` needs. Reversal: remove the record when the error is returned; the "left for inspection" wording is then true, and the next worker refuses the unmarked directory until a person removes it. Test: `TestTrialCreateFailureSaysTheNextWorkerMakesItAgain` in `gatekeeper_test.go`: make the trial's create fail after `worktree add` (a failing `template.post_clone` or a `copy` entry that cannot be read, whichever the fixture makes cheap; say which in the close-out), check that the loop's error names the removal and that `trial.making` still names the trial, then let the create succeed and check the next loop gates the batch and marks the trial. It fails with the fix off on the wording check.
3. **F4, conventions L1 in `trial.go`.** Split `refreshDeps`, `ensureTrial` and `makeTrial` by concept, toward 6 to 15 lines each. Candidate cuts, which you may better: in `refreshDeps`, the run-and-read of the post-clone commands and the red-or-start-error choice after a failure; in `makeTrial`, the marker write and the failed-create cleanup from step 2; in `ensureTrial`, the handling of a directory that the record names. Behaviour does not change. Test: the six trial tests named in "What is fixed", step 2's test, and every other test in `internal/wk` pass with no edit to their bodies in this step's commit. A refactor has no fix to turn off; say so in the close-out instead of a fix-off result. Do this step after step 2, because step 2 changes `makeTrial`.
4. Run the gate below. Write a `## Close-out, <date>` section at the end of this file: what changed, by step; each build-level call with its reversal; each test with whether it fails with its fix turned off; the line counts of the three functions and their new helpers; the gate output with its time and load.
5. Review: spawn one Opus 5.5 subagent in its own worktree with your branch checked out there, briefed to read `gatestart.go`'s `neverRan` and `wrapperFailed` and all of `trial.go` for any path where a start error is now blamed on a changeset, a red tree now reads as a start error, a stopped or failed trial create leaves a record that names a directory that is not the trial, or the split changed an order of operations (record, create, marker, record removal; `unknown`, run, blob). It edits nothing and returns findings with file and line. Check its diffstat, and record each finding's outcome in the close-out.

## Ask before building

Nothing is the owner's call for the build. Steps 1 and 2 each name a default and its reversal; take the default unless a fact you find contradicts it, then say so in the close-out with the fact.

These stay open, as owner decisions, and are not built here:

- **owner's call:** F3. `post_clone` in the trial (`trial.go` line 150) runs with no timeout, under `trial.lock`, so a hung install holds the worker for ever. A limit needs a new configuration key (a `template.post_clone_timeout_seconds`, or reusing a `[gates]` dial), and each choice changes what `wk new` promises too. Either answer may be no while no install has hung.
- **owner's call:** review N3 of row 15. A gate with `timeout_seconds` runs with `Setpgid`, outside the worker's process group, so a Ctrl-C or a group kill of the worker leaves it (and a slotted gate's place) running, and the next worker could `read-tree` under it. Row 15 named the fix (catch SIGINT, SIGTERM and SIGHUP in `runFor` in `internal/gate/run.go` and kill the group before exiting). It touches signal handling for the whole worker, so it is its own row if wanted. Latent while no live gate sets `timeout_seconds`.
- **owner's call:** review N4 (from row 11 and row 5's B1). A changeset that deletes or breaks a gate's own script reads as a start error and stops the queue. Telling that from a missing tool needs a policy on what a gate's own files are.

## Not in scope, whoever asks

- F3, review N3 and review N4 above.
- Any change to the batch policy (`batch.Pick`, `batch_max`, the idle wait), to blame or to bisection (`batch.Blame`, `batch.Halves`, `tryBlame`, `bisect`), or to the green rule.
- Any new configuration key, and any change to `timeout_seconds`'s meaning beyond step 1 (the time still counts from the start of `wk run`).
- `create`'s wording in `new.go`, unless step 2's reversal is taken.
- `internal/gate`, `internal/slot`, `internal/batch`, `internal/landing`, `internal/config`, `gatekeeper.go`, `gateplan.go`, `status.go` and `gc.go`. If a fix seems to need one of them, stop and say why in the close-out.
- Writing `timeout_seconds` into any `~/.wk` file, and the landing timer's plist `PATH` (the owner's action from row 15).

## Hand back

Gate, run in your worktree:

```bash
GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'
```

If it fails only on `TestHookPreToolUseTiming` in `internal/slot` under a load average above about 35, run it again when the load is lower, and report every run with its load.

Done when:

- F1: `TestSlottedGateTimeoutAfterTakingAPlaceIsAFail` passes and fails with the fix off; `TestSlottedGateTimeoutWhileWaitingIsAStartError` passes with its still-waiting and never-waited rows and checks the new wording.
- F2: `TestTrialCreateFailureSaysTheNextWorkerMakesItAgain` passes and fails with the fix off.
- F4: `refreshDeps`, `ensureTrial` and `makeTrial` are each near 15 lines or under, and the six trial tests pass unedited.
- README line 277 says both timeout cases.
- The gate above exits 0 in your worktree.

Hand back the gate output, the diffstat, each new test with its result, the review's findings and their outcomes, and the close-out section in this file. After the lead lands this row, `GOTOOLCHAIN=local go install ./cmd/wk` from `main` puts the new binary where the worker runs it. Commit on your worktree branch only, with no `Co-Authored-By:` trailer and no "Generated with" line.
