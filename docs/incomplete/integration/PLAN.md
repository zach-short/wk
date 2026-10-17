# Plan: one task lifecycle for wk and personal-config

Status: DECIDED, written 2026-10-16 by the integration session (Fable 5.1) from the research review of the same day and the owner's decision record (`DECISIONS.md`, Appendix F of the review). The owner delegated every owner call on 2026-10-16 at 22:25 EDT; a later call is decided by the standing rule in that record (prevent a silent failure first, keep work landing without the owner second, least code third) and written here or in the item file's close-out, with its reversal. The ratified designs this plan builds on are wk `docs/incomplete/landing-layer/DESIGN.md` D1 to D26 and personal-config `docs/incomplete/autonomous-autopilot/` D1 and S1; it reverses none of them except where a decision number below says so.

## 1. The end state

Two tools, two languages, one lifecycle. wk (Go) is the lifecycle engine: it owns the runtime state and the deterministic steps. personal-config (TypeScript) owns the task definitions, the standard, the skills that hold the judgment, and `doctor`. Neither names the other in code; a skill calls the engine where a repository has configured it (`~/.wk/<repo>.toml`), and falls back to its own claim otherwise (decision 2, S1 stands).

| Component | Owner | Store | Interface | Decision it keeps, amends or reverses |
|---|---|---|---|---|
| Claims and path leases | wk | `~/.wk/<repo>/leases.json`, written under an exclusive `flock` on `~/.wk/<repo>/leases.lock` by temp file and rename; readers read without the lock | `wk claim <row>`, `wk claim --none`, `wk lease add <path>...`, `wk release`, `wk status --json` (`schema: 1`); exit 0 done, 1 error, 2 refused by policy (overlap, no claim) | Keeps D10 to D13. Decision 3 chooses files over the daemon (the kill test: 0 lost and 0 stale for both, 100 of 100 restarts and 100 of 100 denied hook calls for the daemon only). Decision 15: leases live in files, never in git refs. Row 3. |
| The workspace record | wk | `~/.wk/<repo>/workspaces.json`, same lock as the leases: root, row, branch, session, created | Written by `wk new <row>`, `wk claim`, the `WorktreeCreate` hook; read by `wk status`, gc and the lease hook (the worktree list the hook needs without a git call, PD-2) | New; it replaces the binding that lives today only in the branch and directory names. Row 3. |
| The edit-time lease hook | wk | reads `leases.json` and `workspaces.json` directly; no socket, no process to be down | `wk hook lease --repo <primary>` on `PreToolUse` for the edit tools and `Bash`; installed only when `leases.enabled = true`; `onFailure: "block"` on the edit-tool entries; every verb recovers from a panic; a command whose first word is the wk binary is always allowed | Keeps D11 and D12's fail-closed judgment; drops D12's "deny when the socket does not answer", which has no equivalent with files. Decision 4: shipped disabled. Row 3. |
| The Bash word walk | wk | none | `leases.bash_guard`; the walk is an allowlist: a command with any character outside a safe set is refused as "cannot tell" | Row 7, relaunched by the lead at 22:26 EDT under the owner's rule (loud over silent). A later row may replace the walk with a parser (`mvdan.cc/sh`, PD-4's reversal) once row 200 is on `main`. |
| Submit, the queue, the landing ref | wk | `~/.wk/<repo>/queue.jsonl` (append only, PD-8); `refs/heads/wk/landing` | `wk submit -m <msg> [--row <n>]` prints the id; `wk land` (dry run), `wk land --apply --expect <hash>`, `wk sync`; `wk status --json` lists every changeset | Keeps D14, D15, PD-6 to PD-8. Row 4. |
| The landing loop | wk | the queue file and the refs; no state between passes (`KILL-TEST.md`) | `wk land --loop`: a detached run-to-completion worker (P1's `spawnFill` pattern) started by `wk submit` after it appends, and by the launchd timer; one at a time under `~/.wk/<repo>/landing.lock`; exits when no `submitted` record remains | Amends row 4 step 3 (no daemon goroutine). Before each `Land` it appends a `landing` record with the predicted tree; a `submitted` or `landing` record whose tree equals the tip's tree is `landed` without a new commit (closes F18). Decision 3. |
| Gates | wk | `[[gates.run]]` in `~/.wk/<repo>.toml`, one list per repository, resolved through the git common directory so every worktree finds it | `wk gates --json` prints the list; the trial runs it on a batch's final tree; `landing.ff_main` moves `main` on green; `landing.require_review` defaults off (decision 5) | Keeps D16 for the batch, amended by decisions 8 and 20: `batch_max` stays 1 (serial) until landings per hour times gate hours passes 0.5; bisection replaces drop-the-last when a batch holds more than one changeset. Row 5. |
| gc | wk | marker files in the private git directory; the launchd timer (PD-5) | `wk gc`, `wk gc --apply`, `wk gc --json` carrying `schema`, `repo`, absolute `path`, a verdict enum | Row 13 landed F1 and F2 on 2026-10-16 (`a692538`). Row 5 adds the `wk-trial` marker instead of reusing `land-worktrees-trial` (PD-9 reversed by the review). |
| Repository identity | wk | `~/.wk/<name>/common-dir` | step 1, the loud guard: record the common git directory and refuse a mismatch (row 3); step 2, a repository id with the state directory migrated by unloading the launchd agent, renaming, reloading, in that order | Decision 16 (F5). |
| `wk version` | wk | none | prints the module version and the commit; `wk hooks install`, `gc schedule install` and `wk up`-class commands refuse a binary older than the config keys they write | F22. Row 15. |
| Task definitions | personal-config | the board (`PASSOFF.md`) and one item file per row | `passoff next`, `passoff next --json` and `passoff show <n> --json` (`schema: 1`: number, task, status, model, lane, waitsOn, files as parsed tokens) | Keeps per-item-board D1 to D16. Rows 123. No JSON header in item files until the standard's text names one (frozen `standard/`). |
| The claim fallback | personal-config | `.personal-config.claim.lock` beside the board while a claim runs (O_EXCL, pid and time, 10 s wait, 10 min staleness) and `writeText` by temp file and rename | `passoff claim <n>` | Row 121 (this session, 2026-10-16). Decision 2: the only built-in fallback. Decision 19: any status beginning `IN FLIGHT` is in flight for the collision check. |
| The "Files it owns" grammar | personal-config | the board cell | a `doctor` rule that reports, and does not refuse, a token that is not a path or a directory with a trailing slash, or a cell holding `|` | Decision 17. Row 124. The matcher then compares directory prefixes, not strings alone. |
| The claim reconcile | personal-config and wk | the board and `leases.json` | `wk status --json` read by a `doctor` rule `claims`: every `IN FLIGHT` row has a live claim and every claim an `IN FLIGHT` row; the engine's claim wins and the board is repaired to match | Decision 15. Ships with the first build that writes both (row 3 plus a personal-config row). |
| Config and board discovery | personal-config | `.personal-config.json` and the board, found through the git common directory | `passoff`, `undo`, the completion gate and `doctor` work from a linked worktree | Row 122, Opus 5.5 build with a Fable 5.1 review (decision 11): a wrong repository is silent. |
| The prose land step | personal-config | `src/render/skills.ts` | rendered only for profiles that set the flag; the shipped default omits it | Decision 13. Row 125. Deleted from each of the owner's repositories once P4 moves `main` there. |
| `undo` | personal-config | `~/.config/personal-config/backups/` | restores the last run that touched the directory it runs in; refuses without a TTY unless `--yes` | Row 121. |
| The daemon package | wk | none | `SaveJSON`, `LoadJSON`, `Spawn` stay in use; `Server` and `Client` are kept with a doc comment that nothing calls them and deleted after P3 lands | Decision 3's reversal keeps the interface available. |

What the end state does not have: a resident daemon; a socket; runtime state in markdown as the source of truth; `Co-Authored-By` lines; a second numbering scheme (board row numbers stay the task ids, never reused); a Rust or all-Go rewrite (git subprocess time is the cost, not start time); Gas Town, agent teams, jj or Beads (named and set aside by the research; nothing found here reopens them).

## 2. The immediate next step

In order, with the row that owns each piece and the model by the routing rule. "Lead" is the wk autopilot lead; "this session" is the integration session; "new" is a session proposed at the end of this session's work.

1. wk row 13, F1 and F2: done, `a692538`, this session (Fable 5.1). `go install` from `main` done under decision 7; the gc dry run on ezhomesteading matched the 22:30 scheduled pass.
2. wk row 7: the allowlist walk, in flight since 22:26 EDT. Lead, Opus 5.5 with an Opus 5.5 audit.
3. wk row 3 (P2, files): the amendments in `KILL-TEST.md` plus the owner amendments already in its item file. Lead, Opus 5.5 with an Opus 5.5 audit (standing order). Waits on 7 and ezhomesteading item 200.
4. personal-config row 121: the claim lock and rename, `inFlight`, `undo` scope, the adapted-stamp fix. This session. Landing 2026-10-16.
5. personal-config row 122: config and board discovery through the git common directory, and the completion gate reading the repository's gate from there. New session, Opus 5.5 build, Fable 5.1 review. Decision 18: a hook template change reaches the installed hooks only through `setup`; `--dry-run` first.
6. personal-config row 123: `passoff next --json` and `passoff show <n> --json`. New session, Opus 5.5, no review.
7. personal-config row 124: the `doctor` rule for the "Files it owns" grammar, reporting only. New session, Opus 5.5, no review. Decision 14: once 121 and 124 are on `main`, the memory rule "one builder per repository" is retired.
8. personal-config row 125: the prose land step behind a profile flag (decision 13). New session, Opus 5.5 (the skill tests assert rendered text, so a wrong render is loud). Row 120 waits on it.
9. wk row 4 (P3, the detached worker): lead, Fable 5.1, after 3.
10. wk row 5 (P4, serial gate, bisection, `wk-trial`): lead, Opus 5.5 with an Opus 5.5 audit, after 4.
11. wk row 14: F6 (`.env` through a symlink), F23 (settings written by a fixed temp name with no lock), F24 (`viz` Host check), `wk version` (F22). New session, Opus 5.5; F6 and F23 are silent, so a Fable 5.1 review.
12. Decision 6: the inventory of which gate reads which variable, then `template.copy_files` trimmed. New session, Sonnet 5.5 for the read-only inventory; the toml edit by the session that reads the result.
13. Decision 10: done in part by the lead at 22:26 EDT (`landing-layer-phase3` pruned, `p1-phase-3` deleted, the no-op auditor worktrees removed with their tips recorded); whatever is left after the run ends is a Sonnet 5.5 session with the rule verbatim.

## 3. The migration sequence

Each phase has a reversal and a parity test. Nothing in flight stops.

Phase A, personal-config alone (row 121, done this week). Claims are serialized by the lock and written by rename. Parity: the race harness (two `passoff claim` at once, 30 runs, two shapes) at 0 double claims and 0 lost updates; before the change it was 30 and 30 of 30. Reversal: revert the commit; the board format is unchanged.

Phase B, wk P2 in the wk repository, then ezhomesteading, with leases disabled (rows 7, 3). `wk claim` writes `leases.json` and `workspaces.json`; `personal-config passoff claim` keeps writing the board; the two coexist, and the `claims` reconcile rule reports a mismatch with the engine winning. Parity: the claim race harness run against `wk claim` (0 of 30 double claims); `passoff next` output on copies of the three live boards byte-compared before and after the rows land. Reversal: `leases.enabled = false` (already the default) and the skills call `passoff claim` alone.

Phase C, wk P3 in the wk repository first (row 4), then ezhomesteading, then personal-config. The worker lands onto `wk/landing`; `main` moves only by `wk land --apply`; the prose land step stays in each repository's rendered skills. Parity: a replay of the 2026-10-16 landings of rows 1, 2, 6 and 8 in a scratch clone, comparing the fast-forward target of each with `main`'s history, and the shim log of row 10 run over the loop. Reversal: `landing.enabled = false`.

Phase D, wk P4 (row 5) in the same order of repositories. The serial gate runs in the trial; `main` moves on green; then, per repository, the prose land step is deleted from the rendered skills (decision 13) and the completion gate reads the gate list from `~/.wk/<repo>.toml`. Parity: twenty changesets through the queue in a scratch repository with a bare remote, every one landed by the worker, `main` equal to the landing ref, and the pushed `main` equal to the local one. Reversal: `landing.ff_main = false`.

Phase E, after a week of P3 and P4 in each repository: `/land-worktrees` is retired (its scripts are the owner's, so this is a proposal, not a row here); `internal/daemon.Server` and `Client` are deleted; leases are turned on where decision 4's test says so.

What is deleted when: the daemon's socket half after P3 lands (phase E); the prose land step per repository after P4 lands there (phase D); `/land-worktrees` after phase E; the `drop-the-last` fallback when row 5 lands; nothing from `standard/` (frozen).

## 4. Acceptance criteria

Each is a command and its expected result.

- Row 121: the race harness prints `case_A_double_claims=0 case_B_lost_updates=0` over 30 runs; `bun test tests/claim-lock.test.ts` passes; `personal-config undo` from a directory with no backup of its own prints "No backup of files under"; `undo` with no TTY and no `--yes` exits 1 and names the flag; a claim on the worked-example board (adapted stamp) writes two files and the row reads `IN FLIGHT`.
- Row 3: `wk claim 3` in a workspace then `wk status --json | jq .schema` prints `1`; a second `wk claim` of an overlapping lease from another root exits 2 and names the holder; `kill -9` of `wk claim` at any point leaves `leases.json` parseable and the retry idempotent; `wk hooks install` writes no `PreToolUse` entry while `leases.enabled = false`.
- Row 4: with `landing.enabled = true` in a scratch repository, `wk submit` returns an id, `wk land --loop` exits 0 with the queue empty, `git rev-parse wk/landing` equals the predicted commit, and a `kill -9` of the worker between the CAS and the append produces one landing commit after the next pass, not two.
- Row 5: `batch_max = 1` lands one changeset per gate run; with `batch_max = 2` and a red gate, the loop re-gates each half and marks exactly the failing changeset `failed`; `main` moves only after a green gate; `git push` sends it when `landing.push` is set.
- Row 122: from a linked worktree, `personal-config passoff next` prints the board; the completion gate runs the repository's gate there (its log names the command).
- Row 123: `passoff show 121 --json | jq .schema` prints `1`; `passoff next --json` prints the same row `passoff next` prints.
- Row 124: `personal-config doctor .` on ezhomesteading's board reports each token in "Files it owns" that is prose or a glob, and exits as before (reports, no refusal).
- The reconcile: `wk status --json` and the board agree, checked by the `claims` rule, which exits 0 when every `IN FLIGHT` row has a live claim and every live claim an `IN FLIGHT` row.
- The builder-push text: the item prompts of wk rows 3, 4 and 5 and `PLAN-P2-P4.md:220` say one thing about who pushes `main` (done by the lead on 2026-10-16 for the unlaunched rows; row 7's file after it lands).
- `wk gc --json | jq '.[0] | keys'` includes `schema`, `repo`, `path`, `verdict`.

## 5. The risk register

Silent failures that remain, each with the signal that reveals it.

| Risk | Where | Signal | Closed by |
|---|---|---|---|
| The file matcher compares tokens by string, so `src/lib/` does not match `src/lib/board.ts` and a glob matches nothing | `src/lib/board.ts` `ownedFiles` | `doctor` grammar findings on the three live boards | Row 124, then a prefix-aware matcher |
| A `\|` inside a cell shifts every later column | the board parser | the grammar rule reports the cell | Row 124 |
| Three board parsers drift (`board.ts`, `session-banner.sh`, `internal/board`) | personal-config and wk | golden `passoff next` output on the three live boards, byte-compared | Row 123's `--json` becomes the one reader the banner and wk call |
| The completion gate runs nothing in a linked worktree | `templates/hooks/completion-gate.sh` | the gate's log is empty for a worktree stop | Row 122 |
| F18, a crash between the CAS and the `landed` append lands twice | row 4 | two landing commits with one tree | Row 4's `landing` record |
| F5, two checkouts with one basename share a lease store | `internal/gitx` | the common-dir guard refuses | Row 3 (step 1), decision 16 (step 2) |
| `go install` replaces the binary ezhomesteading's hooks and gc run | the owner's machine | `go version -m ~/go/bin/wk` names a commit not on `main` | Install only from `main` after the gate, with the dry-run comparison (decision 7) |
| Two writers on wk's `HANDOFF.md` and `PASSOFF.md` (the lead and a builder's close-out) | wk | a lost line in the ledger | `single_writer` leases once `leases.enabled` is on; until then one writer by convention |
| A stamped file that a person edited is overwritten by a re-run | the stamp guard | the preview shows the diff; the backup and `undo` are there | Record guard (`replace: never` on the seeds), unchanged by this plan |
| The daemon's memory ahead of disk after a failed save | `internal/daemon` | probe D of the kill test | Decision 3: no resident daemon |
| `tests/fold-migrate-ranges` depends on `readdir` order | personal-config CI | Linux job red | Fixed 2026-10-16 (`2136562`); CI's Linux job stays the proof |
| A wrong repository found from a worktree by `cwd` | `repo-config.ts`, `passoff.ts` | none today | Row 122, Fable 5.1 review |

## 6. Decisions this session took under the standing rule

- Row 7: this session wrote option (a) in `KILL-TEST.md` at 22:30 EDT; the lead had decided the allowlist at 22:26 EDT and relaunched. The lead's call stands (its row, decided first, fail closed); the note in `KILL-TEST.md` says so.
- Row 121 absorbs the adapted-stamp fix (an in-place `edit()` is exempt from the stamp guard by an explicit `inPlace` mark), because it sits in the claim's write path and was silent: `buy-in`'s and `felto`'s boards and ezhomesteading's ledger are adapted-stamped. Reversal: drop the mark; the guard then keeps adapted files again.
- The claim lock waits up to 10 s before it refuses, instead of refusing at once, so two claims of different rows both land (0 refusals in 30 runs). Reversal: `waitMs: 0`.
- `undo` keeps `--force` as skip-the-confirm in a terminal and takes `--yes` as the no-terminal answer, mirroring `confirmWrite`. Reversal: one flag.
- The kill-test harness lives in a scratch clone, not on `main`: it is a probe, and `cmd/kt` would ship a second binary.
