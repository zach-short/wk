# Autopilot run, wk, 2026-10-26

## 1. In one paragraph

The run started 2026-10-26 at 02:27 EDT on Opus 5.5 with rows 21, 22 and 23 in flight (row 20 had landed as v0.1.6 before the run) and ended at 07:03 EDT with nothing in flight. It paused once, when the session limit stopped every agent at about 03:16 EDT until the reset at 06:30 EDT. Rows 21 and 23 landed on `main` and were pushed: row 21 is `wk stats`, a report of slot use, queue overlap, long waits, routing and deadline misses and warm times (c0449b2, three audits, two fix rounds); row 23 makes the two slotted gate timeout tests fire their kill through a test seam so they no longer race a 1 s wall clock under load (547b0f6, one audit). Row 22, the hook's git lookup and the wider routing, is held after its third blocking audit: everything but one redirect form audited clean, and the fix is about five lines. The release tag v0.1.7 was refused by the permission classifier, so `main` at 547b0f6 is unreleased. A side task outside the board, a per-binary test database for ezhomesteading, finished on that repository's branch `wk/298` and waits on the owner.

## 2. Audit

Not run (wk has no `docs/audit-lanes.md`).

## 3. Rows

| Row | Tier, model | Worktree, branch | Gates seen | Audits | Fix rounds | Status |
|---|---|---|---|---|---|---|
| 20 | Default, Opus 5.5 | `wk-20`, `wk/20` | green at d271a4e | none (built in this session before the run) | 0 | DONE, d271a4e, released v0.1.6 |
| 21 | Default, Opus 5.5 | `agent-a542831c0a9a9ea05`, `worktree-agent-a542831c0a9a9ea05` | green at 142d851, 2532911, c0449b2 | Opus three times: blocking (pool and template split wrong when creates overlap), blocking (the wording made "template: none" read as no clones), clean | 2 | DONE, c0449b2 |
| 22 | Default, Opus 5.5 | `agent-a386db652fd7aa7e4`, `worktree-agent-a386db652fd7aa7e4` | green at 33d65e1, 6c2bc1c; 2c7432f green with `internal/wk` run alone after the row 23 flake; green on the merge with main (2047ee6) | Opus three times: blocking (CommonDir disagreed with git; `2>&1` inside `$( )`), blocking (fd-dup stderr forms), blocking (`>&/dev/stdout` with no fd number) | 2 | HELD |
| 23 | Default, Opus 5.5 | `agent-a0bc3c6ff2eab3424`, `row-23-build-2` | green at 547b0f6 | Opus once, clean | 0 | DONE, 547b0f6 |

The builder and audit reports are under **Reports** in `STATE.md` beside this file.

## 4. What the numbers say

`wk stats --since 2026-10-17 --until 2026-10-26` on the real logs, checked by an independent count: 4141 slot runs, 3715 of them in the check class; 113.1 hours of agent time spent waiting, inside 8.4 hours of wall clock with a waiter; a peak of 26 queued; 118 waits over 10 minutes worth 106.4 hours, held mostly by backend `go test` runs. The missed-routing log has 112 lines (68 "already holds wk run", 14 `time`, 11 `$( )`, 9 `env`, 5 `sh -c`), and errors.log has 42 hook deadline misses at 202 to 599 ms. Row 22's builder measured the hook at 5 to 25 ms before its change and under 1 ms after, because one `git rev-parse` took 85 to 95 percent of the hook's time.

## 5. Defects found and not fixed

- Row 22, blocking and held: `internal/slot/match.go:242` rewrites a gate inside `$( )` that carries `>&/dev/stdout` or `>&/dev/fd/1` with no fd number, so wk run's stderr lines enter the capture. The fix and the non-blocking notes are in the row's item file under "Held, 2026-10-26".
- Row 21, open and non-blocking: `wk stats` cannot attribute a warm create while another create is open, because hook.log does not name the create on its `claim pool slot` and template clone lines. On the real log every template clone overlaps another create, so ezhomesteading shows "template: none attributed" beside 200 unattributed creates. The fix is a hook change after row 22 lands. Smaller items: a bad `wait=` or `run=` value parses as 0; runs are filtered by end time; the text header omits the "warm line with no create" cause.
- Row 23, non-blocking: the fake timer's stop-before-fire branch and its 30 s backstop are checked by reading only.
- The 200 ms hook deadline misses were not reproduced under CPU load; the builder's best explanation is that a git process start stalls under memory pressure (swap was 7.6 GB in use). Row 22's change removes the git process from the common path, so the misses should stop once it lands.

## 6. Collisions and merge order

Row 22's branch edits `internal/wk/gatekeeper_test.go`, which row 23 owned. Row 23 was told to leave `TestSlottedRoutesOnlyAMatchedGate` alone, and row 22's branch merged cleanly with `main` at c0449b2 in its third audit. It has not been merged with 547b0f6 yet; row 23 changed other functions in that file.

The first row 23 builder stopped at the session limit before any edit, and `wk gc` removed its clean worktree in the gap. A second builder rebuilt the row from the same decision.

## 7. Owner decisions

1. Release: tag v0.1.7 on 547b0f6 (`git tag -a v0.1.7 -m "wk v0.1.7" 547b0f6 && git push origin v0.1.7`), then `brew upgrade zach-short/tap/wk`. The run's tag push was refused by the permission classifier.
2. Row 22: lift the hold. One builder pass fixes `match.go:242` (and normalizes the target path at `:252`), merges `main`, and one audit checks the substitution rule again.
3. ezhomesteading branch `wk/298` (commit 21808dde0): each DB test binary runs on its own copy of the test database, and the five suite advisory locks are gone. Two full suites at once took 232 s each against 705 s and 708 s on `main`. CI has not run it. If it merges, backend `go test` can move from the `db` slot class (capacity 1) to a class with more places, which is an edit to `~/.wk/ezhomesteading.slots.toml`. The agent's container `ezh-testdb-298` is still running (`docker rm -f ezh-testdb-298`).
4. `/usr/bin/time -p go test` is not routed; row 22 routes the bare `time` keyword only.

## 8. Judgment calls

Each is in `STATE.md` under **Judgment calls** with its reversal: row 21's attribution rule, row 22's and row 23's cell changes, the decider's timer design for row 23, letting row 23 build while row 22 is held, and the restart of row 23 after the session limit.

## 9. Worktrees

None removed by hand. wk's hourly `gc` removes a worktree that is clean, quiet and merged. Row 22's builder worktree and its three audit worktrees hold unmerged commits and stay.

## 10. After the run, 2026-10-26

The owner tagged v0.1.7 on 547b0f6, and `brew upgrade` installed wk 0.1.7 at 11:45 EDT. At 11:47 EDT the owner lifted row 22's hold. One builder pass merged the held branch into `main` with no conflict and fixed the audit 3 item (2942187). Audit 4 found one more blocking form, a leading-zero fd such as `02>&1` that bash reads as fd 2, and one fix round closed it (505528c). Audit 5 was clean, with every gate green. Row 22 landed on `main` at 505528c by fast-forward and was pushed at 12:02 EDT. It is unreleased until the owner tags v0.1.8.

On 2026-10-26 the owner approved a pass to finish ezhomesteading `wk/298`. Main had gained two DB test packages (`backend/drive`, `backend/controllers/stripe`) that still held the old suite lock and kept a session on the source database, which blocks `CREATE DATABASE ... TEMPLATE`; the branch's own guard test caught both after the merge. An Opus builder merged main, moved both packages to the per-binary copy (07303aec0), and after an Opus audit with non-blocking findings fixed three of them: a leftover copy with the run's own pid, the CI skip check's package list, and three statements in `docs/gates.md` (40846d446). Two parallel full suites took 110 s each against about 705 s on main. The branch is pushed as pull request 121. Its CI passed every job but `web e2e`, which fails on `write-flows.spec.ts:173` on main as well (run 40387540287), so the PR did not cause it. Open: `chaos_compose_test.go:726` reads `pg_locks` with no database filter. The merge is the owner's.

Row 29 (the open items of rows 26 and 28) landed on `main` at 5b7b1b8 after five audits. Its parse of `<shell> -c` strings was taken out by the owner's choice after the fourth audit showed that parsing leaves forms silent that the text match slotted; the other five items stayed. The same evening this session cleared the Go build cache (65 GB) when the disk had 360 MiB free and gates failed with "No space left on device"; the wk gc launchd agents had stopped at 11:10 EDT with exit 78, probably for the same cause.
