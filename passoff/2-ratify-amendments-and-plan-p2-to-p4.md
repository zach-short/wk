# 2. Ratify the P1 amendments and write the plan for P2 to P4

> **Claimed `IN FLIGHT` 2026-10-16.**

*Opened 2026-10-16, when the landing-layer work moved here from `~/Projects/ezhomesteading` (its board row 196, now SUPERSEDED).*

**Model: Fable 5.1. Lane A. Waits on nothing.**

**Model: Fable 5.1** (Deep tier): P3 moves `main` and pushes, and a wrong landing passes every gate. If you are not on Fable 5.1, stop and say so.

You are picking up the landing layer, a planned layer over git in four phases: P1 cheap workspaces (built: `wk new`, `wk gc`, the hooks, at `30989b5`), P2 path leases, P3 a landing queue, P4 one gate run per batch. The design is `docs/incomplete/landing-layer/DESIGN.md` (ratified 2026-10-16, D1 to D20; D14 and D20 supersede the "Zach moves main" text). The P1 plan and its as-built notes are `docs/incomplete/landing-layer/PLAN.md`. Build slots are `docs/incomplete/landing-layer/SLOTS-SCOPE.md` and `SLOTS-PLAN.md`, in flight as ezhomesteading row 200. These files were copied here from `~/Projects/ezhomesteading/docs/incomplete/landing-layer/` on 2026-10-16; this copy is now the live one.

Ask before building, in chat, each with a recommendation:
1. Ratify the amendments the P1 build made: BD-16 (amends D8), BD-17 (amends D7 and D5), BD-19 (amends D9) and BD-22 (section 8 targets), as `PLAN.md` line 394 lists them. Record the answer in `DESIGN.md`.
2. The design is written for ezhomesteading only (its composition roots, gates, `.env` files and paths). The owner wants wk in personal-config and in wk itself too. Should P2 to P4 read repo-specific rules from `~/.wk/<repo>.toml`, and what does that cost?

Then write `docs/incomplete/landing-layer/PLAN-P2-P4.md`: one phase per P, each with its scope items, its "Done when" proof from `DESIGN.md` section 8, the gate line, the dials, and where a Fable review sits. Then fill the prompts of items 3, 4 and 5 on this board from it, keeping their rows.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'` (no code changes here, but run it to record the baseline).
Not in scope, whoever asks: building any phase; row 200's slots work.

Hand back: the ratification and the decisions recorded, the plan, the three item prompts, and one log line. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.

## Close-out, 2026-10-16

Built by a Fable 5.1 session in the worktree `ap-row-2` (branch `ap/row-2`) from `a4d4ed2`, under `/autopilot`: the commit is on the branch only; the orchestrating session merges, pushes and removes the worktree after its audit.

**What changed and why.**

- `docs/incomplete/landing-layer/DESIGN.md`: D22 to D25 record the four P1 amendments (BD-16 amends D8; BD-17 amends D7 and D5; BD-19 amends D9; BD-22 amends section 8's targets) as dated decisions after D21, with the amended text of D7, D8, D9 and the section 8 P1 row tagged in place and nothing deleted. D26 records the one-file config decision (every repository-specific rule of P2 to P4 in `~/.wk/<repo>.toml`, one section per phase, safe defaults when absent). D14 carries a dated tag that its condition is met: `~/.claude/rules/commits.md` now allows every git command, so P3 waits on nothing but P2. The Answers section records the two answers by label. Nothing else in the file was rewritten.
- `docs/incomplete/landing-layer/PLAN-P2-P4.md`: the plan, 217 lines. Section 0 holds 20 facts verified in the code on 2026-10-16 with file and line, and the baseline gate (green, exit 0). Section 1 holds PD-1 to PD-12, each with a reversal. Sections 2 to 4 hold P2, P3 and P4, each with numbered scope items citing the design, the config keys with defaults, a sub-row table with exact files owned, the "Done when" from `DESIGN.md` section 8 as D14, D20 and D25 amend it, the gate line, the review and its model, and "Not built, and why". Section 5 is the order and model map; section 6 the session protocol; section 7 the answers.
- `passoff/3-*.md`, `4-*.md`, `5-*.md`: the placeholder prompts replaced by the integration prompts of P2, P3 and P4, each standalone, keeping rows 3, 4 and 5 and their wait chain. Row 3 also waits on ezhomesteading item 200.
- `passoff/6-*.md` to `passoff/12-*.md`: seven new sibling prompts, lane A, one package each, by Zach's instruction of 2026-10-16 to divide each item among more builders: 6 `internal/daemon`, 7 `internal/lease`, 8 `internal/board` (P2, wait on 2); 9 `internal/changeset`, 10 `internal/landing` (P3, wait on 3); 11 `internal/gate`, 12 `internal/batch` (P4, wait on 4). Siblings of one phase own disjoint files; the shared wiring (`cmd/wk/main.go`, `internal/config/config.go`, `internal/wk/*`) belongs to the integration row.

**Questions answered.** Both asked 2026-10-16 in chat and answered by option label. 1. The four amendments: "Ratify all four (Recommended)", recorded as D22 to D25. 2. Repository-specific rules from `~/.wk/<repo>.toml`: "Yes, one file (Recommended)", recorded as D26 and carried into the plan as `[leases]`, `[landing]` and `[gates]` plus `wk config show`. Also from the same round, recorded in the plan: an Opus 5.5 review on rows 3 and 5 ("opus review of 3,5"), a lean plan, and the sub-row split.

**Build-level calls made here, each reversible.** PD-1 to PD-12 in the plan. The one that touches a ratified decision: D13 L3 (insert-only ledger edits with the claimed-row exception) is not in the plan; the ledger files are single-writer paths instead. It is the one owner's call in row 3's "Ask before building", with the recommendation to defer it.

**Now fixed.** The P1 amendments have a home in the design; P3's wait on the commit rule is closed; the three build rows have prompts that stand alone and ask the owner one question between them.

**Left owed.** The orchestrating session: appends rows 6 to 12 to `PASSOFF.md` from the board lines in the row 2 report, updates the "Files it owns" cells of rows 3, 4 and 5, merges this branch, pushes and removes the worktree; writes the log line. Not verified here: the `PreToolUse` deny JSON field names on Claude Code 2.1.301 (row 3 step 1 verifies them before the hook is written). The baseline gate ran once more after the documents were written, with no code change, and was green.
