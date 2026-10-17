# Row 14 fix round 2, Opus 5.5, agent aa81df3e0bf24ad6e

Sent 2026-10-17 11:30 EDT, returned 2026-10-17 11:36 EDT. Commit `619eb01` on `ap/row-14` after a merge of main (59aa77f). Not merged, not pushed.

Gate run 1 at load 4.4 to 6.0: exit 0, every package `ok` (lease 3.8 s, slot 7.1 s, landing 13.3 s, wk 46.9 s), `0 issues.` Verbose lease: 418 PASS, 0 FAIL (400 at ef61fe5). Diffstat `main...HEAD` 3 files +499/-39 (bash.go 317 changed, bash_test.go 144, item 77); `ef61fe5..HEAD` bash.go 174, bash_test.go 23, item 28 (the run's state files in the range come from the main merges). bash.go 1210 (1222 at ef61fe5, 967 on main); longest function `git()` 36. No exported name added. Patch rewritten.

## The splitter

`valueOptions{shorts, longs, stop}` lists a verb's value-taking options; `split` returns the options and operands as git reads them, and a value goes into neither list. A value-taking short letter ends its cluster and takes the rest of the cluster or the next word (`-f.git/x` and `-fl` are `-f` with a value; `-qf` is two flags). Long names match whole or abbreviated and take the next word when no `=` joins them. The option keeps its cluster up to its letter or gets its value joined by `=`; after `--` every word is an operand. Three tables from git 2.49.0's `-h` output: `configValues` (`-f`, `-t`, `--file`, `--blob`, `--type`, `--default`, `--comment`; stops at the first operand), `worktreeValues` (`-b`, `-B`, `--reason`), `checkoutValues` (`-b`, `-B`, `--orphan`, `--conflict`). `configReads`, `worktreePath`, `outputs` and `checkoutWrites` use it; the four hand loops are gone; `checkoutWrites` is 2 lines.

## Per finding

- B1: `configReads` reads the options `split` returns, so a value is never read as an option; the eight audit inputs and the `--comment=-l` control are refused as config writes.
- B2: `protocol.allow` and `protocol.*.allow` are program keys; any `-c` whose key or value holds `ext::` is refused. `programKeys` and `keyMatches` are now one anchored regexp (about 9 lines saved).
- B3: `worktreePath` takes the operand after `add` from `split`; `--reas r docs/x` and `--reaso r docs/x` place `/w/docs/x`.
- N-a: `checkout -qf -b x` and `-fq -b x` are tree writes; `-bfeat` is still a new branch named `feat`.

18 new walk rows (B1 9, B2 4, B3 2, N-a 3); `TestWriteTargetsGitProgramKeys` gains five keys. Every ef61fe5 row passes unchanged. `### Fix round 2, 2026-10-17` appended: the splitter, each finding, the corrected F1, F5 and F6 statements, the line count; the N1 note says `git var -l` passes and `git var GIT_EDITOR` is refused by the `GIT_` rule; N-b, N-c, N-d open lines.

## Not verified

`outputs` still matches only the whole `--output` option while `split` treats an abbreviation such as `--outp x` as taking a value (git refuses the abbreviation, so git writes nothing); the live hook through a built binary; git versions other than 2.49.0; `env -i EDITOR=... git commit`; no new dry runs this round (the B1 to B3 proofs are the auditor's). Git ran only in the worktree; HANDOFF.md, PASSOFF.md, README.md untouched; no call denied.
