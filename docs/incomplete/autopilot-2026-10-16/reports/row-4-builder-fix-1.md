# Row 4 fix round 1, Fable 5.1, agent a7ee7d2520d69799a

Sent 2026-10-17 11:27 EDT, returned 2026-10-17 11:34 EDT. Commit `0adb43f` on `ap/row-4` after a merge of main (`6a840d4`, brings 941667c and the row state files). Not merged, not pushed, no `go install`, nothing under `~/.wk`. No trailer. Working tree clean.

Gate at 11:32 EDT, load 6.4: exit 0 first run (cmd/wk 6.0 s, changeset 4.5 s, config 1.7 s, landing 13.2 s, wk 56.8 s), `0 issues.` Output at scratchpad `row-4/gate-fix1.out`.

## Per finding

- F1: `changeset.Record` gains `Predicted string` (`omitempty`), with `Tree` the submitted tree for the record's whole life and `Predicted` right only against the tip it was made on. `landOne` predicts from `r.Tree` and `r.Base` on every pass, sets `r.Predicted` on the `landing` line, and the no-commit shortcut is `r.Tree == tipTree || r.Predicted == tipTree`. `TestLoopLandsWhatIsAlreadyOnTheTipWithoutACommit` sets `Predicted`. New `TestLoopAfterACrashLandsOnlyTheSubmittedChange`: `a` lands and is applied; `b` edits `docs/incomplete/plan/PLAN.md`; a git shim first on PATH exits 137 on the first swap of the landing ref; `LandLoop` returns an error with the record `landing` and `Predicted != Tree`; main moves ahead by a hand revert of `web/page.ts` to `1`; the rerun lands, `git diff --name-only <commit>^ <commit>` is exactly the PLAN file and `<commit>:web/page.ts` is `export const page = 1;`. Close-out and As built corrected; a new bullet names the two-field journal with the single field as the reversal.
- N1: `Config.validate` refuses `landing.ref == "refs/heads/" + workspace.base` (PD-6 comment); two rows in `TestLandingValidation`.
- N2: `landingState` gains `remote` from `remoteBehind(main)`: reads `refs/remotes/<remote>/<base>`; when it exists, is not at the base branch and is an ancestor of it, one line "origin/main is behind main at <hash>, so the push did not go through; wk land --apply --expect <hash> pushes it", printed by `Land` and `QueueStatus`. New `TestLandAndStatusSayWhenThePushWasRefused` with a bare origin and a `pre-receive` hook exiting 1; the line appears twice, then disappears after the hook is removed and the apply pushes.
- N5: `require_review` precedes the tip-tree shortcut; `TestRequireReviewRefusesAChangesetWithoutOne` adds an empty submit without a review, refused.
- N3, N4, N6 left open, one line each. `### Fix round 1, 2026-10-17` appended.

## Live crash case

`proof.sh` section 8 with the audit's shim (`WK_KILL_ON_SWAP`), binary rebuilt from the fixed tree (`row-4/proof-fix1.out`; sections 1 to 7 as before). Worktree `c` at M0 edits only `docs/NOTES.md`; submit `c3abe25423fa`; the shim killed the worker before `update-ref refs/heads/wk/landing 1dfd211 b4269a7`; the record was `landing` with `tree` and `predicted` different; main moved ahead to `bef0f6d` by a hand revert; `wk land --loop` exit 0 printed `landed c3abe25423fa as 7a7a627`, parent `bef0f6d`, ref at `7a7a627`; `page.ts` in the landed tree is `export const page = 1;`, `NOTES.md` is the edit; `git diff --stat` against the parent: `docs/NOTES.md | 2 +-`, 1 file changed. Two `landing` lines and one `landed` line for the id. Ref writes over the proof: seven on the landing ref (creation with an empty old value, each swap with its old value, the killed swap logged and never run) and one on `refs/heads/b`; naming `main`: 0. Note: the proof's section 4 compares the submitted tree (a `"tree":"` pattern), equal to the prediction in sections 1 and 3 because the base was the tip; section 8's diff check is where they differ.

## Diffs

`main...HEAD`: 12 files +1388/-11 (landing.go 340, landing_test.go 425, queue.go 18, config.go 43, config/landing_test.go 62, status.go 52, submit.go 113, cmd tests 58, main.go 93, README 67, PLAN 16, passoff/4 112). `484f93d..HEAD`: 15 files +413/-26, of which the builder's: queue.go 9, config.go 5, config/landing_test.go 2, landing.go 38, landing_test.go 101, status.go 2, PLAN 2, passoff/4 17; the rest from the main merge. Patch refreshed (1590 lines). landing.go 340, submit.go 113 (`Submit` 40), status.go 52; `landOne` 31, `Sync` 29, `landingState` 23, `remoteBehind` 8.

## Could not verify

The launchd timer, the installed binary, a live Claude Code session; the retry rule not exercised (slot test passed). The unit test's crash is a failed `update-ref` (exit 137 from the shim) rather than a SIGKILL of the worker; the live section 8 used the real kill -9.
