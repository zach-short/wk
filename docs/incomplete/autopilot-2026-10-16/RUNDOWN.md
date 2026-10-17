# Autopilot run, wk, 2026-10-16 to 2026-10-17

## 1. In one paragraph

The run started 2026-10-16 at 20:33 EDT on Fable 5.1 with board rows 1 to 5 (the wk MVP), paused twice (the owner's stop order from 23:17 to 01:32 EDT, and the 95 percent usage cap from 03:40 to 08:53 EDT) and ended 2026-10-17 at 11:55 EDT with nothing in flight. Twelve rows landed on `main` and were pushed: rows 1 and 2 (Part 0 and the plan, which split each phase into sibling rows 6 to 12), the siblings 6, 8, 9, 10, 11 and 12 (the daemon, board, changeset, landing, gate and batch packages), row 7 (the lease store, the judge and the allowlist Bash walk, rebuilt as pass 2 under the owner's decision rule after three blocking audits of the first walk, then two fix rounds for heredoc bodies), row 3 (path leases on the file store, two fix rounds and two scoped passes for symlink, case and git pathspec spellings, five audits), row 14 (opened from row 3's audit: the walk reads git's file-fed pathspecs, aliases, `--output`, abbreviated options, transports, `GIT_` variables, `worktree add` and program-naming config keys; two fix rounds, three audits) and row 4 (the landing queue on the file store, a Deep row: one Fable fix round for a crash re-prediction that could land content the submitter never wrote, two Fable audits). Row 13 (the gc fix) landed from the other Fable session. The daemon-or-files question was decided by the owner's rule from the kill test: files. Row 5 (P4, the batch gate) is `OPEN` and not launched: when row 4 landed the weekly all-models usage read 24 percent against the 25 percent cap the owner set through a peer session, and a P4 build with its audits cannot fit under it. `~/go/bin/wk` is built from `main` at 01120ce; `[leases]` is off in both repositories; `[landing]` is on in `~/.wk/wk.toml` and off in `~/.wk/ezhomesteading.toml`; the launchd timer is the owner's to install.

## 2. Audit

not run (wk has no `docs/audit-lanes.md`).

## 3. Rows

| Row | Tier, model | Worktree, branch | Gates seen | Audits | Fix rounds | Status |
|---|---|---|---|---|---|---|
| 1 | Default, Opus 5.5 | `ap-row-1`, `ap/row-1` | green at a4d4ed2, 51f1282, 7275a2c | Opus twice (non-blocking) | 1 | DONE, 7275a2c |
| 2 | Deep, Fable 5.1 | `ap-row-2`, `ap/row-2` | green at d51f151, 9ea0524, d465437 | Fable twice (non-blocking) | 1 | DONE, d465437 |
| 6 | Default, Opus 5.5 | `ap-row-6`, `ap/row-6` | green at a543f46, 9a2a1bd | Opus (non-blocking) | 0 | DONE, 9a2a1bd |
| 7 | Default, Opus 5.5 | `ap-row-7`, `ap/row-7` | pass 1 green at 49d7b2c, b8d2921, 82e9dcf; pass 2 green at f9ee256, 713b8b4, cd9f720, 80e0102 | pass 1: Opus three times (blocking); pass 2: Opus (blocking B1 heredoc body quotes), Opus (blocking B2 whitespace after `<<`), Opus (clean) | 2 in pass 1, 2 in pass 2 | DONE, 80e0102 |
| 8 | Default, Opus 5.5 | `ap-row-8`, `ap/row-8` | green at 3c8c8ac, a564db6, 0e7d664 | Opus twice (non-blocking) | 1 | DONE, 0e7d664 |
| 9 | Deep, Fable 5.1 | `ap-row-9`, `ap/row-9` | green at 1b05d88, ec19f31 | Fable (non-blocking) | 0 | DONE, ec19f31 |
| 10 | Deep, Fable 5.1 | `ap-row-10`, `ap/row-10` | green at cf36d6d, 7e1f1bd, 475ebd1 | Fable twice (non-blocking, one escalated) | 1 | DONE, 475ebd1 |
| 11 | Default, Opus 5.5 | `ap-row-11`, `ap/row-11` | green at 6fdca3a, e5132b6, 96ec79e | Opus twice (non-blocking) | 1 | DONE, 96ec79e |
| 12 | Default, Opus 5.5 | `ap-row-12`, `ap/row-12` | green at 94b2c74, d948f0e | Opus (non-blocking) | 0 | DONE, d948f0e |
| 13 | Deep, Fable 5.1 | the other Fable session's `fx/gc` | green there | that session's | 0 | DONE, ddc541b (via a692538) |
| 3 | Default, Opus 5.5 | `ap-row-3`, `ap/row-3` | green at f0b1eb8, 1c4a1fd, 53be9d5, a028e54, 146c6f7 (second run, after four red runs on row 200's slot timing test under load), 2eb9ba0 and 2a03612 (quiet-load loop) | Opus five times: blocking B1 to B4 (case, cwd outside every root, dangling link, link then `..`), blocking B5 to B7 (`cd` through a link, a link at a leased path, an Edit judged only physically), blocking B8 (a link entry through a link then `..`), blocking B9 to B11 (git cleans pathspecs by text; a regression from the lead's pass 2 premise), clean on the change with B12 and B13 in row 7's walk | 2, plus passes 2 and 3 under the owner's rule | DONE, 2a03612 |
| 14 | Default, Opus 5.5 | `ap-row-14`, `ap/row-14` | green at 9c7cec3, ef61fe5, 619eb01, b7f5db1 | Opus three times: blocking F1 to F7 (one own, six older git write paths), blocking B1 to B3 (holes in the fix, scoped verdict), clean | 2 | DONE, b7f5db1 |
| 4 | Deep, Fable 5.1 | `ap-row-4`, `ap/row-4` | green at 484f93d (twice), 0adb43f, 01120ce | Fable twice: blocking F1 (the crash re-prediction), clean with N7 to N9 | 1 | DONE, 01120ce |
| 5 | Default, Opus 5.5 | none | none | none | 0 | OPEN, waits on nothing; not launched at the usage cap |

Every report is in `reports/` beside this file, one per builder report, fix round, pass and audit (57 files).

## 4. Defects found and not fixed

- Row 3, open and loud: rule 4 over-refuses `cd web; cd ..; rm docs/a.md`; the safe-set gaps `go fmt ./web/...`, `npx prettier --write`, `bunx prettier --write`, `unzip -o -d`, `golangci-lint run --fix`, `mkdir -p`, `chown` (listed in the README); the expansion rule coded twice; `Judge` has no idea of the caller's root; `checkout -t<x>` over-refused; the `GIT_` text rule over-refuses `git var GIT_EDITOR` and comments; `EDITOR=` inert while `GIT_EDITOR` is set; a `-c` value that names a script passes as an unknown command; repository hooks outside the walk; the hook not run from a desktop session or an `isolation: "worktree"` subagent; a `cd` with `..` over-refuses at more places; the hook drops gone claims in memory only; `Server` and `Client` in `internal/daemon` unused.
- Row 4, open and loud: `wk submit` appends to the queue with no lock of its own (a crash mid-write leaves a torn line that `Load` refuses until a hand repair); `wk sync` folds the branch before `landing.Sync` runs (recoverable from the reflog line); a transient binary makes `spawnLand` return silently; the behind line prints with `push = false`; a swapped `landing` record is refused on the rerun when `require_review` was turned on in between; the behind line's hint is stale once a later landing moved the ref; the gc absorption amendment unbuilt (reversal named).
- Row 1: Part 0 steps 0.4, 0.6, 0.7 and 0.8 on the ignored `docs/AGENT-PRACTICES.md` (a session card was proposed for Sonnet).
- The slot timing test (row 200's, ezhomesteading) read red at one-minute load above about 10 inside the full parallel gate; fixed by that session's row 234 at 941667c (CPU time, a 5 s test timeout).

## 5. Collisions and merge order

None between rows. Landing order on `main`: 1, 2, 6, 8, 9, 11, 12, 10 (2026-10-16), then 13 from the other session, 7 (80e0102), 3 (2a03612), 14 (b7f5db1), 4 (01120ce). Rows 4 and 14 ran in parallel on disjoint files; row 14 landed first and row 4 merged it. Ezhomesteading item 200 (the slots hook) landed on wk `main` at b0d1839 between row 7 and row 3 on the files row 3 also owns; row 3 built on it. Two of this session's own commits to `main` during row 3's land step refused the first fast-forward (section 6).

## 6. Judgment calls

Each with its reversal, in `STATE.md` under Judgment calls. Since the first rundown: the kill test's table applied the owner's rule (files); row 7 re-scoped from a denylist to an allowlist walk as pass 2 under the owner's rule instead of staying held; audit worktrees made detached so nothing unmergeable is left; a relayed usage cap (95 percent of the 5-hour window, 25 percent of the week) applied as a cap with hourly wake chains; a stop order relayed and obeyed with a chain to 01:30 EDT; row 3's third and fourth blocking audits answered with scoped passes 2 and 3 under the owner's rule instead of a hold, the fourth because the lead's own brief premise caused the regression; row 3 landed with B12 and B13 as a new row 14 because they were in row 7's landed walk; row 14's second and third audits given a scoped verdict (regressions and holes in the fix block, older gaps are findings); row 4's two owner questions decided (gc absorption unbuilt, `wk sync` folds without a flag) and its six non-blocking findings landed open because none is silent; row 4's config writes and reinstall moved from the build to the land step after row 3's `[leases]` key broke main-built binaries; row 3's land gate retried under a quiet-load loop with a probe; the lead's two land-step mistakes (a commit to `main` under the builder, an install from `main` before the fast-forward) recorded and reversed at once.

## 7. Decisions taken for the owner

- Files, not the daemon, for rows 3 and 4 (the kill test's table under the owner's rule).
- Row 7: the allowlist walk over the kill test's option (a) and over a hold; K4 fails closed.
- Row 3: defer L3 (the owner's answer); the hook denies with JSON and exits 0; the wk-binary allow is a single wk command; the edit entry gets `onFailure: block`, the Bash entry `|| true`; a cwd outside every worktree cannot write into one; case folded on APFS; paths read as the kernel does, with the entry spelling and, for edit tools, the cleaned spelling; git operands with `..` or pathspec magic refused.
- Row 14: `add`, `commit`, `push` on the read list; every verb outside the list refused; one shared option splitter.
- Row 4: the gc absorption amendment unbuilt; `wk sync` folds without a flag; the base branch is `workspace.base`; a record whose tree is already on the tip lands without a commit; the timer interval 60 s; N3, N4, N6 to N9 open.
- Usage: nothing launched at or above 95 percent of the 5-hour window; row 5 not launched at 24 percent of the week against the 25 percent cap.

### Decisions waiting for the owner

- **Row 5** (P4, the batch gate): launch past the 25 percent weekly cap, or wait for the reset on 2026-10-24.
- **Turn leases on**: set `leases.enabled = true` in `~/.wk/ezhomesteading.toml` and `~/.wk/wk.toml`, run `wk hooks install` in each repository, copy the main checkout's `.claude/settings.local.json` into each existing workspace (or remake it), then every session runs `wk claim <row>` or `wk claim --none` before its first edit; the lead's own writes to `HANDOFF.md` and `PASSOFF.md` need a lease too (row 3's hand-back).
- **The launchd timer** for `wk land --loop` in wk (plist text in the README; `landing.enabled = true` is already set in `~/.wk/wk.toml`, so `wk submit` from a wk workspace starts a worker now).
- **A follow-up row** for row 4's open findings N3, N4, N6 to N9 and row 3's open list.
- **`gateCommand`** in `.personal-config.json` (still empty); the `git worktree prune` of a stale entry; the pre-existing `doctor` findings in the landing-layer documents.
- **Part 0** on `docs/AGENT-PRACTICES.md` (the proposed Sonnet session).

## 8. Budget

- Usage: 2026-10-16 20:35 EDT weekly all models 50 percent, Fable 24, 5-hour 0; 2026-10-16 23:17 EDT 5-hour 98 (the stop order); 2026-10-17 01:32 EDT 5-hour 0 after its reset, weekly 76; 03:00 EDT the weekly reset; 03:40 EDT 5-hour 95 (the cap, the run slept); 08:53 EDT 5-hour 5, weekly 11; 11:51 EDT 5-hour 52, weekly all models 24, Fable 32.
- Agents: 44 in all. Fable 5.1: 4 builders (rows 2, 9, 10, 4), 7 audits (rows 2 twice, 9, 10 twice, 4 twice). Opus 5.5: 9 builders (rows 1, 6, 7 twice, 8, 11, 12, 3, 14), 24 audits (rows 1 twice, 6, 7 six times, 8 twice, 11 twice, 12, 3 five times, 14 three times). Every fix round, pass and land step resumed the row's builder. No Sonnet or Haiku agent ran.
- Hours: 15 h 22 min of wall time, 20:33 EDT to 11:55 EDT, of which about 7 h 25 min paused (the stop order 2 h 15 min, the cap 5 h 13 min).
- This session's context: one compaction at about 23:00 EDT; afterwards 606 k tokens at 11:51 EDT on a 1M window.

## 9. Not verified

- The lease hook and the landing worker in a real Claude Code desktop session; every live proof ran in a throwaway repository with a scratch binary and `WK_HOME` (rows 3 and 4) or through `Judge` on the sources (row 14).
- The launchd timer; the installed binary against the real ezhomesteading board (leases are off there).
- Git versions other than 2.49.0 and bash 3.2.57, zsh 5.9; whether Claude Code 2.1.301 normalizes `..` in `file_path` or follows a dangling link on `Write` (both closed for either behaviour).
- `wk sync` with a `landing.Sync` failure after the fold (reasoned only).
- CI's Linux job on the landed rows.

## 10. Warnings

- `~/.wk/ezhomesteading.toml` now holds `[leases]` and `[landing]`; a `wk` binary built from any commit before 01120ce refuses the file. Reinstall from `main` only.
- `landing.enabled = true` in `~/.wk/wk.toml`: a `wk submit` from a wk workspace now appends to `~/.wk/wk/changesets.jsonl` and spawns a worker that lands onto `refs/heads/wk/landing`; `main` moves only by `wk land --apply --expect`.
- The primary checkout holds an uncommitted `docs/conventions-go.md` change that belongs to another session; this run left it alone.
- Worktrees of other runs under `.claude/worktrees/`: `ap-row-200`, `ap-row-200-audit`, `ap-row-211-audit`, `ap-row-212-audit`, `ap-row-213-audit` (ezhomesteading's run) and `ap-row-234-audit`, `ap-row-234-audit-2` (its row 234); this run removed every worktree it made.
- The completion-gate hook ran the full gate in the primary on every stop and read red three times on "parallel golangci-lint is running" while an auditor's gate ran; each was a false alarm.
- `gofmt -l .` in the primary reads every worktree; the gate as written is fine inside a worktree.

## 11. Landed

| Row | Branch | On `main` | Pushed |
|---|---|---|---|
| 1 | `ap/row-1` | 7275a2c | yes |
| 2 | `ap/row-2` | d465437 | yes |
| 6 | `ap/row-6` | 9a2a1bd | yes |
| 8 | `ap/row-8` | 0e7d664 | yes |
| 9 | `ap/row-9` | ec19f31 | yes |
| 11 | `ap/row-11` | 96ec79e | yes |
| 12 | `ap/row-12` | d948f0e | yes |
| 10 | `ap/row-10` | 475ebd1 | yes |
| 13 | `fx/gc` (the other session) | ddc541b, merged at a692538 | yes |
| 7 | `ap/row-7` | fast-forward to the merge commit 80e0102 | yes |
| 3 | `ap/row-3` | fast-forward to the merge commit 2a03612 | yes |
| 14 | `ap/row-14` | fast-forward to the merge commit b7f5db1 | yes |
| 4 | `ap/row-4` | fast-forward to the merge commit 01120ce | yes |

Rows landed and not pushed: none. Every branch this run made is merged and deleted; every worktree it made is removed. The run commits add `docs/incomplete/autopilot-2026-10-16/STATE.md`, `RUNDOWN.md` and `reports/*.md`, and the fold's removal of the `DONE` item files (rows 3, 4, 7, 13, 14 in this fold; rows 1, 2, 6, 8 to 12 in the first). The archive repository (`/Users/zachshort/Projects/archive`, its own git) holds `wk/INDEX.md` and `wk/passoff/*`; `HANDOFF.md` and `PASSOFF.md` are ignored by git and carry the log and the board as untracked records.

## For personal-config row 117

What the land step, the deciders or the cross-board wait did that the skill text did not say. Items 1 to 14 are from the first rundown (2026-10-16 22:18 EDT) and stand; 15 on are from the continuation.

1. The session's working directory was personal-config, not wk; every worktree was made by hand with `git -C wk worktree add` and each builder told to `cd` into it per command.
2. `/delegate` step 3's reproduce line (`git merge --ff-only <builder branch>`) assumes the base has not moved; auditors merged instead and left unmergeable branches. A detached auditor worktree (`git worktree add --detach`) leaves nothing to delete; used from row 7's audit 5 on.
3. `/delegate` step 6 lands rows one at a time; signed-off rows queue.
4. Sibling briefs pointed builders at their tracked item file instead of carrying the prompt.
5. Non-blocking verdicts were escalated to fix rounds where a wrong text or a silent allow would otherwise land.
6. The leader on the Deep model decided a silent-class call inline.
7. Ignored ledger files were written by the lead from the builders' reports.
8. Untracked board files had to be committed before launch.
9. Sibling rows got their own lanes.
10. Reports saved under `reports/` with a summary line in `STATE.md`.
11. Each returned agent raised its own wake; the fallback never fired for them.
12. Owner amendments mid-run were applied to the item files, the board and the design.
13. The cross-board wait was read at every wake.
14. Audit worktrees accumulated per held row.
15. The owner's standing decision rule (silent failure first, then landing, then least code) replaced the skill's hold after two fix rounds three times (row 7 pass 2; row 3 passes 2 and 3) and decided five owner questions; each call carries a reversal. The skill needs a clause for a decision rule the owner sets.
16. The audit brief's "any silent allow is blocking" hunts a landed dependency's whole surface through an integration row (row 3: five audits, four of them on row 7's walk). A scope clause was added from row 14's audit 2 on: a regression or a hole in the fix blocks; an older gap is a finding for a new row.
17. The lead's own brief premise (pass 2 of row 3: "no program a shell runs cleans `..` by text") caused a regression that audit 4 caught. Auditors should test the lead's premises; the lead records its own errors as judgment calls.
18. Two land-step mistakes of the lead: a commit to `main` while the builder gated (the fast-forward refused) and a `go install` from `main` before the fast-forward (a pre-row binary for one minute). The land step should say: nothing to the base between the land message and the fast-forward, install only after it.
19. A build that writes machine config keys its branch adds (row 3's `[leases]` in `~/.wk/ezhomesteading.toml`) breaks every binary built from `main` until it lands; the config writes and the reinstall belong in the land step (done for row 4).
20. A load-sensitive test in another row's package (the slot timing test) made the full gate read red at one-minute load above about 10; the land step ran a quiet-load loop with a probe, and the test's owner session fixed it as its own row. The land step needs a rule for a red gate on a file outside the row.
21. Findings against another row's landed code (B12, B13 in row 7's walk) became a new board row (14) opened mid-run, on a lane parallel to row 4.
22. A peer session relayed usage rules and a stop order as the owner's words; they were applied as caps with hourly wake chains and recorded as relayed. One scheduled wake (05:32 EDT) did not fire until 08:53 EDT; the chain must tolerate a missed wake.
23. The owner asked this session to message the other board's lead to prioritize item 200; cross-session messages unblocked the wait and later carried the slot test fix.
24. The lead wrote estimated timestamps ahead of the clock several times and corrected them in later commits; `date` before every stamp.
25. The fold at the end took rows 3, 4, 7, 13 and 14; the archive is its own repository with its own commit and push.
26. Rows in flight ran to completion past the weekly cap (the owner's "do not stop rows in flight"); the cap stopped only new launches.
