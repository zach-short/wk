# Row 4 audit 2 (fix round 1), Fable 5.1, agent a2ef7e906338c2dbe

Launched 2026-10-17 11:36 EDT, returned 2026-10-17 11:46 EDT. Detached worktree `.claude/worktrees/ap-row-4-audit-2` at `0adb43f`. Model matches the Deep assignment.

## Verdict: non-blocking findings, three

F1 is fixed. No path remains where the tree on the landing ref differs from a merge of the submitted tree onto the tip it landed on; every ref write carries the old value; no duplicate landing commit and no wrong `landed` hash after a crash on either side of the swap; every gate green. The row can land.

- N7. `remoteBehind` (landing.go:318-325) ignores `landing.push`: with `push = false`, `wk land --apply` moves `main` and leaves `origin/main` behind by design, and `wk land` then prints the "push did not go through" line on every run, false twice. Fix: return "" when `push` is off, or word the line without the hint.
- N8. The `require_review` check now precedes the `Predicted` shortcut, so a `landing` record whose swap went through before a crash is `refused` on the rerun if `require_review` was turned on in between; its commit stays on the ref with a record that says refused and the leases unreleased. Needs an owner config edit between a crash and the next worker; the ref's tree is still right. Fix: test `State == Landing && Predicted == tipTree` before the review check.
- N9. The behind line's hint `wk land --apply --expect <main hash>` pushes only while the landing ref is still at `main`; once a later landing moved the ref, the hinted apply is refused as stale and the owner applies with the ref's hash. The comment at landing.go:316-317 is true only in that window.

Open from audit 1, still open and named in the close-out: N3, N4, N6.

## Reproduction

HEAD 0adb43f; merge-tree with main (703ec3b) exit 0; go mod download ok; `main...HEAD` 12 files +1388/-11, the twelve names exact; `484f93d..HEAD` 15 files +413/-26, the builder's eight as reported, the other seven from the merge of main; `internal/landing` no diff against main; README unchanged this round; no attribution lines. Read in full: the fix diff and the whole of landing.go, submit.go, status.go, queue.go, `internal/landing`, the cmd/wk diff.

## Gates

Full gate at 11:35 EDT, load 5.7: exit 0 first run, every package `ok` (wk 62.6 s, landing 15.4 s, cmd/wk 8.3 s), `0 issues.` Verbose at 11:38, load 6.0: 209 PASS (148 top-level), 0 FAIL, 0 SKIP. Build exit 0; every live run used that binary.

## F1 by reasoning

Readers of `Tree` and `Predicted`: the shortcut (landing.go:98), `Predict` (:101, takes `Base` and `Tree`, never `Predicted`), the landing line (:111 sets `Predicted`), `markLanded` and `refuse` carry the fields unchanged; `Submit` sets `Tree` and `Base` only. `landing.Land` commits `result` with parent `tip` and swaps from `tip`, so a commit on the ref is always the merge of `(Base, tip, Tree)` onto the tip it names as parent, or the ref did not move. Crash after the swap: the rerun's `Ensure` returns the ref's commit, `tipTree == Predicted`, `markLanded` names it; the crashed record is the first pending one in queue order, so the hash is right. After `ErrMoved` or a divergence reset: `Predicted` matches only when the content is already on the ref, else `Predict` from `Tree`. `Base` no longer an ancestor after a reset: `merge-tree --merge-base=<Base>` merges from the submitter's base, which is the definition (live c2). Two `landing` lines per id: `Load` keeps the last. `Predicted` with state `submitted`: impossible by the write order. A queue written by 484f93d would re-predict from the prediction; none exists (never installed).

## Live table

Fresh repositories with a bare origin, scratch `WK_HOME`, the fresh binary, the first auditor's shim plus `WK_KILL_AFTER_SWAP`. (a) kill before the swap, main moved by a revert, rerun: one-file landing, `page.ts = 1`, tree EQUAL to merge-tree, 2 landing lines, 1 landed, 1 commit, ref writes with old values. (b) kill after the swap: the rerun appends `landed` naming the commit already on the ref, no second commit or swap. (c) kill before the swap, hand commit on main, divergence reset, rerun: lands once on the reset tip with only its change, the discarded landing not carried. (c2) base not an ancestor after a reset: lands with main's content and the submitted edit only. (d) kill on `c`'s record, then `b` submits: `c` lands first, `b` on top, each tree EQUAL to its own merge; queue order. (e) N1: `landing.ref = refs/heads/main` refused at config load and at submit with the message; `trunk` refused under `workspace.base = trunk`; `main2` accepted. (f) N2: the refused push prints the behind line from `wk land` and `wk status`; after the hook is removed the hinted apply pushes and the line is gone; (f2) after a further landing the hinted hash is stale (N9); (f3) `push = false` prints the line (N7). (g) N5: an empty submit without a review refused under `require_review`, with `--review` landed at the tip with no commit. (h0) crash after the swap then `require_review = true`: refused with the commit on the ref (N8). Audit 1 rows A1 to A7, B1/C1, C2, C3, B3, B4, C4, C5, B8, C6: same results; B7 now the one-file landing; B9 now refused; 13 ref writes with old values except the creation, 0 naming main.

## Lean

landing.go 340, submit.go 113, status.go 52; `Submit` 40, `landOne` 31, nothing over 40; `remoteBehind` adds two git calls per status, no duplication. `Predicted` could have been avoided (the rerun's `Predict` onto the landed commit returns its tree) at the cost of one `merge-tree` per crashed record and no prediction to assert the commit against; keeping it is defensible. The three tests assert the right things (the diff against the parent, not main; the behind line twice then gone; the empty submit refused).

## Documents

Fix round 1 note present with F1, N1, N2, N5 fixed and N3, N4, N6 open; call 5 reworded with the reversal under the build-level calls; As built corrected; no contractions, dashes or AI-signal words; TOML lines and plist unchanged; README unchanged. One stale sentence in the As built proof paragraph ("the tree of the commit equals the tree of the record's `landing` line", now true of `predicted`).

## Could not verify

The launchd timer, the installed binary, a live Claude Code session, CI's Linux job; a queue file from the first build; `wk sync` with a `landing.Sync` failure after the fold.

Audit worktree status empty; nothing committed; `~/.wk` untouched (newest mtime 10:17); nothing run in `~/Projects/wk` or the builder's worktree.
