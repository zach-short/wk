# 14. Read git's `--pathspec-from-file` and alias spellings in the Bash walk

> **Claimed `IN FLIGHT` 2026-10-17.**

*Opened 2026-10-17 by the autopilot run, from findings B12 and B13 of row 3's fifth audit (`docs/incomplete/autopilot-2026-10-16/reports/row-3-audit-5.md`). Both are spellings row 7's landed git walk does not read; row 3's README names them as open gaps.*

**Model: Opus 5.5. Lane D. Waits on nothing (row 3 landed). Files it owns: `internal/lease/bash.go`, `internal/lease/bash_test.go`.**

You fix two gaps in the allowlist Bash walk (`internal/lease/bash.go`, `WriteTargets`) through which git writes a leased worktree file and the lease hook allows the call. Read first: `internal/lease/bash.go` (`git`, `gitCommand`, `operands`, `pathspecs`, `refusal`), `internal/lease/bash_test.go` (`TestWriteTargets`, the walk table), and `passoff/7-build-the-lease-store-and-judge.md` close-out and sign-off (the walk's rules: refuse loudly what cannot be placed; every refusal names what it could not read).

The facts, proven on git 2.49.0 with `-n` dry runs by row 3's auditor:

1. `git rm --pathspec-from-file=<file>` (and `--pathspec-from-file <file>`, and `=-` for stdin) removes the paths listed in the file. `operands()` (about bash.go:713-724) drops every word that starts with `-`, so the call reaches the judgment with no target. From a workspace, `echo web/page.ts > l && git rm --pathspec-from-file=l` removes `web/page.ts`, leased elsewhere, and the hook allows it.
2. `git -c alias.x=rm x web/page.ts` runs `git rm`. `git()` (about bash.go:614-640) walks a `-c` value as a shell command after trimming `!`, then passes the alias name `x` to `gitCommand` as the subcommand, which is neither `rm`, `mv` nor a tree writer, so nothing is judged. `git -c alias.x=checkout x -- web/page.ts` gets past the tree-writer refusal the same way. A stored alias (`git config alias.x rm`, then `git x web/page.ts`) is the same class and cannot be read from the command at all.

Do these, in order:
1. In `gitCommand`: any `--pathspec-from-file` argument (both spellings, any value) of `git rm`, `git add`, `git mv`, `git checkout`, `git restore`, `git reset`, `git stash push` and `git commit` is unplaceable, refused with a message that names the option and says to give the paths on the command line. Smallest shape: a check beside `pathspecs`.
2. In `git()`: a `-c` value whose key starts with `alias.` is refused with a message that names the alias and says to run the git verb directly. Do not try to read the alias's value as the subcommand: an alias can carry options and a second alias. A stored alias cannot be seen; `gitCommand` already refuses an unknown subcommand only when it is a tree writer, so add: a subcommand that is not in the walk's known list of read-only git verbs and not a known writer is refused as "not a git verb the lease walk reads". If that list does not exist, build it from the verbs the walk already names plus git's read-only porcelain (`status`, `log`, `diff`, `show`, `blame`, `grep`, `ls-files`, `ls-tree`, `rev-parse`, `rev-list`, `cat-file`, `describe`, `shortlog`, `branch` without `-d`/`-D`/`-m`/`-M`, `tag` without `-d`, `remote` without a writing subcommand, `fetch`, `config --get`, `worktree list`, `merge-base`, `name-rev`, `for-each-ref`, `reflog show`, `help`, `version`), and say in the close-out that the list is the allowlist's new edge.
3. Walk rows in `bash_test.go` for each spelling above: the four `--pathspec-from-file` forms on `git rm`, one on `git add`, one on `git checkout`; `git -c alias.x=rm x web/page.ts`, `git -c alias.x=mv x -f a b`, `git -c alias.x=checkout x -- web/page.ts`, `git -c 'alias.x=!rm' x`; `git x web/page.ts` (unknown verb, refused); `git status`, `git log --oneline -5`, `git diff`, `git fetch` (still allowed). Keep every existing row passing.
4. Not in scope, whoever asks: `README.md` (row 4 owns it while in flight; the lead removes the two gap sentences from the known-spellings paragraph when this row lands), any other walk gap (N2's `go fmt`, `npx prettier`, `unzip -o -d`, `golangci-lint run --fix`, `mkdir -p`, `git merge-file`, `git submodule update --init`, `chown`, which stay listed), the lease hook, the judge.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`. If it fails only on `TestHookPreToolUseTiming` in `internal/slot` under a load average above about 35, run it again when the load is lower; report every run with its load.

Hand back: the gate output, the diffstat, the new rows and their results, the line count of `bash.go` (967 before this row), and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. Commit on your own worktree branch only; the lead merges, pushes and removes the worktree after the audit. No `Co-Authored-By:` trailer and no "Generated with" line.

## Close-out, 2026-10-17

Built on `ap/row-14` by Opus 5.5. Only `internal/lease/bash.go` and `internal/lease/bash_test.go` changed, plus this section. `bash.go` went from 967 to 1068 lines; most of the growth is the verb list of step 2.

What changed, by step:

1. `gitCommand` first asks `fromFile`: for `rm`, `add`, `mv`, `checkout`, `restore`, `reset`, `stash` and `commit`, any `--pathspec-from-file` before a `--` (with `=` or with its value in the next word, and abbreviated as git allows) returns one target, `<git paths from OPTION>`, and nothing else. The denial reads "with `--pathspec-from-file=l` git reads the paths from a file or standard input, which the lease walk does not read; give the paths on the command line". A dry run on git 2.49.0 in this worktree showed git takes `--pathspec-fr=` and `--pathspec-from=`, so the check matches abbreviations; every shorter prefix is an ambiguous-option error in git, so refusing it costs nothing.
2. In `git()`, a `-c` key that starts with `alias.` in any case (git reads `ALIAS.x` the same, proven by dry run) adds `<git alias NAME>` and stops the walk; the value is not read. The denial names the alias and says to run the git verb directly. After `gitWritesTree`, a verb that `gitReads` does not accept adds `<git verb NAME>`, "not a git verb the lease walk reads". This catches a stored alias, and an alias from `--config-env=alias.y=X` too, which the dry run showed git runs. An alias cannot take the name of a builtin verb (git ignores it, proven by dry run), so no alias reaches a verb on the list.
3. 23 walk rows under "Audit 5" in `TestWriteTargets`, each passing, and `TestRefusalNamesWhatItCouldNotRead`, which checks that each new denial names what it could not read. Two existing rows changed their expected result on purpose: `git -c alias.x='!rm web/x.ts' x` (rule 6) and `git -c alias.x='!rm /r/b/web/x.ts' x` (hunt 2) now give the alias refusal in place of the walked file. Step 2 refuses every `alias.` key, so those rows could not keep their old result; both are still denied.

`refusal` now reads its prefixed denials from one table (`prefixed`), and `unread` builds those targets; `pathspecs` uses it. No exported name was added.

The new edge of the allowlist. A git verb passes the walk only when it is one of these:

- a verb the walk judges: `rm` and `mv` (by operand), the tree writers, `reset`, `checkout`, `stash` and `worktree` (by `gitWritesTree`, as before);
- a verb in `gitReaders`: `status`, `log`, `diff`, `show`, `blame`, `grep`, `ls-files`, `ls-tree`, `rev-parse`, `rev-list`, `cat-file`, `describe`, `shortlog`, `fetch`, `merge-base`, `name-rev`, `for-each-ref`, `help`, `version`, and `add`, `commit` and `push`;
- `branch` without `-d`, `-D`, `-m`, `-M`, `--delete` or `--move`; `tag` without `-d` or `--delete`; `remote` with no operand or with `show` or `get-url`; `reflog` with no operand or with `show`, `list` or `exists`; `config` with `--get` and its kin, `--get-colorbool`, `--list` or `-l`, or the `get` or `list` subcommand.

`add`, `commit` and `push` are not in the brief's read-only list. They are on the list because they write the index, objects and refs only, and refusing them would deny the commands every workspace runs. The lead may remove them.

What is left:

- Every git verb outside that list is now refused, so `git merge-file` and `git submodule update --init`, which the README lists as open gaps, are now refused as "not a git verb the lease walk reads". The README's known-spellings paragraph can drop them with the two gap sentences for B12 and B13. `git init`, `git clone`, `git gc`, `git notes` and `git config user.name` (a one-key read) are refused the same way. `git branch -c` (a copy) still passes, since the brief's list names only `-d`, `-D`, `-m` and `-M`.
- An option word that is the value of another option (`git commit -m --pathspec-from-file`, `git tag -m '-d x'`) is read as the option and refused. This over-refuses and never allows.
- The value of a `-c alias.` key is no longer walked, because the command is refused at the key.
- Not verified: git versions other than 2.49.0, and the live hook from a workspace (the walk rows and the judge path through `refusal` are the proof here).
