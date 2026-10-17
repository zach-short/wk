# Row 4 audit 1, Fable 5.1, agent a0cd3bbbf4c05586f

Launched 2026-10-17 11:11 EDT, returned 2026-10-17 11:26 EDT. Detached worktree `.claude/worktrees/ap-row-4-audit` at `484f93d`. Model matches the Deep assignment.

## Verdict: blocking, one finding

F1. A `landing` record re-predicted after a crash can land a tree the submitter never wrote. `internal/wk/landing.go:109` overwrites `r.Tree` with the predicted tree before the journal line, and `landing.go:99` re-predicts a `landing` record from that predicted tree with the record's original `Base`. When the tip moves between the crash and the rerun (main moved ahead by a hand commit so `Ensure` moved the ref; a hand move of the ref; the status line's own `git update-ref` reset after a divergence), the re-merge treats everything the old tip had changed since `Base` as the changeset's own change and carries it over the new tip. Reproduced live (proof.out, B7): workspace `c` at M0 edits only `docs/NOTES.md`; the worker is SIGKILLed between the `landing` line and the swap; main is moved ahead by a hand commit reverting `web/page.ts` to `1`; the rerun printed `landed 99797d1c4b05 as b44d28c` and the landed commit changed two files, `docs/NOTES.md` and `web/page.ts` back to `"ab"`. The same inputs merged from the submitted tree give `page.ts = 1`. The close-out's claim ("re-predicted from its predicted tree, which merges onto a moved tip as its workspace tree would") is false; the worker printed `landed`, nothing is loud. Fix, about ten lines: keep `Tree` as the submitted tree and journal the prediction in a second field (`Predicted`); the shortcut at `landing.go:93` compares `Predicted` (or `Tree`) to the tip's tree; `Predict` always takes `r.Tree`; `TestLoopLandsWhatIsAlreadyOnTheTipWithoutACommit` needs the new field; add a test: crash, move main ahead with a revert, rerun, assert the landed diff touches only the submitted paths. This is build-level call 5, filed in the close-out with no reversal.

## Reproduction

HEAD 484f93d; merge-tree with main exit 0 at the start (ffa06b6) and the end (52e57fc); go mod download exit 0; diffstat 12 files +1246/-9 and the name list exact; `internal/landing` untouched; the whole change in `internal/changeset` is queue.go (the `Landing` constant, `Record.Review`, two doc comments), justified by the amendments; files outside "Files it owns" are the three test files, queue.go and the item file, each named in the close-out; no attribution lines; audit worktree status empty, nothing committed.

## Gates

Full gate at 11:11 EDT, load 5.6: every package `ok` (wk 55.8 s, landing 15.6 s, slot 6.5 s), `0 issues.`, exit 0 first run. Verbose over wk, config, cmd/wk, landing, changeset at load 5.1: 207 PASS, 0 FAIL, 0 SKIP. Build exit 0.

## Step 9 review

Non-blocking:
- N1. config.go:287 accepts `landing.ref = "refs/heads/main"` (any `refs/heads/` name, including `workspace.base`); live B9: the worker CAS-moved `main` itself and the main checkout was left with ` M f`; PD-6 bypassed by one config line. Fix: `validate` refuses `Landing.Ref == "refs/heads/"+Workspace.Base`.
- N2. landing.go:262-276: after `FastForward` succeeds and `Push` is refused, `main` is moved locally and the command exits 1 with git's message (live C5), but later `wk land` and `wk status` print `at main; nothing to land` with no sign that `origin` is behind.
- N3. Queue writers: `wk submit` appends with no lock (submit.go:52), the worker under `landing.lock`; queue.go:60-65 relies on `O_APPEND` and one `write()` per line; a torn line only from a crash mid-write, after which `Load` (queue.go:91) fails for every reader until a hand repair.
- N4. `foldCommits` (landing.go:212-230) moves the branch before `landing.Sync` runs; a failure after the fold leaves the branch at the merge base with the work in the tree; the stderr line names the reflog hash (true, live C6).
- N5. landing.go:93-98: the tip-tree shortcut precedes the `require_review` check, so an empty submit lands without a review and releases the leases.
- N6. `spawnLand`: a `Spawn` failure is said and the record stays `submitted`; when `installedExe` fails or the binary is transient it returns silently.
- Clean: every `update-ref` carries an old value (8 of 8 and 13 of 13 live, one empty old value each at the creation); `main` named outside `FastForward` only read-only plus `Push` under `LandApply`; a refused record never retried; `TryTake` released by `defer` on every return and on a panic, a SIGKILLed worker leaves the lock free; `releaseLanded` idempotent and scoped to `r.Workspace`; `ErrMoved` and `ErrDiverged` end the worker with the record left `landing` or `submitted` and the rerun lands once (when the tip did not move as F1 exploits); a detached-HEAD submit accepted (option R does not forbid); `--apply --expect` refuses a stale hash, an ancestor hash and a blocker with `main` unchanged.

## Live table (fresh repository, bare origin, scratch `WK_HOME`, scratch binary, row 10's shim)

A1 submit from `a`: id alone, landed as 8a48263 "row 1: add the page". A2 overlapping submit from `b`: refused, `paths: ["web/page.ts"]`, names `wk sync`. A3 `wk sync` in `b`: conflict line, exit 1, one marker; resubmit landed as 4c85f1d. A4 both landed trees EQUAL to merge-tree and the journal. A5 shim: 4 `update-ref` (creation, two swaps with old value, `refs/heads/b` with old value), 2 `commit-tree`, no ref write naming `main`, 100 calls. A6 status and land as expected. A7 apply: `0000...` exit 1 unchanged; an ancestor hash exit 1 unchanged; the right hash exit 0, pushed, both `refs/heads/main` at 4c85f1d. B1/C1 two and three workers: the others leave on the lock, one `landed`, one commit. C2 kill -9 between the `landing` line and the swap: state `landing`, ref unmoved, lock free; rerun lands once, commit tree EQUAL to the journal, parent the unmoved ref. C3 ref moved by hand (`ErrMoved`): worker stops with the reason, record stays `landing`, rerun lands once on top. B3 `enabled = false`: submit and loop refused. B4 main checkout: refused. C4 blocker then stale hash: exit 1, main UNCHANGED. C5 push rejected: local main moved, origin not, later `nothing to land` (N2). B8 divergence: workers stop with the reason, record `submitted`, ref unmoved, status prints the reset command. B7: F1. B9 `landing.ref = refs/heads/main`: accepted, worker moved main (N1). C6 fold: one commit folded, reflog holds it. `~/.wk` mtimes unchanged; nothing run in `~/Projects/wk`.

## Lean

landing.go 324, submit.go 113, status.go 52; `Submit` 40 lines (at the limit), `landOne` 31, `Sync` 29, `landPass` 27. Duplication: `workspaceOnly` and `landing.notPrimary` both refuse the primary; `foldCommits` repeats `landing.Sync`'s ancestor test; the store lock reused through `changeLeases`. The seven calls are the small shape; call 1 defensible (reversal about 30 lines with a decision on forced removal); call 3 defensible (reversal under ten lines); call 5 is the wrong call (F1).

## Documents

README, As built, close-out: sentence case, no contractions, dashes or AI-signal words. Close-out names five calls with reversals, the TOML lines, the plist, the gate runs; calls 5 and 7 sit under other headings, call 5 with no reversal. As built records the proof counts; the builder's `proof.out` section 7 printed `grep: invalid option -- P` three times (BSD grep), so its `update-ref` lines were not shown, but `proof/git.log` lines 19, 23, 67, 90 bear the claims out. Plan section 3 "Done when": all five addressed; steps 7 and 9 owed.

## Could not verify

The launchd timer load, the installed binary, a live Claude Code session running `wk submit`, CI's Linux job; `wk sync` after a fold whose `landing.Sync` fails midway (reasoned, not run).
