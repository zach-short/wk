# Row 3 fix round 2, Opus 5.5, agent a325a2caad08630e4

Sent 2026-10-17 08:54 EDT, returned 2026-10-17 08:59 EDT. Commit `53be9d5` on `ap/row-3` on top of 1c4a1fd. No merge, push, `go install`, `~/.wk`, HANDOFF.md or PASSOFF.md change.

Gate exit 0 (every package `ok`, internal/wk 48.2 s, `0 issues.`). Diffstat `main...HEAD` 21 files +1840/-46; this round 8 files +157/-31 (README 6, cmd/wk/leases_test.go +27, PLAN +7, internal/lease/bash.go 20, bash_test.go 7, internal/wk/lease_test.go +36, leasehook.go 67, passoff/3 +18). Commits 98d1844, 5a93108, f0b1eb8, 1c4a1fd, 53be9d5. Patch refreshed. Line counts: bash.go 950, lease.go 333, leasehook.go 350. Test proof: the new test files on `git archive 1c4a1fd` fail all seven new B rows and pass now.

## Per finding

- B5: in internal/lease/bash.go, `moved` keeps the raw, uncleaned directory beside the cleaned one for a `cd` operand with `..`; `place` no longer cleans a relative target when its directory holds `..` (one condition beyond what the brief named, needed because `filepath.Join` would clean the raw directory again); `absOrUnknown` unchanged; three walk rows now list the raw directory too, one new row. Tests: `cd sub/e/../web && touch page.ts`, `cd -P sub/e/../web && echo x > new.ts`.
- B6: `judgeFolded` judges each call under three spellings, any deny denies: `kernelPath` (every link followed), `entryPath` (parent followed, last component not), `cleanedPath` (`..` removed by text, then links followed). Tests: `rm`, `git rm`, `mv`, `ln -sf` of `docs/lnk` with `docs/` leased elsewhere and a real link in a temp dir.
- B7: `cleanedPath` covers it; Edit of `<b>/out/../web/page.ts` denied, an edit through `out` to outside every worktree still allowed. All three in `TestHookJudgesEverySpellingOfAPath` through `LeaseHook` with hook JSON.
- N7: `TestLeaseHookPanicOutsideTheJudgmentDenies` (cmd/wk): the first write of the deny panics, `runLeaseHook` prints the deny and exits 0 (passes on 1c4a1fd too). N9: `checkTargets` takes `insensitive bool`.
- README 191 (unfollowed last component and text-cleaned path judged, `cd` with `..` followed both ways), 201 (each edit judged at the cleaned and the kernel path), 203 (the false over-refusal item replaced). As built has the audit 2 outcomes, cites the report, keeps B4. `### Fix round 2, 2026-10-17` appended: corrects round 1's over-refusal claim; build-level calls with reversals: every path gets all three spellings for every verb, a `cd` with `..` keeps two directories, a Bash command is now walked ten times per call (N6, N10 grow). N5, N6, N8, N10 open.

## Timing

20 runs each with a scratch build: denied Edit max 0.01 s; denied `sed -i` Bash 0.00 s at 10 ms resolution.

## Could not verify

What Claude Code 2.1.301 does with `..` in `file_path` (B7 closed for both behaviours); whether `Write` follows a dangling link. No `claude -p`. The builder believes `~/go/bin/wk` holds 5a93108 code; the ezhomesteading lead reports it reinstalled from f0b1eb8 at item 214. The land step reinstalls either way.
