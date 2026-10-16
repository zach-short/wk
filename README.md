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

`wk hooks install` adds three hooks to `.claude/settings.local.json` in the main checkout and in the template. It keeps every other key in the file in its order, refuses a file that is not a JSON object, and changes nothing when the entries are already there. It never writes `~/.claude/settings.json` or a tracked settings file. `wk hooks uninstall` removes only wk's entries.

- `WorktreeCreate` runs `wk hook worktree-create`. Every worktree Claude Code makes, for a session, a desktop-app worktree session or a subagent with `isolation: "worktree"`, then comes from wk, named as Claude Code names its own (`.claude/worktrees/<name>` on `worktree-<name>`) and starting from the HEAD of the session that asked. Claude Code has no fallback when this hook fails, so wk falls back to a plain `git worktree add` whenever the warm path fails, and exits non-zero only when git cannot make a worktree either. Setting `WK_HOOK_FAIL=1` in the hook's command makes the warm path fail on purpose, to prove the fallback.
- `SubagentStop` runs `wk hook subagent-stop`, which marks a stopped subagent's own worktree for `wk gc`. It marks nothing unless the worktree at the subagent's working directory is named for that subagent, and it always exits 0, because a failing stop hook would keep the subagent running.
- `WorktreeRemove` runs `wk hook worktree-remove`. Claude Code sends it only when the owner discards a worktree explicitly; wk then removes it with `--force` and keeps its branch unless the base already holds it.

Claude Code never removes a hook-made worktree by itself, so `wk gc` is what cleans up after agents. Every call is logged in `~/.wk/<repo>/hook.log`.

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
```

Both `workspace.dir` and `workspace.branch` must contain `{row}`, both `hook` paths `{name}`, and `pool.dir` `{slot}`. The values shown, except `refresh_on`, `copy_files`, the commands and `[gates.env]`, are the defaults. The `gc` windows, the interval and `new.max_parallel` must be at least 1, because a quiet window of zero could let `wk gc` remove a worktree that a session made a second ago. The `[new]` table also holds four proof thresholds that wk never enforces: `max_seconds_cold`, `max_mb_per_workspace`, `burst_size` and `burst_max_seconds`.

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
