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

Hand back: the gate output, the exported surface with each function's one-line contract, the list of recognised spellings as tested, the findings for row 3, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.

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
- `WriteTargets(cwd, command) []string`: the files the command writes, absolute, or as written when the walk cannot place them.

The deny messages, as the tests write them out:

- ``wk: this workspace has not claimed a row; run `wk claim <row>` or `wk claim --none` in <root>``
- `wk: <rel> is leased by row <row> in <holder root>; leave it to that workspace or wait until it releases`
- ``wk: <path> is single-writer (leases.single_writer) and this workspace does not lease it; run `wk lease add <path>` in <root>``
- `wk: cannot tell which file this writes ("<path>"); give the path in full`
- A `Bash` denial adds ` (the command writes <target>)`.

The spellings the walk recognises, each in `bash_test.go` (58 commands after fix round 1, PD-13 asks for 25): `sed -i` in its GNU, attached-suffix (`-i.bak`), macOS empty-suffix (`-i ''`), clustered (`-Ei`), `-e` and `--` forms; `tee` and `tee -a`, also in a pipe; `>`, `>>`, `>|`, `&>`, `>file`, `2>file`, with `2>&1` and `<` read as no target; `cp` (last argument); `mv`, `rm`, `touch` (every operand, `--` honoured); `git rm -r`, `git mv`, `git -C <dir>`; `python`, `python3` with `-c` and `open(` with a write mode, also `mode=`; `node -e` with `writeFileSync(`. The walk also covers leading `VAR=value` words, `cd` then `&&` or `;`, an absolute `cd`, a subshell whose `cd` ends at `)`, shell keywords (`if ... then`), newlines, comments, quoted and escaped spaces, `>` inside quotes, heredoc bodies (skipped, with `<<-` too), and an open quote (no targets).

### Build-level calls, each with its reversal

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
- Spellings that pass unseen, for the README's statement that `Bash` coverage is partial: wrappers (`sudo`, `env`, `xargs`, `sh -c`, `bash -c`), `find -delete` and `-exec`, `perl -i`, `dd of=`, `ln`, `chmod`, `truncate`, `install`, `patch`, `git checkout -- <file>`, `git restore`, `cp -t`, a program fed on standard input (`python3 - <<EOF`), backticks and `"$(...)"` inside double quotes, `command`, `nohup`, `timeout`, `gofmt -w`, `prettier --write`, `pathlib` `write_text`, `git apply`, `git stash pop`, `rsync`, `tar -C` and `awk -i inplace`. The commit guard's `raw_words` differs on purpose: it is an allowlist that refuses any character outside its safe set, while this walk is a denylist by design (D11) that refuses only what it recognises.
- Known over-refusals, each costing one message: `[[ a > b ]]` and `(( a > b ))` read `>` as a redirection, and a glob or mid-word `$` in the working directory (`rm *.log`) is judged as the whole tree, so it is refused while any other root holds a lease.

### Fix round 1, 2026-10-16

The Opus 5.5 audit of `49d7b2c` found two wrong allows and three points to tighten, all fixed in one commit. A `<<` inside `$(( ))` or `(( ))` opened a false heredoc that hid the lines after it, so `echo $((1<<2))` followed by `rm web/x.ts` on the next line wrote a leased file unseen; the splitter now counts parentheses inside arithmetic and opens a heredoc only outside it, and a `<<` that gets no delimiter on its line is dropped. `pushd web && rm x.ts` placed the target in the old directory; `pushd`, `popd`, `builtin cd` and `command cd` now leave the directory unknown, as `cd -` and `cd $X` already did, so the target is refused. A second `Claim` by one root is refused until `Release`, because the builder and auditor in one root would lose each other's leases. `Judge` matches tool names exactly and fails closed on any other tool. The gap list above gained the auditor's spellings. `sedFiles` and `step` were each split once (`sedLong`; `newline` and `comment`). Each new walk test was run against the old `bash.go` and failed there.
