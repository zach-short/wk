# 9. Build the changeset tree and the queue file in `internal/changeset`

*Opened 2026-10-16 by row 2, as a P3 sibling of row 4 (`docs/incomplete/landing-layer/PLAN-P2-P4.md` section 3).*

**Model: Fable 5.1. Lane A. Waits on 3.**

**Model: Fable 5.1** (Deep tier): the tree this package writes is what lands, and a tree that silently leaves out an untracked file or takes in an ignored one passes every gate. If you are not on Fable 5.1, stop and say so.

You are building one package of P3 of the landing layer, `wk`. Read first: `PLAN-P2-P4.md` sections 1 (PD-7, PD-8) and 3, the row 9 line of its sub-row table (binding), `DESIGN.md` option O and D15, `docs/conventions-go.md`, `internal/gitx/gitx.go` (`Run`, `Commit`; you may call `gitx`, never change it) and `internal/testrepo/testrepo.go` (your fixture). You own only `internal/changeset/`; row 4 wires it. Nothing in your package imports `internal/wk`, `internal/config`, `internal/daemon` or `internal/landing`.

Do these, in order:
1. `internal/changeset/tree.go`: `Tree(workspace string) (tree string, err error)`: with `GIT_INDEX_FILE` set to a temporary file beside the workspace's git directory, `read-tree HEAD`, `add -A`, `write-tree`; the temporary index is removed on every path, and the workspace's own index is never opened for writing. `Base(workspace, tip string) (base string, err error)`: `merge-base HEAD <tip>`, an error naming both when there is none (PD-7). `Head(workspace) (commit, branch string, err error)`.
2. `internal/changeset/queue.go`: `Record{ID, State, Tree, Base, Message, Row, Workspace, Commit string, Paths []string, Gate, Reason string, At time.Time}`; states `submitted`, `refused`, `landed`, `gated`, `failed` as constants; `Queue{Path string}` with `Append(Record) error` (one JSON line, `O_APPEND`, fsync) and `Load() ([]Record, error)` folded to the last line per id in first-seen order, a corrupt line an error naming its number; `NewID(tree, base string, at time.Time) string`, the first 12 hex of the SHA-1 of the three joined by newlines.
3. `internal/changeset/changeset_test.go` in a `testrepo` fixture: an untracked non-ignored file is in the tree (`ls-tree -r`), an ignored file is not, a modified tracked file carries its new blob, a deleted tracked file is absent; the workspace's `.git/index` (or the worktree's private index) is byte-identical before and after `Tree`; no temporary index remains; `Base` on a branch with its own commits returns the merge base and on an unrelated history returns an error; `Load` folds three lines of one id to the last; a corrupt line names its number.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`; the package tests above green.

Ask before building: nothing.
Not in scope, whoever asks: prediction, landing, sync, the daemon ops, any subcommand or config key (rows 10 and 4). A need outside your package is a finding for row 4 in your close-out, never an edit.

Hand back: the gate output, the exported surface with each function's one-line contract, the findings for row 4, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.
