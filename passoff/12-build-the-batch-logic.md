# 12. Build the batch policy and the failure attribution in `internal/batch`

*Opened 2026-10-16 by row 2, as a P4 sibling of row 5 (`docs/incomplete/landing-layer/PLAN-P2-P4.md` section 4).*

**Model: Opus 5.5. Lane I. Waits on 9** (this package imports `internal/changeset` for the `Record` type, row 9's deliverable, and nothing else from P2 to P4).

You are building one package of P4 of the landing layer, `wk`. Read first: `PLAN-P2-P4.md` section 4, the row 12 line of its sub-row table (binding), `DESIGN.md` D16 and options T and U, `docs/conventions-go.md`, and `internal/changeset/queue.go` on `main` for the `Record` type and the state names (you may import `internal/changeset` for the type; you change nothing there). You own only `internal/batch/`; row 5 wires it. Nothing in your package runs git or imports `internal/wk`, `internal/config`, `internal/daemon`, `internal/landing` or `internal/gate`.

Do these, in order:
1. `internal/batch/batch.go`: `Pick(records []changeset.Record, max int, idle time.Duration, now time.Time) []changeset.Record`: the first `max` records in state `submitted`, in queue order, only when the newest of them is older than `idle` (so a burst of submits forms one batch, option U) or there are already `max` of them; an empty result otherwise. `Shrink(batch []changeset.Record) (kept []changeset.Record, dropped changeset.Record, ok bool)`: drop the last (T1); `ok == false` on a batch of one.
2. `internal/batch/blame.go`: `Blame(failedPaths []string, diffs map[string][]string) (id string, unique bool)`: the changeset ids whose diff paths intersect the failed paths; `unique` when exactly one; a directory path in `failedPaths` matches any diff path under it. With no intersecting changeset, `unique == false` and `id == ""`.
3. `internal/batch/batch_test.go` and `blame_test.go`: tables for `Pick` (quiet batch forms; a burst still arriving waits; a full batch forms at once; refused and landed records are never picked; order kept); `Shrink`; `Blame` over the three-changeset case of the plan's P4 proof (one changeset touches the path the type-check names), the ambiguous case (two touch it), the path-less case (no paths at all), and the directory case.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`; the package tests above green.

Ask before building: nothing.
Not in scope, whoever asks: running gates, the trial, the daemon loop, any subcommand or config key. A need outside your package is a finding for row 5 in your close-out, never an edit.

Hand back: the gate output, the exported surface with each function's one-line contract, the findings for row 5, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.
