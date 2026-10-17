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

### Fix round 1, 2026-10-17

From row 14 audit 1 (`docs/incomplete/autopilot-2026-10-16/reports/row-14-audit-1.md`). Only `internal/lease/bash.go` and `internal/lease/bash_test.go` changed, plus this section, after a merge of main (2a48b65, which brings the slot timing fix 941667c). `bash.go` is 1222 lines (1068 after the first commit, 967 on main before the row). Every fix is a refusal that names what the walk could not read, or a write target placed where git writes; none reads an option's value as a command.

- F1. `configReads` reads `git config` options only before the first operand, as legacy `git config` does, with `-f`, `--file`, `--blob`, `--type`, `--default` and `--comment` taking the next word. `git config NAME VALUE -l`, `... --get` and `... --list` are now refused as a verb form that changes config. `git config --get user.name`, `-l`, `--list`, `get` and `list`, and the one-key read `git config user.name` pass; one key with `--unset`, `--add`, `--replace-all`, a section option or `-e` is refused.
- F2. On every verb that reaches the read path (the readers, `stash show`, `stash list`), `--output=<path>` and `--output <path>` before a `--` are write targets, placed like a redirection, from where git runs (`-C` followed). Dry runs on git 2.49.0 showed git refuses every abbreviation of `--output`, so only the whole option is read. No verb on the read list takes `-o`.
- F3. `reset` matches `--hard`, `--merge` and `--keep` abbreviated (`--har`, `--merg`, `--kee`, and `--m`, which git refuses as ambiguous); `checkout` matches `--force` abbreviated (`--forc`). Short `-f` is matched alone, as before.
- F4. `git grep -O<cmd>`, `-O <cmd>`, `--open-files-in-pager=<cmd>` and `--open-files-in-pager <cmd>` are refused: "git grep runs a command on the matched files".
- F5. On `fetch`, `pull`, `push`, `ls-remote`, `clone` and `archive`, `--upload-pack`, `--receive-pack` and `--exec` in any spelling, and any argument that starts with `ext::` or holds `=ext::`, are refused. A git command beside a `GIT_` variable anywhere in the command (an assignment prefix, `env`, or `export`) is refused, naming the variable, unless the walk already refused it with a target it could not place; that keeps `GIT_WORK_TREE=/r/b git checkout main` at its old result. The check lives in `git()` beside the existing `GIT_` handling, which already reads the whole command, so it also sees `export` and `env`.
- F6. `git worktree add <path>` is a write of `<path>`, the first operand after `add` past the values of `-b`, `-B` and `--reason`. `list`, `prune`, `lock` and `unlock` read; `remove` and `move` are unchanged.
- F7. `--config-env=<key>=<var>` and `--config-env <key>=<var>` are refused for every key. A `-c` key in `programKeys` is refused as config that names a program or a file: `include.path`, `includeIf.*`, `core.hooksPath`, `core.sshCommand`, `core.editor`, `core.askpass`, `core.gitProxy`, `sequence.editor`, `protocol.ext.allow`, `diff.external`, `diff.*.command`, `diff.*.textconv`, `merge.tool`, `merge.*.driver`, `mergetool.*`, `difftool.*`, `filter.*`, `credential.*helper`, `remote.*.uploadpack`, `remote.*.receivepack`, `gpg.program` and `gpg.*.program`, in any case. The keys past the audit's list are on the safe side. Other `-c` values, `core.fsmonitor` and `core.pager` among them, are still walked as commands.
- N1. `merge-tree`, `show-ref`, `diff-tree`, `check-ignore`, `show-branch`, `range-diff`, `whatchanged`, `var`, `count-objects` and `ls-remote` are readers; `submodule` reads with no operand or `status`; `symbolic-ref` reads with exactly one operand and no `-d`.
- N2. `mv` is out of `pathspecVerbs`; `git mv --pathspec-from-file=l` has no operand and passes, as git refuses the option.

The corrected edge of the allowlist. A git verb passes the walk only when it is a verb the walk judges (`rm`, `mv`, the tree writers, `reset`, `checkout`, `stash`, `worktree`, with `worktree add` a write of its path), a verb in `gitReaders`, or a read form of `gitReads`: `branch` without `-d`, `-D`, `-m`, `-M`, `--delete` or `--move`; `tag` without `-d` or `--delete`; `remote`, `reflog` and `submodule` with no operand or a read subcommand; `symbolic-ref` with one operand; and `config` with `get` or `list`, a get or list option before its first operand, or one key and no writing option. On every verb that passes, `--output` is a write of its path, and the options, keys and variables above refuse the command.

The corrected reason for `add`, `commit` and `push`: they write the index, objects and refs, not worktree files, but `commit` and `push` run hooks the walk cannot see, and `push` takes `--receive-pack` and `--exec`, which are now refused. The hooks stay a gap of the whole walk, as before this row.

Two 9c7cec3 rows changed their expected result, each forced by a finding: the `--pathspec-from-file` row for the other verbs no longer holds `git mv` (N2), and `git --config-env=alias.y=X y web/page.ts` now gives the config refusal before the verb refusal (F7). Both are still denied where git would act.

What is left: a `core.pager` or `core.fsmonitor` value that names a script is walked as a command whose verb the walk does not know, so it passes, as before this row; hooks in the repository; git versions other than 2.49.0; the live hook through a built binary.

### Fix round 2, 2026-10-17

From row 14 audit 2 (`docs/incomplete/autopilot-2026-10-16/reports/row-14-audit-2.md`). Only `internal/lease/bash.go` and `internal/lease/bash_test.go` changed, plus this section, after a merge of main (59aa77f). `bash.go` is 1210 lines, down from 1222 after fix round 1 (1068 after the first commit, 967 on main before the row).

The splitter. `valueOptions` holds a git verb's options that take a value: short letters and long names, with `stop` for a verb whose options end at its first operand. Its `split` returns the options and the operands as git's option parser reads them. A short letter that takes a value ends its cluster and takes the rest of it, or the next word when it is last (`-f.git/x` and `-fl` are `-f` with a value; `-qf` is two flags). A long name matches whole or abbreviated, as git allows (`--reas` for `--reason`), and takes the next word when no `=` joins it. A value never reaches either list: the option keeps its cluster up to its letter (`-f`), or gets its value joined by `=` (`--file=v`). After `--` every word is an operand. Three tables use it: `configValues` (`-f`, `-t`, `--file`, `--blob`, `--type`, `--default`, `--comment`, stopping at the first operand), `worktreeValues` (`-b`, `-B`, `--reason`) and `checkoutValues` (`-b`, `-B`, `--orphan`, `--conflict`). `configReads`, `worktreePath`, `outputs` and `checkoutWrites` now call it, and their four hand loops are gone. The lists of value-taking options come from `git config --get -h`, `git worktree add -h` and `git checkout -h` on git 2.49.0.

What changed, by finding:

- B1. `configReads` reads the options `split` returns, so an option's value is never an option. `--comment -l`, `--comment --list`, `--comment --get`, `--global --comment -l`, `-f.git/logs/../config`, `-f -l x.y z`, `--file --get x.y z` and `-fl x.y z` followed by a key and a value are refused as a config write, as is the control `--comment=-l`.
- B2. `protocol.allow` and `protocol.*.allow` are program keys (the second covers `protocol.ext.allow`), and a `-c` whose key or value holds `ext::` is refused for any key. That covers `remote.o.url=ext::...` and `url.ext::....insteadOf=...`. `programKeys` is now one anchored regular expression over the same keys, which replaces the list and its matcher.
- B3. `worktreePath` takes the operand after `add` from `split`, so `--reas r` and `--reaso r` are `--reason` with its value, and the write is placed at `docs/x`.
- N-a. `checkoutWrites` is `split` and two `hasOption` calls: a `-f` alone or in a cluster (`-qf`, `-fq`) or `--force` whole or abbreviated is a tree write. `-bfeat` is still a new branch named `feat`.

Corrected statements:

- F1: legacy `git config` reads options only before its first operand, and the value of `-f`, `-t`, `--file`, `--blob`, `--type`, `--default` or `--comment`, attached or in the next word, is never read as an option. It reads with `get` or `list`, a get or list option, or exactly one key and no writing option.
- F5: on the transport verbs, `--upload-pack`, `--receive-pack` and `--exec` in any spelling, and an argument that starts with `ext::` or holds `=ext::`, are refused. Any `-c` whose key or value holds `ext::`, and `-c protocol.allow` or `protocol.<name>.allow`, are refused before the verb.
- F6: `git worktree add` writes the first operand after `add`, as git's option parser reads it, with the values of `-b`, `-B` and `--reason` (whole or abbreviated) consumed.
- N1 (fix round 1): `git var -l` passes; `git var GIT_EDITOR` is refused by the `GIT_` rule (N-b below).

Every ef61fe5 row passes unchanged. 18 new walk rows and 5 more keys in `TestWriteTargetsGitProgramKeys` all pass.

Open, as the lead asked:

- N-b. The `GIT_` rule reads the whole command text, so it over-refuses `git var GIT_EDITOR`, `git log --grep=GIT_X`, `echo GIT_X; git status` and `git status # GIT_X`. It never allows a write.
- N-c. `EDITOR='rm web/page.ts' git commit` passes the walk; with `GIT_EDITOR` unset git runs the editor. The agent environment sets `GIT_EDITOR=true`, and `unset GIT_EDITOR` is refused, so it is inert here; `env -i EDITOR=...` is not verified.
- N-d. A value that names a script (`-c core.fsmonitor=./hook.sh`, `-c core.pager=./script.sh`, `-c core.alternateRefsCommand=x`, `-c uploadpack.packObjectsHook=x`, `HOME=/tmp/h git status` with a planted config, `git --exec-path=/tmp/e fetch .`) passes, as scripts do everywhere in the walk.

### Sign-off, 2026-10-17

- Builder: Opus 5.5 (agent aa81df3e0bf24ad6e; two fix rounds). Auditors: Opus 5.5 (a1bc7b964a99615a2 blocking F1 to F7; ae25226e2a80508f4 blocking B1 to B3 under a scoped verdict; ae59ee80efc3a562a clean).
- Worktree `.claude/worktrees/ap-row-14`, branch `ap/row-14`, tip `619eb01` before this commit. Relay passes: 0. Fix rounds: 2.
- Verdict: clean (audit 3, 2026-10-17 11:49 EDT): gate exit 0, 418 PASS, no input refused by main or ef61fe5 that 619eb01 allows and git writes.
- Open, non-blocking: `checkout -t<x>` over-refused (`t` not in `checkoutValues`); `stash show --output <file>` next-word over-refused; the `ext::` checks are case-sensitive (enough on git 2.49.0); the `GIT_` text rule over-refuses `git var GIT_EDITOR` and comments; `EDITOR=` inert while `GIT_EDITOR` is set; a `-c` value that names a script passes as an unknown command (the walk's known gap); repository hooks stay outside the walk.
- The README's known-spellings paragraph still names `--pathspec-from-file`, aliases, `git merge-file` and `git submodule update --init` as open; the lead removes those sentences after row 4, which owns README.md while in flight, lands.
- Lifted: not a Deep row.
