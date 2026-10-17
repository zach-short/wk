# wk

wk makes warm git workspaces for parallel agents. A workspace is a git worktree whose dependency trees, such as `node_modules`, are already in place. wk does not install them again. It copies them from a template worktree with copy-on-write clones, so a new workspace costs almost no disk and little time. wk also judges which worktrees are finished and removes them.

wk is the first phase (P1) of a planned layer over git in four phases: cheap workspaces, path leases, landing changes on the server side, and one gate run per batch. Only P1 is built. The design and the build plan live in the ezhomesteading repository, under `docs/incomplete/landing-layer/`.

## Requirements

wk needs Go 1.24 or later to build and git to run. `wk new` needs macOS on APFS, because the copy-on-write step uses `clonefile(2)`. On any other system the clone step fails with an error that says so, since a byte copy of `node_modules` would cost the disk that wk exists to save. The `gc schedule` command writes a launchd agent, so it is macOS only as well. `wk gc` finds busy worktrees with `lsof`.

## Install

```bash
git clone https://github.com/zach-short/wk.git
cd wk
go install ./cmd/wk
```

`go install github.com/zach-short/wk/cmd/wk@latest` does the same without a checkout. The scheduled cleanup and the Claude Code hooks run the installed binary at its absolute path, so install it this way and not with `go run`, and reinstall from the branch whose hooks are installed: a binary built from an older commit has no `hook` command, and every worktree request then fails.

## Set up a repository

Run `wk` with no arguments inside a repository, in a terminal. The first time, it opens a guided setup. It reads the repository for package manifests, lockfiles, `.env` files and local branches, then asks which to use. It looks at file names only, so a `.env` file is listed and never opened. After the review step it writes the config to `~/.wk/<repo>.toml`. Two last choices are to build the template at once and, on macOS, to install the scheduled cleanup.

Setup never overwrites a config, because a config can hold hand-written notes. Once the file exists, a bare `wk` opens a home screen that shows the settings in use and offers to open the hive. Piped or run without a terminal, a bare `wk` prints the usage text.

## Make a workspace

```bash
cd "$(wk new 12)"
```

`wk new <row>` creates the workspace for row 12 and prints its absolute path on stdout. Progress and timings go to stderr, so the command substitution above captures only the path. A row is a positive integer of up to nine digits. By default it gives the directory `.claude/worktrees/wk-12` inside the main checkout and the branch `wk/12`, both cut from the base branch. The command refuses to run if that directory or branch already exists.

The shape of the checkout follows `workspace.sparse`. A sparse checkout leaves out the entries of `workspace.sparse_exclude`: a directory excludes everything under it, and an entry with a glob character, such as `docs/incomplete/**/*.png`, is a pattern, so a repository can leave out its screenshots and keep its markdown. `wk new <row> --sparse` and `wk new <row> --full` override the default.

When the pool holds a ready slot of the right shape (see below), `wk new` moves it into place, puts it on the new branch and returns. Otherwise it works from the template. Before it makes anything, it compares the template with the base branch. If the file named by `template.refresh_on` has changed since the template was built, or the template is missing, wk refreshes the template first. If only the base's commit moved, and `template.follow_base` is on, the refresh is light: a checkout, the copies and the post-clone commands, with no build. wk gives up after two refreshes, so a lockfile that changes every minute cannot hold the command forever.

Then wk adds the worktree without a checkout. It checks out the tree and clones every directory in `template.clone_dirs` at the same time, copies the files in `template.copy_files`, and runs the `template.post_clone` commands, unless the workspace sits at the template's own commit, where they would only rewrite what was cloned. At most `new.max_parallel` workspaces clone at once and the rest wait their turn: APFS runs concurrent clones and file writes on one volume one after another, so more at once only makes each of them finish late. If any step fails, the workspace stays on disk for inspection and the error says where.

## The pool

`pool.size` full and `pool.sparse_size` sparse workspaces can be made ahead under `pool.dir`, so the first agents of a burst start in under a second. A marker in each slot's private git directory records its state, its checkout shape and the template it was cloned from. A claim moves the slot to the workspace's path with `git worktree move` and attaches the branch; after every claim, a background `wk pool fill` makes a replacement, with its output in `~/.wk/<repo>/pool.log`. A slot whose template or shape no longer matches is never claimed, and the next fill removes it. `wk pool status` lists the slots. Both sizes default to 0.

## The template

The template is a detached worktree at the base branch's commit, under `template.path`. It is detached because git does not allow a second checkout of a branch that the main checkout already holds. wk writes a marker file in the template's private git directory. It will not take over a directory that is not a template, and `wk gc` never removes one.

`wk template refresh` rebuilds it by hand. A refresh moves the template to the base commit, copies the files, runs `post_clone`, then runs the `template.build` commands with the variables from `[gates.env]` set. A lock file serializes the work. `wk new` takes it shared, so many workspaces can clone at once, and a refresh takes it exclusive, so no clone reads a tree while it is being written. `wk gc --apply` takes it exclusive only for `git worktree prune`, the one step that could race a new workspace.

## The Claude Code hooks

`wk hooks install` adds four hooks to `.claude/settings.local.json` in the main checkout and in the template, and the two entries of the lease hook while leases are on (see "Path leases" below), with the permission twins of the gates the `PreToolUse` hook wraps (see "Permission twins" below). It keeps every other key in the file in its order, refuses a file that is not a JSON object, and changes nothing when the entries are already there. It never writes `~/.claude/settings.json` or a tracked settings file. `wk hooks uninstall` removes only wk's entries and twins.

- `WorktreeCreate` runs `wk hook worktree-create`. Every worktree Claude Code makes, for a session, a desktop-app worktree session or a subagent with `isolation: "worktree"`, then comes from wk, named as Claude Code names its own (`.claude/worktrees/<name>` on `worktree-<name>`) and starting from the HEAD of the session that asked. Claude Code has no fallback when this hook fails, so wk falls back to a plain `git worktree add` whenever the warm path fails, and exits non-zero only when git cannot make a worktree either. Setting `WK_HOOK_FAIL=1` in the hook's command makes the warm path fail on purpose, to prove the fallback.
- `SubagentStop` runs `wk hook subagent-stop`, which marks a stopped subagent's own worktree for `wk gc`. It marks nothing unless the worktree at the subagent's working directory is named for that subagent, and it always exits 0, because a failing stop hook would keep the subagent running.
- `WorktreeRemove` runs `wk hook worktree-remove`. Claude Code sends it only when the owner discards a worktree explicitly; wk then removes it with `--force` and keeps its branch unless the base already holds it.
- `PreToolUse`, on the `Bash` tool only, runs `wk hook pre-tool-use || true`, which routes gate commands through `wk run --slot` (see "Cap heavy gates" below). Its timeout is 5 seconds, and it goes after any other tool's `Bash` hook, such as a commit guard. A binary older than this hook prints its usage and exits 2, and exit 2 from a `PreToolUse` hook blocks the call, so the `|| true` turns an old binary into no answer and every `Bash` call runs unchanged. The guard needs Claude Code to run the command through a shell, which it does for a hook command with no `args`.

Claude Code never removes a hook-made worktree by itself, so `wk gc` is what cleans up after agents. Every call of these four is logged in `~/.wk/<repo>/hook.log`.

## Cap heavy gates

Each type check, lint or test run peaks near 1 GiB, and a web build near 2 GiB, so ten agents that start their gates together can exhaust a 16 GB machine. `wk run --slot <class>` makes them take turns. It holds one of the class's places while its command runs, and waits in arrival order while every place is held. The places are lock files under one directory, so the cap holds across every workspace and every repository on the machine.

```bash
wk run --slot heavy --trim web/.next/cache -- bun run build
wk run --slot check -- bunx tsc --noEmit
wk slots
```

`wk run` passes the command's input, output, signals and exit status through unchanged, and prints nothing when a place is free at once. When it has to wait, it prints one line as the wait starts, naming each holder, and one as the command starts. A command killed by `SIGINT`, `SIGTERM`, `SIGHUP` or `SIGKILL` makes `wk run` die of the same signal, so a shell script stops as it would have; any other signal gives 128 plus its number. A command that is not found gives 127, one that cannot be run gives 126, and a bad `wk run` argument list exits 2 with the usage text. Each `--trim` path, relative to the git top level, is removed after the command ends with any status, while the place is still held. `--keep` skips the trim, and it never runs in the main checkout, the template or a pool slot. A `wk run` inside another one (`WK_SLOT_HELD` is set) takes no place, so a build that starts a type check cannot deadlock against itself. Every slotted run appends one line to `runs.log` in the slots directory.

`wk slots` lists each class with its capacity, the holder of each place, the waiters in queue order and the last five runs, then the count and the newest line of `missed.log` and `errors.log` from the last 24 hours. `wk slots --json` prints the same as one JSON document. It reads the places without taking one.

### The hook

`wk hook pre-tool-use` sees every `Bash` call before it runs. When the command holds a gate spelling of the repository's slots file, the hook returns the command with `<wk> run --slot <class> [--trim <path>]... -- ` inserted before that gate, and changes no other byte:

```
cd web && bunx tsc --noEmit && bun run lint && bun run build
cd web && /Users/me/go/bin/wk run --slot check -- bunx tsc --noEmit && /Users/me/go/bin/wk run --slot check -- bun run lint && /Users/me/go/bin/wk run --slot heavy --trim web/.next/cache -- bun run build
```

It parses the command as bash, with `mvdan.cc/sh/v3`, and rewrites a gate in any statement position: after `&&`, `||`, `;`, `|` or `&`, and inside `( )`, `{ }`, `if`, `while`, `until`, `for` and `case`. It leaves a gate alone inside a word (`$( )`, backticks, `<( )`, the string of `bash -c`), inside a function body and after `time`, and it leaves the whole command alone when it does not parse or already holds `wk run`. Each spelling it leaves alone goes to `missed.log` with the reason, so the list of spellings can grow from evidence. A rewritten call also gets a `Bash` timeout of 600,000 ms, Claude Code's maximum, so a gate that waits for a place is not moved to the background after the default two minutes.

The hook always exits 0. It returns nothing when there is no slots file, when nothing matches, or when its own deadline (`hook.timeout_ms`) has passed, and it logs its own failures to `errors.log`. The one exception is `hook.fail_open = false` with a slots file it cannot use: it then refuses every `Bash` call with a JSON deny, still with exit 0, until the file is fixed.

### The two config files

Both files are optional. Without `~/.wk/machine.toml`, `wk run` runs every command unslotted with one line on stderr. Without `~/.wk/<repo>.slots.toml`, the hook rewrites nothing.

`~/.wk/machine.toml` holds the classes for the whole machine:

```toml
[slots]
dir = "/Users/me/.wk/slots"
order = "fifo"
max_wait_seconds = 0
min_free_mb = 0

[slots.heavy]
capacity = 1

[slots.check]
capacity = 2
```

`~/.wk/<repo>.slots.toml` holds one repository's gate spellings. It is a file of its own, and not a table in `~/.wk/<repo>.toml`, because a binary older than this feature never reads it: an unknown table in the repository's config would make that binary refuse the whole file and fail every worktree request.

```toml
[hook]
fail_open = true
timeout_ms = 200

[[slots.match]]
spelling = "bun run build"
class = "heavy"
trim_after = ["web/.next/cache"]

[[slots.match]]
spelling = "bunx tsc"
class = "check"
```

The dials, with the values ezhomesteading uses:

| Dial | File | Value | What it does |
|---|---|---|---|
| `slots.dir` | machine | `<WK_HOME>/slots`, written out as an absolute path | Where the places, the queue and the logs live |
| `slots.order` | machine | `fifo` | `fifo` admits in arrival order; `any` takes no ticket, and any waiter may take a free place |
| `slots.max_wait_seconds` | machine | 0, no limit | Above 0, a wait of that length ends with exit 75 |
| `slots.min_free_mb` | machine | 0, off | Above 0, a waiter also waits until free, inactive and speculative memory (`vm_stat`) reaches it |
| `slots.heavy.capacity` | machine | 1 | Runs of class `heavy` at once |
| `slots.check.capacity` | machine | 2 | Runs of class `check` at once |
| `slots.match`, class `heavy` | slots file | `next build`, `bun run build`, `bun run e2e`, `playwright test`, `go run ./cmd/lintgate`, `bunx next build`, `bunx playwright test` | The spellings that take a heavy place |
| `slots.match`, class `check` | slots file | `tsc`, `bunx tsc`, `eslint`, `bun run lint`, `vitest`, `bun run test`, `go test`, `bunx eslint`, `bunx vitest` | The spellings that take a check place |
| `trim_after` | slots file | `["web/.next/cache"]` on `next build`, `bun run build` and `bunx next build` only | The `--trim` paths the hook writes |
| `hook.fail_open` | slots file | `true` | `false` refuses every `Bash` call while the slots file is broken |
| `hook.timeout_ms` | slots file | 200 | The hook's own deadline; past it the hook returns nothing |

No class name or spelling is built into wk. An unknown key in either file is an error, so a typo never drops a gate in silence. A spelling may be listed once only, a class name may hold only letters, digits, `-` and `_`, and a `trim_after` path must be relative and hold no `..`.

### Fail open

A gate must never turn red or green because of wk. Every failure of the machinery therefore runs the command unslotted, with one line on stderr that starts `wk run: running unslotted:` and one line in `errors.log`: a broken machine file, an unknown class, a slots directory wk cannot write, or any error in the queue. The hook fails open the same way. An error, a panic, a slow `git` or a broken slots file with `fail_open = true` leaves the command as the agent wrote it. The `|| true` in the settings entry covers the last case, a binary that has no `pre-tool-use` verb at all.

### Permission twins

Claude Code matches its permission rules against the command the hook returns, so `Bash(bun *)` does not cover `/Users/me/go/bin/wk run --slot heavy -- bun run build`. A wrapped gate would lose its allow rule, and a deny rule would stop refusing it with no sign. So `wk hooks install` writes a wrapped twin of every `Bash` allow, ask and deny rule that covers a spelling, into the original's own list. It reads the rules from `~/.claude/settings.json`, the tracked `.claude/settings.json` of the main checkout and the file it writes, and writes twins only into that local file. `Bash(bun *)` gains one twin for each prefix the hook can put before a `bun` gate:

```
Bash(/Users/me/go/bin/wk run --slot heavy --trim web/.next/cache -- bun *)
Bash(/Users/me/go/bin/wk run --slot heavy -- bun *)
Bash(/Users/me/go/bin/wk run --slot check -- bun *)
```

A twin allows exactly what its original allows, because `wk run` runs the words after `--` and nothing else. The twins come from the files as they are at install time, so run `wk hooks install` again after you change the slots file or a permission rule: until then, a new spelling is wrapped without its twins. A twin whose original rule is gone is removed at the next install, and `wk hooks uninstall` removes every rule that starts with `Bash(<absolute path>/wk run --slot `. A settings source that is not valid JSON stops the install before the file is changed.

Two limits are known. A rule that starts with `VAR=value` words, such as `Bash(GOTOOLCHAIN=local go test *)`, gets no twin, because the hook puts its prefix after those words; no gate rule had that shape on 2026-10-16. And wk owns every rule of the twin shape in the local file. A rule in that shape that wk did not compute, written by hand or by the permission prompt's option to stop asking about a wrapped command, is removed at the next `wk hooks install` with no message. Put such a rule in `~/.claude/settings.json` or the tracked `.claude/settings.json`, which wk never writes.

### Install order

Once the `PreToolUse` hook is installed, `~/go/bin/wk` must always be built from a commit that has `wk run` and `wk hook pre-tool-use`. A binary from an older commit has neither: the `|| true` keeps every `Bash` call running, but nothing is slotted, every `Bash` call still starts one wk process, and any command that calls `wk run` itself fails with exit 2. So the order is: install the binary, write the two config files, then run `wk hooks install`, which reads the slots file to write the twins. In ezhomesteading the binary with this feature is installed by board row 214, not by the commit that adds it.

## Path leases

Two agents in two worktrees can edit the same file, and the clash shows only when the branches land. Path leases stop the second edit when it is made. Each workspace claims a board row, takes a lease on the paths in that row's "Files it owns" cell, and a `PreToolUse` hook refuses every edit to a path that another workspace leases. Leases are off until `leases.enabled = true` is in the repository's config.

### The claim

```bash
wk claim 12             # lease the paths of row 12's "Files it owns" cell
wk claim --none         # claim no path, for a session that reads and does not edit
wk lease add web/x.ts   # lease one more path; a trailing / leases a directory
wk release              # drop the claim and every lease
wk status               # every claim, its row and its leases
```

Every session runs `wk claim <row>` or `wk claim --none` in its workspace before its first edit. With leases on, the hook refuses any edit from a workspace that has not claimed. `wk claim` reads the board named by `leases.board` in the main checkout and finds the row's cell by the column header "Files it owns". Each backtick token becomes a lease: an exact path, a directory (a trailing `/`, `/**` or `/**/*`), or a glob, which is expanded against the files git tracks in the workspace. Every token it cannot read (prose, a path starting with `~` or `/`, a path with `..`, a glob that matches nothing) is printed as `unread:`, and only what it read is leased, so a cell written as prose leases nothing and says so. A claim is refused when one of its paths overlaps a lease another workspace holds, and the refusal names that workspace's row and directory. Running the same claim again succeeds and prints the claim it found. A workspace holds one claim at a time, so a second row needs `wk release` first. A lease has no time limit; it ends with `wk release`, or when its worktree is removed.

The leases live in `~/.wk/<repo>/leases.json`. Each change takes an exclusive lock on `~/.wk/<repo>/leases.lock`, rewrites the file and renames it into place, so a crash leaves the old file or the new one. Readers take no lock. The file records the repository's git directory, and wk refuses a store that belongs to another repository with the same name.

### The hook

`wk hooks install` adds two `PreToolUse` entries while `leases.enabled` is true, and removes them when it is false. Both run `wk hook lease --repo <main checkout>` with a timeout of 5 seconds.

- The first matches `Edit|Write|MultiEdit|NotebookEdit` and carries `"onFailure": "block"`, so Claude Code (2.1.295 or later) refuses the edit when the hook cannot run at all: a missing binary, a crash or a timeout.
- The second matches `Bash` and ends in `|| true`, without `onFailure`, so a missing or older binary leaves `Bash` free and the binary can always be installed again.

The hook refuses an edit, with a reason that says what to do next, when the workspace has not claimed, when another workspace leases the path, when the path is in `leases.single_writer` and this workspace does not lease it, and when the path is in another worktree of the repository than the session's own. A path outside every worktree of the repository is allowed. The hook reads `leases.json` itself and runs no git, and it places each path through symbolic links before it compares it, so `/tmp/x` and `/private/tmp/x` are one path on macOS. It measured 20 ms or less over 20 runs on 2026-10-17. Each refusal is a line in `~/.wk/<repo>/hook.log`.

The guard fails closed. With leases on, a config file wk cannot read, a store wk cannot read, input it cannot parse, or a failure inside the hook refuses the edit, because an edit that passes by mistake is a silent clash, and a refusal costs one message. The hook always exits 0 and refuses by printing Claude Code's deny on standard output, so a non-zero exit never blocks a call by accident.

A worktree not made by `wk new` or by the `WorktreeCreate` hook has no `.claude/settings.local.json`, so the guard is absent in it. Add `.claude/settings.local.json` to `template.copy_files` so that `wk new` copies the hook into each workspace.

### Bash coverage is partial

The `Bash` entry walks the command's words and refuses the write spellings it recognises when they reach a leased path: the `>` redirections, `rm`, `touch`, `tee`, `mv`, `cp`, `sed -i`, `git mv`, `git rm`, the git commands that rewrite the whole worktree, and more. A command it cannot read is refused whole: any `$`, backtick, parenthesis, brace, bracket, `*`, `?`, backslash or non-ASCII byte, a subshell, a shell function, a `case`, an `xargs` with a writing verb, `patch`, `find -exec`, or a shell reading standard input. A command with no write target the walk recognises is allowed without reading the store, and so is a single `wk` command with no operator or redirection, so `wk claim` is never locked out.

The walk is a list of known spellings, so a program it does not know writes unseen: `make`, `go fmt`, `go generate`, `npm install`, `curl -o`, `unzip`, `mkdir -p`, `chown`, a script run by name (`./script.sh`, `python3 f.py`), `perl -e` and `ruby -e` programs without parentheses, and wrappers outside its table such as `flock`, `watch` or `npx`. An alias or function from the shell profile is not seen either. The `Edit`, `Write`, `MultiEdit` and `NotebookEdit` entries have no such gap.

The known over-refusals each cost one message. A command with `$` or a glob anywhere is refused even when it writes nothing leased; a `cd` makes a later relative target count in both directories; a whole-tree git command such as `git pull` or `git merge` is refused while any other workspace holds a lease; and an apostrophe in a heredoc body reads as an open quote.

### The ledger files

In a repository whose `leases.single_writer` lists `HANDOFF.md` and `PASSOFF.md`, the lead session writes them from the main checkout under a lease of its own: `wk claim --none` there once, then `wk lease add HANDOFF.md PASSOFF.md` before it writes. While the lead holds them, another workspace's edit to either file is refused, and the lead's `wk release` frees them. A builder in a worktree cannot write the main checkout's copy in any case, because the hook refuses an edit into another worktree than the session's own.

## Remove finished workspaces

`wk gc` prints one line for every worktree except the main checkout, with a verdict and every reason behind it. It changes nothing. `wk gc --apply` removes the worktrees judged removable.

A worktree is removable only when all of these hold: it is not locked; no process has its working directory inside it; nothing in it has changed for `gc.quiet_minutes` (60 by default); every commit on it is already in the base branch, either as an ancestor or because merging it would change nothing; and it has no changed paths. A kept worktree lists every test it failed, not only the first. The other verdicts are `kept, run /land-worktrees` for a worktree whose commits the base holds but whose tree has changes, `skip` for a template, a pool slot or a land-worktrees trial, and `prunable` for an entry whose directory is gone. The `/land-worktrees` skill is outside this repository.

Subagent worktrees, named `agent-a` and 16 hex digits as Claude Code names them, follow two other windows. One that carries the stop marker is removable when it is clean and has been quiet for `gc.stopped_quiet_minutes` (15), whether or not the base holds its commits, because its branch stays. One without the marker waits `gc.unmarked_agent_quiet_hours` (24), because a running subagent leaves no process in its worktree between tool calls.

`--apply` judges each candidate again at the moment of removal, because a session may have moved in since the pass began. It runs `git worktree remove` without `--force`, so git refuses a worktree with tracked changes or untracked files as a second guard. It deletes the branch with the worktree only when the branch is an ancestor of the base. A branch that landed by squash merge or cherry-pick is kept.

`wk gc --json` prints the same verdicts as one JSON document for other tools.

```bash
wk gc schedule install
wk gc schedule uninstall
```

`install` writes a launchd agent named `com.github.zach-short.wk.gc.<repo>` that runs `wk gc --apply` every `gc.apply_interval_minutes` (60 by default) and loads it. The log is `~/.wk/<repo>/gc.log`. `uninstall` unloads the agent and deletes its plist.

## See the hive

`wk hive` draws every worktree as a cell in the terminal, colored by verdict, and reads the verdicts again every 3 seconds. The arrow keys select a cell and `q` quits. `wk viz` serves the same view as a web page on `127.0.0.1:8787`, or on the address given with `--addr host:port`, and reads the live verdicts on every load. Both views read the dry-run report, so neither can remove anything.

## Configuration

The config for a repository is `$WK_HOME/<repo>.toml`. `WK_HOME` defaults to `~/.wk`, and the repository is the one that holds `--repo <absolute dir>`, or the current directory. Every path in the file is relative to the main checkout and must stay inside it. wk rejects an unknown key, so a typo never silently keeps a default. A missing file is not an error: wk uses the defaults and says so on stderr.

```toml
[template]
path = ".claude/worktrees/wk-template"
refresh_on = "bun.lock"            # rebuild the template when this file changes on the base
follow_base = true                 # a light refresh when only the base's commit moved
clone_dirs = ["node_modules"]      # cloned copy-on-write into each workspace
copy_files = [".env"]              # copied into each workspace, never printed

[[template.post_clone]]
dir = "."
run = ["bun", "install", "--frozen-lockfile"]

[[template.build]]                 # runs in the template after a full refresh
dir = "."
run = ["bun", "run", "build"]

[gates.env]                        # extra environment for the build commands
CI = "1"

[workspace]
dir = ".claude/worktrees/wk-{row}"
branch = "wk/{row}"
base = "main"
sparse = false
sparse_exclude = []                # directories, or patterns such as "docs/**/*.png"

[hook]
dir = ".claude/worktrees/{name}"
branch = "worktree-{name}"

[gc]
quiet_minutes = 60
stopped_quiet_minutes = 15
unmarked_agent_quiet_hours = 24
apply_interval_minutes = 60

[new]
max_parallel = 2

[pool]
size = 0
sparse_size = 0
dir = ".claude/worktrees/wk-pool-{slot}"

[leases]
enabled = false                    # true turns the guard on; see "Path leases"
board = "PASSOFF.md"               # the board wk claim reads, in the main checkout
single_writer = []                 # paths an edit may touch only under its own lease
bash_guard = true                  # walk Bash commands for writes
```

Both `workspace.dir` and `workspace.branch` must contain `{row}`, both `hook` paths `{name}`, and `pool.dir` `{slot}`. The values shown, except `refresh_on`, `copy_files`, the commands and `[gates.env]`, are the defaults. `wk config show` prints the effective config as TOML, with the file it read and whether that file was found. The `gc` windows, the interval and `new.max_parallel` must be at least 1, because a quiet window of zero could let `wk gc` remove a worktree that a session made a second ago. The `[new]` table also holds four proof thresholds that wk never enforces: `max_seconds_cold`, `max_mb_per_workspace`, `burst_size` and `burst_max_seconds`.

## Disk use

`du` counts a cloned directory at its full size, because it adds up file sizes and cannot see that APFS shares the blocks. Only `df` shows the real cost of a workspace. In ezhomesteading on 2026-10-16, a sparse workspace without the screenshots under `docs/incomplete` cost about 100 MB by `df`, a full one about 520 MB, nearly all of it git's checkout, and a worktree made before wk with its own install about 2 to 2.7 GB.

## Development

```bash
go build ./...
go test ./...
```

The tests build throwaway git repositories, stub launchd, never fork a pool fill or write a real settings file, and run their refusal cases from an empty directory, so no test touches a real repository or loads an agent into your session.

## Provenance

Several ideas come from Anthropic's article "Redesigning source control at Anthropic" (https://www.anthropic.com/engineering/redesigning-source-control). Most of them belong to the phases after P1:

- Merges happen away from the client. Here the planned daemon predicts and prepares every merge with `git merge-tree --write-tree`, and no workspace runs `git merge`.
- A reference moves by compare-and-swap, separately from creating the change. Here that is `git update-ref <ref> <new> <old>`, git's own compare-and-swap, and the tree is written long before the reference moves.
- No global repository lock. Predictions run at the same time and only the reference update is serialized; leases are per path, not per repository.
- The unit of work is a changeset in a write-ahead log, not a commit on a branch. Here it is a tree hash with a base and a message, appended to a JSONL queue, and the commit is made only when it lands.
- Sparse and optional checkouts. Here that is the sparse default and its patterns, and a single trial workspace that gates for every changeset in place of each workspace gating itself.

This tool departs from the article in one place on purpose. The article tests after the merge; this tool runs the gates before a change lands, once per batch, because a machine with 16 GB of memory cannot afford to find a red main branch afterwards and because the repository's own rule is that the gates are green before a change is done. The planned daemon then writes the landing commits itself, with a message the tool writes and no attribution line.

These ideas are local, not from the article:

- APFS directory clones (`clonefile(2)`) for `node_modules` and build caches, measured at about 4.3 s for the 96,147 files of ezhomesteading's root `node_modules`.
- The warm pool, which moves a finished workspace into place with `git worktree move`, and the stop marker that lets gc remove a finished subagent's worktree within minutes.
- Path leases seeded from a board row's "Files it owns" cell and enforced by a Claude Code `PreToolUse` hook, with an insert-only rule for the two ledger files (planned for P2).
- The owner's two commands, a dry run and an apply, inherited from the `/land-worktrees` skill, which also contributed the absorption test, the trial worktree and the four-way sort of dirty files.
