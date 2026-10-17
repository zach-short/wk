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

## Close-out, 2026-10-16

Built by an Opus 5.5 builder on branch `ap/row-1` under `/autopilot`, from `a4d4ed2`. No code changed.

What changed and why. `CLAUDE.md` is filled from the Part 0 inventory (`PART0-PROMPT.md` section 0.1): the stack with versions, the architecture, a directory map, the gates as one-command blocks, the gates that lie, what an agent cannot verify, how the repo ships, and the repo-local traps. Every command in it was run on 2026-10-16 in the worktree, and the full gate line also ran green in a `git archive HEAD` copy with an empty module cache. Its stamp now ends `· adapted`, so a later `personal-config setup` leaves it alone. The ledger sections (Environment, Code map, Invariants, Known facts and quirks) and the two Appendix A placeholders of `docs/AGENT-PRACTICES.md` are untracked files this run could not write; their text went to the orchestrating session in the hand-back.

What is now fixed. The gate is `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, green in about 44 s (`internal/wk` about 40 s under `-race`). The fresh-checkout step is `GOTOOLCHAIN=local go mod download`; nothing gitignored is needed for the gates. The build is `GOTOOLCHAIN=local go build ./...`.

Questions answered (PART0-PROMPT.md 0.1). The gates are the four steps above, taken from the toolchain because there is no CI. Four can mislead: `gofmt -l .` in the main checkout reads every nested worktree under `.claude/worktrees/`; `test -z "$(gofmt -l .)"` passes on a file that does not parse; a Go command without `GOTOOLCHAIN=local` may run on a downloaded toolchain; the `plutil -lint` check skips when `plutil` is missing. A fresh checkout lacks only the module cache. There is no numbered shared resource beyond the board's item numbers (`git grep -il migrat` found nothing). Nothing ships on merge: there are no tags and no CI, and ezhomesteading runs `~/go/bin/wk`, built on 2026-10-16 from `30989b5` with a dirty tree.

Left owed. Writing the untracked text into `HANDOFF.md` and `docs/AGENT-PRACTICES.md` (orchestrating session). Setting `gateCommand` in `.personal-config.json`; the proposed value is in the hand-back. The rest of Part 0 in `docs/AGENT-PRACTICES.md` (0.4 cuts, 0.6 GATE 0, 0.7 stamp and delete), which needs that untracked file. Three owner questions in the hand-back: the `gateCommand` value and its 45 s cost on every stop, the stale `README.md:5` pointer to ezhomesteading, and the prunable `landing-layer-phase3` worktree entry.
