# Row 14 audit 3 (fix round 2), Opus 5.5, agent ae59ee80efc3a562a

Launched 2026-10-17 11:38 EDT, returned 2026-10-17 11:44 EDT. Detached worktree `.claude/worktrees/ap-row-14-audit-3` at `619eb01`. Scoped verdict.

## Verdict: non-blocking findings only

No regression, no hole in this round's fix, no red gate. Every B1, B2, B3 and N-a input from audit 2 is refused or its write placed and refused. Across 463 inputs, 619eb01 allows six that ef61fe5 or main refused; each run in git 2.49.0 writes no worktree file. Every splitter claim hunted matches git in a scratch repo.

## Reproduction

HEAD 619eb01118b2; merge-tree with main exit 0; go mod download exit 0; git 2.49.0 (Homebrew), go 1.24.1; `main...HEAD` 3 files +499/-39 (bash.go 317, bash_test.go 144, item 77), README not in the diff; the `ef61fe5..HEAD -- internal/lease` diff read in full (adds `valueOptions` and `split`, `programKeys` as a regexp plus an `ext::` check in `gitConfig`, moves `worktreePath`, `outputs`, `configReads`, `checkoutWrites` onto `split`, shortens comments, one-line first operand in `gitWritesTree`); no attribution lines.

## Gates

Full gate at load 5.7: exit 0, every package `ok` (lease 3.67 s, slot 6.26 s, landing 13.4 s, wk 46.2 s), `0 issues.`, no retry. Verbose lease: 418 PASS, 0 FAIL (two runs). Build exit 0.

## Walk (Judge harness on 619eb01's sources; /r/w1 row 1 `web/page.ts`, /r/w2 row 2 `docs/`)

B1: all eight inputs and the `--comment=-l` control DENY as a config write. B2: the two `-c` forms, `-c protocol.allow=always fetch 'ext::...'`, `-c url.ext::sh.insteadOf=https:// fetch https://x` DENY config. B3: `--lock --reas r docs/x`, `--lock --reaso r docs/x` from wk-1 DENY leased by row 2. N-a: `checkout -qf -b x`, `-fq -b x` DENY leased by row 1.

Hunt: `config -t bool -l k v` and its cluster forms ALLOW (git: wrong number of arguments, no write); `config -t bool k.y true`, `--typ bool`, `--t bool` DENY (git wrote); `config -f c --get k v` ALLOW (a read); `-f c -l k v`, `--file=c -l k v`, `-l -f c k v` ALLOW (git exits 129); `--comment "x" --comment -l k.y v` DENY (git wrote `y = v # -l`); `config -- -l k v` DENY (git exits 129); `config k v --` DENY (git wrote; trailing `--` is a value pattern); `--blob HEAD:c k v` DENY (git refuses); `--default x --get k` ALLOW (a read); `--f c k.y v` DENY (ambiguous in git); `--fi c -l k v` ALLOW (ambiguous). `worktree add` in ten forms from wk-1 all DENY at `docs/x` (git placed every form that ran at `docs/x`; `--reason` without `--lock` refused by git); `worktree add -b feat --reason docs/x web/x` from wk-2 ALLOW at `web/x` (right). `checkout` in ten write forms DENY (git with `-qf -b` and `-b -f` reverted the dirty file; `-bf x` is branch `f`); `checkout -b x`, `-bfeat`, `--conflict merge -b x`, `--orphan x` ALLOW (git kept the dirty file); `-tb x`, `-b -f` ALLOW (git errors). `stash show --output web/page.ts` DENY; `--outp` forms ALLOW (git refuses the abbreviation); `log --output=web/page.ts -- x`, `diff --output web/page.ts` DENY; `log -- --output=...` ALLOW (a path); `log --output -- web/page.ts` ALLOW (git: option requires a value); `range-diff --outp=` ALLOW (ambiguous in git). `-c PROTOCOL.ALLOW=always`, `-c protocol.file.allow=always`, `-c 'url.insteadOf=ext::'` DENY config. `EXT::` and `Ext::` forms ALLOW (git 2.49.0: "cannot handle remote-EXT as a builtin", no command ran). `-c alias.x=rm x` DENY alias. All 20 B12 and 17 B13 rows as at ef61fe5 (the N2 `mv` row allowed).

## Regression diff

Audit 2's 384 inputs through ef61fe5 and 619eb01: every change is ALLOW to DENY or added targets on a DENY (the B1, B2, B3, N-a rows and `-c protocol.ext.allow` with an `ext::` remote); none DENY to ALLOW. With 79 hunt inputs (463), six refused by main or ef61fe5 are now allowed and none writes: `config -t bool -l k v`, `--fi c -l k v`, `--d x --get k` (ef61fe5 denied, main allowed); `checkout -bfeat`, `--conflict merge -b x`, `-tb x`. main's test tables on 619eb01 fail only the two intended alias rows; ef61fe5's tests all pass.

## Non-blocking

1. `checkout -t<x>`: git reads `-t` as `--track` with an attached optional value; `checkoutValues` does not list `t`, so `-tf -b x` is over-refused. 2. `stash show --output <file>` next-word: git reads the word as a stash ref and writes nothing, the walk places it (older over-refusal). 3. The `ext::` checks are case-sensitive, enough on git 2.49.0 Homebrew. 4. Over-refusals: `-c protocol.file.allow`, `-c url.insteadOf=ext::`, `worktree add --reason=r docs/x` without `--lock`.

## Lean

bash.go 1210; `split` 26 lines, three one-line tables; no function over 40 (`git()` 37); `outputs` already goes through `split` and matches only `--output=`, safe on 2.49.0; `programKeys` anchored and readable.

## Documents

Fix round 2 note complete: the splitter, B1 to B3, N-a, the corrected F1, F5, F6, the N1 `var` note, N-b to N-d open; no dashes or contractions; README untouched.

## Could not verify

The live hook through the built binary; git builds other than 2.49.0 Homebrew (whether another honors `EXT::`); `env -i EDITOR=...`.

Audit worktree status empty, HEAD 619eb01, nothing committed; writes only in the scratch repo with `GIT_CONFIG_GLOBAL=/dev/null`; the builder's worktree, HANDOFF.md, PASSOFF.md, `~/.wk` untouched; no `go install`; no command denied; one of the auditor's responses was stopped by a safety classifier after the gate step and not repeated (every result above is from completed tool output).
