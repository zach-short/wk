# 9. Build the changeset tree and the queue file in `internal/changeset`

> **Claimed `IN FLIGHT` 2026-10-16.**

*Opened 2026-10-16 by row 2, as a P3 sibling of row 4 (`docs/incomplete/landing-layer/PLAN-P2-P4.md` section 3).*

**Model: Fable 5.1. Lane F. Waits on 2** (this package imports only `internal/gitx` and `internal/testrepo`, nothing from P2, so it starts with the P2 siblings).

**Model: Fable 5.1** (Deep tier): the tree this package writes is what lands, and a tree that silently leaves out an untracked file or takes in an ignored one passes every gate. If you are not on Fable 5.1, stop and say so.

You are building one package of P3 of the landing layer, `wk`. Read first: `PLAN-P2-P4.md` sections 1 (PD-7, PD-8) and 3, the row 9 line of its sub-row table (binding), `DESIGN.md` option O and D15, `docs/conventions-go.md`, `internal/gitx/gitx.go` (`Run`, `Commit`; you may call `gitx`, never change it) and `internal/testrepo/testrepo.go` (your fixture). You own only `internal/changeset/`; row 4 wires it. Nothing in your package imports `internal/wk`, `internal/config`, `internal/daemon` or `internal/landing`.

Do these, in order:
1. `internal/changeset/tree.go`: `Tree(workspace string) (tree string, err error)`: with `GIT_INDEX_FILE` set to a temporary file beside the workspace's git directory, `read-tree HEAD`, `add -A`, `write-tree`; the temporary index is removed on every path, and the workspace's own index is never opened for writing. `Base(workspace, tip string) (base string, err error)`: `merge-base HEAD <tip>`, an error naming both when there is none (PD-7). `Head(workspace) (commit, branch string, err error)`.
2. `internal/changeset/queue.go`: `Record{ID, State, Tree, Base, Message, Row, Workspace, Commit string, Paths []string, Gate, Reason string, At time.Time}`; states `submitted`, `refused`, `landed`, `gated`, `failed` as constants; `Queue{Path string}` with `Append(Record) error` (one JSON line, `O_APPEND`, fsync) and `Load() ([]Record, error)` folded to the last line per id in first-seen order, a corrupt line an error naming its number; `NewID(tree, base string, at time.Time) string`, the first 12 hex of the SHA-1 of the three joined by newlines (PD-13).
3. `internal/changeset/changeset_test.go` in a `testrepo` fixture: an untracked non-ignored file is in the tree (`ls-tree -r`), an ignored file is not, a modified tracked file carries its new blob, a deleted tracked file is absent; the workspace's `.git/index` (or the worktree's private index) is byte-identical before and after `Tree`; no temporary index remains; `Base` on a branch with its own commits returns the merge base and on an unrelated history returns an error; `Load` folds three lines of one id to the last; a corrupt line names its number.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`; the package tests above green.

Ask before building: nothing.
Not in scope, whoever asks: prediction, landing, sync, the daemon ops, any subcommand or config key (rows 10 and 4). A need outside your package is a finding for row 4 in your close-out, never an edit.

Hand back: the gate output, the exported surface with each function's one-line contract, the findings for row 4, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.

## Close-out, 2026-10-16

Done 2026-10-16 on Fable 5.1, branch `ap/row-9` from `main` at 344f8c7, in the commit that carries this section. Under `/autopilot` the orchestrating session merges and pushes after a Deep audit, so this row did not merge, push or remove its worktree.

What changed and why. Three new files, nothing else touched:

- `internal/changeset/tree.go`: `Tree(workspace) (tree string, err error)` runs `read-tree HEAD`, `add -A`, `write-tree` with `GIT_INDEX_FILE` set to a file created by `os.CreateTemp` inside the workspace's git directory (`gitx.GitDir`, so a linked worktree uses its private directory and `git status` never lists it). The empty file is removed again before `read-tree`, because git refuses an index file shorter than its header and creates one that is absent. A deferred `removeIndex` drops the index and its `.lock` on every return. `runIndexed` is `gitx.Run` with one more environment variable, copied because `gitx` takes no environment and `GIT_INDEX_FILE` has no `-c` form. `Base(workspace, tip)` is `merge-base HEAD <tip>` (PD-7); a tip that is not a commit and two histories with no common commit are each an error, the second naming HEAD's hash and the tip. `Head(workspace)` returns the commit and the branch without `refs/heads/`, empty when detached.
- `internal/changeset/queue.go`: `Record` with the eleven fields the prompt lists plus `At`, JSON tags in lowercase (`id`, `state`, `tree`, `base`, `message`, `row`, `workspace`, `commit`, `paths`, `gate`, `reason`, `at`; `row`, `commit`, `paths`, `gate` and `reason` carry `omitempty`). The states are the untyped string constants `Submitted`, `Refused`, `Landed`, `Gated`, `Failed`; row 12 imports these names. `Queue{Path}.Append` marshals one line, opens with `O_APPEND|O_CREATE`, writes, `Sync`s, closes. `Load` reads the file, folds to the last line per id in first-seen order, treats a missing file as an empty queue and names the line number of one that does not parse. `NewID` is the first 12 hex of `sha1(tree + "\n" + base + "\n" + at.UTC().Format(RFC3339Nano))` (PD-13).
- `internal/changeset/changeset_test.go`: nine tests in a `testrepo` fixture with a linked worktree, `TestMain` calling `testrepo.Isolate()`.

What is now fixed, and the one hazard this row checked. The prompt's sequence was probed in the scratchpad inside a sparse worktree shaped as `wk new` makes one (`sparse-checkout set --no-cone /* !/docs/incomplete/`). `read-tree HEAD` into a fresh index sets no skip-worktree bit, so the question was whether `add -A` would stage every path outside the cone as a deletion. It does not: `git add` checks each path against the worktree's sparse patterns itself, and the path outside the cone kept its HEAD blob. `TestTreeKeepsPathsOutsideTheSparseCone` pins that, so a later change to the sequence (for example copying the workspace's index instead of `read-tree HEAD`) is caught. The other tests cover the item's list: the untracked non-ignored file is in the tree, the ignored `.env` is not, the modified `README.md` carries its new content, the deleted `web/page.ts` is absent, the workspace's own index is not staged, `index` is byte-identical before and after in both the main checkout and a worktree, no `wk-index-*` file remains on success or after a failed `read-tree`, `Base` returns the fork point when both sides moved and errors on an orphan commit naming both hashes, `Head` on a branch and detached, `Load` folds four lines of two ids to two records with the last state, a corrupt second line names `line 2`, and `NewID` is 12 hex, deterministic and time-sensitive.

Questions it answered. "Ask before building: nothing", and none arose. One prompt detail resolved by probe: `rev-parse --symbolic-full-name HEAD` prints `HEAD` when detached, which `Head` maps to the empty branch.

Findings for row 4, none of them edits here:

1. An untracked file outside a sparse workspace's cone is not in the tree: `add -A` skips it, as git does for any add outside the cone. `wk submit` may want to say so when `workspace.sparse_exclude` is set, since the file is silently not submitted.
2. `Append` does not create the parent directory. `~/.wk/<repo>/` exists for the logs, so row 4 owns it.
3. `runIndexed` is a copy of `gitx.Run` plus `GIT_INDEX_FILE`. Reversal: a `gitx.RunEnv(dir, env, args...)` and this copy deleted, in the same change that folds row 10's second copy of the sequence (plan section 3, "Not built").
4. `Base` and `Head` take any directory git accepts; the main-checkout refusal (D15) is row 4's, before either is called.
5. `Record.State` is a plain `string`, as the prompt binds it, so the constants are untyped; a `type State string` (conventions T1) is a one-line change if row 12 wants it typed.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'` exit 0, `0 issues`, every package ok; `go test -race -count=1 ./internal/changeset/...` ok.

Left owed: nothing in this package. The merge into `main`, the push and the worktree removal belong to the orchestrating session under this run.
