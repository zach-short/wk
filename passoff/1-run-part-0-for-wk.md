# 1. Run Part 0 for wk: fill CLAUDE.md and the ledger's environment

> **Claimed `IN FLIGHT` 2026-10-16.**

*Opened 2026-10-16, when the landing-layer work moved here from `~/Projects/ezhomesteading` (its board row 196, now SUPERSEDED).*

**Model: Opus 5.5. Lane B. Waits on nothing.**

You are picking up `wk`, a Go CLI that makes warm git worktrees for parallel agents (read `README.md`). `CLAUDE.md` and `HANDOFF.md` here were written on 2026-10-16 by `personal-config init` as empty templates: no rules, no stack detail, no architecture, no directory map, no commands. Every later item needs the gates written down and verified.

Do these, in order:
1. Read `PART0-PROMPT.md` at the repo root and follow it. It is the Part 0 inventory prompt for this repo.
2. Fill `CLAUDE.md`: the stack (Go 1.24, `GOTOOLCHAIN=local`, macOS on APFS only), the architecture (`cmd/wk`, `internal/*`), the directory map, and the gate line below as fenced one-command blocks, each verified by running it.
3. Fill `HANDOFF.md` sections Environment, Code map, Invariants and Known facts from what you verify. The design history is in `docs/incomplete/landing-layer/`; cite it, do not copy it.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`

Ask before building: nothing.
Not in scope, whoever asks: any code change; the landing-layer plan (item 2).

Hand back: the gate output, the filled files, and one log line in `HANDOFF.md`: `- <date> · closed item 1`. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.
