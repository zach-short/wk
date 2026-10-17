# Row 3 audit 2 (fix round 1), Opus 5.5, agent a7ac1c4d0a4856a0e

Launched 2026-10-17 03:19 EDT, returned 2026-10-17 03:36 EDT. Detached worktree `.claude/worktrees/ap-row-3-audit-2` at `1c4a1fd`.

## Verdict: blocking findings

Fix round 1 closes B1 to B4 as stated, every regression row matches audit 1, gates green. The narrow hunt found two silent allows that both commits share (B5, B6) and one input that f0b1eb8 denied and 1c4a1fd allows (B7, real only if Claude Code normalizes `..` in `file_path`, unverified).

### Blocking

- B5. A `cd` through a symlink and then `..` is still cleaned by text (B4's class through a `cd` operand). `<wk-2>/sub/e -> <wk-1>/docs`; from wk-2 Bash `cd sub/e/../web && touch page.ts`: ALLOW, no log line; same for `cd -P ...` and `cd sub/e/../web && echo x > new.ts`. On disk `bash -c 'cd sub/e/../web && echo x > new.ts'` created `<wk-1>/web/new.ts`, leased by row 1 (bash and zsh land in wk-1/web since the logical path does not exist). Cause: `moved` (internal/lease/bash.go:281-284) calls `absOrUnknown`, which calls `filepath.Clean` (bash.go:240) before `Canon` reads the link. The f0b1eb8 binary allows it too. Suggested fix: for a `cd` operand with `..`, keep both the cleaned directory and the raw one; judge later targets at both.
- B6. A symlink at a leased path is judged at its target, so removing or replacing the link passes. wk-2 leases `docs/`; in wk-1 `docs/lnk -> ../web/free.ts`. From wk-1: `rm docs/lnk`, `git rm docs/lnk`, `mv docs/lnk web/z.ts`, `ln -sf ../web/page.ts docs/lnk` all ALLOW, no log line; control Edit of `docs/a.md` from wk-1 DENY. `rm` really deletes the leased entry. Cause: `kernelPath` follows the last component too (internal/wk/lease.go:296-309); the old `EvalSymlinks` did the same; f0b1eb8 allows this too. The fix-round note and README.md:203 call this "over-refusal only", which is false. Suggested fix: for `rm`, `mv`, `git rm`, `git mv`, `ln` and `unlink` targets, judge both the link's own path (parent resolved, last component not followed) and its target.
- B7, conditional. `<wk-2>/out -> <scratch>/outside/d`; Edit of `<wk-2>/out/../web/page.ts`: 1c4a1fd ALLOW, f0b1eb8 DENY. If Claude Code applies `path.resolve` or `normalize` to `file_path`, the write lands on `<wk-2>/web/page.ts`, leased by row 1. Suggested fix: for the edit tools, deny when either the text-cleaned path or the `kernelPath` path denies.

### Non-blocking

- N7. `TestHookPanicIsADeny` exercises the recover inside `decide`; nothing covers the new recover in `runLeaseHook` (cmd/wk/main.go:240-245).
- N8. A repo under a non-ASCII path with another Unicode normalization in the hook input fails closed with confusing messages. The 2.1.301 binary contains `cwd:e.cwd?.normalize("NFC")`.
- N9. `checkTargets` uses `fold("A") == "a"` as its folding flag; a bool would be clearer.
- N10. One Bash command is walked four times (`skipsBash`, `targets` twice, `judgeBash`); under 1 ms.
- N5 and N6 stay open.

## Reproduction

HEAD 1c4a1fd, merge base 26f14e3, merge-tree with main exit 0 (tree 479e5d4); main at 441521b with record commits only. go mod download exit 0. `f0b1eb8..HEAD` 9 files +284/-59, all modified, matches. Under `internal/lease` only `place` (bash.go +8/-1) and one `TestWriteTargets` row. No attribution lines. Whole diff read.

## Gates

Full gate exit 0 (every package `ok`, internal/wk 59.1 s, `0 issues.`). Verbose over wk and lease: 421 PASS, 0 FAIL, 0 SKIP (the case-folding test ran). Scratch build exit 0.

## Live table (fresh repo `lt`, scratch `WK_HOME`; wk-1 row 1 `web/page.ts` plus `lease add web/new.ts`; wk-2 row 2 `docs/`; every call exit 0; each DENY printed the JSON and one hook.log line; messages name paths in lower case on APFS)

| Case | Result |
|---|---|
| B1 Edit `<wk-2>/WEB/Page.ts` | DENY, leased by row 1 |
| B1 Bash `rm Web/page.ts` | DENY, the same, with the write path |
| B1 Edit `<wk-2>/web/Free.ts` | ALLOW |
| B1 Edit `<wk-2>/docs/é.md` | DENY, cannot compare a non-ASCII path on a volume that ignores case |
| B2 cwd /private/tmp, Edit wk-1 page | DENY, works outside every workspace |
| B2 cwd /private/tmp, Edit /private/tmp/x.md | ALLOW |
| B2 cwd /private/tmp, Bash rm wk-1 page | DENY |
| B3 Write `dang.ts` (link to wk-1 new.ts) | DENY, in the workspace wk-1 |
| B3 Bash `echo x > dang.ts` | DENY |
| B3 Edit `a` (a -> b -> a) | DENY, a symlink loop or an unreadable link |
| B3 Write `own.ts` -> wk-2 `web/fresh.ts` | ALLOW |
| B4 Bash `rm sub/e/../web/page.ts` | DENY, workspace wk-1 |
| B4 Edit `<wk-2>/sub/e/../web/page.ts` | DENY |
| B4 Edit `<wk-2>/docs/../web/free.ts` | ALLOW |
| B4 Bash `rm docs/../web/page.ts` | DENY, leased by row 1 |
| Audit 1 rows 1 to 10 and extras | same results as audit 1 |
| `<scratch>/x/wk rm 1` | ALLOW (N5) |

## Hunt

DENY where right, ALLOW where free: root in different case; a relative link with `..`; a link to a directory then a different-case leased name; trailing slash, `.`, `//`; cwd a symlink to wk-2; cwd in different case; 31 and 32 links resolve and deny, 33 deny as a loop (the kernel agrees); `cd sub/e && rm ../web/page.ts`, `cd -P sub/e && cd ..`, `set -P; cd sub/e/..` all deny (the walker keeps earlier directories). Failures: B5, B6, B7.

Timing (Perl Time::HiRes): 20 denied Edits mean 14.1 ms, max 14.9 ms wall including process start, 0 to 1 ms inside the hook; one call through 31 links 14.8 ms.

## Lean

lease.go 333, leasehook.go 323, lease_test.go 348. No changed function over 40 lines. Small duplication: `canonical` a wrapper of `kernelPath`; the `canon` closure in `request`; the hook's raw join repeats `place`'s `..` branch; `Canon` runs on each Bash target in `targets()` and `judgeBash`. Folding is the small and only complete shape (a file not yet made cannot be placed by real names); no case that folding opens.

## Documents

README.md:191 false for `cd` operands (B5) and for the last component of `rm`, `mv`, `ln`, `git rm` (B6); README.md:201 true only if Claude Code passes `file_path` unnormalized (B7); README.md:203's new "rm of a symbolic link" item wrongly calls itself an over-refusal only (B6). N3 turn-on steps right (`eachSettingsFile`, hooks.go:148-168, roots `e.Repo.Primary` and `e.TemplateDir()` only). Fix round 1 note and As built: no contractions, dashes or AI-signal words; As built cites the audit 1 path; its "B4 fixed" needs B5 added.

## Not verified

Whether Claude Code 2.1.301 normalizes `..` in `file_path` (B7) and whether `Write` follows a dangling link. A `grep -b -F` over the Claude Code binary was denied by the auto-mode classifier, not retried; two regex searches timed out. No `claude -p` run.

## Housekeeping

Audit worktree status empty, HEAD 1c4a1fd, no commit. Scratch under `row-3-audit-2/` (probe, gate output, binaries `wk` and `wk-old` from f0b1eb8, repos). Slip reported: one `git -C <scratch wk-1> status`, read-only on the throwaway repo. `~/.wk`, the builder's worktree, HANDOFF.md, PASSOFF.md untouched; no `go install`.
