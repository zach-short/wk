# wk

`wk` makes warm git workspaces for parallel coding agents on one Mac. A workspace is a linked git worktree whose `node_modules` and build caches are APFS copy-on-write clones of a template's, so a new one is ready to run every gate in a few seconds and costs little real disk until something writes to it.

It is the first phase (P1) of a planned layer over git with four phases: cheap workspaces, path leases, landing changes on the server side, and one gate run per batch. Only P1 is built. The design and the build plan live in the ezhomesteading repository, under `docs/incomplete/landing-layer/`.

## Requirements

macOS on APFS (`clonefile(2)` exists nowhere else, so `wk` refuses other systems), git 2.49 or later, and Go 1.24. The repository it serves must be a git repository with a main checkout; ezhomesteading is the first and so far the only one proven.

## Install

```bash
go install github.com/zach-short/wk/cmd/wk@latest
```

From a checkout of this repository, `go install ./cmd/wk` does the same. The binary lands in `~/go/bin/wk`. Anything that runs `wk` later, such as the hook or the gc schedule, records this absolute path, and both refuse a `go run` build because the build cache deletes it.

## Configuration

Every dial lives in `~/.wk/<repo>.toml`, where `<repo>` is the name of the directory that holds the repository's common `.git`. A missing file means the generic defaults; an unknown key, an absolute path or a path that climbs out of the main checkout is an error. `WK_HOME` moves `~/.wk`. Paths are relative to the main checkout.

```toml
[template]
path = ".claude/worktrees/wk-template"
refresh_on = "bun.lock"
clone_dirs = ["node_modules", "web/node_modules", "native/node_modules", "web/.next/cache"]
copy_files = ["web/.env", ".claude/settings.local.json"]

[[template.post_clone]]
dir = "."
run = ["bun", "install", "--frozen-lockfile"]

[[template.build]]
dir = "web"
run = ["bun", "run", "build"]

[workspace]
dir = ".claude/worktrees/wk-{row}"
branch = "wk/{row}"
base = "main"
sparse_exclude = ["docs/incomplete"]

[hook]
dir = ".claude/worktrees/{name}"
branch = "worktree-{name}"

[pool]
size = 2
sparse_size = 1
dir = ".claude/worktrees/wk-pool-{slot}"

[gc]
quiet_minutes = 60
apply_interval_minutes = 60
```

The files in `copy_files` are copied from the main checkout inside the `wk` process and never printed, because several of them are secrets.

## Commands

`wk template refresh` makes or moves the template, a detached worktree at the base branch's commit, then installs and builds in it. `wk new` refreshes it on its own when the blob of `refresh_on` on the base branch has changed since the last refresh.

`wk new <row> [--sparse]` makes the workspace for a board row and prints its absolute path on stdout, with all progress on stderr. When the pool holds a ready slot of the right kind, `wk new` moves it into place, puts it on the new branch and returns. Otherwise it adds the worktree, checks the tree out while it clones every `clone_dirs` entry from the template at the same time, copies the files and runs the post-clone commands. `--sparse` leaves out `workspace.sparse_exclude`.

`wk pool fill` tops the pool up to `pool.size` full and `pool.sparse_size` sparse slots, and `wk pool status` lists them. `wk new` and the hook start a fill in the background after every workspace they make, in a session of its own, with its output in `~/.wk/<repo>/pool.log`. A slot made from an older template is never claimed; the next fill removes it and makes a new one.

`wk gc` judges every worktree but the main checkout and prints one verdict per line. A worktree is removable only when its commits are already on the base branch, it has no changes, it is not locked, no process has its working directory inside it, and nothing in it has moved for `gc.quiet_minutes`. An absorbed worktree that still has changes is kept with the note `run /land-worktrees`. The dry run changes nothing; `wk gc --apply` removes what is removable, judging each one again at the moment of removal, and never with `--force`. `wk gc --json` prints the same verdicts for tools.

`wk gc schedule install` writes and loads a launchd agent that runs `wk gc --apply` every `gc.apply_interval_minutes`, logging to `~/.wk/<repo>/gc.log`; `uninstall` removes it.

`wk viz` serves a page that draws the gc verdicts, and `wk hive` draws them in the terminal. Both read the dry run and change nothing.

## The Claude Code hook

`wk hooks install` adds one `WorktreeCreate` entry to `.claude/settings.local.json` in the main checkout and in the template. It keeps every other key in the file in its order, refuses a file that is not a JSON object or that already has another tool's `WorktreeCreate` hook, and changes nothing when the entry is already there. It never writes `~/.claude/settings.json` or a tracked settings file. `wk hooks uninstall` removes only that entry.

With the hook installed, every worktree Claude Code makes for a session or for a subagent with `isolation: "worktree"` comes from `wk hook worktree-create`. It reads Claude Code's JSON input, makes a warm workspace named as Claude Code names its own (`.claude/worktrees/<name>` on `worktree-<name>`), starting from the HEAD of the session that asked, and prints the path. Claude Code has no fallback of its own when a `WorktreeCreate` hook fails, so the hook falls back to a plain `git worktree add` whenever the warm path fails, and exits non-zero only when git cannot make a worktree either. Every call is logged in `~/.wk/<repo>/hook.log`. Setting `WK_HOOK_FAIL=1` in the hook's command makes the warm path fail on purpose, to prove the fallback.

No `WorktreeRemove` hook is installed, so Claude Code keeps the worktrees the hook makes, and `wk gc` collects them under its own tests.

## Disk use

`du` counts a cloned directory at its full size, because it adds up file sizes and cannot see that APFS shares the blocks. Only `df` shows the real cost of a workspace: about 520 MB for a full one in ezhomesteading against several GB by `du`, nearly all of it git's checkout.

## Self-test

```bash
test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...
```

The tests build throwaway repositories shaped like a bun monorepo under a temporary directory, with git isolated from the owner's configuration, and never fork a pool fill, load a launchd agent or write a real settings file.

## Provenance

Several ideas come from Anthropic's article "Redesigning source control at Anthropic" (https://www.anthropic.com/engineering/redesigning-source-control). Most of them belong to the phases after P1:

- Merges happen away from the client. Here the planned daemon predicts and prepares every merge with `git merge-tree --write-tree`, and no workspace runs `git merge`.
- A reference moves by compare-and-swap, separately from creating the change. Here that is `git update-ref <ref> <new> <old>`, git's own compare-and-swap, and the tree is written long before the reference moves.
- No global repository lock. Predictions run at the same time and only the reference update is serialized; leases are per path, not per repository.
- The unit of work is a changeset in a write-ahead log, not a commit on a branch. Here it is a tree hash with a base and a message, appended to a JSONL queue, and the commit is made only when it lands.
- Sparse and optional checkouts. Here that is `wk new --sparse`, and a single trial workspace that gates for every changeset in place of each workspace gating itself.

This tool departs from the article in one place on purpose. The article tests after the merge; this tool runs the gates before a change lands, once per batch, because a machine with 16 GB of memory cannot afford to find a red main branch afterwards and because the repository's own rule is that the gates are green before a change is done. The planned daemon then writes the landing commits itself, with a message the tool writes and no attribution line.

These ideas are local, not from the article:

- APFS directory clones (`clonefile(2)`) for `node_modules` and build caches, measured at about 4.3 s for the 96,147 files of ezhomesteading's root `node_modules`.
- The warm pool, which moves a finished workspace into place with `git worktree move` so a new one is ready in well under the time a clone takes.
- Path leases seeded from a board row's "Files it owns" cell and enforced by a Claude Code `PreToolUse` hook, with an insert-only rule for the two ledger files (planned for P2).
- The owner's two commands, a dry run and an apply, inherited from the `/land-worktrees` skill, which also contributed the absorption test, the trial worktree and the four-way sort of dirty files.
