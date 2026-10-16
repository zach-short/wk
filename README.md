# wk

wk makes warm git workspaces for parallel agents. A workspace is a git worktree whose dependency trees, such as `node_modules`, are already in place. wk does not install them again. It copies them from a template worktree with copy-on-write clones, so a new workspace costs almost no disk and little time. wk also judges which worktrees are finished and removes them.

## Requirements

wk needs Go 1.24 or later to build and git to run. `wk new` needs macOS on APFS, because the copy-on-write step uses `clonefile(2)`. On any other system the clone step fails with an error that says so, since a byte copy of `node_modules` would cost the disk that wk exists to save. The `gc schedule` command writes a launchd agent, so it is macOS only as well. `wk gc` finds busy worktrees with `lsof`.

## Install

```bash
git clone https://github.com/zach-short/wk.git
cd wk
go install ./cmd/wk
```

The scheduled cleanup runs the installed binary, so install it this way and not with `go run`.

## Set up a repository

Run `wk` with no arguments inside a repository, in a terminal. The first time, it opens a guided setup. It reads the repository for package manifests, lockfiles, `.env` files and local branches, then asks which to use. It looks at file names only, so a `.env` file is listed and never opened. After the review step it writes the config to `~/.wk/<repo>.toml`. Two last choices are to build the template at once and, on macOS, to install the scheduled cleanup.

Setup never overwrites a config, because a config can hold hand-written notes. Once the file exists, a bare `wk` opens a home screen that shows the settings in use and offers to open the hive. Piped or run without a terminal, a bare `wk` prints the usage text.

## Make a workspace

```bash
cd "$(wk new 12)"
```

`wk new <row>` creates the workspace for row 12 and prints its absolute path on stdout. Progress and timings go to stderr, so the command substitution above captures only the path. A row is a positive integer of up to nine digits. By default it gives the directory `.claude/worktrees/wk-12` inside the main checkout and the branch `wk/12`, both cut from the base branch. The command refuses to run if that directory or branch already exists.

Before it makes anything, `wk new` compares the template with the base branch. If the file named by `template.refresh_on` has changed since the template was built, or the template is missing, wk refreshes the template first. wk gives up after two refreshes, so a lockfile that changes every minute cannot hold the command forever.

Then wk adds the worktree without a checkout. It checks out the tree and clones every directory in `template.clone_dirs` at the same time, copies the files in `template.copy_files`, and runs the `template.post_clone` commands. If any step fails, the workspace stays on disk for inspection and the error says where.

`wk new <row> --sparse` checks out a sparse tree that leaves out the paths in `workspace.sparse_exclude`. Setting `workspace.sparse = true` makes that the default.

## The template

The template is a detached worktree at the base branch's commit, under `template.path`. It is detached because git does not allow a second checkout of a branch that the main checkout already holds. wk writes a marker file in the template's private git directory. It will not take over a directory that is not a template, and `wk gc` never removes one.

`wk template refresh` rebuilds it by hand. A refresh moves the template to the base commit, copies the files, runs `post_clone`, then runs the `template.build` commands with the variables from `[gates.env]` set. A lock file serializes the work. `wk new` takes it shared, so many workspaces can clone at once, and a refresh or `wk gc --apply` takes it exclusive, so no clone reads a tree while it is being written.

## Remove finished workspaces

`wk gc` prints one line for every worktree except the main checkout, with a verdict and every reason behind it. It changes nothing. `wk gc --apply` removes the worktrees judged removable.

A worktree is removable only when all of these hold: it is not locked; no process has its working directory inside it; nothing in it has changed for `gc.quiet_minutes` (60 by default); every commit on it is already in the base branch, either as an ancestor or because merging it would change nothing; and it has no changed paths. A kept worktree lists every test it failed, not only the first. The other verdicts are `kept, run /land-worktrees` for a worktree whose commits the base holds but whose tree has changes, `skip` for a template or a land-worktrees trial, and `prunable` for an entry whose directory is gone. The `/land-worktrees` skill is outside this repository.

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
clone_dirs = ["node_modules"]      # cloned copy-on-write into each workspace
copy_files = [".env"]              # copied into each workspace, never printed

[[template.post_clone]]
dir = "."
run = ["bun", "install", "--frozen-lockfile"]

[[template.build]]                 # runs in the template after a refresh
dir = "."
run = ["bun", "run", "build"]

[gates.env]                        # extra environment for the build commands
CI = "1"

[workspace]
dir = ".claude/worktrees/wk-{row}"
branch = "wk/{row}"
base = "main"
sparse = false
sparse_exclude = []

[gc]
quiet_minutes = 60
apply_interval_minutes = 60
```

Both `workspace.dir` and `workspace.branch` must contain `{row}`. The values shown for `path`, `clone_dirs`, `dir`, `branch`, `base`, `sparse` and the two `gc` dials are the defaults. `gc.quiet_minutes` and `gc.apply_interval_minutes` must be at least 1, because a quiet window of zero could let `wk gc` remove a worktree that a session made a second ago.

## Development

```bash
go build ./...
go test ./...
```

The tests build throwaway git repositories and stub launchd, so no test loads an agent into your session.
