# Row 3 audit 4 (pass 2), Opus 5.5, agent ae40d88d8d0b1bbe3

Launched 2026-10-17 09:15 EDT, returned 2026-10-17 09:24 EDT. Detached worktree `.claude/worktrees/ap-row-3-audit-4` at `a028e54`.

## Verdict: blocking findings

Pass 2 does what it says: B8 closed for every spelling audit 3 named, N11 and N12 done, gates green, every regression row matches audit 3. But the N11 change opens a new silent allow, a regression (53be9d5 denied, a028e54 allows). Cause: git cleans its pathspec by text; pass 2 assumed nothing a Bash command runs removes a `..` by text, which is false for git.

### Blocking

- B9. Regression from N11: `git rm` and `git mv` operands with a link and then `..`. `spellings("Bash")` dropped `cleanedPath` (leasehook.go:176-181); the walk sends `git rm` and `git mv` operands raw (bash.go:641-642); git removes each `..` by text against its physical cwd. Proof on git 2.49.0 (dry runs in the auditor's worktree with `zzout -> <scratch>/outside/d`): `git rm -n zzout/../README.md` printed `rm 'README.md'`; `git mv -n zzout/../README.md zzmoved.md` printed `Renaming README.md to zzmoved.md`. Live, each ALLOW on a028e54 and DENY on 53be9d5: from wk-2 with `out -> <S>/outside/d`: `git rm out/../web/page.ts`, `git rm -f ...`, `git rm -r out/../web`, `git mv out/../web/page.ts web/moved.ts`, `git mv -f notes.txt out/../web/page.ts`, `git rm --cached out/../web/page.ts`; from cwd `<wk-1>/docs/inner` with `o -> <S>/outside/d`: `git rm o/../../a.md` (removes `<wk-1>/docs/a.md`, leased by row 2). The pass 2 note's reason (passoff/3:180) and README.md:191 are false in the same way. Fix options: give Bash all three spellings again (closes B9 only); least code: in `gitCommand`, treat a `git rm` or `git mv` operand that holds `..` as unplaceable (closes B10 too).
- B10. Older, same root: git applies `..` after its physical cwd, so when the walk's directory is a logical path through a link no spelling gives git's answer. ALLOW on both binaries: `cd q && git rm o/../../a.md` from `<wk-1>/web`; `git -C p rm o/../../a.md` from wk-1 (removes `<wk-1>/docs/a.md`). Proof: `cd zzq && git rm -n zzo/../../lease/judge.go` with `zzq -> internal/wk` printed `rm 'internal/lease/judge.go'`; `git rm -n ../README.md` there failed, so git did not use bash's logical PWD. The `..` refusal closes it.
- B11. Older, outside `entryPath` and `spellings`: git pathspec magic. ALLOW on both: `git rm :/docs/a.md` from `<wk-1>/web`, `git rm :docs/a.md` from wk-1. Proof: `git rm -n :/README.md` from `internal/` and `git rm -n :README.md` from the top both printed `rm 'README.md'`. `git mv` does not read magic (allow is right there); `:(top)` is refused on `(`. Fix: treat a `git rm` operand that starts with `:` as unplaceable.

### Non-blocking

- N14. The brief's premise for `cd out/.. && rm web/page.ts` is reversed: a logical `cd` collapses `out/..` by text (`bash -c 'cd out/.. && pwd'` printed `<w>`; zsh the same), so the command removes `<wk-2>/web/page.ts` and DENY is right.
- N15. Stale notes in the passoff file: line 163 ("all three spellings for every verb and every tool") no longer true for Bash; line 165 "ten times" is now seven; line 144 "Over-refusal only" was disproved by B6.
- Over-refusal: `git rm p/../web/page.ts` from wk-1 denied as `docs/web/page.ts`; git's answer is wk-1's own `web/page.ts`.
- N5, N6/N10 (seven walks), N8, N13 open.

## Reproduction

HEAD a028e541f216; merge-tree with main exit 0; go mod download exit 0; `53be9d5..HEAD` 5 files +59/-14, names match; diff read in full, holds only `entryPath`, `spellings(tool)` and its call site, the `mark` closure, tests and documents, nothing under `internal/lease`; no attribution lines; `main...HEAD` 21 files +1885/-46. Test proof: the new lease_test.go over `git archive 53be9d5` fails the four new rows and passes at HEAD.

## Gates

Full gate exit 0 first run (every package `ok`, internal/wk 47.4 s, `0 issues.`). Verbose over wk, lease, cmd/wk: 468 PASS, 0 FAIL, 0 SKIP. Scratch build exit 0 (sha1 76a72f59).

## Live table

Setup: audit 3's setup copied with `cp -R`, `sed` on gitdir files, `.git` links, leases.json and hk.sh, absolute links recreated, `wk-1/web/q -> ../docs/inner` added; fresh binary, scratch `WK_HOME`; wk-1 row 1 `web/page.ts` and `web/new.ts`; wk-2 row 2 `docs/`. Every call exit 0; each DENY printed the JSON and one hook.log line; no ALLOW logged.

B8 from wk-1: `rm p/../lnk`, `unlink p/../lnk`, `git rm p/../lnk`, `mv p/../lnk web/z.ts`, `git mv p/../lnk web/z.ts`, `ln -sf ../web/page.ts p/../lnk`, `cd -P p/.. && rm lnk`, `rmdir p/../sub`, `rm -f p/../sub`: each DENY (docs/lnk or docs/sub leased by row 2). From `<wk-1>/web`: `cd q && rm ../lnk`, `cd -P q/.. && rm lnk` DENY. Control `rm docs/inner/../lnk` DENY. `rm p/../../web/free.ts` ALLOW (wk-1's own). Edit `<wk-1>/p/../lnk` DENY.

N11: Bash `echo x > out/../web/page.ts` ALLOW; Edit `<wk-2>/out/../web/page.ts` DENY; Bash `rm out/../web/page.ts` ALLOW; Bash `cd out/.. && rm web/page.ts` DENY (right, N14).

N12, cwd `<wk-2>/a` (loop): Edit `web/free.ts` DENY (cannot tell); Bash `touch web/free.ts` DENY; absolute Edit DENY (outside every workspace).

Regression: audit 1 rows 1 to 10, 1b, the 3 variants and extras; audit 2 B1 to B4 (14 rows); audit 3 B5 (6), B6 (10), B7 (3); `ln -s ... dang2.ts` ALLOW: every row as audit 3 reported, the B7 Bash row now ALLOW by N11. The reaped-claim extra not re-run (destructive to the shared setup).

## Hunt

`rm p/..`, `rm -r p/..` DENY; `rm p/../lnk/`, `rm -f p/../lnk/` ALLOW (right: ENOTDIR, removes nothing); `rm -rf p/../sub/` ALLOW (right: removes wk-1's own `web`, the link stays); `rm p/../lnk/.`, `rm p/../lnk/..` ALLOW (rm refuses); `rm p/..//lnk`, `rm p//..//lnk`, `rm p/..///lnk` DENY; `rm p/../LNK`, `rm P/../lnk`, `rm P/../Sub` DENY; two links and two `..` (`p2`, `r -> p`, `q2`) DENY in every form; `rm ./p/../lnk`, `rm -- p/../lnk`, `rm "p/../lnk"` DENY; from wk-2 `rm -rf sub/e/`, `rm -rf sub/e/../e/` DENY; Edit `<wk-1>/p/..`, `<wk-1>/p/../..`, `<wk-2>/out/..` DENY; Edit `<wk-1>/p//../lnk`, relative `p/../lnk`, `<wk-1>/P/../LNK` DENY; Edit `<wk-1>/p/../lnk/` ALLOW (ENOTDIR; a tool that cleans writes `<wk-1>/lnk`, which wk-1 owns).

Which programs clean by text: `cp`, `cp -R`, `install`, `rsync` (openrsync), `rsync -a`, `tee`, `touch`, `ln -s`, `dd`, `truncate`, `mv`, `tar -C`, `python3 open`, `perl open`, `sed -i` all write where the kernel resolves. Only the logical `cd` and `pushd` clean by text (covered by the walk), and git (B9, B10); git pathspec magic is B11.

## Timing

20 denied Edits mean 15.0 ms, max 17.7 ms; 20 denied Bash `rm p/../lnk` mean 16.5 ms, max 25.2 ms (wall with process start); inside the hook 0 to 2 ms; highest of 290 logged calls 14 ms.

## Lean

leasehook.go 362, lease.go 333, bash.go 950, lease_test.go 396. `entryPath` is 6 lines with no `filepath.Dir`; no function in leasehook.go over 25 lines; `mark` removes duplication (net about +1 line; its `spell` parameter shadows the outer `spell`, harmless).

## Documents

README.md:191 true for B8; its claim "a shell never does that, so `Bash` does not get it" leads to B9. README.md:203 true. Pass 2 note and As built: no contractions, dashes or AI-signal words; As built records B8 fixed, N11 and N12 done, N13 open, with the audit 3 path, B6 line corrected; the build-level call has its reversal, its reason is false (B9); turn-on steps (passoff 124-126, 147) unchanged and right.

## Could not verify

Claude Code 2.1.301's handling of `..` and a trailing slash in `file_path`; git on the scratch repo itself (proven with `-n` dry runs in the auditor's own worktree only); Homebrew rsync 3 (not installed).

## Housekeeping

Audit worktree status empty, HEAD a028e54, nothing committed. Temporary links `zzout`, `zzq`, `internal/wk/zzo` made in the audit worktree for the git dry runs and removed (one existed under a second while the background gate ran; the gate passed). Scratch under `row-3-audit-4/`. `~/.wk`, the builder's worktree, HANDOFF.md, PASSOFF.md untouched; no `go install`; no command denied.
