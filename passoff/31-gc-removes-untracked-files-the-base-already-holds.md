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

## Close-out, 2026-11-21

Built on branch `row-31-build` by Opus 5.5, with `go1.26.11 darwin/arm64` and `GOTOOLCHAIN=local`.

What changed and why. `internal/gitx/gitx.go` has two new helpers and `DirtyPaths` is unchanged. `Untracked` runs the same `git ls-files --others --exclude-standard -z` as the untracked half of `DirtyPaths`, so gc can tell an untracked path from a tracked edit. `HeldBlob` applies rules 2 to 4: `os.Lstat` must give a regular file, `git --literal-pathspecs ls-tree -z --full-tree <base> -- <path>` must give exactly one `blob` entry at that exact path with mode `100644` or `100755`, and `git hash-object --no-filters <absolute path>` must equal that blob id. In `internal/wk/gc.go`, `judge` computes the held list only when every changed path is untracked and held (one path that is not held keeps all of them), drops the "changed paths" reason in that case, and adds "N untracked files the base already holds are deleted with it" to the removable line. The `kept, run /land-worktrees` verdict now counts only paths that are not held. `removeOne` calls the new `deleteHeld` after the re-judge: it checks every held file again against the same base commit and blob id, keeps the worktree whole and deletes nothing if one fails, then deletes the files one at a time with `os.Remove`. `git worktree remove` still runs with no `--force`, and a refusal after the delete says the deleted files are still in the base. `internal/wk/report.go` has a new `held` field in each `--json` worktree item; no existing field was renamed. `README.md`, section "Remove finished workspaces", describes the rule and the second check.

Tests. `internal/gitx/gitx_test.go` is new (equal file, equal executable, different bytes, a path the base lacks, a regular file over a base symbolic link, a directory, a revision before the file, a symbolic link to an equal file, a glob character in the name, and `Untracked`). `internal/wk/gc_test.go` has the seven tests this item lists, plus `TestGCDeleteHeldDeletesNothingWhenOneFileChanged`, which proves the second check in `deleteHeld` on its own, apart from the re-judge.

What is now fixed. The read-only proof on ezhomesteading with the branch binary (`wk gc`, no `--apply`) gives `24 of 43 removable`, against `0 of 43` from the installed v0.1.10. All 24 were kept by v0.1.10 only for "N changed paths", every one of their paths is an untracked file under `docs/incomplete/audit-2026-11-19/` or `docs/incomplete/audit-2026-11-06/`, and together they hold 70 files. `agent-a5f4b26e12cb826ad` stays kept for "5 changed paths" and `agent-ae37bc0555c861b4f` for "1 changed path".

Judgment calls. `hash-object` runs with `--no-filters`, so a file that equals the blob only after a clean filter or a line-ending conversion is not held; this deletes fewer files than the filtered hash would. To reverse it, drop `--no-filters` in `HeldBlob`. The `held` field is in `report.go`, which this item's "Files it owns" did not list, because the item asks for the count in the `--json` report. Kept lines do not mention held files; only the removable line and the `held` count do.

Left owed. The Fable 5.2 review of the deletion path (`deleteHeld`, `removeOne`, `HeldBlob`). A release tag and `brew upgrade`, by the owner. The gate's `TestRunPassesTheExitStatusThroughAPlace` in `cmd/wk` fails when the gate runs under a `wk run --slot` wrapper, because the child inherits `WK_SLOT_HELD`; it fails the same way on a clean `main` copy, and passes with `env -u WK_SLOT_HELD`. That is a test isolation gap outside this item.

### Fix round 1, 2026-11-21

The Fable 5.2 review of `b082b2b` found one blocking defect, B1. `heldByBase` checked only that a path was in the untracked half of `DirtyPaths`, so a path in both halves passed. After `git rm --cached <file>`, the path is a staged deletion and an untracked file with the base's bytes. gc called it removable, `--apply` deleted the file, and then `git worktree remove` refused, which left the staged deletion with the file gone. The fix: `gitx.Tracked` runs the same `git diff --name-only --no-renames -z HEAD` as the tracked half of `DirtyPaths`, which is unchanged, and `heldByBase` returns no held files when that list has any path. `TestGCKeepsAnUntrackedFileWhoseDeletionIsStaged` proves the worktree is kept and the file stays, in both the dry run and `--apply`; it fails on `b082b2b`. `TestTrackedIsTheTrackedHalfOfDirtyPaths` covers the helper.

N4 is also fixed. `deleteHeld` now reports a git error from `HeldBlob` as "the second check of <path> failed: <error>; nothing deleted" and returns the error. A file that changed, or a file that is gone (`fs.ErrNotExist`), still gives "changed after it was judged; nothing deleted".

N1, N2, N3 and N5 from the same review stay open, as the coordinator decided.

After the fix, the gate is green with `env -u WK_SLOT_HELD` and a fresh lint cache. The read-only `wk gc` on ezhomesteading gives `24 of 43 removable` again, with the same verdict lines as the first proof.

### Fix round 2, 2026-11-21

The second Fable 5.2 review of `3c614b5` closed B1 and found N6 and N7.

N6: the round 1 check missed a change that is only in the index, because `gitx.Tracked` compared the working tree with HEAD. A staged edit undone on disk (`MM README.md`) and a file added and then deleted (`AD N.txt`) both gave a removable verdict, and `--apply` would delete the held file before `git worktree remove` refused. The fix replaces `gitx.Tracked` with `gitx.OnlyUntracked`, which runs `git status --porcelain=v1 -z --untracked-files=all` and is true only when every entry is `??`, the same test `git worktree remove` applies. `heldByBase` finds no held files unless it is true. `Tracked` and its test are deleted, so the round 1 note above names a helper that no longer exists. The B1 test still passes, because a `git rm --cached` path is a `D ` entry in the status. `TestGCKeepsAHeldFileBesideAChangeOnlyInTheIndex` covers both cases: kept, nothing held, no held text in the dry run, and the held file still on disk after `--apply`. With the round 1 `git diff HEAD` check put back, both subtests fail with a removable verdict.

N7: `HeldBlob` did not compare the executable bit. It now requires an x bit on disk for base mode `100755` and no x bit for `100644`. `TestGCKeepsAnEqualFileWhoseExecutableBitDiffers` and two new `HeldBlob` cases (an executable copy of a `100644` file, a plain copy of a `100755` file) fail without the check.

The plural is fixed: a kept line now says "the 1 file deleted before it is still in main", and "files ... are" for more than one.

Open from the reviews, left as they are by the coordinator's decision:

- N1: the base ref can move after the re-judge, so the "still in main" line can then be inexact. The bytes stay in the old commit.
- N2: `deleteHeld` checks all files, then deletes all. The gap is about two git processes per held file. It is bounded, and it comes after the quiet-window and process checks.
- N3: no test covers the two kept lines after a partial delete (the `git worktree remove` refusal in `removeOne` and the `os.Remove` failure in `deleteHeld`).
- N5: each judge runs two git processes per untracked file, also in the dry run and in `--json`.

Outside row 31: `git diff --name-only HEAD` can rewrite the index of a stat-dirty worktree, so it can reset the quiet window for one pass. `DirtyPaths` still runs it. The round 1 `Tracked` added a second run, and round 2 drops `Tracked`, so that second run is gone. The new `git status` runs through `gitx.Run` with `GIT_OPTIONAL_LOCKS=0`, so it does not write the index.

After the fix, the gate is green with `env -u WK_SLOT_HELD` and a fresh lint cache. The read-only `wk gc` on ezhomesteading gives `24 of 43 removable`, with the same verdict lines as the first proof.
