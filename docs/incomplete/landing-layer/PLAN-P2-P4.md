# Plan: landing layer, P2 path leases, P3 the landing queue, P4 one gate run per batch

Status: PLANNED (Stage 4), written 2026-10-16 by a Fable 5.1 session in the worktree `ap-row-2` of `~/Projects/wk` at `a4d4ed2` (wk board row 2). The design is `DESIGN.md` in this folder, frozen at D1 to D26; D22 to D26 were ratified on 2026-10-16 in this session and carry the P1 amendments and the one-file config decision. The P1 plan and its as-built notes are `PLAN.md`; its decisions are BD-1 to BD-23 and are not repeated here. This plan records build-level calls as PD-1 to PD-n. Model check: the row names Fable 5.1 and this session runs Fable 5.1.

Shape, by Zach's instruction of 2026-10-16 ("divide up the work for each item to more builders"): each phase is two or three sub-rows that build at the same time in separate worktrees on disjoint files, plus one integration sub-row that runs last, owns the shared wiring (`cmd/wk/main.go`, `internal/config/config.go`, `internal/wk/*`), and carries the phase's live proof. A sub-row waits only on the phase before it, never on a sibling. Each phase is as small as its section 8 proof allows; what was cut is under "Not built, and why" with its reversal.

## 0. Facts verified 2026-10-16

Read between 20:52 and 21:10 EDT in the worktree `ap-row-2` at `a4d4ed2`, which is `main`.

| Claim | Verified state | Citation |
|---|---|---|
| Toolchain | go1.24.1 darwin/arm64; `go 1.24.0` directive; git 2.49.0, which has `merge-tree --write-tree --merge-base`, `--name-only` and `update-ref <ref> <new> <old>`; `golangci-lint` on PATH; Claude Code 2.1.301 | `go version`; `go.mod:3`; `git --version`; `DESIGN.md` 1.1; `claude --version` |
| Dependencies | `BurntSushi/toml` v1.6.0, `bubbletea` v1.3.10, `lipgloss` v1.1.0, `x/sys` v0.41.0; no shell parser | `go.mod` |
| Baseline gate | Green: `gofmt -l .` empty, `go vet` clean, `go test -race -count=1 ./...` ok for `cmd/wk`, `internal/config`, `internal/hive`, `internal/procs`, `internal/settings`, `internal/setup`, `internal/viz`, `internal/wk` (39.7 s); `golangci-lint run ./...` prints `0 issues.`; exit 0 | run at 20:55 EDT in `ap-row-2` |
| Config loader | `config.Load(path)` reads `~/.wk/<repo>.toml` over `Defaults()`; a missing file returns the defaults with `found` false; an unknown key is an error; `validate()` refuses absolute paths, `..`, empty commands and missing placeholders. Sections: `template`, `gates` (only `env`), `workspace`, `hook`, `gc`, `new`, `pool` | `internal/config/config.go:98-235` |
| The live ezhomesteading file | `~/.wk/ezhomesteading.toml` holds exactly those seven sections; `~/.wk/ezhomesteading/` is the state directory; no `~/.wk/wk.toml` and no `~/.wk/machine.toml` exist yet | `ls ~/.wk`; `grep '^\[' ~/.wk/ezhomesteading.toml` |
| `Env` | `wk.Open(home, dir, log)` resolves the repository, loads the config and makes `State = <home>/<repo>`; `Env.Refill` is set by the command line only; `Env.create(dir, branch, commit, sparse)` is the one workspace maker, used by `New` and the hook | `internal/wk/env.go:19-50`; `internal/wk/new.go:44-78` |
| Repository resolution | `gitx.Resolve(dir)` runs `rev-parse --git-common-dir` and `worktree list --porcelain`; `Repo{Primary, CommonDir, Name}`, `Name` is the base name of the common directory's parent. `gitx.Run` sets `GIT_OPTIONAL_LOCKS=0` on every call | `internal/gitx/gitx.go:21-65` |
| git helpers that exist | `Worktrees`, `BranchExists`, `Commit`, `Blob`, `DirtyPaths`, `IsAncestor`, `IsAbsorbed` (uses `merge-tree --write-tree`), `GitDir` | `internal/gitx/gitx.go:81-202` |
| Locks | `flock.TryTake` and `flock.Take` on a file; a lock dies with its holder; `takeCreateSlot` polls `create-<n>.lock` every 100 ms | `internal/flock/flock.go:14-36`; `internal/wk/new.go:121-138` |
| Hook registration | `settings.Entries` holds three entries (`WorktreeCreate` sole, `WorktreeRemove` sole, `SubagentStop`), each `{Event, Verb, TimeoutSeconds, Sole}`; `Install` merges them into a settings file and keeps every other key; `IsWK` recognises wk's commands; `Env.HooksInstall` writes the main checkout's and the template's `.claude/settings.local.json` and never `~/.claude/settings.json` | `internal/settings/settings.go:21-60`; `internal/wk/hooks.go:33-67` |
| Hook commands | `wk hook worktree-create` prints one path on stdout and exits 2 on failure; `wk hook subagent-stop` always exits 0; `stdin` is a package variable the self-test swaps; `installedExe()` refuses a `go run` build | `cmd/wk/main.go:66-124`, `:148-160`, `:243` |
| Detached process pattern | `spawnFill` starts `wk --repo <primary> pool fill` from `/` in its own session with its output in `pool.log`; a test binary never forks | `cmd/wk/main.go:188-212` |
| gc markers | gc skips the template (`wk-template`), pool slots (`wk-pool`) and any worktree whose private git directory holds `land-worktrees-trial` | `internal/wk/gc.go:19-21`; `internal/wk/template.go:18`; `internal/wk/pool.go:23` |
| The gc schedule | A launchd agent `com.github.zach-short.wk.gc.ezhomesteading` is installed and runs `wk gc --apply` every 15 minutes; P2 does not replace it (PD-5) | `ls ~/Library/LaunchAgents`; `internal/wk/schedule.go:22` |
| Self-test fixture | `testrepo.New(t)` makes an isolated throwaway repository with a fake install and a config; `Isolate()` sets `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1` | `internal/testrepo/testrepo.go:15-120` |
| Row 200 (ezhomesteading, IN FLIGHT) | Owns in wk: `cmd/wk/main.go`, a new `internal/slot`, `internal/config`, `internal/settings`, `README.md`, and `~/.wk/machine.toml`. Its plan adds `Entry.Matcher`, a `PreToolUse` entry on `Bash` that rewrites gate commands through `wk run --slot` and fails open, `mvdan.cc/sh/v3` as a dependency, and the repository's slot keys in `~/.wk/<repo>.slots.toml` (SB-4) | `~/Projects/ezhomesteading/passoff/200-*.md`, the row; `SLOTS-PLAN.md` SB-2, SB-4, SB-14, phase 2 step 4 |
| `updatedInput` from a project's `.claude/settings.local.json` reaches a subagent's `Bash` call, and a hook on `Bash` sees the original command | Verified by row 200's planning session on 2.1.301 and 2.1.299 with headless `claude -p` | `SLOTS-PLAN.md` section 0.2 |
| The commit rule | `~/.claude/rules/commits.md` reads "Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`" and forbids only the trailer and the "Generated with" line. D14's condition is met; P3 waits on nothing but P2 | `~/.claude/rules/commits.md:4`; `DESIGN.md` D14, tag of 2026-10-16 |
| wk's own commit policy | `agent-pushes`: a row lands on `main` and is pushed inside the run | `~/Projects/wk/CLAUDE.md`, "Never do this"; the autopilot brief of row 2 |
| The hook deny shape | `DESIGN.md` 1.4 gives two: exit 2 with the reason on stderr, or JSON `hookSpecificOutput.permissionDecision: "deny"`. Not re-verified here; row 3 step 1 verifies the field names on the installed version before the hook is written | `DESIGN.md` 1.4 |

## 1. Decisions taken in this plan

Each has a one-line reversal. A build row that deviates cites the number.

- **PD-1. One socket per repository, under the state directory.** `~/.wk/<repo>/wk.sock`, beside the locks and logs P1 already keeps there, with a single-instance lock `~/.wk/<repo>/daemon.lock`. `DESIGN.md` section 5 named `~/.wk/<repo-hash>.sock`; the name is simpler and the macOS 104-byte socket path limit is checked at start. Reversal: the hashed name.
- **PD-2. The hook makes no git call.** `wk hooks install` bakes `--repo <primary>` into the lease hook's command, the hook derives the socket from it, and the daemon resolves which worktree holds a path from its own worktree list. A hook that runs on every edit of every agent must cost one connect and one round trip. Reversal: `gitx.Resolve` in the hook, two git runs per edit.
- **PD-3. Leases are stored in the daemon and saved to `~/.wk/<repo>/leases.json`** by atomic rename after every change, loaded at start (option M). Reversal: none needed.
- **PD-4. The `Bash` guard walks words without a shell parser.** D11 says "as the commit guard does"; a quote-aware word split with `cd` tracking and the recognised spellings of section 2 needs no new dependency, so sub-row 7 does not touch `go.mod` while row 200 adds `mvdan.cc/sh/v3`. Reversal: parse with `mvdan.cc/sh/v3/syntax` once row 200 is on `main`.
- **PD-5. The launchd gc agent stays.** `PLAN.md` section 4 said P2's daemon takes the schedule over; it works and is measured, and a ticker in the daemon saves nothing. Reversal: a ticker calling `GCApply`.
- **PD-6. P3 lands ungated onto `wk/landing`; `main` moves only by `wk land --apply` until P4.** D14 fast-forwards `main` after a green P4 gate, so before P4 the automatic path stops at the landing ref and the manual override is the only way `main` moves. P4 moves the land step behind the gate. Reversal: none; this is D14 read in order.
- **PD-7. A changeset's base is `merge-base HEAD <landing tip>`**, computed at submit time, so a workspace whose branch carries commits (today's `agent-pushes` practice) still submits one tree. `wk sync` refuses a workspace whose HEAD is not an ancestor of the tip, because it moves the branch ref and keeps no commits. Reversal: base is HEAD and `wk submit` refuses a branch with commits.
- **PD-8. The queue is one JSONL file, `~/.wk/<repo>/queue.jsonl`, written only by the daemon.** Every state change is one appended line (`submitted`, `refused`, `landed`; P4 adds `gated`, `failed`); the current state of a changeset is its last line. Not a dial. Reversal: a state file rewritten in place.
- **PD-9. The trial workspace reuses the `land-worktrees-trial` marker**, so `wk gc` already skips it and `/land-worktrees` treats it as a trial. Reversal: a `wk-trial` marker and one more gc skip.
- **PD-10. Machine limits for gates come from row 200's slots, not from P4.** The trial's gate commands run through `wk run --slot` by the same matcher the hook uses, so one `next build` at a time and the memory floor are row 200's. P4 keeps only `gates.min_free_mb`, the pause before each gate, because section 8's proof names it. Reversal: `gates.parallel_tsc` and `gates.next_build_concurrency` as dials.
- **PD-11. No gates configured means no fast-forward.** A repository with `[landing] ff_main = true` and no `[[gates.run]]` lands onto `wk/landing` and `wk status` says why `main` did not move. Reversal: treat an empty gate list as green.
- **PD-12. P2 starts from whatever `main` holds when it launches.** Row 200 builds `internal/config` and `cmd/wk/main.go` at the same time; P2's config keys are additive and its subcommands are new names, so the merge is mechanical. Where row 200's `Entry.Matcher` exists, P2 uses it; where it does not yet, row 3 adds the field in the same shape (`SLOTS-PLAN.md` phase 2 step 4). Reversal: none.

## 2. P2, path leases

Scope items, each citing its source:

1. The daemon: `wk up` starts it detached (the `spawnFill` pattern), `wk down` stops it, any lease command starts it on demand when the socket is missing, the hook never does (D12; options M, Z; PD-1 to PD-3).
2. `wk claim <row>`: reads the board (`leases.board`), takes row `<row>`'s "Files it owns" cell, parses backtick tokens into exact paths, directory prefixes (a trailing `/` or a `**` pattern) and globs expanded against `git ls-files` of the workspace, prints every token it could not read, and takes leases only for what it read; refuses when a lease is held elsewhere, naming the holder's row and workspace (D10, option K, G3). `wk claim --none` claims no paths. `wk lease add <path>...` and `wk release` (option K).
3. The hook, `wk hook lease --repo <primary>`, on `PreToolUse` for `Edit|Write|MultiEdit|NotebookEdit` and for `Bash` (D11, option I2): resolves the path against `cwd`, asks the daemon, and denies with the holder named when another workspace leases it; denies every edit from a workspace that has not claimed (D10); denies an edit to a `leases.single_writer` path the workspace does not lease (D13 L1); denies everything with "start the daemon: wk up" when the socket does not answer (D12). A path outside every worktree of the repository is allowed. The daemon keys holders by worktree root (option H).
4. `wk config show`: the effective config as TOML, the file path, and whether it was found (D26).
5. The `[leases]` section (D26), additive to the P1 file; `wk hooks install` adds the two `PreToolUse` entries (D6, BD-8).

The `[leases]` keys, all with safe defaults when absent:

| Key | Default | Meaning | Source |
|---|---|---|---|
| `leases.enabled` | `false` | `true` turns the guard on: every edit needs a claim (G3) and the hook fails closed (D12). Absent means no leases, and the hook allows with one log line | D10, D12, D26 |
| `leases.board` | `PASSOFF.md` | the board file `wk claim` reads, relative to the main checkout | D10; D13 note on the board layout |
| `leases.single_writer` | `[]` | paths an edit may touch only under this workspace's lease; ezhomesteading lists `backend/routes/routes.go`, `backend/main.go`, `HANDOFF.md`, `PASSOFF.md` | D13 L1 |
| `leases.bash_guard` | `true` | the `Bash` word walk is on | D11 |

Sub-rows (wk board rows 6, 7, 8 build at once; row 3 integrates):

| Row | Owns | Builds | Model, review |
|---|---|---|---|
| 6 | `internal/daemon/` (new: `daemon.go`, `client.go`, `spawn.go`, `state.go`, `daemon_test.go`) | the unix-socket server and client, newline-delimited JSON, one request per line, `{"op": ...}` in and `{"ok": bool, "error": ..., "data": ...}` out; built-in ops `ping` and `shutdown`; handlers registered by name; single instance by exclusive flock, a stale socket file removed only after the lock is held; the 104-byte path check; `Spawn(exe, args, log)` detached from `/`; `SaveJSON` by atomic rename and `LoadJSON` | Opus 5.5, Opus 5.5 review |
| 7 | `internal/lease/` (new: `store.go`, `judge.go`, `bash.go`, tests) | the store: workspaces keyed by root, each with row, session, leases, claimed time; `Claim`, `Release`, `Add`, `Holder(path)`; the conflict rule (an exact path or a directory prefix; two leases overlap when one is a prefix of the other); `Resolve(roots, cwd, path)` by longest prefix; `Judge(req) Decision` with the four deny reasons of scope item 3, each naming what to do next; the `Bash` walk: `sed -i`, `tee`, `>` and `>>`, `cp`, `mv`, `rm`, `touch`, `git mv`, `git rm`, `python -c` with `open(` and a `w` mode, with leading `VAR=value` words skipped and `cd <dir>` at the top of a `&&` or `;` list tracked, failing closed on what it recognises and passing what it does not; JSON round trip of the store | Opus 5.5, Opus 5.5 review |
| 8 | `internal/board/` (new: `board.go`, `board_test.go`) | `Cell(text, row)`: the last cell of the table line whose first cell is `row`; `Parse(cell)`: backtick tokens become paths, a trailing `/` or `**` becomes a directory prefix, a token with `*`, `?` or `[` is a glob, everything else (prose, `~`, absolute paths, "to" ranges) is returned as unread; `Expand(globs, files)` against a tracked-file list with `path.Match` per segment | Opus 5.5, Opus 5.5 review |
| 3 | `cmd/wk/main.go`, `internal/config/config.go`, `internal/settings/settings.go` (sequential overlap with row 200), `internal/wk/lease.go`, `internal/wk/leasehook.go`, `internal/wk/daemon.go`, `README.md`, `~/.wk/wk.toml`, `~/.wk/ezhomesteading.toml` | wires 6, 7 and 8: the lease ops on the daemon, the subcommands, the hook verb, the config section, `wk config show`, the settings entries, the README section, the live proof | Opus 5.5, Opus 5.5 review |

Done when, for each sub-row: its package tests green plus the gate line. For row 3, the gate line plus the proof of `DESIGN.md` section 8, P2, recorded in this file under "As built, P2":

- In a throwaway repository with a fixture board, two headless sessions (`claude -p`, each in its own worktree, the technique of `SLOTS-PLAN.md` 0.2) claim rows 1 and 2; an `Edit` by session 2 to a path session 1 leases is refused with session 1's row and workspace named; a `sed -i` to the same path is refused the same way; after `wk down` every edit is refused with "start the daemon: wk up".
- The hook's wall time, measured with `/usr/bin/time -p` over 20 runs fed the `Edit` input, is under 50 ms.
- `wk claim` on the live wk board (`PASSOFF.md` of this repository) prints the unread tokens of every row's cell, and reads every backtick path.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`

Review: Opus 5.5 on every P2 row, by Zach's answer of 2026-10-16 ("opus review of 3,5"). Recommendation he has not taken: a Fable 5.1 review of `internal/lease/bash.go` and `judge.go` alone, because a pass that should be a refusal is quiet until landing (D17, option X).

Not built, and why:

- D13 L3, insert-only edits on the ledger files with an exception for the claimed row. Section 8's proof needs L1 only; in this repository `HANDOFF.md` and `PASSOFF.md` are untracked and written by one orchestrating session, and the board is now a table plus per-item files, so the exception would parse a layout that is still moving. The ledger files are `single_writer` instead, which serialises two sessions that both want to append. Reversal: an `insert_only` list and the check that `new_string` contains `old_string`. Owner's call, asked in row 3.
- `lease.ttl_minutes`: D10 fixes it at 0, so it is behaviour, not a dial. Reversal: the key.
- `lease.fail_closed`: D12 fixes it; a key to fail open is the silent failure. Reversal: the key.
- `hook.timeout_ms` 200: Claude Code's hook timeout is whole seconds; the entries carry `timeout: 5` and the hook is measured under 50 ms. Reversal: none.
- The daemon taking over the gc schedule (PD-5).
- Hooks reaching a worktree that was not made by wk or by the desktop app: those carry no `.claude/settings.local.json`, so the guard is absent there; the README says so.

## 3. P3, the landing queue

Scope items:

1. `wk submit -m "<message>"` in a workspace: builds the tree through a private index (`GIT_INDEX_FILE`: `read-tree HEAD`, `add -A`, `write-tree`), so untracked non-ignored files are in and the workspace's own index is untouched; base by PD-7; refuses the main checkout (D15); sends `{tree, base, message, row, workspace}` to the daemon and prints the changeset id (option O). The row comes from the P2 claim; `--row <n>` overrides; with none the message carries no prefix. The message is `row <n>: <message>` in lowercase, no trailer, the author from `git config` (option Q).
2. The landing loop in the daemon: for each submitted changeset in queue order, `merge-tree --write-tree --name-only --merge-base=<base> <tip> <tree>`; on a clean result, `commit-tree <result> -p <tip> -m <message>` and `update-ref <landing.ref> <new> <tip>` (the CAS), then `landed`; on conflicts, `refused` with the paths (option P, D14). The landing ref is created at `main` when missing; when `main` has moved ahead of it the ref is moved to `main` first; when the two diverged the loop stops and `wk status` says what to do (merge by hand, or reset the ref). The workspace's P2 leases are released on `landed` (option K).
3. `wk sync`: PD-7's refusal, then merge-tree of the workspace tree against the tip, `read-tree -u --reset` of the result, `update-ref refs/heads/<branch> <tip> <old>`, `reset -q`; conflict markers stay in the working tree and nothing is committed (option R).
4. `wk land` prints the landing tip, `main`, whether a fast-forward is possible and which dirty files in the main checkout block it (the `land.sh` blockers, section 6); `wk land --apply --expect <landing commit>` runs `merge --ff-only --no-autostash` in the main checkout and, with `landing.push`, `push <remote> main`. In P3 this is the only way `main` moves (PD-6).
5. `wk status`: every changeset with its state and reason (section 3 of the design; P4 extends it).
6. The `[landing]` section (D26).

The `[landing]` keys:

| Key | Default | Meaning | Source |
|---|---|---|---|
| `landing.enabled` | `false` | `true` lets `wk submit` and the loop run; absent means no landing | D26 |
| `landing.ref` | `refs/heads/wk/landing` | the ref the daemon moves by CAS | section 5 |
| `landing.remote` | `origin` | the remote `wk land --apply` pushes to | D20 |
| `landing.push` | `true` | push after a fast-forward of `main` | D20 |

`landing.ff_main` is a P4 key: nothing in P3 fast-forwards `main` by itself.

Sub-rows (rows 9 and 10 build at once; row 4 integrates). Every P3 row is Fable 5.1 with a Fable 5.1 review (D17): a wrong tree on the landing ref compiles, passes every gate and is wrong.

| Row | Owns | Builds | Model, review |
|---|---|---|---|
| 9 | `internal/changeset/` (new: `tree.go`, `queue.go`, tests) | `Tree(workspace) (tree string, err)` through a private index that is removed afterwards; `Base(workspace, tip)` by PD-7 with a refusal when there is no merge base; `Queue` at a path: `Append(record)`, `Load() []Record` folded to the last line per id, ids from the first 12 hex of `sha1(tree + base + time)`; states `submitted`, `refused`, `landed`, `gated`, `failed`; a test in a `testrepo` fixture shows an untracked file is in the tree, an ignored file is not, and the workspace's index is byte-identical before and after | Fable 5.1, Fable 5.1 review |
| 10 | `internal/landing/` (new: `predict.go`, `land.go`, `sync.go`, `ff.go`, tests, and a `git` shim script the tests put first on `PATH`) | `Predict(repo, base, tip, tree) (result string, conflicts []string, err)`; `Land(repo, ref, tip, result, message) (commit string, err)` by `commit-tree` and the CAS `update-ref`, refusing when the ref moved; `Ensure(repo, ref, main)` for scope item 2's three cases; `Sync(workspace, base, tip)` as scope item 3; `FastForward(primary, branch, to, expect) (blockers []string, err)` and `Push(primary, remote, branch)`. The shim test: every git invocation of the package is logged, and the log shows that the only ref writes are `update-ref <ref> <new> <old>` on the landing ref and `update-ref refs/heads/<branch>` in `Sync`, that `commit-tree` is given exactly the predicted tree, and that nothing but `FastForward` names `main` | Fable 5.1, Fable 5.1 review |
| 4 | `cmd/wk/main.go`, `internal/config/config.go`, `internal/wk/landing.go`, `internal/wk/submit.go`, `internal/wk/status.go`, `README.md`, `~/.wk/wk.toml`, `~/.wk/ezhomesteading.toml` | wires 9 and 10: the `submit` and `status` ops, the loop as a goroutine of the daemon, the subcommands, the config section, lease release on `landed`, the README section, the live proof | Fable 5.1, Fable 5.1 review |

Done when, for each sub-row: its package tests green plus the gate line. For row 4, the gate line plus the proof of `DESIGN.md` section 8, P3, as D14 and D20 amend it, recorded under "As built, P3":

- In a throwaway repository, two workspaces submit overlapping changesets; the first lands, the second is `refused` with the conflicting path named, `wk sync` in the second workspace brings it to the tip with the conflict in the working tree, and a resubmit after the fix lands.
- The landing commit's tree equals the predicted tree byte for byte (`rev-parse <commit>^{tree}` against the `Predict` result), for every landing in the proof.
- The shim log of row 10, run again over the daemon's whole loop, shows no ref write but the CAS on the landing ref, and no write to `refs/heads/main`.
- `wk land` prints the plan; `wk land --apply --expect <hash>` moves `main` by fast-forward and pushes it in the throwaway repository with a local bare remote; `--expect` with a stale hash is refused.
- In this repository: one real changeset from a workspace lands on `wk/landing`, and `wk land --apply` fast-forwards `main` and pushes to `origin`.

Gate: the P2 gate line.

Not built, and why:

- A `session_id` on a changeset (option O): nothing reads it. Reversal: one field.
- `queue.path` as a dial (PD-8).
- Predictions run concurrently (section 9): the queue lands in order on one ref, so one goroutine is the whole concurrency. Reversal: none needed.
- `wk submit` from the main checkout (D15 refuses it).
- Re-prediction at apply time: the CAS refuses a moved ref, and the loop predicts again on the next pass. Reversal: none.

## 4. P4, one gate run per batch

Scope items:

1. The trial workspace: one workspace at `gates.trial_dir`, detached, marked by PD-9, made by `Env.create` and reused; `read-tree -u --reset` of a batch's final tree into it, held by a lock from the read to the last gate (section 6).
2. The batch (option U, D16): when the trial is idle and the queue holds submitted changesets, take up to `gates.batch_max` in order after `gates.batch_idle_seconds` of quiet; predict each on top of the previous prediction; gate the final tree once; on green, land every changeset of the batch in order (PD-6 moves the land step here) and fast-forward `main` with `landing.ff_main`, then push with `landing.push` (D14, D20). On red, attribute by T3 (paths in the gate's output against each changeset's `diff-tree --name-only base tree`); when exactly one changeset matches it is `failed` with the gate and the paths, and the rest re-gate without it; otherwise T1: drop the last and re-gate.
3. The gate runner (option S): the `[[gates.run]]` list in order, each with `gates.env` added, `gates.fresh_caches` pointed at fresh directories per batch, `pass`, `fail` or `skip` (a gate whose `needs_env` names an unset variable is `skip` with the reason, never `pass`); the pause while free memory is under `gates.min_free_mb` (PD-10).
4. `wk status` shows per changeset `gated`, `failed` (gate and reason), `landed`, and why `main` did not move (PD-11).
5. The `[gates]` keys and `landing.ff_main` (D26).

The keys:

| Key | Default | Meaning | Source |
|---|---|---|---|
| `landing.ff_main` | `true` | fast-forward `main` to the landing ref after a green batch | D14 |
| `gates.trial_dir` | `.claude/worktrees/wk-trial` | the trial workspace | section 3 P4 |
| `gates.batch_max` | `6` | changesets per batch | D16 |
| `gates.batch_idle_seconds` | `120` | quiet before a batch forms | D16 |
| `gates.min_free_mb` | `2048` | gating pauses under this | section 5 |
| `gates.fresh_caches` | `["GOCACHE", "GOLANGCI_LINT_CACHE"]` | variables set to a fresh directory per batch | section 6 |
| `[[gates.run]]` | none | `{name, dir, run, needs_env}`; none means no gate and no fast-forward (PD-11); ezhomesteading lists the `CLAUDE.md` gates minus the DB suites in CI's order, wk lists its four | option S |

Sub-rows (rows 11 and 12 build at once; row 5 integrates):

| Row | Owns | Builds | Model, review |
|---|---|---|---|
| 11 | `internal/gate/` (new: `run.go`, `paths.go`, `memory.go`, tests) | `Run(dir, gates, env, caches, log) []Result` with `{Name, Status, Reason, Output}`; `Paths(output) []string`, the repository-relative paths a `tsc`, eslint, `go build`, `go vet`, `gofmt -l`, `golangci-lint` and madge failure prints, with a table test over real output samples written into the test; `FreeMB()` from `vm_stat` (free plus inactive plus speculative pages); `WaitForMemory(min, sample, log)` | Opus 5.5, Opus 5.5 review |
| 12 | `internal/batch/` (new: `batch.go`, `blame.go`, tests) | pure logic on records: `Pick(records, max, idle, now)`; `Blame(failedPaths, diffs map[id][]string) (id string, unique bool)`; `Shrink(batch)`; the state transitions of scope item 2, table-tested over the three-changeset case of the proof and the fallback case | Opus 5.5, Opus 5.5 review |
| 5 | `cmd/wk/main.go`, `internal/config/config.go`, `internal/wk/trial.go`, `internal/wk/gatekeeper.go`, `internal/wk/landing.go` (the loop P3 wrote), `internal/wk/status.go`, `README.md`, `~/.wk/wk.toml`, `~/.wk/ezhomesteading.toml` | wires 11 and 12: the trial, the batch loop, the land-after-green and the fast-forward and push, the status, the config section, the README section, the live proof | Opus 5.5, Opus 5.5 review |

Done when, for each sub-row: its package tests green plus the gate line. For row 5, the gate line plus the proof of `DESIGN.md` section 8, P4, recorded under "As built, P4":

- In a throwaway repository with three gates in config, a batch of three changesets where one breaks the type-check gate is gated once plus one re-gate; the broken one is `failed` with the gate and the path named; the other two land and `main` is fast-forwarded and pushed to the local bare remote.
- Free memory sampled every second through the proof; the runner paused whenever a sample was under `gates.min_free_mb`, and the samples are recorded.
- A gate with `needs_env = ["EZH_TEST_DB"]` shows as `skip` with "EZH_TEST_DB is unset", never as `pass`.
- In this repository: two real changesets form one batch, the four wk gates run once in the trial, both land, `main` is fast-forwarded and pushed by the daemon with no command from Zach.

Gate: the P2 gate line.

Review: Opus 5.5 on every P4 row, by Zach's answer. Recommendation he has not taken: a Fable 5.1 review of `internal/batch/blame.go` and of the land-after-green step in row 5, because a wrong attribution lands the wrong two of three.

Not built, and why:

- `gates.parallel_tsc` and `gates.next_build_concurrency` (PD-10): row 200's slots cap them machine-wide.
- A `df`-based disk report in `wk status` (`PLAN.md` section 4): nothing in the proof reads it. Reversal: one line from `df -k`.
- Folding `/land-worktrees` (D13, later row).
- Gating in the article's post-merge order: `DESIGN.md` section 9 states the divergence on purpose.

## 5. Order, dials and the model map

| Row | Phase | Model | Waits on | Review |
|---|---|---|---|---|
| 6, 7, 8 | P2 siblings | Opus 5.5 | 2 | Opus 5.5 |
| 3 | P2 integration | Opus 5.5 | 2; 6, 7, 8; ezhomesteading item 200 | Opus 5.5 |
| 9, 10 | P3 siblings | Fable 5.1 | 3 | Fable 5.1 |
| 4 | P3 integration | Fable 5.1 | 3; 9, 10 | Fable 5.1 |
| 11, 12 | P4 siblings | Opus 5.5 | 4 | Opus 5.5 |
| 5 | P4 integration | Opus 5.5 | 4; 11, 12 | Opus 5.5 |

Every dial of sections 2 to 4 lives in `~/.wk/<repo>.toml` and nothing is compiled in (D19, D26). The section 5 dials of the design that this plan drops are named under each phase's "Not built, and why". Each integration row installs the binary (`GOTOOLCHAIN=local go install ./cmd/wk`) before it writes any new key, because the hooks run `~/go/bin/wk` and an older binary refuses a file with keys it does not know (`PLAN.md`, "As built, phase 3", the 15:18 incident).

## 6. Session protocol

- One row per worktree. A sub-row edits only the paths in its "Owns" cell; a path it needs outside that cell is a finding for the integration row, written in the hand-back, never an edit.
- The integration row starts by reading this file, the three sibling hand-backs and `main` as it then is, and measures its context after that.
- Absolute dates; a citation on every claim; no contractions and no em dashes in this folder.
- Each phase ends with an "As built, P<n>" note under its section here, with the proof output, and the integration row's commit hash.
- Git: Claude may run any git command (`~/.claude/rules/commits.md`); wk's policy is `agent-pushes`. No `Co-Authored-By:` trailer and no "Generated with" line. A row built under `/autopilot` commits on its branch only; the orchestrating session merges and pushes after the audit.

## 7. Answers

- 2026-10-16, Zach, before this plan, by option label: "Ratify all four (Recommended)" (D22 to D25); "Yes, one file (Recommended)" (D26); "opus review of 3,5" (section 5's review column); "lean, concise", and not over-engineered (the "Not built, and why" lists); "divide up the work for each item to more builders" (the sub-row shape).
