# Row 3 audit 3 (fix round 2), Opus 5.5, agent a73f7256189e02850

Launched 2026-10-17 09:01 EDT, returned 2026-10-17 09:27 EDT. Detached worktree `.claude/worktrees/ap-row-3-audit-3` at `53be9d5`.

## Verdict: blocking finding (B8)

Fix round 2 closes every input audit 2 named for B5, B6 and B7; every regression row matches audit 2; gates green. The B6 fix reopens B4's class on the new spelling.

- B8. A link entry reached through a symlink and then `..` is judged with the `..` cleaned by text. In wk-1: `p -> docs/inner` (a real directory) and `docs/lnk -> ../web/free.ts`; wk-2 leases `docs/`. From wk-1 each is ALLOWED with no log line: `rm p/../lnk`, `unlink p/../lnk`, `git rm p/../lnk`, `mv p/../lnk web/z.ts`, `git mv p/../lnk web/z.ts`, `ln -sf ../web/page.ts p/../lnk`, `cd -P p/.. && rm lnk`; from cwd wk-1/web with `q -> ../docs/inner`: `cd q && rm ../lnk`, `cd -P q/.. && rm lnk`; also `rmdir p/../sub` and `rm -f p/../sub` where `docs/sub -> ../web`. Proven on disk: `rm p/../lnk` deleted `<wk-1>/docs/lnk`; `ln -sf` repointed it; `cd p && rm ../lnk` deleted it. Control: `rm docs/inner/../lnk` DENIED. Cause: `entryPath` (internal/wk/leasehook.go:178-181) calls `kernelPath(filepath.Dir(p))`, and `filepath.Dir` cleans; all three spellings then miss the entry. Suggested fix: split the raw string at its last `/`, run `kernelPath` on the raw parent, join the last component; fall back to `kernelPath` when the last component is `.`, `..` or empty; add rows `rm p/../lnk` and `cd -P p/.. && rm lnk`. README.md:191 and the As built B6 line are false for this input.

### Non-blocking

- N11. For Bash, `cleanedPath` over-refuses with a wrong message: `echo x > out/../web/page.ts` from wk-2 with `out` pointing outside really writes `<S>/outside/web/page.ts`; the hook denies it as a write to `<wk-2>/web/page.ts`. Bash never cleans by text. Not in README.md:203's over-refusal list.
- N12. `request` reads the cwd with `cwd, _ := kernelPath(...)` and drops `ok`; a loop cwd keeps its raw text and places by text (an Edit of `web/free.ts` from a loop cwd is allowed). A real process cwd cannot be a loop. Not named in the fix-2 report.
- N13. The walker keeps the directory from before a `cd`; a `cd` with `..` makes a later relative target count at more places, some in the main checkout: `cd sub/../web && cd ../docs/.. && touch web/free.ts` is denied as a write to `<lt>/.claude/worktrees/web/free.ts`. Documented over-refusal; past the 16-directory bound the walk fails closed.
- N5, N6/N10 (ten walks per Bash call: one to skip, then three per spelling), N8 open.

## Reproduction

HEAD 53be9d563e8e; merge-tree with main exit 0; go mod download exit 0; `1c4a1fd..HEAD` 8 files +157/-31, every per-file figure and the name list match; whole row 21 files +1840/-46. Under `internal/lease`: `place` (new condition), new helper `dotDot`, `moved`, one new and three updated walk rows. No attribution lines on any of the five commits.

## Gates

Full gate first run exit 3 ("parallel golangci-lint is running", another lint held the lock, no test failed); second run exit 0, every package `ok` (internal/wk 73.0 s), `0 issues.` Verbose over wk, lease, cmd/wk: 468 PASS, 0 FAIL, 0 SKIP. Scratch build exit 0, byte-identical to the live-check binary.

## Live table

Setup: audit 2's repo copied with `cp -R`, gitdir paths rewritten with `sed`, fresh `WK_HOME`; wk-1 row 1 `web/page.ts` plus `lease add web/new.ts`; wk-2 row 2 `docs/`. Every call exit 0; each DENY printed the JSON and one hook.log line; no ALLOW logged.

B5 (from wk-2, `sub/e -> <wk-1>/docs`): `cd sub/e/../web && touch page.ts` DENY (wk-1); `cd -P sub/e/../web && echo x > new.ts` DENY; `cd sub/e/../web && echo x > fresh.ts` DENY (wk-1); `cd docs/../web && touch free.ts` ALLOW; `cd docs/../web && touch page.ts` DENY (row 1); `cd sub/e && cd .. && touch web/page.ts` DENY.

B6 (wk-1: `docs/lnk -> ../web/free.ts`, `docs/sub -> ../web`): `rm`, `git rm`, `mv`, `ln -sf`, `unlink` of `docs/lnk` each DENY (row 2); `rmdir docs/sub` DENY; control `rm web/free.ts` ALLOW; `mv web/free.ts docs/moved.ts` DENY; Edit `docs/a.md` and Edit `docs/lnk` from wk-1 DENY.

B7 (`<wk-2>/out -> <S>/outside/d`): Edit `out/../web/page.ts` DENY (row 1); Edit `out/file.md` ALLOW; Bash `echo x > out/../web/page.ts` DENY (N11).

Regression, audit 1 rows 1 to 10 and extras: identical to audit 2. Regression, audit 2 B1 to B4: identical. `ln -s <wk-1>/web/new.ts dang2.ts` ALLOW (writes nothing leased).

## Hunt

`<wk-2>/x -> <wk-1>/web`, Edit `x/page.ts` DENY; `<S>/outside/ln -> <wk-2>/web/deep`, Edit and Bash `<S>/outside/ln/../page.ts` DENY (cleaned path outside, kernel path leased), `free.ts` form ALLOW; `cd nope/../web && touch page.ts` DENY, `free.ts` ALLOW; `cd ../wk-1/web && touch page.ts` DENY; `cd $(pwd)/web`, `cd "$HOME"` DENY for `$`; a `cd` with `..` in a heredoc body DENY; heredoc then real `cd sub/e/../web` DENY; `pushd sub/e/../web; touch page.ts` DENY; two `cd`s with `..` DENY (N13); `rm -r docs` from wk-1 DENY; `cp web/free.ts docs/lnk` DENY; `rm docs/into2/page.ts` DENY; link plus `..` reaching a leased link entry: B8 ALLOW.

## Timing

20 denied Edits mean 14.1 ms, max 15.3 ms; 20 denied Bash `rm` mean 14.4 ms, max 15.4 ms (wall, process start included); inside the hook 0 to 1 ms; highest over 189 logged calls 47 ms. Ten walks per call stays far under the 5 s timeout.

## Lean

bash.go 950, lease.go 333, leasehook.go 350, lease_test.go 384. No changed non-test function over 40 lines (`judgeFolded` about 25). No duplicated logic: `canonical` is `kernelPath` without `ok`; `cleanedPath` is `kernelPath(Clean(p))`; `entryPath` is `kernelPath(Dir(p))` plus `Base`, whose hidden `Clean` is B8. Each spelling alone: `kernelPath` catches B3, B4, links into another worktree, the raw directory of B5; `entryPath` catches B6; `cleanedPath` catches B7 for the edit tools. None can be dropped. Smaller shape: `entryPath` on the raw parent closes B8; `cleanedPath` limited to the edit tools removes N11.

## Documents

README.md:191 true for `cd` both ways, false for B8; 201 true; 203 true as an over-refusal statement, does not name N11. Fix round 2 note and As built: no contractions, dashes or AI-signal words; As built records B5 to B7 with the audit 2 path and keeps B1 to B4; its B6 "fixed" claim is false for B8. Each build-level call has a reversal. N3 turn-on steps unchanged and right.

## Could not verify

Claude Code 2.1.301's handling of `..` in `file_path` and of a dangling link on `Write`; whether git accepts `git rm p/../lnk` and `git mv p/../lnk` (no git run on the scratch repo).

## Housekeeping

Audit worktree status empty, HEAD 53be9d5, nothing committed; scratch under `row-3-audit-3/`; no git command run by the auditor (the `wk claim` runs invoked git inside its scratch copies); `~/.wk`, the builder's worktree, HANDOFF.md, PASSOFF.md untouched; no `go install`.
