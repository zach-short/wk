# 2. Ratify the P1 amendments and write the plan for P2 to P4

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
