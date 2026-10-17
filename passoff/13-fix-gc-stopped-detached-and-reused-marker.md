# 13. Fix gc for a stopped subagent on a detached HEAD, and clear the stop marker on reuse

*Opened 2026-10-16 by the integration session (Fable 5.1), from findings F1 and F2 of the wk code audit in the research review of 2026-10-16. Pre-authorized by the review's prompt (section 2.1, item a) and by decision 11 of the owner's decision record.*

**Model: Fable 5.1. Lane J. Waits on nothing. Files it owns: `internal/wk/gc.go`, `internal/wk/hook.go`, `internal/wk/agent_test.go`, `internal/wk/hook_test.go`.**

F1 (silent): `internal/wk/gc.go` skipped the "base holds its commits" test for a worktree with the `wk-agent-stopped` marker and marked it removable. A detached HEAD has no branch, so `wk gc --apply` removed the worktree and its commits ended on no ref; the removable line itself said "its detached commits are not on main". `README.md:67` defends the exception with "because its branch stays", which only a branch can do.

F2 (loud alone, silent together with F1): only `SubagentStop` writes the marker and nothing cleared it, and `WorktreeCreate` reused an existing worktree of the same name with the marker in place, so a live worktree got a stopped subagent's 15-minute quiet window.

Do these:
1. In `judge`, take the stopped exception only when `w.Branch != ""`. A detached HEAD with unlanded commits is kept with the reason "its detached HEAD has commits main does not hold". `stoppedReason` loses its detached arm, which nothing reaches.
2. In `hookRun.create`, remove the stop marker from a reused worktree and write one line to `hook.log`.
3. A test for each: a stopped, detached, clean worktree with an unlanded commit is kept and survives `--apply`, then goes once `main` holds the commit; a reused worktree has no marker afterwards.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, in the worktree.

Not in scope: `README.md` (ezhomesteading item 200 owns it, and its sentence stays true), the gc skip list (row 5), `go install` (decision 7, run after this lands).

## Close-out, 2026-10-16

Built by the integration session in `.claude/worktrees/fx-gc` on branch `fx/gc` from `029aaa6`, fix commit `ddc541b`. Both tests ran red at `029aaa6` before the fix (`TestGCKeepsAStoppedDetachedSubagentWorktreeWithUnlandedCommits`: verdict "removable", reasons ["stopped subagent, clean and idle; its detached commits are not on main"]; `TestHookReuseClearsTheStopMarker`: "the stop marker survived the reuse") and green after it. Gate exit 0 on the fixed tree (`internal/wk` 36.5 s under `-race`, 0 lint issues). Changed: `internal/wk/gc.go`, `internal/wk/hook.go`, `internal/wk/agent_test.go`, `internal/wk/hook_test.go`. Not verified: a live gc pass on ezhomesteading with the new binary; decision 7 runs that comparison before the install stays.
