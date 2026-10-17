# Autopilot run, wk, 2026-10-16

## 1. In one paragraph

The run started 2026-10-16 at 20:33 EDT on Fable 5.1 with board rows 1 to 5 (the wk MVP) and ended at 22:17 EDT with nothing in flight. Eight rows landed on `main` and were pushed: row 1 (Part 0, `CLAUDE.md` and the ledger), row 2 (the P1 amendments ratified as D22 to D26 and the P2 to P4 plan, which split each phase into sibling rows 6 to 12 at the owner's request), and the siblings 6 (`internal/daemon`), 8 (`internal/board`), 9 (`internal/changeset`), 10 (`internal/landing`), 11 (`internal/gate`) and 12 (`internal/batch`). Every landed row had a builder and at least one independent auditor on its tier, with fix rounds on rows 1, 2, 8, 10 and 11 and a Deep audit on rows 2, 9 and 10. Row 7 (`internal/lease`, the lease store, the edit judgment and the `Bash` word walk) is `HELD` after three blocking audits of the word walk: each audit found new wrong allows in forms the walk claims to handle, so the walk's design is the owner's call. Rows 3 and 4 are `HELD` by the owner's amendment of 22:15 EDT on the "daemon" or "files" answer from a kill test running in another session, and row 3 also waits on row 7 and on ezhomesteading item 200, which was `IN FLIGHT` on every read. Row 5 waits on row 4. `/clean-up` folded the eight `DONE` item files and their rows to `/Users/zachshort/Projects/archive/wk` (archive commits ed0e2b1 and 6589686). The run ended because no row could launch: the open rows are held or wait on another board.

## 2. Audit

not run (wk has no `docs/audit-lanes.md`).

## 3. Rows

| Row | Tier, model | Worktree, branch | Gates seen | Audits | Fix rounds | Status |
|---|---|---|---|---|---|---|
| 1 | Default, Opus 5.5 | `.claude/worktrees/ap-row-1`, `ap/row-1` | item gate green at a4d4ed2, 51f1282, 7275a2c; also in an archive copy and with a fresh lint cache | Opus (non-blocking, 6), Opus (non-blocking, 2) | 1 | DONE, landed 7275a2c |
| 2 | Deep, Fable 5.1 | `ap-row-2`, `ap/row-2` | baseline gate green at d51f151, 9ea0524, d465437 | Fable (non-blocking, 13 plus the early-start finding), Fable (non-blocking, 3) | 1 | DONE, landed d465437 |
| 6 | Default, Opus 5.5 | `ap-row-6`, `ap/row-6` | gate green at a543f46 and 9a2a1bd; package flake-checked 20 and 5 runs | Opus (non-blocking, 6) | 0 | DONE, landed 9a2a1bd |
| 7 | Default, Opus 5.5 | `ap-row-7`, `ap/row-7` (kept) | gate green at 49d7b2c, b8d2921, 82e9dcf; 160 package tests | Opus (blocking B1, B2), Opus (blocking K1 to K3, K4 decided), Opus (blocking B1 to B6) | 2 | HELD at 82e9dcf |
| 8 | Default, Opus 5.5 | `ap-row-8`, `ap/row-8` | gate green at 3c8c8ac, a564db6, 0e7d664 | Opus (non-blocking, 6), Opus (non-blocking, 1) | 1 | DONE, landed 0e7d664 |
| 9 | Deep, Fable 5.1 | `ap-row-9`, `ap/row-9` | gate green at 1b05d88, ec19f31; tree equality proven byte for byte | Fable (non-blocking, 5) | 0 | DONE, landed ec19f31 |
| 10 | Deep, Fable 5.1 | `ap-row-10`, `ap/row-10` | gate green at cf36d6d, 7e1f1bd, 475ebd1; CAS race and sparse bit proven | Fable (non-blocking, 6; finding 1 escalated), Fable (non-blocking, 3) | 1 | DONE, landed 475ebd1 |
| 11 | Default, Opus 5.5 | `ap-row-11`, `ap/row-11` | gate green at 6fdca3a, e5132b6, 96ec79e | Opus (non-blocking, 7), Opus (non-blocking, 3) | 1 | DONE, landed 96ec79e |
| 12 | Default, Opus 5.5 | `ap-row-12`, `ap/row-12` | gate green at 94b2c74, d948f0e | Opus (non-blocking, 6) | 0 | DONE, landed d948f0e |
| 3 | Default, Opus 5.5 | none | none | none | 0 | HELD: owner's "daemon" or "files"; waits on 7, item 200 |
| 4 | Deep, Fable 5.1 | none | none | none | 0 | HELD: owner's "daemon" or "files"; waits on 3 |
| 5 | Default, Opus 5.5 | none | none | none | 0 | OPEN, waits on 4; prompt amended |

Every report is in `reports/` beside this file, one per builder report, fix round and audit.

## 4. Defects found and not fixed

- Row 7: the `Bash` word walk allows writes in forms it claims to handle (`reports/row-7-audit-3.md`: `..` after a glob or brace, a `cd` after `&&` or `||` that did not run, a heredoc piped to `sh` on a later line, `\'` inside `$'...'`, `<<` inside `${...}`), plus new gaps (`FOO+=x`, `sed -I`, `python3 -Bc`, `eval 'rm'`, shell functions, `time`, `coproc`, `exec -a`, `source /dev/stdin`, `unlink`, whole-tree git writes, a `cd` into a missing directory). Carried by the hold.
- Row 1: `README.md:5` says the design lives in ezhomesteading; it is in `docs/incomplete/landing-layer/` here. `README.md` is in ezhomesteading item 200's files, so no row here may touch it until 200 is DONE.
- Row 1: Part 0 steps 0.4, 0.6, 0.7 and 0.8 on the ignored `docs/AGENT-PRACTICES.md` (cut CI and migrations; fix the Part 12 survivors; the GATE 0 question; stamp and delete Part 0; `· adapted`). `doctor` reports its stamp-drift.
- Landed with findings open for the integration rows (each in the row's close-out): row 6 (no read deadline or request size limit; handler-mutex access for row 4), row 9 (sparse-cone cases for `wk submit`), row 10 (nine findings for row 4, among them `Sync` moving the branch on a conflict by design and a divergent-move refusal), row 11 (nine findings for row 5: paths relative to the gate's dir, no per-gate timeout, `Seconds` includes the grace, a backgrounded child is not killed), row 12 (attribution rules now in row 5's prompt), row 8 (four findings for row 3).

## 5. Collisions and merge order

None. No two rows claimed one number. Landing order on `main`: 1 (7275a2c), 2 (d465437), 6 (9a2a1bd), 8 (0e7d664), 9 (ec19f31), 11 (96ec79e), 12 (d948f0e), 10 (475ebd1). Rows 8 to 12 branched from 344f8c7 and each merged `main` in its own worktree before its gate, so each landed as a fast-forward of the builder's merge commit. Row 3's "Files it owns" overlaps ezhomesteading item 200 on `cmd/wk/main.go`, `internal/config/config.go` and `internal/settings/settings.go`; that overlap is sequential by row 3's wait.

## 6. Judgment calls

Each with its reversal, in `STATE.md` under Judgment calls. In short: the untracked board files were committed to `main` before launch (reversal: `git rm --cached`); worktrees were made by hand because the Agent tool's isolation makes a worktree of the session's own directory, which is personal-config (reversal: run from a wk session); the ledger sections of row 1 were written by this session from the builder's report because git ignores `HANDOFF.md`; the holding wake was 1800 s; the default builder cap of 4 was lifted to 6 on the owner's words; siblings got lanes C to I so they launch in parallel; rows 9, 10 and 11 waited on 2 instead of 3 and row 12 on 9, on the Fable auditor's import evidence; non-blocking findings in `CLAUDE.md`, the plan, the board parser, the gate runner and the landing package were escalated to fix rounds where a wrong text or a silent allow would otherwise land; two cite fixes and three wording fixes were folded into land-step commits on the auditors' own verified cites; row 7's K4 rule (an open quote fails closed) was decided inline on the Deep model; row 5's prompt step 3 was amended from row 12's audit; row 7 was held after the third blocking audit.

## 7. Decisions taken for the owner

- K4 on row 7: a command the walk cannot split fails closed, because bash runs each complete line before it reports an open quote on a later line. Decider: this session (Fable 5.1). Reversal: return no targets, as the prompt wrote. Moot while row 7 is held.
- Row 7 N3: a second claim by one root refuses until `Release` (the builder's own reversal). Decider: this session. Reversal: replace on reclaim.
- Row 1's gate command, README line and prunable worktree: left to the owner (below).
- Row 3's L3 question: answered by the owner at 22:15 EDT: defer.

### Decisions waiting for the owner

- **Row 7's word walk.** (a) Land at 82e9dcf with `leases.bash_guard` default `false` and the gap list in the README, the walk as defense in depth and a later row for its design (recommended: the edit-tool judgment held across three audits, and the walk's failures are wrong allows that never block work); (b) an allowlist like `commit-guard.sh`'s `raw_words` (refuse any character outside a safe set, loud but safe); (c) a real parser (`mvdan.cc/sh`), which touches `go.mod` while item 200 adds the same module. One word: `land`, `allowlist` or `parser`.
- **"daemon" or "files"** for rows 3 and 4, from the kill test in the "Integrate wk and personal-config" session (the owner's amendment 1).
- **`gateCommand`** in `/Users/zachshort/Projects/wk/.personal-config.json`: the row 1 builder proposed `export GOTOOLCHAIN=local && ! gofmt -l cmd internal | grep -q . && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...` (about 45 s on every stop, no double quotes because the hook reads the value with sed and runs it with eval). Set it, or choose a lighter line without `-race`.
- **`git worktree prune`** of `/Users/zachshort/Projects/landing-layer-phase3` (`p1-phase-3`, 30989b5, already in `main`, directory gone) and the deletion of its branch. Not this run's worktree, so untouched.
- **Kept auditor worktrees** (section 10) to remove with `git worktree remove` and `git branch -D`.
- **The relative words** at `PLAN-P2-P4.md` lines 44 and 93 (left open at row 2's sign-off) and the pre-existing `doctor` findings in the landing-layer documents (relative dates, absence evidence), none of them this run's.

## 8. Budget

- Usage: start 20:35 EDT weekly all models 50%, Fable 24%, five-hour 0%. 21:58 EDT weekly 63% (13 of the 25-point cap), Fable 42%, five-hour 48%. The weekly window resets 2026-10-17 02:59 EDT; the run ended before it.
- Agents: 27 in all. Fable 5.1: 3 builders (rows 2, 9, 10), 5 audits (rows 2 twice, 9, 10 twice). Opus 5.5: 6 builders (rows 1, 6, 7, 8, 11, 12), 13 audits (rows 1 twice, 6, 7 three times, 8 twice, 11 twice, 12). Every fix round and land step resumed the row's builder. No Sonnet or Haiku agent ran.
- Hours: 1 h 46 min, 20:33 to 22:17 EDT.
- This session's context: peak 471,280 tokens at 21:58 EDT on a 1M window (readings in `STATE.md`), no compaction.

## 9. Not verified

- The daemon, the lease hook, `wk claim`, `wk submit`, `wk sync`, `wk land` and a batch gate have not run live; every proof so far is a package test or an auditor's throwaway repository. The live "Done when" proofs belong to rows 3, 4 and 5.
- The `PreToolUse` deny JSON field names on Claude Code 2.1.301 (row 3 step 1 checks first); `vm_stat` field names beyond the captured sample; the 103-byte socket limit.
- Sparse-worktree behaviour of `Sync` was probed and tested in throwaway repositories, not on ezhomesteading.
- Row 7's branch at 82e9dcf passed its gate but is not on `main`.
- A real launchd load, a real hook call, a `df` measurement, the TTY screens and `wk viz` (row 1's close-out).

## 10. Warnings

- Auditor worktrees kept because each holds a merge commit of the builder's branch with a moved `main`, which `main` does not hold, so `git branch -d` refuses: `ap-row-2-audit-2` (c1a9afd), `ap-row-10-audit` (a5d2d31), `ap-row-10-audit-2` (6473ffb), `ap-row-11-audit-2` (e369c27), `ap-row-12-audit` (e46bf28), `ap-row-7-audit` (49d7b2c, an ancestor of ap/row-7), `ap-row-7-audit-2` (012f063), `ap-row-7-audit-3` (8a691cf). Content-wise each equals a state `main` passed. This run never forces.
- `gofmt -l .` in the primary checkout reads every worktree under `.claude/worktrees/`; the gate as the item writes it is fine inside a worktree, and the proposed `gateCommand` uses `cmd internal` for that reason.
- The golangci-lint cache is shared across checkouts; a fresh-cache run agreed on 2026-10-16.
- `~/go/bin/wk` was built from 30989b5 with a dirty tree and is what ezhomesteading's hooks and launchd agent run; nothing this run landed is installed.
- The row 7 third auditor's probe script carried a stray `rm -f /dev/null`, which failed on permissions; `/dev/null` is intact.

## 11. Landed

| Row | Branch | On `main` | Pushed |
|---|---|---|---|
| 1 | `ap/row-1` | fast-forward to 7275a2c | yes |
| 2 | `ap/row-2` | fast-forward to the builder's merge commit d465437 | yes |
| 6 | `ap/row-6` | fast-forward to 9a2a1bd | yes |
| 8 | `ap/row-8` | fast-forward to the merge commit 0e7d664 | yes |
| 9 | `ap/row-9` | fast-forward to the merge commit ec19f31 | yes |
| 11 | `ap/row-11` | fast-forward to the merge commit 96ec79e | yes |
| 12 | `ap/row-12` | fast-forward to the merge commit d948f0e | yes |
| 10 | `ap/row-10` | fast-forward to the merge commit 475ebd1 | yes |

Rows landed and not pushed: none. Every push went; `main` is level with `origin/main` at each landing. Other commits on `main` by this session: f8cea0a (the untracked board files tracked), a4d4ed2 (claims 1 and 2), 344f8c7 (claims 6 to 11), b1d97ea (claim 12), bd29fba (row 5 step 3 amended), 789b345 (item 7 held), c2b8bc9 (owner amendments), and the run commit below.

The run commit (step 4 of the end) adds `docs/incomplete/autopilot-2026-10-16/STATE.md`, `RUNDOWN.md` and `reports/*.md`, and the fold's removal of `passoff/1-*.md`, `2-*.md`, `6-*.md`, `8-*.md`, `9-*.md`, `10-*.md`, `11-*.md` and `12-*.md`. The archive repository (`/Users/zachshort/Projects/archive`, its own git) holds `wk/INDEX.md` and `wk/passoff/*` at ed0e2b1 and 6589686; `HANDOFF.md` and `PASSOFF.md` are ignored by git and carry the log line and the board as untracked records.

## For personal-config row 117

What the land step, the deciders or the cross-board wait did that the skill text did not say:

1. The session's working directory was personal-config, not wk; `isolation: "worktree"` would make a worktree of the wrong repo, so every worktree was made by hand with `git -C wk worktree add` and each builder was told to `cd` into it per command. `/delegate` step 2 assumes the session sits in the repo it delegates.
2. `/delegate` step 3's reproduce line (`git merge --ff-only <builder branch>`) assumes the base has not moved since the builder branched. With six siblings landing, it refuses from the second row on; auditors were told to `git merge --no-edit` instead, which leaves a merge commit `git branch -d` refuses at step 7. A detached auditor worktree at the builder's tip would leave nothing to delete.
3. `/delegate` step 6 lands rows one at a time: each builder merges `main` in its worktree, and a second fast-forward refuses once the first moved `main`. Signed-off rows queued behind each other (8, then 9, then 11, then 12, then 10).
4. The sibling briefs pointed each builder at its tracked item file in its worktree instead of carrying the prompt text in the brief (`/delegate` step 2 says "the item's own prompt in full").
5. Non-blocking verdicts were escalated to fix rounds where a wrong sentence in `CLAUDE.md`, a plan line that contradicted a prompt, or a silent wrong allow would otherwise land; and small cite or wording fixes the auditor had itself verified were folded into the land step's step 1 commit. The skill's step 4 signs off any non-blocking verdict with the findings open.
6. The wake procedure's step 4 says a Deep subagent decides the silent class; the leader, on the Deep model, decided row 7's K4 inline.
7. `HANDOFF.md`, `PASSOFF.md` and `docs/AGENT-PRACTICES.md` are ignored by the owner's global git ignore, so no worktree holds them; row 1's ledger sections came back as text in the report and this session wrote them. The `{{WORKTREE_SETUP}}` and `{{BUILD_CMD}}` placeholders were filled the same way.
8. The untracked `.gitignore`, `CLAUDE.md`, `docs/conventions-go.md` and `passoff/*.md` had to be committed before launch, or the builders' worktrees would not hold the item files and the fast-forward would refuse over the untracked `CLAUDE.md`.
9. The board runs one lane's items in order, so sibling rows written on lane A would have serialized; each got its own lane (C to I).
10. Builders' reports were saved under `reports/` beside `STATE.md` rather than inline under its Reports section, with a summary line in `STATE.md`; the full ledger paste for row 1 would have made `STATE.md` unreadable.
11. Each returned agent raised a wake on its own, so the 1800 s fallback never fired; the loop's own prompt was re-armed on every return.
12. The owner sent amendments mid-run (22:15 EDT) for rows not launched; they were applied to the item files, the board and `DESIGN.md` from the primary checkout and committed, and rows 3 and 4 were set `HELD` on the owner's word. The skill has no step for an owner message after the opening round.
13. The cross-board wait on ezhomesteading item 200 was read at every wake and stayed `IN FLIGHT`; it never decided the end, because rows 7 and the owner's holds held the chain first.
14. Three auditor worktrees per held row and two per fixed row accumulated; step 7 removes only merged ones.
