# 4. Build P3, the landing queue: `wk submit`, merge prediction and the landing of `main`

*Opened 2026-10-16, when the landing-layer work moved here from `~/Projects/ezhomesteading` (its board row 196, now SUPERSEDED).*

**Model: Fable 5.1. Lane A. Waits on 3.**

Build phase P3 of the landing layer as `docs/incomplete/landing-layer/PLAN-P2-P4.md` describes it. Item 2 writes that plan and replaces this prompt with the full one. If the plan does not exist, or this prompt still says so, stop and report: the row is not ready.

Model: Fable 5.1. The design is `docs/incomplete/landing-layer/DESIGN.md` (sections 3, 4 and 8 for P3).

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, plus the "Done when" proof of P3 in `DESIGN.md` section 8.

Hand back: the gate output and the proof, recorded under the phase in the plan, and one log line. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.
