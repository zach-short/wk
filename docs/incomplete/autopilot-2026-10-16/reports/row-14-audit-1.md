# Row 14 audit 1, Opus 5.5, agent a1bc7b964a99615a2

Launched 2026-10-17 10:59 EDT, returned 2026-10-17 11:07 EDT. Detached worktree `.claude/worktrees/ap-row-14-audit` at `9c7cec3`.

## Verdict: blocking findings

F1 is a hole in the refusal this row adds. F2 to F7 are older: main's walk allows each too, so none is a regression, but each is a git verb on the row's new list that writes a leased file while the walk allows it. Every allow the row closes is closed correctly; the row only narrows the allowlist.

### Blocking

- F1 (the row's own code). The `config` read-form check scans past the operands: `gitReads`, the `config` case, `has("l", ... "--list")` (bash.go:741) through `option()` (bash.go:751). Legacy `git config` stops reading options at the first non-option, so in `git config NAME VALUE -l` the `-l` is a third positional and git stores the value. Proof (git 2.49.0, scratch config): `git config --file=c alias.x rm -l` wrote `x = rm`; `--get`, `--get-colorbool`, `--list` each appended it; `GIT_CONFIG_GLOBAL=gc git config --global core.fsmonitor "touch p10; false" -l` stored the value and the next `git status` ran it. Walk: `git config core.fsmonitor 'rm web/page.ts' -l && git status`, `git config --global core.fsmonitor '...' --get`, `git config diff.external '...' --list && git diff HEAD~1` all ALLOW. Fix: read options only before the first operand.
- F2. `--output=<file>` on verbs in `gitReaders`: `diff`, `log`, `show`, `shortlog` wrote the file; `rev-list` and `blame` truncated it; `stash show` and `stash list` take it. Walk: all ALLOW with `--output=web/page.ts` from wk-2. The `gitReaders` comment (bash.go:710) is false for these forms.
- F3. Abbreviated tree-writer options: `gitWritesTree` matches `--hard`, `--merge`, `--keep` exactly (bash.go:786); `checkoutWrites` matches `-f` and `--force` exactly (bash.go:806); git accepts abbreviations. Proof: with `web/page.ts` dirty, `git reset --har` reverted it; `git checkout -b nb1 --forc` discarded the change. Walk: `git reset --har`, `--kee HEAD~1`, `--merg HEAD~1`, `git checkout -b nb1 --forc`, `git checkout --forc -b nb1` all ALLOW.
- F4. `git grep -O<cmd>` and `--open-files-in-pager=<cmd>` run the command with the matched files as arguments (proof: the command received `go.mod`). Walk: ALLOW.
- F5. Transport commands run through the shell: `git fetch --upload-pack=<cmd> .`, `git push --receive-pack=<cmd>` and `--exec=<cmd>` to `.`, `git -c protocol.ext.allow=always fetch 'ext::sh -c ...'`, `GIT_SSH_COMMAND=... git fetch ssh://...` each ran the command. Walk: all ALLOW.
- F6. `git worktree add <path>` checks out a whole tree at that path: `gitReads` returns true for `worktree` (bash.go:735) and `gitWritesTree` judges only `remove` and `move`. Proof: `git worktree add docs/x` created `docs/x/docs` and `docs/x/web`. Walk: `git worktree add docs/x` from wk-1 ALLOW though wk-2 leases `docs/`; `web/x` and `--detach web/x HEAD` too.
- F7. Config that arrives other than as a `-c` value: `git()` walks only `-c` values as commands (bash.go:649). `X=... git --config-env=diff.external=X diff HEAD~1`, `--config-env=core.fsmonitor=X status`, `GIT_EXTERNAL_DIFF=... git diff` each ran; `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.fsmonitor GIT_CONFIG_VALUE_0=... git status`, `git -c include.path=/tmp/c status`, `git -c core.hooksPath=/tmp/h commit` ALLOW in the walk (the last two not run in git). Control `git -c core.fsmonitor='rm web/page.ts' status` DENY.

### Non-blocking

- N1. Over-refusal, now denied as "not a git verb": `git merge-tree --write-tree`, `git symbolic-ref --short HEAD`, `git show-ref`, `git ls-remote`, `git diff-tree`, `git check-ignore`, `git show-branch`, `git range-diff`, `git whatchanged`, `git submodule status`, `git var`, `git count-objects`, `git config user.name`.
- N2. `pathspecVerbs` includes `mv`, but `git mv` on 2.49.0 has no `--pathspec-from-file` (comment at bash.go:685 wrong; harmless over-refusal). `rm`, `add`, `checkout`, `restore`, `reset`, `commit`, `stash push` do take it.
- N3. Decision 2 (`add`, `commit`, `push` in the read list): accept; no regression. The stated reason is incomplete: `commit` and `push` run hooks the walk cannot see, and `push` takes `--receive-pack` (F5).
- N4. Decision 1 (the two changed rows): accept; both DENY with the alias message.
- N5. Refuted: `git -c receive.denyCurrentBranch=updateInstead push . HEAD:refs/heads/b1` is rejected by git; the worktree unchanged.

## Reproduction

HEAD 9c7cec3a29d7; main e13aa27; merge-tree exit 0 (tree a072499); go mod download exit 0; diffstat 3 files +185/-11 (bash.go +119/-11, bash_test.go +50, item +27), names match; no attribution lines; git 2.49.0; diff read in full.

## Gates

Full gate run 1 at 11:00 (load 6.0 to 9.1): exit 0, every package `ok` (lease 3.9 s, slot 6.7 s, wk 67.7 s), `0 issues.` Verbose lease: 354 PASS, 0 FAIL. Build exit 0. No retry needed.

## Walk table (through `Judge` on a scratch copy of the row's lease sources; roots /r/w1 row 1 `web/page.ts`, /r/w2 row 2 `docs/`; cwd wk-2 unless said)

B12: every `--pathspec-from-file` form on `rm`, `add`, `checkout`, `restore`, `reset`, `stash push`, bare `stash`, `commit`, `mv`, the `--pathspec-fr=` and `--pathspec-from` abbreviations, `--pathspec-file-nul`, `-- web`, `--end-of-options`, `git -C web rm`, `git --no-pager rm`, `git commit -m --pathspec-from-file` (over-refusal): DENY with the paths-from-file message; `git rm -- --pathspec-from-file=l` ALLOW as the path `/r/w2/--pathspec-from-file=l` (right). Git dry runs: `--pathspec-fr=` and `--pathspec-from=` accepted; `--pathspec-f=` and `--pathspec=` ambiguous; `--pathspec-file-nul` with a NUL list removes, with a newline list fails; `--pathspec-from-file=l -- web` is a fatal conflict.

B13: `-c alias.x=rm x web/page.ts`, `alias.x=mv x -f a b`, `alias.x=checkout x -- web/page.ts`, `'alias.x=!rm' x`, `ALIAS.Y=rm Y`, with `-c core.pager=less`, quoted, empty value, `alias.rm=status rm` (git runs the builtin), with `--no-pager`: DENY alias. `--config-env=alias.y=X y`, `--config-env alias.y=X y`, `git -C sub x`, `git x` (stored alias), `GIT_CONFIG_COUNT=1 ... git x`: DENY verb. Dry runs: `ALIAS.Y` runs rm; `--config-env=alias.y=X` runs rm; `alias.x=` fails; `-calias.x=rm` unknown; `-c ' alias.x=rm'` invalid key; a value `-c alias.y=rm y` chains, so not reading the value is right.

Read list ALLOW: `status`, `log --oneline -5`, `diff`, `fetch`, `add -A && commit -m msg && push`, `branch feature`, `branch -a`, `tag v1`, `remote -v`, `remote show origin`, `reflog`, `config --get user.name`, `config -l`, `-c core.pager=less log`. DENY verb: `branch -D old`, `branch --del old`, `tag -d v1`, `remote add o u`, `reflog expire --all`, `config alias.x rm`.

Hunt: ALLOW and right (ref or index only): `commit -a -m msg`, `add -p`, `push --force`, `branch -c a b`, `branch -f main HEAD~1`, `tag -f v1`, `reset web/page.ts`, `checkout -b x`. DENY verb: `update-ref`, `symbolic-ref`, `filter-branch`, `archive -o web/page.ts HEAD`, `bundle create web/page.ts HEAD`, `format-patch -o web HEAD~1` (N2's gap now refused), `ls-remote --upload-pack=x .`. DENY tree writer: `switch`, `apply`, `am`, `cherry-pick`, `revert`, `rebase`, `merge`, `read-tree`, `checkout-index -a`, `sparse-checkout set`. `worktree list` ALLOW (right); `worktree add` ALLOW (F6). The rest: F1 to F7.

## Lean

bash.go 1068 (967 on main). `gitReaders` a map like `treeWriters`; a sorted slice no shorter. `fromFile` 6 lines, the alias check 4; `git()` 32, `gitReads` 19, `option` 17; `prefixed` shortens `refusal`. F1 and F3 both fit `option()` with a stop-at-first-operand mode.

## Documents

Close-out names the new edge, the two decisions, what is left; no contractions, dashes or AI-signal words; README untouched. Corrections needed: the edge statement does not know F1; the `gitReaders` comment and the add/commit/push reason overstate (F2, N3); the `pathspecVerbs` comment wrong about `mv` (N2).

## Could not verify

The live hook through the built binary (Judge on the same source only); git versions other than 2.49.0; `include.path` and `core.hooksPath` in git itself.

Scratch under `row-14-audit/`; audit worktree status empty, HEAD 9c7cec3; nothing committed; no command denied; the repo's config unchanged.
