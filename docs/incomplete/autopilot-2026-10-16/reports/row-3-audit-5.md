# Row 3 audit 5 (pass 3), Opus 5.5, agent afef196fe51493a9e

Launched 2026-10-17 09:49 EDT, returned 2026-10-17 10:08 EDT. Detached worktree `.claude/worktrees/ap-row-3-audit-5` at `146c6f7`.

## Verdict as returned: blocking findings (B12, B13, older, in the git walk)

Pass 3's own change is clean: B9, B10 and B11 closed for every spelling audit 4 named and every hunt variant; every regression row the same as audit 4; the second full gate exited 0. The hunt found two older silent allows in git's argument walk (row 7's `internal/lease/bash.go`), present on a028e54 too and not in pass 3's diff. The lead's disposition is in STATE.md (judgment call of 10:10 EDT): row 3 lands with both named in the README's gap list; the walk fix is a new board row.

- B12. `git rm --pathspec-from-file`. `operands()` (bash.go:713-724) drops every word that starts with `-`, so the paths git reads from the file are never seen. Proof (git 2.49.0, `-n`): `git rm -n --pathspec-from-file=<list>` with `README.md` in the list printed `rm 'README.md'`; the `=-` form from stdin the same. Live from wk-2, each ALLOW with no log line on both binaries: `echo web/page.ts > l && git rm --pathspec-from-file=l`, `echo web/page.ts | git rm --pathspec-from-file=-`, `git rm --pathspec-from-file=list.txt`, `git rm --pathspec-from-file list.txt`; each removes `<wk-2>/web/page.ts`, leased by row 1. Fix, about 2 lines: in `gitCommand`, any `--pathspec-from-file` argument of `git rm` is unplaceable.
- B13. `git -c alias.<x>=<subcommand>`. `git()` (bash.go:614-640) walks a `-c` value as a shell command after trimming `!`, then passes the alias name to `gitCommand` as the subcommand, which is neither `rm`, `mv` nor a tree writer. Proof: `git -c alias.x=rm x -n README.md` printed `rm 'README.md'`; `git -c alias.x=mv x -n README.md zz.md` printed `Renaming README.md to zz.md`. Live ALLOW on both binaries: from wk-2 `git -c alias.x=rm x web/page.ts`, `git -c alias.x=mv x -f notes.txt web/page.ts`, `git -c alias.x=checkout x -- web/page.ts` (past the tree-writer refusal); from wk-1 `git -c alias.x=rm x :/docs/a.md`. Fix, about 3 lines: in `git()`, for a `-c alias.*=` value without `!`, refuse the command or take the value's first word as the subcommand. A stored alias (`git config alias.x rm`, then `git x`) is the same class, not tested live.

### Non-blocking

- N16. passoff/3 line 180 deleted the clause "because a shell never removes a `..` by text," instead of appending the dated correction; lines 144, 163 and 165 append only, which is right.
- N17. README.md:191 overclaims ("every other program writes where the kernel resolves"); the known-spellings paragraph (201) names neither B12 nor B13.
- Slot timing test (row 200's code): in the first full gate `TestHookPreToolUseTiming` failed with empty stdout, a different symptom from the "mean over 50 ms" of the alone runs; probably a timeout under load; this branch does not touch `internal/slot`.
- N5, N6/N10 (seven walks), N8, N13 open as audit 4 left them.

## Reproduction

HEAD 146c6f7a67dd; merge base 26f14e3, main at e7d743f (ledger commits only); merge-tree with main exit 0; go mod download exit 0; `a028e54..HEAD` 6 files +51/-7, names match; diff read in full: the `pathspecPrefix` constant, its case in `refusal`, the `pathspecs` call in `gitCommand`, the `pathspecs` function, 9 walk rows, a `pathspec()` test helper, 1 hook row, plus README, PLAN and the passoff file; `internal/slot` diff against main empty; no attribution lines; whole row 21 files +1930/-47.

## Gates (load given)

| Run | Load | Result |
|---|---|---|
| Full gate, first run | 37 at start, 60 to 80 during | exit 1: gofmt and vet pass, 15 packages `ok` (internal/wk 318 s), `internal/slot` FAIL on `TestHookPreToolUseTiming` (empty stdout), lint not reached |
| `./internal/slot/...` alone, run 1 | 41 | exit 1: mean 79.9 ms over 50 ms |
| alone, run 2 | 37 | exit 1: mean 57.9 ms |
| alone, run 3 | 39 | exit 0 |
| The row's packages (lease, wk, cmd/wk, config, settings, board) | 38.6 | exit 0 (internal/wk 126.6 s) |
| `golangci-lint run ./...` | about 21 | `0 issues.`, exit 0 |
| gofmt and vet alone | about 21 | exit 0 |
| Full gate, second run | 19.6 at start, 12.6 at end | exit 0: 16 packages `ok` (slot 7.5 s, wk 81 s), `0 issues.` |

Verbose over wk, lease, cmd/wk (load 11.8): 477 PASS, 0 FAIL, 0 SKIP (audit 4 had 468; the 9 new walk rows). Scratch build exit 0 (sha1 173d4860).

## Live table

Setup: audit 4's setup copied with `cp -R`, `sed` on both `.git` files, both gitdir files, leases.json, hk.sh and reg.sh, 8 absolute links recreated, hook.log emptied; fresh binary, scratch `WK_HOME`; wk-1 row 1 `web/page.ts` and `web/new.ts`; wk-2 row 2 `docs/`. Every call exit 0; every DENY printed the JSON and one hook.log line; no ALLOW logged. "Pathspec" below is the new refusal naming the operand.

B9: `git rm out/../web/page.ts`, `-f`, `-r out/../web`, `git mv out/../web/page.ts web/moved.ts`, `git mv -f notes.txt out/../web/page.ts`, `git rm --cached out/../web/page.ts` (wk-2), `git rm o/../../a.md` (`<wk-1>/docs/inner`): all DENY pathspec. B10: `cd q && git rm o/../../a.md` (`<wk-1>/web`), `git -C p rm o/../../a.md` (wk-1): DENY pathspec. B11: `git rm :/docs/a.md`, `git rm :docs/a.md`: DENY pathspec; `git mv x :/zz.go` ALLOW (right: `git mv -n :README.md zz.md` gives `fatal: bad source`). Controls: `git rm web/free.ts` ALLOW; `git rm docs/a.md` DENY (row 2); `git mv web/free.ts web/free2.ts` ALLOW; `git rm p/../lnk` DENY pathspec; `git add out/../web/page.ts` ALLOW (index only); `git checkout -- out/../web/page.ts` and `git restore out/../web/page.ts` DENY (tree writer); `git rm -- ...`, `git rm ./out/...`, `git rm '...'` DENY pathspec.

Audit 4 B8 rows all as reported (`git rm p/../lnk` and `git mv p/../lnk web/z.ts` still DENY, now with the pathspec reason). N11 rows and N12 rows as reported. `reg.sh` over 62 rows from audits 1 to 3: every decision identical (diff empty). Audit 4's hunt rows all the same; `git rm p/../web/page.ts` now DENY with the pathspec message, which closes audit 4's wrong-path over-refusal message.

## Hunt

`git rm -r out/..`, `git mv notes.txt out/../web/moved.ts`, `git rm -- :docs/a.md`, `git rm ':!docs/a.md'`, `git rm :`, `git rm docs/../docs/a.md`: DENY pathspec. `git rm -r .` from `<wk-1>/docs/inner`: DENY leased. `git clean -f out/..`, `git checkout out/../web/page.ts`, `git checkout -- docs/a.md`: DENY tree writer. `git -C out/.. rm web/page.ts`: DENY leased. `git rm "\:docs/a.md"`: DENY on `\`. `git rm -r docs` (wk-1), `git rm -r web` (wk-2), `git stash push -- web/page.ts`: DENY. `git mv :web/page.ts x.ts` ALLOW (right); `git rm .../web/page.ts` ALLOW (right, an ordinary name). B12 and B13: the only ALLOW of a leased write found.

## Timing (load 9.6 at 10:04)

20 denied Edits mean 17.4 ms, max 40.8 ms; 20 denied Bash `git rm out/../web/page.ts` mean 18.1 ms, max 26.8 ms (wall with process start); inside the hook 0 to 3 ms.

## Lean

bash.go 967; `pathspecs` 9 lines; `gitCommand` 10; `refusal` 15; the `pathspecPrefix` message one case (bash.go:103); no function in the diff over 40 lines.

## Documents

README.md:191 true for B9 to B11 with the N17 overclaim; 203 true. Dated corrections present at 144, 163, 165, 180 (180 deleted its original clause, N16). Pass 3 and As built additions: no contractions, dashes or AI-signal words. As built (PLAN 146-149) records B9 to B11 fixed, N14 no change, N15 marked, with the audit 4 path. Turn-on steps unchanged and right. "Nine walk rows and one hook row" true.

## Could not verify

git on the scratch repo itself (dry runs in the audit worktree only); git versions other than 2.49.0; a stored `git config alias.x rm` then `git x`; Claude Code's handling of `file_path`; audit 4's two-link rows (exact commands not listed); the reaped-claim extra.

## Housekeeping

Audit worktree status empty, HEAD 146c6f7, nothing committed; git run only in the audit worktree (diffs, merge-tree, `-n` dry runs); scratch under `row-3-audit-5/`; `~/.wk`, the builder's worktree, HANDOFF.md, PASSOFF.md untouched; no `go install`; no command denied.
