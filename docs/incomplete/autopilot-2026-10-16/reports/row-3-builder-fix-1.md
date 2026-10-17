# Row 3 fix round 1, Opus 5.5, agent a325a2caad08630e4

Sent 2026-10-17 03:07 EDT, returned 2026-10-17 03:17 EDT. Commit `1c4a1fd` on `ap/row-3` on top of f0b1eb8. No merge, push, `go install`, `~/.wk`, HANDOFF.md or PASSOFF.md change.

Gate exit 0 (every package `ok`, internal/wk 81.5 s, `0 issues.`). Diffstat `main...HEAD` 21 files +1710/-42; this round 9 files +284/-59; no new files; patch refreshed. Commits 98d1844, 5a93108, f0b1eb8, 1c4a1fd.

## Per finding

Proof: the new test file copied onto `git archive f0b1eb8` fails each B row there and passes now; B1 did not skip on this APFS volume.

- B1: `judgeFolded` in leasehook.go folds the request path, cwd, roots, store keys and leases, and the single-writer list to lower case when the volume ignores case (`caseInsensitive`: `.git` and `.GIT` are one entry, or the check cannot tell). `checkTargets` refuses a non-ASCII path below a worktree on such a volume. Test `TestHookFoldsCaseOnACaseInsensitiveVolume` (skips when `.GIT` does not resolve).
- B2: `callerRoot` renamed `checkTargets`; a cwd in no worktree denies every target inside a worktree; a path outside every worktree still allowed. Recorded as a build-level call. Test `TestHookDeniesAWriteFromOutsideEveryWorkspace`.
- B3: new `kernelPath` in lease.go walks one component at a time with `Lstat`, follows each link with `Readlink` (dangling included), stops after 32 links; a loop or unreadable link is unplaceable, so an edit denies. `canonical` is a thin wrapper for git and command-line paths. Test `TestHookFollowsLinksAsTheKernelDoes` (dangling Write, `echo x > dang.ts`, the loop).
- B4: `place` in internal/lease/bash.go no longer cleans a target with a `..` component (the only walk change; 318-row table still passes, one row added); the hook joins a relative `file_path` to the cwd without cleaning; `kernelPath` applies each `..` after the link before it. Covered in the same test (Edit and `rm sub/e/../web/page.ts`).
- N1: `LeaseHook` prints the deny before it logs; a panic in logging is swallowed; `runLeaseHook` prints a deny through the new exported `wk.LeaseDeny` for any panic outside `LeaseHook`. N2: `TestHookPanicIsADeny` (nil reader); the main.go recover path has no test.
- N3 and N4: README and hand-back updated; the heredoc-apostrophe sentence replaced by the new over-refusal (`rm` of a link judged at its target). Step 10: each finding's outcome in "As built, P2", citing the audit report path. N5 and N6 open. `### Fix round 1, 2026-10-17` appended under the close-out.

## Build-level calls this round

B2 rule; lower-case folding on macOS (deny messages and hook.log name paths in lower case; reversal: case correction against known roots, about 30 lines); `rm` of a symlink judged at its target; `ln -s <leased> dang2.ts` stays allowed (writes only a new link in the session's own worktree; a later write through it is denied).

## Correction to the brief

`HooksInstall` writes only the main checkout's and the template's `.claude/settings.local.json`, even with `--repo <workspace>`, so the N3 instruction "run `wk hooks install` in each existing workspace" does not work. The README and hand-back say: run `wk hooks install`, then copy the main checkout's settings file into each existing workspace, or remake each workspace.

## Timing

20 runs of a scratch build on the proof repository's denied Edit: all 0.01 s. The first launch of the fresh binary took 0.42 s with zero user and sys time.

## Could not verify

Whether Claude Code 2.1.301's `Write` follows a dangling link; whether it keeps `..` in `file_path`; whether a hook's `cwd` can be outside the project. No `claude -p` this round. `~/go/bin/wk` still holds 5a93108 until the land step reinstalls it.
