# Row 14 builder, Opus 5.5, agent aa81df3e0bf24ad6e

Launched 2026-10-17 10:49 EDT, returned 2026-10-17 10:57 EDT. Worktree `.claude/worktrees/ap-row-14`, branch `ap/row-14`, fast-forwarded from 5bf4e7d to e13aa27 (STATE.md only). Commit `9c7cec3`. No merge, no push.

## Gates

Full gate run 1 (load 5.8 to 12.6): exit 0, every package `ok` (lease 3.9 s, slot 6.5 s, wk 72.6 s), `0 issues.` Verbose `./internal/lease/...`: 354 PASS, 0 FAIL. The gate ran after the last code edit; only the close-out changed after it.

## Diff

`main...HEAD`: 3 files, +185/-11 (bash.go +119/-11, bash_test.go +50, item file +27). bash.go 967 to 1068, mostly the verb list of step 2. No new exported name; `refusal` reads its prefixed denials from one table, `prefixed`. Patch: scratchpad `row-14.patch` (315 lines).

## Step 1 (B12)

`gitCommand` calls `fromFile` first, over `rm`, `add`, `mv`, `checkout`, `restore`, `reset`, `stash`, `commit`. Any `--pathspec-from-file` before `--` gives one target `<git paths from OPTION>` and nothing else (the `=` form, the next-word form, `=-`, and abbreviations: git 2.49.0 accepts `--pathspec-fr=` and `--pathspec-from=`; `--pathspec-f=` is ambiguous). Message: "with `--pathspec-from-file=l` git reads the paths from a file or standard input, which the lease walk does not read; give the paths on the command line".

## Step 2 (B13)

In `git()`, a `-c` key starting with `alias.` in any case gives `<git alias NAME>` and ends the walk; the value is not read (dry runs: git reads `ALIAS.Y` as `alias.y`; an alias cannot shadow a builtin). After `gitWritesTree`, a verb `gitReads` rejects gives `<git verb NAME>`, "not a git verb the lease walk reads": catches a stored alias and `--config-env=alias.y=X` (a dry run showed git runs it as an alias).

The allowlist's new edge: a git verb passes only when it is a verb the walk already judges (`rm`, `mv`, the tree writers, `reset`, `checkout`, `stash`, `worktree`); or in `gitReaders` (the brief's read-only list plus `add`, `commit`, `push`); or `branch` without `-d`, `-D`, `-m`, `-M`, `--delete`, `--move`; `tag` without `-d` or `--delete`; `remote` with no operand, `show` or `get-url`; `reflog` with no operand, `show`, `list` or `exists`; `config` with `--get` and kin, `--list`, `-l`, or the `get` or `list` subcommand.

## Step 3, walk rows (all pass)

B12 refused with the paths-from-file message: `echo web/page.ts > l && git rm --pathspec-from-file=l` (`/w/l` and the refusal), `... --pathspec-from-file=-`, `git rm --pathspec-from-file=list.txt`, `git rm --pathspec-from-file list.txt`, `git add --pathspec-from-file=l`, `git checkout --pathspec-from-file=l`, one row each for `restore`, `reset`, `stash push`, `commit`, `mv`, `--pathspec-fr=l`, `--pathspec-from l`. `git rm -- --pathspec-from-file=l` gives the path `/w/--pathspec-from-file=l`. B13 refused with the alias message: `git -c alias.x=rm x web/page.ts`, `git -c alias.x=mv x -f a b`, `git -c alias.x=checkout x -- web/page.ts`, `git -c 'alias.x=!rm' x`, `git -c ALIAS.Y=rm Y web/page.ts`. Unknown verb refused: `git x web/page.ts`, `git --config-env=alias.y=X y web/page.ts`, `git -C sub x web/page.ts`, `git branch -D old`, `git branch --del old`, `git tag -d v1`, `git remote add o u`, `git reflog expire --all`, `git config alias.x rm`. Still allowed: `git status; git log --oneline -5; git diff; git fetch`, `git add -A && git commit -m msg && git push`, `branch feature`, `branch -a`, `tag v1`, `remote -v`, `remote show origin`, `reflog`, `config --get user.name`, `config -l`, `-c core.pager=less log`. New test `TestRefusalNamesWhatItCouldNotRead`.

## Decisions for the lead

1. Two existing rows changed their expected result on purpose: rule 6 `git -c alias.x='!rm web/x.ts' x` and hunt 2 `git -c alias.x='!rm /r/b/web/x.ts' x` now expect the alias refusal in place of the walked file; both stay denied.
2. `add`, `commit` and `push` were added to the list (not in the brief): they write the index, objects and refs only, and refusing them would deny the commands every workspace runs.

## For the README (row 4 owns it)

Every verb outside the list is refused, including `git merge-file` and `git submodule update --init` (listed as open gaps in the README), `git init`, `git clone`, `git gc`, `git notes`, and the one-key read `git config user.name`. `git branch -c` still passes. Over-refusal: an option value that looks like an option is read as one (`git commit -m --pathspec-from-file`, `git tag -m '-d x'`).

## Not verified

git versions other than 2.49.0; the live hook from a workspace (walk rows and the message test are the proof); the PASS count on main. Git ran only in the worktree; HANDOFF.md, PASSOFF.md, README.md and the primary untouched; no call denied.
