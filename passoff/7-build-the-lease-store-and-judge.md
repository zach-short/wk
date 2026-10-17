# 7. Build the lease store, the edit judgment and the `Bash` word walk in `internal/lease`

> **Claimed `IN FLIGHT` 2026-10-16.**

*Opened 2026-10-16 by row 2, as a P2 sibling of row 3 (`docs/incomplete/landing-layer/PLAN-P2-P4.md` section 2).*

**Model: Opus 5.5. Lane D. Waits on 2.**

You are building one package of P2 of the landing layer, `wk`. Read first: `PLAN-P2-P4.md` sections 1 (PD-3, PD-4) and 2, the row 7 line of its sub-row table (binding), `DESIGN.md` D10, D11, D12, D13 (L1 only; L3 is not built, plan section 2), options H, I, J, K and section 6 (the `file_path` hazard), `docs/conventions-go.md`, and `/Users/zachshort/Projects/personal-config/templates/hooks/commit-guard.sh` (651 lines; `raw_words` from line 108 and the walk that follows it) for the word-walk precedent. You own only `internal/lease/`; row 3 wires it. Nothing in your package imports `internal/wk`, `internal/config` or `internal/daemon`, and it adds no dependency to `go.mod` (PD-4).

Do these, in order:
1. `internal/lease/store.go`: `Store` holding workspaces keyed by worktree root, each `{Row string, Session string, Leases []string, ClaimedAt time.Time}`; a lease is a repository-relative path, with a trailing `/` for a directory prefix. `Claim(root, row, session, leases) error` refusing when any lease overlaps one held by another root and naming that root and its row in the error; `Add(root, leases) error` with the same rule; `Release(root)`; `Holder(path) (root, row string, ok bool)`; `Claimed(root) bool`. Overlap: equal paths, or one is a directory prefix of the other. JSON tags on every field so the daemon can save it with `SaveJSON`.
2. `internal/lease/judge.go`: `Resolve(roots []string, cwd, path string) (root, rel string, ok bool)`: the absolute path by longest-prefix match over the roots; `ok == false` for a path in no root. `Request{Tool, Cwd, Path, Command string}` and `Decision{Allow bool, Reason string}`. `Judge(store, cfg Rules, roots, req) Decision` with `Rules{SingleWriter []string, BashGuard bool}` in this order: a path in no root is allowed; a root that has not claimed is denied with "run `wk claim <row>` or `wk claim --none` in <root>"; a path another root leases is denied naming that root and its row; a `SingleWriter` path this root does not lease is denied naming the rule; else allowed. For `Tool == "Bash"`, the targets come from step 3 and every target is judged; any denial denies the command.
3. `internal/lease/bash.go`: `WriteTargets(cwd, command string) []string`: a quote-aware split into words (single and double quotes, backslash), lists split on `&&`, `||`, `;`, `|` and newlines; leading `VAR=value` words skipped; `cd <dir>` at the top of a list moves the directory for the words after it; recognised write spellings: `sed -i` (every non-flag argument after the expression), `tee` and `tee -a`, `>` and `>>` (the next word, also the `>file` and `2>file` forms), `cp` and `mv` (the last argument), `rm`, `touch`, `git mv` (the last argument), `git rm`, `python`, `python3` and `node` with `-c` or `-e` whose code contains `open(` and a `w` or `a` mode (the string literal before the mode). Targets are resolved against the directory in effect and returned absolute. A command it cannot split returns no targets (fail closed only on what it recognises, D11). It never rewrites the command.
4. `internal/lease/lease_test.go`, `judge_test.go` and `bash_test.go`: a table for the overlap rule (equal, prefix, sibling, parent); a table for `Judge` with each deny reason and its message text written out; a table for `WriteTargets` with at least 25 commands (PD-13) including heredocs (no target), `cd web && sed -i 's/a/b/' x.ts`, `FOO=1 tee -a log`, a quoted path with a space, `>` inside quotes (no target), `git rm -r dir`, and `python3 -c "open('f','w')"`; a JSON round trip of the store.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`; the package tests above green.

Ask before building: nothing.
Not in scope, whoever asks: parsing the board (row 8), the socket (row 6), the hook, any subcommand or config key, a shell parser dependency (PD-4), D13 L3. A need outside your package is a finding for row 3 in your close-out, never an edit.

Hand back: the gate output, the exported surface with each function's one-line contract, the list of recognised spellings as tested, the findings for row 3, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. Commit on your own worktree branch only; the lead merges, pushes and removes the worktree after the audit. No `Co-Authored-By:` trailer and no "Generated with" line.

## Close-out, 2026-10-16

Built on `ap/row-7` from `main` at `344f8c7` by Opus 5.5, the model the row names. The package is `internal/lease/` and nothing else changed except this section. It imports only the standard library, so `go.mod` is untouched (PD-4), and it imports no sibling package.

### What changed and why

`store.go` holds the claims, `judge.go` resolves a path to its worktree and judges a tool call, and `bash.go` finds the files a `Bash` command writes. The three test files hold the tables the prompt asks for. The exported surface, with each contract:

- `Store{Workspaces map[string]Workspace}` and `Workspace{Row, Session, Leases, ClaimedAt}`, JSON tags on every field (`workspaces`, `row`, `session`, `leases`, `claimed_at`). The zero value is ready. A `Store` is not safe for concurrent use.
- `(*Store).Claim(root, row, session, leases) error`: records the claim; refuses a root that has claimed already (``<root> has already claimed row <row>; run `wk release` there first``), a malformed lease, or a lease that overlaps one of another root, with the message `lease <l> overlaps <h>, held by row <row> in <root>`.
- `(*Store).Add(root, leases) error`: the same rule; refuses a root with no claim.
- `(*Store).Release(root)`: drops the claim and its leases.
- `(*Store).Claimed(root) bool`: true for any claim, `--none` included.
- `(*Store).Holder(rel) (root, row, ok)`: the first root, in sorted order, holding a lease that overlaps `rel`, where `rel` counts as a possible directory.
- `Resolve(roots, cwd, path) (root, rel, ok)`: joins a relative path to `cwd`, cleans it, and picks the longest root that contains it; `rel` is slash-separated and `.` for the root itself; `ok` is false outside every root. It reads no disk.
- `Judge(store, Rules{SingleWriter, BashGuard}, roots, Request{Tool, Cwd, Path, Command}) Decision{Allow, Reason}`: the order of section 2, with every target of a `Bash` command judged and one denial denying the command. Tool names match exactly (`Bash`, `Edit`, `Write`, `MultiEdit`, `NotebookEdit`); any other tool with a path is judged as an edit, and any other tool without one is denied with ``wk: cannot tell which file the tool "<tool>" writes; give the path in full``.
- `WriteTargets(cwd, command) []string`: the files the command writes, absolute, or as written when the walk cannot place them. A command it cannot split (an open quote) returns the one target `<open quote>`, which `Judge` refuses with `wk: cannot tell which file this writes: the command has an open quote, and bash runs the lines before it; close the quote` (fix round 2).

The deny messages, as the tests write them out:

- ``wk: this workspace has not claimed a row; run `wk claim <row>` or `wk claim --none` in <root>``
- `wk: <rel> is leased by row <row> in <holder root>; leave it to that workspace or wait until it releases`
- ``wk: <path> is single-writer (leases.single_writer) and this workspace does not lease it; run `wk lease add <path>` in <root>``
- `wk: cannot tell which file this writes ("<path>"); give the path in full`
- A `Bash` denial adds ` (the command writes <target>)`.

The spellings the walk recognises, each in `bash_test.go` (96 commands after fix round 2, PD-13 asks for 25): `sed -i` in its GNU, attached-suffix (`-i.bak`), macOS empty-suffix (`-i ''`), clustered (`-Ei`), `-e` and `--` forms; `tee` and `tee -a`, also in a pipe; `>`, `>>`, `>|`, `&>`, `>file`, `2>file`, with `2>&1` and `<` read as no target; `cp` (last argument); `mv`, `rm`, `touch` (every operand, `--` honoured); `git rm -r`, `git mv`, `git -C <dir>`; `python`, `python3` with `-c` and `open(` with a write mode, also `mode=`; `node -e` with `writeFileSync(`. The walk also covers leading `VAR=value` words, `cd` then `&&` or `;`, an absolute `cd`, a subshell whose `cd` ends at `)`, shell keywords (`if ... then`), newlines, comments, quoted and escaped spaces, `>` inside quotes, heredoc bodies (skipped, with `<<-` too; walked when the line names a shell or the delimiter never comes), arithmetic (`((`, `$((`, `$[`) where `<<` is a shift, a trailing backslash, brace expansion (narrowed like a glob), process substitution (the outer command goes on after its `)`), `sh -c` and `bash -lc` (the program walked from the directory in effect), `exec`, and an open quote (refused).

### Build-level calls, each with its reversal

- A command the walk cannot split is refused, not passed (fix round 2, K4, a decision the orchestrating Fable 5.1 session took for the owner and recorded in the run's state file): bash runs every complete line before it reports an open quote on a later one, so `rm web/x.ts` then `echo "` deletes the file. The prompt said no targets. Reversal: return no targets.
- After a `case` word no directory is known, the saved ones included, because a pattern's `)` closes no subshell; a `cd` beside `|`, `|&` or `&`, and `eval`, leave the directory unknown too (fix round 2). The cost is that a relative write after them is refused. Reversal: track `case` patterns to `;;` and follow those `cd`s.

- `mv` and `git mv` judge every operand, not only the last: a moved source disappears from the tree, so a pass on it is a collision. Reversal: the last operand only.
- A fifth denial, before the four of section 2, for a path the judgment cannot place: an empty path, a relative path with no absolute `cwd`, a `Bash` target that starts with `$`, a backtick or `~`, or a relative target after a `cd` the walk cannot follow. Treating these as outside every root would be a silent pass. Reversal: allow them as outside every root.
- A word with a glob or a mid-word `$` is narrowed to its literal directory (`web/*.ts` is `web`), and every target is judged as a possible directory, so `rm -r web` is refused when another root leases anything under `web/`. Reversal: judge the word as written.
- A write mode is `w`, `a`, `x` or `+`, and Node's `openSync(`, `writeFile(`, `writeFileSync(`, `appendFile(` and `appendFileSync(` count, because Node has no `open(` idiom. Reversal: `open(` with `w` or `a` alone.
- A second `Claim` by one root is refused until `Release` (fix round 1; first built as a replace). Reversal: replace the claim.
- `pushd`, `popd`, `builtin cd`, `command cd` and `cd -` leave the walk's directory unknown, so a relative target after them is refused (fix round 1). Reversal: follow them.
- Any `((`, also `$((`, opens arithmetic, where `<<` is a shift and opens no heredoc; a `<<` with no delimiter on its line opens none (fix round 1). Reversal: none; skipping lines the walk cannot account for is the silent pass.
- `Claim` and `Add` refuse a lease that is empty, absolute, holds `..` or `.`, or is not in clean form. Reversal: accept and clean.
- No mutex in `Store`; the daemon serialises. Reversal: a mutex and a locking `MarshalJSON`.

### What is now fixed

The overlap rule, the claim refusal that names the holder, the judgment order and its messages, and the `Bash` walk exist and are tested. The package gate line ran green on 2026-10-16: `go test -race -count=1 ./internal/lease/...` gave `ok`, and the whole gate exited 0 with `0 issues.` from golangci-lint.

### Questions answered

Whether the walk needs a shell parser: it does not for the spellings of section 2 (PD-4 holds). Whether an untracked worktree nested under the main checkout resolves to itself: yes, by longest prefix (`TestResolve`, "nested root wins").

### Left owed, findings for row 3

- Canonical paths. `Resolve` reads no disk. Row 3 must make the roots from `git worktree list` and the hook's `cwd` and `file_path` agree on symlinks (macOS `/tmp` is `/private/tmp`, `/var` is `/private/var`), or a path inside a root reads as outside every root and is allowed. A symlink inside a worktree that points into another worktree (DESIGN.md section 6) is judged where it is written.
- The judgment keys on the root that holds the path, not the root of `cwd`. A session in worktree A that writes an absolute path in worktree B is judged as B's write. If row 3 wants the session's own root to be the holder, `judgePath` needs the `cwd` root as well.
- `Judge` has no `enabled` flag: with `leases.enabled` false, row 3 must not call it. The daemon-down refusal is row 3's too.
- Call `Judge` only for the five tools `Bash`, `Edit`, `Write`, `MultiEdit` and `NotebookEdit`, with the name exactly as Claude Code sends it. Map `NotebookEdit`'s `notebook_path` and `MultiEdit`'s `file_path` into `Request.Path`.
- Hold one lock around each store operation and the `SaveJSON` after it.
- `Claim` refuses the whole claim on one malformed lease. Row 3 should turn row 8's `**` tokens into a trailing `/`, and route a token that is not clean into the unread list before it calls `Claim`.
- Spellings that pass unseen, for the README's statement that `Bash` coverage is partial: wrappers (`sudo`, `env`, `xargs`), a script run by a shell (`bash script.sh`), `find -delete` and `-exec`, `perl -i`, `dd of=`, `ln`, `chmod`, `truncate`, `install`, `patch`, `git checkout -- <file>`, `git restore`, `cp -t`, a program fed on standard input (`python3 - <<EOF`), backticks and `"$(...)"` inside double quotes, `command`, `nohup`, `timeout`, `gofmt -w`, `prettier --write`, `pathlib` `write_text`, `git apply`, `git stash pop`, `rsync`, `tar -C` and `awk -i inplace`, and a `cd` inside a shell function (`f() { cd web; }; rm x.ts; f` places `x.ts` in `web`). The commit guard's `raw_words` differs on purpose: it is an allowlist that refuses any character outside its safe set, while this walk is a denylist by design (D11) that refuses only what it recognises.
- Known over-refusals, each costing one message: `[[ a > b ]]` and `(( a > b ))` read `>` as a redirection, and a glob or mid-word `$` in the working directory (`rm *.log`) is judged as the whole tree, so it is refused while any other root holds a lease. An unquoted `$(...)` as an argument (`cp a $(echo b) x`) leaves the word `$`, which cannot be placed, so the command is refused. After `case`, beside a pipe or after `eval`, a relative write is refused.

### Fix round 1, 2026-10-16

The Opus 5.5 audit of `49d7b2c` found two wrong allows and three points to tighten, all fixed in one commit. A `<<` inside `$(( ))` or `(( ))` opened a false heredoc that hid the lines after it, so `echo $((1<<2))` followed by `rm web/x.ts` on the next line wrote a leased file unseen; the splitter now counts parentheses inside arithmetic and opens a heredoc only outside it, and a `<<` that gets no delimiter on its line is dropped. `pushd web && rm x.ts` placed the target in the old directory; `pushd`, `popd`, `builtin cd` and `command cd` now leave the directory unknown, as `cd -` and `cd $X` already did, so the target is refused. A second `Claim` by one root is refused until `Release`, because the builder and auditor in one root would lose each other's leases. `Judge` matches tool names exactly and fails closed on any other tool. The gap list above gained the auditor's spellings. `sedFiles` and `step` were each split once (`sedLong`; `newline` and `comment`). Each new walk test was run against the old `bash.go` and failed there.

### Fix round 2, 2026-10-16

The second Opus 5.5 audit of `b8d2921` confirmed round 1 and found three more wrong allows and one unsafe rule. `echo $[1<<2]` opened a false heredoc: `$[` now counts as arithmetic, and, for the whole class, a heredoc whose delimiter never comes is walked as commands from its first line, which can only refuse more. A backslash at the end of the input failed the split; it is now a continuation and the words are kept. `rm {web,docs}/x.ts` named no lease; `{` now narrows a word to its literal directory, as a glob does. A command with an open quote passed with no targets although bash runs the lines before it; it is now refused (K4 above). The auditor's `case`, pipeline, background and `eval` cases are closed by making the directory unknown; the shell function case is listed above. My own hunt then ran 60 further spellings in the forms the walk claims (arithmetic, quoting, continuation, expansion, directory tracking) and found four more: `let x=1<<2` (closed by the unterminated-heredoc rule), a heredoc fed to `sh` or `bash`, whose body runs (now walked), `tee >(cat) web/x.ts`, where the `(` cut `tee` short (process substitution now keeps the outer command), and `exec rm a` (`exec` is now skipped like a keyword). While there I walked `sh -c` and `bash -c` programs, which took them off the gap list. Twenty-four hunt spellings are now table rows. Every new row was run against the round-1 `bash.go` and failed there, and passes now.

## Held, 2026-10-16

Built under the autopilot run of 2026-10-16 (`docs/incomplete/autopilot-2026-10-16/`), on branch `ap/row-7` in worktree `.claude/worktrees/ap-row-7`, commits 49d7b2c, b8d2921 (fix round 1) and 82e9dcf (fix round 2), gate green at each. Done: the lease store (`Claim` refuses a second claim until `Release`), `Resolve`, `Judge` for the five edit tools with exact tool names (every required probe right across three audits), and the `Bash` word walk with a 96-command table and 160 passing tests. Left: the word walk still allows a write in forms it claims to handle; three audits (Opus 5.5) each found new ones (round 1: `<<` in arithmetic, `pushd`; round 2: `$[`, a trailing backslash, brace expansion, an open quote on a later line; round 3: `..` after a glob or brace, a `cd` after `&&` or `||` that did not run, a heredoc piped to `sh` on a later line, `\'` inside `$'...'`, `<<` inside `${...}`). The findings are in `docs/incomplete/autopilot-2026-10-16/reports/row-7-audit-1.md`, `row-7-audit-2.md` and `row-7-audit-3.md`. A denylist word walk with no shell parser (PD-4) cannot be closed by another fix round: the row is held for the owner's call on the walk's design. Rows 3, 4 and 5 wait on this row.

## Pass 2, 2026-10-16

Built on `ap/row-7` by Opus 5.5, the model the row names, after `main` was merged in at `eb0c622` (the one conflict was this file; the Held section above is kept as it stands). The store and the edit judgment are unchanged; `bash.go` is rewritten, `judge.go` changes in `judgeBash` alone, and every exported signature is as pass 1 left it. No dependency and no change to `go.mod` (PD-4).

### The design decision

The owner's rule chose an allowlist because it prevents a silent allow, lands without the owner, and is the least code. Three audits had each found new wrong allows in the denylist walk, and every one of them sat in a part of the shell the walk tried to model: arithmetic, brace and glob expansion, `$'...'` quoting, `${...}`, a backslash at the end of the input, heredoc bodies it skipped. The walk now reads a command only when every byte is in the safe set of the commit guard's precedent (`raw_words`): ASCII letters and digits, space, tab, newline and `- _ . / = : , + @ ~ % ' " | & ; < > # !`. Any other byte refuses the whole command with "wk: cannot tell which file this writes: the command holds `X`, which the lease walk does not read; write it without that character or ask for the path in full". Inside the set no word can expand, so the code that modelled expansion is gone.

### What changed

- `WriteTargets` scans the command first and returns the one target `<character X>` for the first byte outside the set (a zsh numeric glob such as `<1-9>` is refused the same way, since it is a glob spelled in safe characters). `judgeBash` turns that target, and `<open quote>`, into its denial through `refusal()`.
- Deleted as unreachable: arithmetic counting (`((`, `$((`, `$[`), glob and brace narrowing and the `exp` index in `place()`, process substitution and the subshell frames, backslash escapes and continuations, the heredoc-skipping machinery, and the Python and Node `-c` program reader (every write call in those languages needs a parenthesis).
- Every heredoc body is walked as commands, and so is every here-string's word. A `<<` with no delimiter walks on.
- Directory tracking keeps a set: every `cd`, `pushd`, `builtin cd` and `command cd`, wherever it sits, adds its directory and keeps the ones before it, so a later relative target is judged at each. `popd`, `cd -`, a bare `cd`, `pushd +1`, zsh's two-word `cd`, and a relative `cd` in a command that names `CDPATH` add an unknown directory. Past 16 directories the walk cannot tell. `eval` and `case` leave the directory unknown as before, and `eval`'s arguments, a `trap` program and an `alias` value are walked as commands (the last two from an unknown directory).
- A table of wrappers is stripped before the verb, each with the options it may carry; any other option refuses the command (`sudo -D`, `env -C`, `env -S`, a long option): `command`, `builtin`, `exec -a`, `nohup`, `time -p`, `timeout`, `env` with assignments, `sudo -u`, `coproc`, `nice`, `busybox`, and zsh's `noglob`, `nocorrect`, `-` and `repeat`. `xargs` gives its command one operand the walk cannot see, so `xargs rm` is refused and `xargs grep` passes. A leading `NAME+=value` is skipped like `NAME=value`.
- Verbs added: `unlink`, `rmdir`, `truncate`, `rsync`, `chmod` (every operand); `cp -t`, `ln`, `install`; `sed -I`; `dd of=`; `find -delete` (its starting points) and `find -exec` (refused); `tar` (its operands, and where it extracts); `gofmt -w`, `goimports -w`, `prettier --write`, `perl -i`, `awk -i`; `patch` (refused); `zsh -c`, `dash -c`, `ksh -c` and kin; a shell or `source` reading standard input (refused).
- Git: `reset --hard` (also `--merge` and `--keep`), `clean`, `checkout` (all but `-b <new>` alone), `switch`, `restore`, `merge`, `pull`, `rebase`, `stash` (all but `list` and `show`), `apply`, `am`, `cherry-pick`, `revert`, `worktree remove` and `move`, `bisect`, `checkout-index`, `read-tree`, `sparse-checkout` and `mergetool` write the worktree root. `judgeWrite` resolves the target to its root and judges it on leases alone, so it is denied whenever another root holds a lease and allowed otherwise. `--git-dir`, `--work-tree` or any `GIT_` variable in the command make git's directory unknown; a `git -c` value is walked as a command from an unknown directory.
- zsh redirections `>!`, `>>!`, `>>|`, `&>|`, `&>!`, `>&|`, `>&!` and `>>&` are writes. An unquoted word that starts with `~` or `=`, or holds `~` or `#`, cannot be placed (zsh expands them, the last two under `extended_glob`).

### What is now fixed

Every wrong allow in the three audits is closed, each by a table row: audit 1 B1 and B2, audit 2 K1, K2 and K3, and audit 3 B1 to B6 are refused by rule 1 or caught by the directory set (`test -f docs/Makefile && cd docs; rm web/x.ts` and `cd docs/web 2>/dev/null || cd web; rm x.ts` are denied at the old directory), and the heredoc piped to `sh` is walked. The audits' gap lists are caught or refused: backticks and `"$(...)"`, `command`, `nohup`, `timeout`, `gofmt -w`, `prettier --write`, `pathlib` (parentheses), `git apply`, `git stash pop`, `rsync`, `tar -C`, `awk -i inplace`, `FOO+=x`, `a[0]=1`, `sed -I ''`, `python3 -Bc`, `open(r'...')`, `node --eval=`, `eval 'rm ...'`, a shell function, `time -p`, `coproc`, `exec -a`, `source /dev/stdin <<EOF`, `unlink`, the whole-tree git writes, and `cd nope; rm web/x.ts`. My own hunt inside the safe set found and closed eleven more: `git checkout -f`, a here-string fed to `source /dev/stdin`, a pipe into `source /dev/stdin` or `bash -`, `CDPATH`, `ln` with one operand, `chmod`, `busybox`, `fish -c`, `git bisect`, the zsh redirections above, and `git -c alias.x='!rm ...'`. The walk table has 211 rows (96 in pass 1, each kept; a row whose command now holds an unsafe character expects the refusal), and the package runs 295 passing tests.

### Over-refusals, for row 3's README

- Any `$`, backtick, parenthesis, brace, bracket, `*`, `?`, backslash or non-ASCII byte anywhere in the command refuses the whole command, inside quotes too. So do a Python or Node program, a subshell, a command substitution, a `case` and a shell function.
- A heredoc or here-string whose data holds a write-looking line refuses when that line names a leased path (`> there`, `rm x`), and an apostrophe in a heredoc body is an open quote, which refuses the command.
- A `cd` makes a later relative target judged at two directories, the old and the new; after `popd`, `cd -`, a bare `cd`, `eval`, or a fifth `cd` that brings the directories past 16, a relative target is refused.
- A wrapper option outside the table refuses (`timeout --foreground`, `nice -5`, `sudo -s`).
- `xargs` with a writing verb, `patch`, `find -exec`, `bash` or `source` on standard input (a heredoc fed to a shell included) refuse.
- A whole-tree git command (`git pull`, `git merge`, `git checkout <branch>`, bare `git checkout`, `git stash`) is denied whenever another root holds any lease, and anywhere a `GIT_` variable is named.
- `tar -x` is judged as a write of the directory it runs in; `rsync` and `mv` judge every operand, the sources too; perl, awk and chmod judge their program or mode word as a path.
- An unquoted word with `~` or `#` in it, or starting with `=`, cannot be placed.

### Gaps that remain

The safe set closes the characters; the verb table is still a denylist (D11), so these pass unseen:

- Any program not in the table that writes: `make`, `go fmt`, `go mod tidy`, `go generate`, `npm install`, `curl -o`, `wget -O`, `unzip`, `sponge`, `cpio`, `git archive -o`, `git format-patch -o`, an editor driven by a script (`ed`, `ex`, `vim -c`), and a script run by name (`bash script.sh`, `./script.sh`, `source f`, `python3 f.py`).
- An interpreter whose language calls without parentheses: `perl -e 'unlink "f"'`, `ruby -e 'File.delete "f"'`, sed's own `w` and `e` commands.
- Wrappers outside the table: `stdbuf`, `ionice`, `flock`, `watch`, `caffeinate`, `script -c`, `su -c`, `parallel`, `doas`, `npx`.
- A verb under another name: an alias or function from the shell profile, a git alias or hook from git config, `hash -p`, `enable -f`, a copy or link of a binary, `mapfile -C` and other builtins that keep a command for later.
- The environment: a `CDPATH` exported by the profile (the walk follows `cd web` to the working directory's `web`), and zsh options set by the profile, such as `cdable_vars`.
- A `cd` inside a shell function is now refused (a function needs braces or parentheses), but a function defined by the profile is not seen.

### Build-level calls, each with its reversal

- The extra wrappers (`nice`, `busybox`, the zsh precommand modifiers), the extra verbs of the hunt, the extra git tree writers (`--merge`, `--keep`, bare `stash`, `worktree move`, `bisect` and the plumbing four) and `checkout -f` go beyond the brief's list; each closes a silent allow inside the safe set. Reversal: drop the table entries.
- A shell with no `-c` and no script, and `source` of `/dev/stdin`, are refused, so a heredoc fed to a shell is refused although its body is walked. Reversal: delete the `fromStdin` checks.
- A whole-tree git write skips the single-writer rule, because the single-writer files are mostly untracked and no git command rewrites them; judging it there would refuse every `git merge` in a root that does not lease them. Reversal: pass the rules through in `judgeWrite`.
- The deny message for a whole-tree write names the root as `.` ("wk: . is leased by row 9 in /r/c ... (the command writes /r/b)"), as pass 1 did for a directory target. Reversal: a separate message in `judgeWrite`.
- A wrapper with an option it does not know refuses the command rather than guessing where the verb starts. Reversal: treat the option as a flag.
- `bash.go` is 846 lines, against 653 after pass 1: the allowlist deleted about 250 lines of expansion handling, and the wrapper table, the git tree writers, the directory set and the rule 7 verbs added more. Reversal: drop the verbs beyond the brief's list.

### Left owed

- Row 3: the README states the over-refusals and the gaps above; `WriteTargets` returns `<character X>` and the `<...>` targets of unseen writes (`<xargs input>`, `<standard input>`, `<sudo option>`), which `Judge` refuses; a whole-tree git write appears in `WriteTargets` as the directory git runs in. The pass 1 findings for row 3 stand.
- Not verified in a real shell: a scratch probe of the zsh claims (`>!`, `>>&`, `x<1-9>.ts`, `-`, `repeat`, `noglob`) and of `source /dev/stdin <<<` in bash 3.2 was denied by Claude Code's safety check because it ran `rm` inside `zsh -c`. The auditor should run it.
