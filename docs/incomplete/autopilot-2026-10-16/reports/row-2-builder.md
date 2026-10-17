# Row 2, builder report (Fable 5.1, agent a2ba9583664514a09), arrived 2026-10-16 21:03 EDT

Commit `d51f151` on `ap/row-2` from `a4d4ed2`. Not merged, not pushed. No trailer. No hook block. Patch: `/private/tmp/claude-501/-Users-zachshort-Projects-personal-config/eae52d80-e99c-41e1-8131-0863be440d2a/scratchpad/row-2.patch` (602 lines).
Diffstat main...HEAD: 13 files, +453/-26: DESIGN.md 21; PLAN-P2-P4.md 217 (new); passoff/2 19 (close-out); passoff/3 27, passoff/4 28, passoff/5 25 (rewritten as integration prompts); passoff/6 to 12 (7 new files, 142 lines).
Files added (8): docs/incomplete/landing-layer/PLAN-P2-P4.md, passoff/6-build-the-wk-daemon.md, passoff/7-build-the-lease-store-and-judge.md, passoff/8-build-the-board-cell-parser.md, passoff/9-build-the-changeset-and-queue.md, passoff/10-build-the-landing-plumbing.md, passoff/11-build-the-gate-runner.md, passoff/12-build-the-batch-logic.md.

## Gates seen
Item gate twice (baseline 20:55 EDT before edits; again before the commit; no code changed): exit 0 both, internal/wk 39.7 s then 43.6 s, 0 issues. Contraction and em dash sweep (regex of ~/.claude/hooks/no-em-dash.sh) over the plan and passoff/*.md: nothing.

## What changed
1. DESIGN.md: block "Amendments ratified 2026-10-16, D22 to D26" after D21: D22 = BD-16 (amends D8), D23 = BD-17 (amends D7, D5), D24 = BD-19 (amends D9), D25 = BD-22 (section 8 targets), D26 = one-file config (extends D19, D13). D7, D8, D9 and the section 8 P1 row carry dated "(Amended/Superseded ... by D2n)" tags in place; nothing deleted. D14 tagged: condition met (~/.claude/rules/commits.md:4), so P3 waits on P2 only. Answers section records both answers by label, "opus review of 3,5" and the lean instruction (quote shortened to "lean, concise" because the hook refuses contractions inside quotes).
2. PLAN-P2-P4.md (217 lines): section 0, 20 verified facts with file and line and the baseline gate; section 1 PD-1 to PD-12 with reversals; sections 2 to 4 (P2, P3, P4): scope items citing the design, toml keys with defaults, sub-row table with exact owned files, Done when from section 8 as D14/D20/D25 amend it, gate line, review model, "Not built, and why"; 5 order and model map; 6 protocol; 7 answers.
3. Rows 3, 4, 5 rewritten as integration prompts; rows 6 to 12 as sibling prompts. Row 2 close-out appended.

## Sub-rows (owner's split request)
Each phase = 2 or 3 siblings on disjoint new packages + the integration row (3, 4, 5), which alone owns cmd/wk/main.go, internal/config/config.go, internal/wk/*, README.md and the config files. Siblings wait only on the phase before; integration waits on its siblings.
Board rows (Task | Model | Lane | Waits on | Files it owns):
- 6 | Build the wk daemon: unix-socket server and client in `internal/daemon` | Opus 5.5 | A | 2 | `internal/daemon/` (daemon.go, client.go, spawn.go, state.go, daemon_test.go)
- 7 | Build the lease store, the edit judgment and the `Bash` word walk in `internal/lease` | Opus 5.5 | A | 2 | `internal/lease/` (store.go, judge.go, bash.go, tests)
- 8 | Build the board cell parser in `internal/board` | Opus 5.5 | A | 2 | `internal/board/` (board.go, board_test.go)
- 9 | Build the changeset tree and the queue file in `internal/changeset` | Fable 5.1 | A | 3 | `internal/changeset/` (tree.go, queue.go, changeset_test.go)
- 10 | Build the landing plumbing in `internal/landing`: predict, land by CAS, sync, fast-forward | Fable 5.1 | A | 3 | `internal/landing/` (predict.go, land.go, sync.go, ff.go, landing_test.go, the git shim)
- 11 | Build the gate runner, the path extractor and the memory floor in `internal/gate` | Opus 5.5 | A | 4 | `internal/gate/` (run.go, paths.go, memory.go, tests)
- 12 | Build the batch policy and the failure attribution in `internal/batch` | Opus 5.5 | A | 4 | `internal/batch/` (batch.go, blame.go, tests)
New cells for rows 3, 4, 5:
- Row 3 Files: `cmd/wk/main.go`; `internal/config/config.go`; `internal/settings/settings.go` (sequential overlap with ezhomesteading item 200); `internal/wk/lease.go`, `internal/wk/leasehook.go`, `internal/wk/daemon.go`, `internal/wk/hooks.go`; `README.md`; `~/.wk/wk.toml`; `~/.wk/ezhomesteading.toml`; `docs/incomplete/landing-layer/PLAN-P2-P4.md` (As built, P2). Waits on: 2; 6, 7, 8; /Users/zachshort/Projects/ezhomesteading/PASSOFF.md item 200.
- Row 4 Files: `cmd/wk/main.go`; `internal/config/config.go`; `internal/wk/landing.go`, `internal/wk/submit.go`, `internal/wk/status.go`; `README.md`; `~/.wk/wk.toml`; `~/.wk/ezhomesteading.toml`; `docs/incomplete/landing-layer/PLAN-P2-P4.md` (As built, P3). Waits on: 3; 9, 10.
- Row 5 Files: `cmd/wk/main.go`; `internal/config/config.go`; `internal/wk/trial.go`, `internal/wk/gatekeeper.go`, `internal/wk/landing.go`, `internal/wk/status.go`; `README.md`; `~/.wk/wk.toml`; `~/.wk/ezhomesteading.toml`; `docs/incomplete/landing-layer/PLAN-P2-P4.md` (As built, P4). Waits on: 4; 11, 12.

## Build-level calls (PD-1 to PD-12, reversals in the plan)
PD-1 socket at ~/.wk/<repo>/wk.sock; PD-2 the hook makes no git call (--repo baked in); PD-3 leases.json by atomic rename; PD-4 Bash guard walks words with no shell parser (so row 7 never touches go.mod while row 200 adds mvdan.cc/sh); PD-5 launchd gc agent stays; PD-6 P3 lands ungated onto wk/landing, main moves only by wk land --apply until P4; PD-7 base = merge-base HEAD <tip>, wk sync refuses a branch with its own commits; PD-8 one JSONL queue; PD-9 trial reuses the land-worktrees-trial marker; PD-10 gate machine limits come from row 200's slots, P4 keeps only gates.min_free_mb; PD-11 no gates configured means no fast-forward; PD-12 P2 starts from whatever main holds.
Touches a ratified decision: D13 L3 (insert-only ledger edits) not built; ledger files are single_writer (L1). Reversal: an insert_only key and the new_string-contains-old_string check.

## Questions for the owner
1. Row 3 "Ask before building", marked owner's call, recommendation defer: build D13 L3 now, or leave the ledger files single-writer. Siblings do not depend on it. Rows 4 to 12 ask nothing.
2. Not a blocker: turning leases on (leases.enabled = true) in ~/.wk/ezhomesteading.toml and ~/.wk/wk.toml is the owner's step after row 3; row 3 ships both with enabled = false.

## Numbers claimed
Plan 217 lines; 20 facts; patch 602 lines; sibling prompts 18 to 23 lines each. No measurement beyond the gate. No ledger or migration number.

## Not verified
PreToolUse deny JSON field names on Claude Code 2.1.301 (row 3 step 1 verifies first); vm_stat field names (row 11); the 103-byte socket path limit (from sun_path, not re-measured).

## Left owed to this session
Append rows 6 to 12 to PASSOFF.md; update rows 3, 4, 5 cells; land after the audit; log line. Builder touched nothing in HANDOFF.md, PASSOFF.md, CLAUDE.md, SLOTS-*.md or the primary checkout.
