# 31. gc removes a worktree whose only changes are untracked files the base already holds

*Opened 2026-11-21, at the owner's request ("ezh config changes", then "continue"), from a read of wk's own logs on this machine for 2026-11-06 to 2026-11-21.*

**Model: Opus 5.5 builds; a Fable 5.2 review reads the deletion path. Lane gc. Waits on nothing. Files it owns: `internal/wk/gc.go`, `internal/wk/gc_test.go`, `internal/gitx/gitx.go` (a new helper only; `DirtyPaths` stays as it is), `internal/gitx/gitx_test.go`, `README.md` (the "Remove finished workspaces" section).**

The tier: the change makes gc delete files. A wrong rule compiles, passes every gate and loses work with no sign, so one narrow Deep review reads the deletion path. The build is Default because the rule is stated here in full and the tests can prove each branch.

## What is true now

- In ezhomesteading on 2026-11-21, the last 44 hourly `wk gc --apply` passes removed 0 of 39 worktrees (`~/.wk/ezhomesteading/gc.log`).
- 26 of the 39 are kept only for "N changed paths", and every one of those paths is an untracked file under `docs/incomplete/audit-2026-11-19/` or `docs/incomplete/audit-2026-11-06/`. An `/audit` agent writes its finding file in its own worktree, the lead copies it into the main checkout and commits it there, and the agent's copy stays behind.
- 74 of those 76 untracked files are byte-equal to the file at the same path in the main checkout, which commit `92b0ff895` and the 2026-11-06 audit commit put on `main`. Two differ: `docs/incomplete/audit-2026-11-06/raw/security/changed-files.txt` and `docs/incomplete/audit-2026-11-06/raw/parity/triage.md`.
- `judge` in `internal/wk/gc.go` keeps any worktree for which `gitx.DirtyPaths` lists a path. `DirtyPaths` lists tracked changes and untracked files that are not ignored, and its comment requires it to call clean exactly what the land-worktrees `lib.sh` calls clean. So do not change `DirtyPaths`.
- `removeOne` runs `git worktree remove` with no `--force`, so git refuses a worktree with an untracked file. The CLAUDE.md rule "Never let gc force" stands.

## The rule to build

An untracked path is **held by the base** when all of these are true at the moment gc reads it:

1. It is in the untracked half of `DirtyPaths` (`git ls-files --others --exclude-standard`), not the tracked half. A tracked edit is never held.
2. It is a regular file. Not a symbolic link, not a directory, not anything else (`os.Lstat`).
3. The base commit's tree (the `survey.base` commit, `refs/heads/<workspace.base>`) has a blob at the same path, with mode `100644` or `100755`.
4. `git hash-object` of the file equals that blob's id.

A worktree whose changed paths are all untracked and all held by the base gets no "changed paths" reason. Every other reason (locked, a process inside, activity inside the quiet window, commits the base does not hold) still keeps it. Report the held files in the verdict text, for example "3 untracked files the base already holds are deleted with it", and give the count in the `--json` report as a new field. Do not rename an existing `Judgment` or `Report` field (CLAUDE.md, rule on `wk.Report`).

In `removeOne`, after the re-judge and before `git worktree remove`:

- Check each held file again: Lstat, a regular file, and the same hash against the same base blob. If any check fails, keep the worktree with a reason and delete nothing.
- Then delete those files one at a time with `os.Remove`, never a recursive removal.
- Then run `git worktree remove` with no `--force`, as now. If git refuses, the worktree stays, and the deleted files are still on the base, so nothing is lost. Say so in the kept line.

`wk gc` without `--apply` changes nothing, as now; it only shows the new verdict.

## Tests (in `gc_test.go` and `gitx_test.go`, each with `testrepo.Isolate()` and a temp `WK_HOME`)

- Removes a worktree whose only change is an untracked file equal to the base's blob, and deletes the branch by the existing ancestor rule.
- Keeps a worktree with an untracked file whose bytes differ from the base's blob at that path.
- Keeps a worktree with an untracked file at a path the base does not have.
- Keeps a worktree with a held file plus one tracked edit.
- Keeps a worktree whose held path is a symbolic link to an equal file.
- Keeps a worktree whose held file changes between the judgment and the removal (use the existing re-judge test pattern in `TestGCApplyJudgesAgainAtTheMomentOfRemoval`), and deletes nothing.
- The dry run (`TestGCDryRunChangesNothing` pattern) leaves the held files on disk.

## Done when

The wk gate, in the builder's worktree:

```bash
GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'
```

And a read-only proof on the real repository with the branch's binary, built to a temp path (never `go install`, never `--apply`): `WK=<tmp>/wk; (cd /Users/zachshort/Developer/dan-short/ezhomesteading && $WK gc)` shows the worktrees that hold only base-equal audit files as removable, and the two with differing files as kept.

## Not in scope

- Any change to `DirtyPaths`, the land-worktrees skill or `lib.sh`.
- Tracked edits, renames, or files equal to the main checkout's working tree but not to the base commit.
- A release tag or `brew upgrade`. The owner tags a release.
- Running `wk gc --apply` on a real repository.

## Rules this prompt carries

Work in your own worktree of `/Users/zachshort/Developer/zach-short/wk`: `git -C /Users/zachshort/Developer/zach-short/wk worktree add /private/tmp/claude-503/-Users-zachshort-Developer-dan-short-ezhomesteading/1c53f89c-6cfb-4a6d-9e3b-fd3208354035/scratchpad/wk-row-31-build -b row-31-build main` (wk has no `~/.wk/wk.toml` on this machine, so this is a plain worktree; it sits outside the repository because `.claude/worktrees/` is not ignored here). `GOTOOLCHAIN=local` on every Go command. Never run `go install ./cmd/wk`. Never write under `~/.wk` or `~/Library/LaunchAgents`. No comment that says what code does; keep the "why" comments. README text uses absolute dates, no contractions and no em dashes. Commit on your branch only, with no `Co-Authored-By:` trailer and no "Generated with" line. Never push.
