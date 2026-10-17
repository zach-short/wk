# 12. Build the batch policy and the failure attribution in `internal/batch`

> **Claimed `IN FLIGHT` 2026-10-16.**

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

## Close-out, 2026-10-16

Built by Opus 5.5 on branch `ap/row-12` from `main` at b1d97ea. The new package `internal/batch/` holds the batch policy and the failure attribution of P4 as pure logic over `changeset.Record`. It imports `internal/changeset` for the type and the state names and nothing else from the repository, runs no git and writes nothing.

### What changed and why

- `internal/batch/batch.go`: `Pick` and `Shrink`, the batch policy of option U and the T1 fallback of D16.
- `internal/batch/blame.go`: `Blame`, the T3 attribution of D16.
- `internal/batch/batch_test.go` and `internal/batch/blame_test.go`: the tables the prompt names, plus edge rows for the calls below.

The exported surface, one contract each:

- `Pick(records []changeset.Record, max int, idle time.Duration, now time.Time) []changeset.Record` returns the first `max` records in state `submitted`, in queue order, when there are already `max` of them or the newest of them is at least `idle` old at `now`; otherwise nil.
- `Shrink(batch []changeset.Record) (kept []changeset.Record, dropped changeset.Record, ok bool)` drops the last record of a red batch; on a batch of fewer than two it returns the batch as `kept`, a zero `dropped` and `ok == false`.
- `Blame(failedPaths []string, diffs map[string][]string) (id string, unique bool)` returns the one changeset id whose diff paths equal or sit under a failed path, with `unique == true`; with no match or several, it returns `""` and false.

The plan's sub-row table also lists "the state transitions of scope item 2". The prompt names no function for them and the prompt is binding, so no transition function was built; the transitions are row 5's loop, and the findings below say what that loop must do with these three results.

### Calls made in the build, each with its reversal

- `Pick` forms a batch when the newest record's age equals `idle` exactly, so `idle == 0` means no wait. Reversal: change `<` to `<=` in the idle test of `Pick` and the "quiet exactly idle long forms" row.
- `Pick` with `max < 1` picks nothing. A negative `max` would otherwise take the whole queue.
- `Blame` returns `""` when several changesets match, never one of them, because the map's iteration order would make the pick random.
- `Blame` cleans each path with `path.Clean` and ignores an empty failed path, `.` and `/`, because every diff path is under the root and a root names no changeset.
- `Shrink` caps `kept` at its own length, so an append to `kept` cannot overwrite the dropped record in the caller's batch (`TestShrinkKeptDoesNotAliasDropped`).

### Now fixed

`go test -race -count=1 ./internal/batch/...` is green, with 10 `Pick` rows, 4 `Shrink` rows, the alias test and 11 `Blame` rows. The gate line `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'` exits 0 with `0 issues.`

### Questions answered

None were asked ("Ask before building: nothing"). The prompt's table cases are each one row: a quiet batch forms, a burst still arriving waits, a full batch forms at once, refused and landed records are never picked, order is kept, and for `Blame` the three-changeset case of the P4 proof, the ambiguous case, the path-less case and the directory case.

### Findings for row 5, and what is left owed

- `Pick` takes only `submitted` records. If the loop writes a `gated` line before the `landed` line and the daemon stops between them, that changeset is never picked again. Write `gated` and `landed` together, or recover `gated` records when the daemon starts.
- A unique `Blame` is a hypothesis, not a proof. When changeset A removes a symbol and the type-check prints the path of changeset B, which calls it, `Blame` names B. Mark the blamed changeset `failed` only after the re-gate without it goes green; when that re-gate is still red, fall back to `Shrink`. This is the silent case the plan's unaccepted Fable review recommendation is about.
- `Shrink` writes nothing: the dropped record stays `submitted`, and after the kept records land it is first in queue order, so the next `Pick` takes it again. A red batch of one (`ok == false`) is the changeset to mark `failed`, with the gate and whatever paths the gate named, which may be none.
- `Blame` needs repository-relative paths on both sides: row 11's `gate.Paths` for `failedPaths`, and each batch member's `diff-tree --name-only` of its own `Base` and `Tree` for `diffs`, keyed by `Record.ID`. An absolute path never matches.
- `Pick` measures quiet by `Record.At` of the folded record, which for a `submitted` record is its submit time, against the `now` the caller passes.
- Owed: the wiring, the trial, the gate runs and the live proof, all row 5's.
