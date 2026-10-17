# Row 4 builder, Fable 5.1, agent a7ee7d2520d69799a

Launched 2026-10-17 10:49 EDT, returned 2026-10-17 11:09 EDT. Worktree `.claude/worktrees/ap-row-4`, branch `ap/row-4`, fast-forwarded from 5bf4e7d to e13aa27. Commits `4774e55` (code and tests), `484f93d` (README, As built, close-out). Not merged, not pushed; no blocked commit; no trailer. Model check: Fable 5.1, matches the Deep assignment.

## Gates

Full gate twice from the worktree: run 1 at 11:01 EDT (load 6.8 to 10.1) every package `ok` (cmd/wk 12.4 s, landing 20.2 s, wk 72.2 s), `0 issues.`, exit code not captured cleanly; run 2 at 11:05 EDT (load 6.3 to 9.7) `gate exit 0`, tail `ok internal/slot 6.433s` ... `ok internal/wk 69.954s`, `0 issues.` (scratchpad `row-4/gate2.out`). The slot timing test did not fail. P3's "Done when" covered by the proof and `internal/wk/landing_test.go`.

## Diffstat `main...HEAD`

12 files, +1246/-9: README.md 67, cmd/wk/landing_test.go 58 (added), cmd/wk/main.go 93, PLAN-P2-P4.md 16, internal/changeset/queue.go 11, internal/config/config.go 38, internal/config/landing_test.go 60 (added), internal/wk/landing.go 324 (added), internal/wk/landing_test.go 326 (added), internal/wk/status.go 52 (added), internal/wk/submit.go 113 (added), passoff/4 97. Patch: scratchpad `row-4.patch` (1446 lines).

## What changed

- config.go: `Landing{Enabled, Ref, Remote, Push, RequireReview}` under `[landing]`, defaults `refs/heads/wk/landing`, `origin`, `push = true`; validation refuses a ref outside `refs/heads/` and an empty remote; additive; `Show` prints it.
- changeset/queue.go: the `Landing` state constant and `Record.Review` (`omitempty`), needed by the amendments; nothing else in `internal/changeset` or `internal/landing`.
- submit.go: `Env.Submit(dir, message, row, review, out)`: refuses an empty message, the main checkout (D15, `workspaceOnly`, shared with sync) and `landing.enabled = false`; tip = the landing ref or the base branch's hash while the ref is missing; `changeset.Base` (PD-7); lists every `add -A` path on stderr; `changeset.Tree`; the row from the P2 claim unless `--row`; message `row <n>: <message>` lowercased; appends `submitted`; prints the id alone.
- landing.go: `Env.LandLoop` (refuses when landing is off; `flock.TryTake` on `landing.lock`; passes until no `submitted` or `landing` record remains), `landPass` (`landing.Ensure` against `workspace.base`'s hash, then each pending record in order), `landOne` (tree equal to the tip's tree gives `landed` with no commit; no review under `require_review` gives `refused`; `Predict`; conflicts give `refused` with paths and "run wk sync"; a result equal to the tip's tree gives `landed` with no commit; else the `landing` line with the predicted tree, `landing.Land`, `landed`, leases released). A git failure (`ErrMoved`, `ErrDiverged` included) ends the worker with the error logged; the next start re-predicts from the `landing` line. `Env.Sync` (fold a branch with commits to its merge base by `update-ref <branch> <base> <head>`, then `landing.Sync`, print `conflict:` lines, exit 1 on conflicts). `Env.Land` (ref against the base branch: missing, at, n ahead, behind, diverged with the reset line; `landing.Blockers`; the `--apply --expect` line). `Env.LandApply` (`landing.FastForward`, then `landing.Push` when `push`). `landingState` read-only.
- status.go: `Env.Status` = `LeaseStatus` then `QueueStatus`.
- main.go: `wk submit -m <message> [--row <n>] [--review <ref>]`, `wk sync`, `wk land [--loop | --apply --expect <hash>]`; `wk status` calls `Env.Status`; `spawnLand` uses `daemon.Spawn` (skipped for a transient binary; a failure to start is printed, the timer runs it later); usage text.
- Tests: config/landing_test.go (4); wk/landing_test.go (8: the section 3 proof in a fixture with the landed tree checked against `landing.Predict`, the journal line and the empty submit landed without a commit, `require_review`, refusals, the held lock, the fold, land/apply with a blocker, a stale hash and the push to a bare remote, the diverged ref in the worker and in status); cmd/wk/landing_test.go (1: id alone on stdout, includes on stderr, worker and status via the CLI, nine usage refusals).
- README "Land work through the queue" (submit, worker, sync, land, the queue file, D14's rule, the launchd plist text) and the `[landing]` keys. PLAN "As built, P3". passoff/4 close-out with the hand-back lines, the TOML lines and the plist.

## Proof (step 8)

Throwaway repository `row-4/proof/landingproof` with bare `origin.git`, scratch `WK_HOME` with `[landing] enabled = true`, worktrees `a` and `b`, scratch binary, row 10's git shim first on PATH with `WK_GIT_LOG`. `a` edited `web/page.ts` and added `notes.txt`; `wk submit -m "Add The Page" --row 1` printed `c2fee877e9f7` alone; the spawned worker landed it as `169de80` ("row 1: add the page"). `b` edited the same line; submit `9d39e3358ca1` refused with `paths: ["web/page.ts"]` naming `wk sync`. `wk sync` in `b`: synced at 169de80, `conflict: web/page.ts`, exit 1, one `<<<<<<<` line; after the fix, submit `747c7a193eb1` landed as `dcb271e`. Both landed trees equal their `landing` line's predicted tree. Shim log: 127 git calls, 4 `update-ref` (ref creation at main with an empty old value; two CAS swaps with the old value; `refs/heads/b` in sync with the old value), 2 `commit-tree` with the predicted tree; no `update-ref`, `symbolic-ref`, `branch` or `reset` names `main`. `wk status` listed three changesets; `wk land` printed `2 commits ahead of main` and the apply line; `--apply --expect 0000...` refused, exit 1; `--apply --expect dcb271e...` moved main and pushed; `refs/heads/main` in the checkout and in `origin.git` both equal dcb271e6b4a205a42b80ba6b581546b299090001. Nothing touched `~/Projects/wk`'s main, origin or `~/.wk`.

## Build-level calls (reversals in the close-out)

1. The gc amendment (absorbed when the tree equals the landed tree) is not built: gc never forces, so `git worktree remove` would refuse the dirty worktree every pass; `wk sync` in the landed workspace makes it clean and the existing rules remove it. Reversal: a queue read in `takeSurvey` plus a `changeset.Tree` compare in `judge`. Owner question 1.
2. The base branch is `workspace.base` (no `landing.base` key).
3. `wk sync` folds a branch's commits without a flag, saying how many and that they stay in the reflog. Reversal: refuse unless `--fold`. Owner question 2.
4. A record whose tree or prediction equals the tip's tree is `landed` at the tip with no commit.
5. A `landing` record is re-predicted from its predicted tree; no second tree field on `Record`.
6. Timer interval 60 s in the plist text.
7. Steps 7 and 9 not done per the lead. TOML lines for the lead: `~/.wk/ezhomesteading.toml` append `[landing]` `enabled = false`; `~/.wk/wk.toml` append `[landing]` `enabled = true`. Plist label `com.github.zach-short.wk.landing.wk`, program `/Users/zachshort/go/bin/wk --repo /Users/zachshort/Projects/wk land --loop`, log `~/.wk/wk/landing.log`.

## Left owed

Step 7 (the lead, at the land step after the reinstall); step 9 (this run's auditor); the gc amendment; row 10's duplicate tree builder in `landing.Sync`; `internal/daemon`'s unused `Server`/`Client`; row 12's `type State string` note.

## Could not verify

The real launchd load of the timer, the installed binary, a live Claude Code session running `wk submit`. Budget: about 28 files read before building; the work-list completed.
