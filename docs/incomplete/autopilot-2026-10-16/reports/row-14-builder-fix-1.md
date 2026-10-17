# Row 14 fix round 1, Opus 5.5, agent aa81df3e0bf24ad6e

Sent 2026-10-17 11:08 EDT, returned 2026-10-17 11:16 EDT. Commit `ef61fe5` on `ap/row-14` after a merge of main (2a48b65, which brings the slot timing fix 941667c). Not merged, not pushed.

Gate run 1 at load 3.9 to 5.8: exit 0, every package `ok` (lease 3.6 s, slot 6.1 s, wk 47.0 s), `0 issues.` Verbose lease: 400 PASS, 0 FAIL (354 at 9c7cec3). Diffstat `main...HEAD` 3 files +446/-23 (bash.go +297, bash_test.go +123, item +49); `9c7cec3..HEAD` bash.go 252, bash_test.go 77, item 22 (the slot test change in the range is the main merge). bash.go 1222 (967 on main, 1068 at 9c7cec3); longest function `git()` 36 lines; no new exported name. Patch rewritten.

## Per finding

- F1: new `configReads` reads `git config` options only before the first operand (`-f`, `--file`, `--blob`, `--type`, `--default`, `--comment` take the next word). `git config NAME VALUE -l`, `--get`, `--list` refused as config writes; `--get user.name`, `-l`, `--list`, `get`, `list`, the one-key read, `-f c.cfg --get x` allowed; one key with `--unset`, `--add`, `--replace-all`, a section option or `-e` refused.
- F2: on every verb on the read path (including `stash show` and `stash list`), `--output=<path>` and `--output <path>` before `--` are write targets placed like a redirection, `-C` followed. Dry runs: git rejects every abbreviation of `--output` on those verbs, so only the whole option matches; no listed verb takes `-o`. Comment corrected.
- F3: `reset` matches `--hard`, `--merge`, `--keep` by prefix (`--m` too; git refuses it as ambiguous); `checkout` matches abbreviations of `--force`; `-f` only alone.
- F4: `git grep` with `-O<cmd>`, `-O <cmd>`, `--open-files-in-pager=<cmd>` or `... <cmd>` refused.
- F5: on `fetch`, `pull`, `push`, `ls-remote`, `clone`, `archive`: `--upload-pack`, `--receive-pack`, `--exec` in any spelling refused; any argument starting with `ext::` or holding `=ext::` refused. A git command is refused naming the variable when a `GIT_` variable appears anywhere in the command (assignment prefix, `env`, `export`), placed in `git()` beside the existing `GIT_` check so it also sees `export GIT_SSH_COMMAND=x; git fetch`; exception: adds nothing when the walk already refused that git command with an unplaceable target (keeps `GIT_WORK_TREE=/r/b git checkout main` at its old result).
- F6: `git worktree add` writes its path (first operand after `add`, past the values of `-b`, `-B`, `--reason`); `list`, `prune`, `lock`, `unlock` read; `remove`, `move` unchanged.
- F7: `--config-env=k=v` and `--config-env k=v` refused for every key. New `gitConfig` refuses a `-c` key in `programKeys` (any case): the brief's keys plus `core.askpass`, `core.gitProxy`, `diff.*.command`, `diff.*.textconv`, `merge.*.driver`, `credential.*helper`, `remote.*.uploadpack`, `remote.*.receivepack`, `gpg.program`, `gpg.*.program`. Every other `-c` value still walked as a command (`core.fsmonitor`, `core.pager`).
- N1: readers now include `merge-tree`, `show-ref`, `diff-tree`, `check-ignore`, `show-branch`, `range-diff`, `whatchanged`, `var`, `count-objects`, `ls-remote`; `submodule` with no operand or `status`; `symbolic-ref` with exactly one operand, refused with `-d`.
- N2: `mv` out of `pathspecVerbs`, comment fixed.

Four new denials in the walk's style, each checked by `TestRefusalNamesWhatItCouldNotRead`. 44 new walk rows (F1 5, F2 the 8 verbs plus next-word, after `-C`, after `--`; F3 5 plus `--mixed`/`--soft` controls; F4 4; F5 9; F6 4; F7 3; N1 3; N2 1) and `TestWriteTargetsGitProgramKeys` (14 keys). Two 9c7cec3 rows changed by a finding (N2 drops `git mv`; `--config-env=alias.y=X y` now gives the config refusal then the verb refusal). Close-out `### Fix round 1, 2026-10-17` with the corrected edge and the corrected reason for `add`, `commit`, `push`.

## Not verified

A `core.pager` or `core.fsmonitor` value that names a script still passes (read as a command with an unknown verb; true before the row, left as told); repository hooks stay a gap of the whole walk; git versions other than 2.49.0; the live hook through a built binary; `include.path` and `core.hooksPath` not run in git itself. Git ran only in the worktree; HANDOFF.md, PASSOFF.md, README.md untouched; no call denied.
