# 32. gc held-file checks left open by row 31's review, and a slot test that reads the caller's slot

*Opened 2026-11-21 from the third Fable review of row 31 (close-out of `passoff/31-gc-removes-untracked-files-the-base-already-holds.md`, "Sign-off"). Re-check every fact on `main` before you build.*

**Model: Opus 5.5 builds; a Fable 5.2 review reads the deletion path again. Lane gc. Waits on nothing. Files it owns: `internal/gitx/gitx.go`, `internal/gitx/gitx_test.go`, `internal/wk/gc.go`, `internal/wk/gc_test.go`, `cmd/wk/slots_test.go`.**

## Do these, in order

1. **N9, first.** `fileBlob` in `internal/gitx/gitx.go` reads the base tree with `ls-tree` and never checks that the blob object is in the store. With the blob missing (a corrupt store, or a partial clone whose blob is only at a remote), `HeldBlob` calls the file held and `wk gc --apply` deletes it. Add `git cat-file -e <blob>` to `HeldBlob`; a missing object means not held. Test it by deleting the loose object in a throwaway repository.
2. **N8.** The x-bit test uses `Perm()&0o111`. Git reads only the owner bit, `0o100`, so a disk mode `0654` against a base `100755` is held although git reads the file as `100644`. Test `0o100` and fix the comment that says "any x bit". Add a test with mode `0654`.
3. **N11, hardening.** Never hold a path whose base name is `.gitignore`: deleting it can make ignored files untracked, and git then refuses the removal after gc deleted files. Add a test.
4. **The slot test.** `cmd/wk` `TestRunPassesTheExitStatusThroughAPlace` (`cmd/wk/slots_test.go:63`) fails when the gate itself runs inside `wk run --slot`, because it inherits `WK_SLOT_HELD` and the nested run does not log. Since 2026-11-21 ezhomesteading's slots file routes `go test`, so a session started there wraps this gate. Make the test clear `WK_SLOT_HELD` for the run it starts (`t.Setenv`, or the child's environment), and prove it passes with `WK_SLOT_HELD=heavy` set.

Not in scope: N1, N2, N3, N5 and N10 (row 31's close-out records them), any change to `DirtyPaths`, a release tag.

## Done when

```bash
GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'
```

passes both with and without `WK_SLOT_HELD=heavy` in the environment, and a dry-run `wk gc` with the branch binary (built with `go build -o` to a temp path, never `go install`, never `--apply` on a real repository) in `/Users/zachshort/Developer/dan-short/ezhomesteading` gives the same removable list as `main`.

## Rules this prompt carries

A plain worktree outside the repository (`.claude/worktrees/` is not ignored here), as row 31 used. `GOTOOLCHAIN=local`. Never `go install ./cmd/wk`. Never write under `~/.wk` or `~/Library/LaunchAgents`. No comment that says what code does. No `Co-Authored-By:` trailer and no "Generated with" line. Never push.
