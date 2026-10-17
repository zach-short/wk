# Plan: landing layer, P2 path leases, P3 the landing queue, P4 one gate run per batch

Status: PLANNED (Stage 4), written 2026-10-16 by a Fable 5.1 session in the worktree `ap-row-2` of `~/Projects/wk` at `a4d4ed2` (wk board row 2). The design is `DESIGN.md` in this folder, frozen at D1 to D26; D22 to D26 were ratified on 2026-10-16 in this session and carry the P1 amendments and the one-file config decision. The P1 plan and its as-built notes are `PLAN.md`; its decisions are BD-1 to BD-23 and are not repeated here. This plan records build-level calls as PD-1 to PD-n. Model check: the row names Fable 5.1 and this session runs Fable 5.1.

Shape, by Zach's instruction of 2026-10-16 ("divide up the work for each item to more builders"): each phase is two or three sub-rows that build at the same time in separate worktrees on disjoint files, plus one integration sub-row that runs last, owns the shared wiring (`cmd/wk/main.go`, `internal/config/config.go`, `internal/wk/*`), and carries the phase's live proof. A sub-row waits only on the rows whose code it imports, never on a sibling of its own phase, so rows 9, 10 and 11 wait on row 2 alone and row 12 on row 9 (section 5). Each sibling runs on a lane of its own (C to I), because the board runs the items of one lane in order. Each phase is as small as its section 8 proof allows; what was cut is under "Not built, and why" with its reversal. The sub-row tables summarise the prompts in `passoff/`; where a table and its prompt differ, the prompt is binding, because the builder builds from the prompt.

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
| Hook commands | `wk hook worktree-create` prints one path on stdout and exits 2 on failure; `wk hook subagent-stop` always exits 0; `stdin` is a package variable the self-test swaps; `installedExe()` resolves the binary's symlinks, and `checkInstalled` with `wk.TransientBinary` refuses a `go run` build | `cmd/wk/main.go:66-124`, `:148-160`, `:214-220`; `internal/wk/hooks.go:17-28` |
| Detached process pattern | `spawnFill` starts `wk --repo <primary> pool fill` from `/` in its own session with its output in `pool.log`; a test binary never forks | `cmd/wk/main.go:188-212` |
| gc markers | gc skips the template (`wk-template`), pool slots (`wk-pool`) and any worktree whose private git directory holds `land-worktrees-trial` | `internal/wk/gc.go:19-21`; `internal/wk/template.go:18`; `internal/wk/pool.go:23` |
| The gc schedule | A launchd agent `com.github.zach-short.wk.gc.ezhomesteading` is installed and runs `wk gc --apply` every 15 minutes; P2 does not replace it (PD-5) | `ls ~/Library/LaunchAgents`; `internal/wk/schedule.go:22` |
| Self-test fixture | `testrepo.New(t)` makes an isolated throwaway repository with a fake install and a config; `Isolate()` sets `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1` | `internal/testrepo/testrepo.go:15-120` |
| Row 200 (ezhomesteading, IN FLIGHT) | Owns in wk: `cmd/wk/main.go`, a new `internal/slot`, `internal/config`, `internal/settings`, `README.md`, and `~/.wk/machine.toml`. Its plan adds `Entry.Matcher`, a `PreToolUse` entry on `Bash` that rewrites gate commands through `wk run --slot` and fails open, `mvdan.cc/sh/v3` as a dependency, and the repository's slot keys in `~/.wk/<repo>.slots.toml` (SB-4) | `~/Projects/ezhomesteading/passoff/200-*.md`, the row; `SLOTS-PLAN.md` SB-2, SB-4, SB-14, phase 2 step 4 |
| `updatedInput` from a project's `.claude/settings.local.json` reaches a subagent's `Bash` call, and a hook on `Bash` sees the original command | Verified by row 200's planning session on 2.1.301 and 2.1.299 with headless `claude -p` | `SLOTS-PLAN.md` section 0.2 |
| The commit rule | `~/.claude/rules/commits.md` reads "Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`" and forbids only the trailer and the "Generated with" line. D14's condition is met; P3 waits on nothing but P2 | `~/.claude/rules/commits.md:4`; `DESIGN.md` D14, tag of 2026-10-16 |
| wk's own commit policy | `agent-pushes`: a row lands on `main` and is pushed inside the run | `/Users/zachshort/Projects/wk/.personal-config.json:21` (`"commitPolicy": "agent-pushes"`, an ignored file); the autopilot brief of row 2 |
| The hook deny shape | `DESIGN.md` 1.4 gives two: exit 2 with the reason on stderr, or JSON `hookSpecificOutput.permissionDecision: "deny"`. Not re-verified here; row 3 step 1 verifies the field names on the installed version before the hook is written | `DESIGN.md` 1.4 |
| The hook deny shape, verified 2026-10-17 | Both forms refuse on Claude Code 2.1.301. JSON on stdout with exit 0, `{"hookSpecificOutput": {"hookEventName": "PreToolUse", "permissionDecision": "deny", "permissionDecisionReason": "<reason>"}}`, reaches the model as the tool result `PreToolUse:Write hook error: <reason>` with `is_error` true. Exit 2 with the reason on stderr reaches it as `PreToolUse:Write hook error: [<hook command>]: <reason>`. Row 3 uses the JSON form, so the hook always exits 0 | headless `claude -p --model haiku` with a one-line hook in the scratch project `scratchpad/row-3/probe`, 2026-10-17 02:36 and 02:37 EDT; `json.out`, `exit2.out` |
| The `PreToolUse` input on 2.1.301 | Top-level keys `cwd`, `effort`, `hook_event_name`, `permission_mode`, `prompt_id`, `scratchpad_dir`, `session_id`, `tool_input`, `tool_name`, `tool_use_id`, `transcript_path`. `tool_input` for `Write` is `file_path`, `content`; for `Edit` `file_path`, `old_string`, `new_string`, `replace_all`; for `Bash` `command`, `description`. Every `file_path` the model sent was absolute, and `cwd` came with symlinks resolved (`/private/tmp/...`) | the probe's `hook.jsonl`, four calls |
| `"onFailure": "block"` on 2.1.301 | A command hook with the key blocks the call when the hook fails: a missing binary (`/bin/sh` exit 127) gives `failed; blocking because onFailure is "block"` plus the shell's stderr, exit 1 the same, and a timeout `timed out; blocking because onFailure is "block"`. The same missing binary without the key let the `Write` through. The binary honours the key on a `command` hook that is not async and on an `http` hook | `ofmissing.out`, `nomissing.out`, `ofexit1.out`, `oftimeout.out`, 02:37 EDT; the 2.1.301 binary's `n1e` |

## 1. Decisions taken in this plan

Each has a one-line reversal. A build row that deviates cites the number.

- **PD-1. One socket per repository, under the state directory.** `~/.wk/<repo>/wk.sock`, beside the locks and logs P1 already keeps there, with a single-instance lock `~/.wk/<repo>/daemon.lock`. `DESIGN.md` section 5 named `~/.wk/<repo-hash>.sock`; the name is simpler and the macOS 104-byte socket path limit is checked at start. Reversal: the hashed name. (Superseded 2026-10-17: void under the files decision of 2026-10-16 22:51 EDT, `docs/incomplete/integration/KILL-TEST.md`; no socket and no `daemon.lock` are built.)
- **PD-2. The hook makes no git call.** `wk hooks install` bakes `--repo <primary>` into the lease hook's command, the hook derives the socket from it, and the daemon resolves which worktree holds a path from its own worktree list. A hook that runs on every edit of every agent must cost one connect and one round trip. Reversal: `gitx.Resolve` in the hook, two git runs per edit. (Amended 2026-10-17: PD-2 holds under the files decision, `docs/incomplete/integration/KILL-TEST.md`; the hook reads `leases.json` itself, and its worktree list comes from a file that `wk claim` and `wk release` keep beside the store, not from a daemon. Row 3's item file, "Decided by the rule", binds.)
- **PD-3. Leases are stored in the daemon and saved to `~/.wk/<repo>/leases.json`** by atomic rename after every change, loaded at start (option M). Reversal: none needed. (Superseded 2026-10-17 by the files decision, `docs/incomplete/integration/KILL-TEST.md`: no process holds the store. Each writer takes an exclusive `flock` on `~/.wk/<repo>/leases.lock`, loads `leases.json`, changes it and renames a temp file over it; readers take no lock. Row 3's item file, "Decided by the rule", binds.)
- **PD-4. The `Bash` guard walks words without a shell parser.** D11 says "as the commit guard does"; a quote-aware word split with `cd` tracking and the recognised spellings of section 2 needs no new dependency, so sub-row 7 does not touch `go.mod` while row 200 adds `mvdan.cc/sh/v3`. Reversal: parse with `mvdan.cc/sh/v3/syntax` once row 200 is on `main`.
- **PD-5. The launchd gc agent stays.** `PLAN.md` section 4 said P2's daemon takes the schedule over; it works and is measured, and a ticker in the daemon saves nothing. Reversal: a ticker calling `GCApply`.
- **PD-6. P3 lands ungated onto `wk/landing`; `main` moves only by `wk land --apply` until P4.** D14 fast-forwards `main` after a green P4 gate, so before P4 the automatic path stops at the landing ref and the manual override is the only way `main` moves. P4 moves the land step behind the gate. Reversal: none; this is D14 read in order.
- **PD-7. A changeset's base is `merge-base HEAD <landing tip>`**, computed at submit time, so a workspace whose branch carries commits (today's `agent-pushes` practice) still submits one tree. `wk sync` refuses a workspace whose HEAD is not an ancestor of the tip, because it moves the branch ref and keeps no commits. Reversal: base is HEAD and `wk submit` refuses a branch with commits.
- **PD-8. The queue is one JSONL file, `~/.wk/<repo>/queue.jsonl`, written only by the daemon.** Every state change is one appended line (`submitted`, `refused`, `landed`; P4 adds `gated`, `failed`); the current state of a changeset is its last line. Not a dial. Reversal: a state file rewritten in place. (Amended 2026-10-17 by the files decision, `docs/incomplete/integration/KILL-TEST.md`: there is no daemon. `wk submit` appends the `submitted` line, and the `wk land --loop` worker, one at a time under `~/.wk/<repo>/landing.lock`, appends the rest, with a `landing` line before each `Land`. Row 4's item file, "Decided by the rule", binds.)
- **PD-9. The trial workspace reuses the `land-worktrees-trial` marker**, so `wk gc` already skips it and `/land-worktrees` treats it as a trial. Reversal: a `wk-trial` marker and one more gc skip.
- **PD-10. Machine limits for gates come from row 200's slots, not from P4.** The trial's gate commands run through `wk run --slot` by the same matcher the hook uses, so one `next build` at a time and the memory floor are row 200's. P4 keeps only `gates.min_free_mb`, the pause before each gate, because section 8's proof names it. Reversal: `gates.parallel_tsc` and `gates.next_build_concurrency` as dials.
- **PD-11. No gates configured means no fast-forward.** A repository with `[landing] ff_main = true` and no `[[gates.run]]` lands onto `wk/landing` and `wk status` says why `main` did not move. Reversal: treat an empty gate list as green.
- **PD-12. P2 starts from whatever `main` holds when it launches.** Row 200 builds `internal/config` and `cmd/wk/main.go` at the same time; P2's config keys are additive and its subcommands are new names, so the merge is mechanical. Where row 200's `Entry.Matcher` exists, P2 uses it; where it does not yet, row 3 adds the field in the same shape (`SLOTS-PLAN.md` phase 2 step 4). Reversal: none.
- **PD-13. Build constants chosen in this plan, not measured.** Each is a choice with its reversal, marked `(PD-13)` where a prompt uses it: the landing loop passes every 5 s and on every `submit` (reversal: a longer interval); the daemon client's dial and read deadline is 2 s (reversal: longer); the socket path limit is 103 bytes, from the 104-byte `sun_path` of macOS less the terminator (a fact, kept here because it is enforced by a constant); the `Bash` walk table holds at least 25 commands (reversal: fewer, if the spellings are covered); the gate runner echoes the last 20 lines of a gate's output to the log (reversal: more); a changeset id is the first 12 hex of a SHA-1 (reversal: 16); the hook is measured under 50 ms of wall time over 20 runs (reversal: a looser bound; the design's 200 ms dial is the ceiling). (Amended 2026-10-17 by the files decision, `docs/incomplete/integration/KILL-TEST.md`: the 5 s pass becomes the launchd timer's interval plus a start on every `submit`, and the client deadline and the socket path limit no longer apply, because no command opens a socket.)
- **PD-14. Dial names, where this plan differs from `DESIGN.md` section 5.** `lease.*` became `leases.*` to match the section name; `roots.single_writer` became `leases.single_writer`; `batch.max` and `batch.idle_seconds` became `gates.batch_max` and `gates.batch_idle_seconds`, because the batch is the gate's unit; `lease.require_claim` is folded into `leases.enabled` (G3 is the behaviour whenever leases are on). Reversal: the design's names, one rename each.

## 2. P2, path leases

(Amended 2026-10-17 under the owner's daemon-or-files rule, decided "files" on 2026-10-16 at 22:51 EDT by `docs/incomplete/integration/KILL-TEST.md`: no daemon, no socket, no `wk up` and no `wk down`. Leases live in `~/.wk/<repo>/leases.json` under an exclusive `flock` on `~/.wk/<repo>/leases.lock`, with temp-file reaping and an idempotent re-claim. The hook reads `leases.json` itself and judges with `lease.Judge`, so D12's deny when the socket does not answer is dropped. Row 6's `Server` and `Client` stay on `main` unused. The binding text is row 3's item file, "Decided by the rule".)

Scope items, each citing its source:

1. The daemon: `wk up` starts it detached (the `spawnFill` pattern), `wk down` stops it, any lease command starts it on demand when the socket is missing, the hook never does (D12; options M, Z; PD-1 to PD-3). (Superseded 2026-10-17: not built; see the note under this section's heading.)
2. `wk claim <row>`: reads the board (`leases.board`), takes row `<row>`'s "Files it owns" cell, parses backtick tokens into exact paths, directory prefixes (a trailing `/` or a `**` pattern) and globs expanded against `git ls-files` of the workspace, prints every token it could not read, and takes leases only for what it read; refuses when a lease is held elsewhere, naming the holder's row and workspace (D10, option K, G3). `wk claim --none` claims no paths. `wk lease add <path>...` and `wk release` (option K).
3. The hook, `wk hook lease --repo <primary>`, on `PreToolUse` for `Edit|Write|MultiEdit|NotebookEdit` and for `Bash` (D11, option I2): resolves the path against `cwd`, asks the daemon, and denies with the holder named when another workspace leases it; denies every edit from a workspace that has not claimed (D10); denies an edit to a `leases.single_writer` path the workspace does not lease (D13 L1); denies everything with "start the daemon: wk up" when the socket does not answer (D12). A path outside every worktree of the repository is allowed. The daemon keys holders by worktree root (option H). (Amended 2026-10-17: the hook reads `leases.json` instead of asking a daemon, the store keys holders by worktree root, and the D12 deny when the socket does not answer is dropped; see the note under this section's heading.)
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
| 6 (lane C) | `internal/daemon/` (new: `daemon.go`, `client.go`, `spawn.go`, `state.go`, `daemon_test.go`) | the unix-socket server and client, newline-delimited JSON, one request per line, `{"op": ...}` in and `{"ok": bool, "error": ..., "data": ...}` out; built-in ops `ping` and `shutdown`; handlers registered by name; single instance by exclusive flock, a stale socket file removed only after the lock is held; the 104-byte path check; `Spawn(exe, args, log)` detached from `/`; `SaveJSON` by atomic rename and `LoadJSON` (Amended 2026-10-17: landed; under the files decision `Server` and `Client` stay unused, and rows 3 and 4 use `SaveJSON`, `LoadJSON` and `Spawn`) | Opus 5.5, Opus 5.5 review |
| 7 (lane D) | `internal/lease/` (new: `store.go`, `judge.go`, `bash.go`, tests) | the store: workspaces keyed by root, each with row, session, leases, claimed time; `Claim`, `Release`, `Add`, `Holder(path)`, `Claimed(root)`; the conflict rule (an exact path or a directory prefix; two leases overlap when one is a prefix of the other); `Resolve(roots, cwd, path)` by longest prefix; `Judge(store, rules, roots, req) Decision` with `Rules{SingleWriter, BashGuard}` and the four deny reasons of scope item 3, each naming what to do next; the `Bash` walk: `sed -i`, `tee`, `>` and `>>`, `cp`, `mv`, `rm`, `touch`, `git mv`, `git rm`, `python -c` with `open(` and a `w` mode, with leading `VAR=value` words skipped and `cd <dir>` at the top of a `&&` or `;` list tracked, failing closed on what it recognises and passing what it does not; JSON round trip of the store | Opus 5.5, Opus 5.5 review |
| 8 (lane E) | `internal/board/` (new: `board.go`, `board_test.go`) | `Cell(text, row)`: the last cell of the table line whose first cell is `row`; `Parse(cell)`: backtick tokens become paths, a trailing `/` or `**` becomes a directory prefix, a token with `*`, `?` or `[` is a glob, everything else (prose, `~`, absolute paths, "to" ranges) is returned as unread; `Expand(globs, files)` against a tracked-file list with `path.Match` per segment | Opus 5.5, Opus 5.5 review |
| 3 (lane A) | `cmd/wk/main.go`, `internal/config/config.go`, `internal/settings/settings.go` (sequential overlap with row 200), `internal/wk/lease.go`, `internal/wk/leasehook.go`, `internal/wk/daemon.go`, `internal/wk/hooks.go`, `README.md`, `~/.wk/wk.toml`, `~/.wk/ezhomesteading.toml` | wires 6, 7 and 8: the lease ops on the daemon, the subcommands, the hook verb, the config section, `wk config show`, the settings entries, the README section, the live proof (Amended 2026-10-17: no `internal/wk/daemon.go`; the lease ops work on the file store; see the note under this section's heading) | Opus 5.5, Opus 5.5 review |

Done when, for each sub-row: its package tests green plus the gate line. For row 3, the gate line plus the proof of `DESIGN.md` section 8, P2, recorded in this file under "As built, P2":

- In a throwaway repository with a fixture board, two headless sessions (`claude -p`, the technique of `SLOTS-PLAN.md` 0.2), each in a worktree made by `wk new` (which copies `.claude/settings.local.json` and so the hook, BD-2; a plain `git worktree add` has no hook and the proof would pass for nothing), claim rows 1 and 2; an `Edit` by session 2 to a path session 1 leases is refused with session 1's row and workspace named; a `sed -i` to the same path is refused the same way; after `wk down` every edit is refused with "start the daemon: wk up". Each refusal is shown to have come from the hook: the deny output in the session's transcript and the denial line in `~/.wk/<name>/hook.log`. (Amended 2026-10-17: the `wk down` step is dropped with D12's down-state deny; see the note under this section's heading.)
- The hook's wall time, measured with `/usr/bin/time -p` over 20 runs fed the `Edit` input, is under 50 ms (PD-13).
- `wk claim` on the live wk board (`PASSOFF.md` of this repository) prints the unread tokens of every row's cell, and reads every backtick path.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`

Review: Opus 5.5 on every P2 row, by Zach's answer of 2026-10-16 ("opus review of 3,5"). Recommendation he has not taken: a Fable 5.1 review of `internal/lease/bash.go` and `judge.go` alone, because a pass that should be a refusal is quiet until landing (D17, option X).

Not built, and why:

- D13 L3, insert-only edits on the ledger files with an exception for the claimed row. Section 8's proof needs L1 only; in this repository `HANDOFF.md` and `PASSOFF.md` are untracked and written by one orchestrating session, and the board is now a table plus per-item files, so the exception would parse a layout that is still moving. The ledger files are `single_writer` instead, which serialises two sessions that both want to append. Reversal: an `insert_only` list and the check that `new_string` contains `old_string`. Owner's call, asked in row 3.
- `lease.require_claim`: folded into `leases.enabled` (PD-14); with leases on, a claim is always required (G3). Reversal: the key, defaulting to true.
- `lease.ttl_minutes`: D10 fixes it at 0, so it is behaviour, not a dial. Reversal: the key.
- `lease.fail_closed`: D12 fixes it; a key to fail open is the silent failure. Reversal: the key.
- `hook.timeout_ms` 200: Claude Code's hook timeout is whole seconds; the entries carry `timeout: 5` and the hook is measured under 50 ms. Reversal: none.
- The daemon taking over the gc schedule (PD-5).
- Hooks reaching a worktree that was not made by wk or by the desktop app: those carry no `.claude/settings.local.json`, so the guard is absent there; the README says so.


### As built, P2

Built 2026-10-17 by wk board row 3 (Opus 5.5) in the worktree `ap-row-3` on the branch `ap/row-3`, from `main` at `26f14e3`, under the files decision: no daemon, no socket, no `wk up`, no `wk down`. The integration commits are `98d1844` (code), `5a93108` (README, and git run from the caller's workspace) and the commit that adds this note.

What it is. `wk claim <row>`, `wk claim --none`, `wk lease add`, `wk release` and `wk status` keep `~/.wk/<repo>/leases.json` under an exclusive `flock` on `leases.lock` from read to rename, reaping temp files older than one minute and the claims of worktrees that are gone; a second claim of the same row with the same or fewer leases succeeds. The file holds the store, the repository's main checkout and common git directory (a mismatch is refused), and the canonical worktree roots, which each change refreshes from git in the claiming workspace. `wk hook lease --repo <primary>` reads the config and the store itself and runs no git (PD-2), computes the `Bash` write targets before it reads either, allows a single `wk` command and a command with no target, places every path through symlinks, adds any worktree of the repository it finds on disk around the path, refuses a write into another worktree than the session's own, judges with `lease.Judge`, and prints the JSON deny with exit 0; a panic is a deny. `wk hooks install` writes its two entries only while `leases.enabled` is true: `Edit|Write|MultiEdit|NotebookEdit` with `"onFailure": "block"`, and `Bash` guarded by `|| true` with no `onFailure`. `wk config show` prints the effective config.

The proof, in the throwaway repository `scratchpad/row-3/proof/leaseproof` with `WK_HOME` in the scratchpad, `[leases] enabled = true`, `copy_files = [".claude/settings.local.json"]`, the hooks installed by `wk hooks install`, and two workspaces made by `wk new 1` and `wk new 2` (each received the settings file with both lease entries). The fixture board gives row 1 `web/page.ts` and row 2 `docs/`.

- Session 1 (`claude -p` in `wk-1`) ran `wk claim 1`: `claimed: row 1 in .../wk-1`, `lease: web/page.ts`.
- Session 2 (in `wk-2`) ran `wk claim 2` (`unread: ; the rest of the docs`, `lease: docs/`), then `Edit` on `wk-2/web/page.ts`. Its transcript holds the tool result `PreToolUse:Edit hook error: wk: web/page.ts is leased by row 1 in .../wk-1; leave it to that workspace or wait until it releases`, and `hook.log` the line `2026-10-17T02:50:52-04:00 lease Edit .../wk-2/web/page.ts cwd=.../wk-2 (0ms) deny: wk: web/page.ts is leased by row 1 in .../wk-1; ...`. The file kept `page = 1`.
- Session 2 stopped after the refusal without running its third step, so a third session in `wk-2` ran `sed -i '' 's/page = 1/page = 3/' web/page.ts`: the transcript holds `PreToolUse:Bash hook error: wk: web/page.ts is leased by row 1 in .../wk-1; leave it to that workspace or wait until it releases (the command writes .../wk-2/web/page.ts)`, and `hook.log` the matching deny line at `02:51:12` with `(2ms)`. The file kept `page = 1`.
- The `wk down` step was dropped with D12's down-state deny.
- The hook's wall time, `/usr/bin/time -p` over 20 runs fed the `Edit` input that is denied: 14 runs at 0.00 s, 4 at 0.01 s, 2 at 0.02 s (the timer's resolution is 10 ms); mean 4 ms, max 20 ms, under the 50 ms of PD-13.
- `wk claim` on the live wk board (`PASSOFF.md` in the main checkout, read through this worktree with `WK_HOME` in the scratchpad, each claim released after it): rows 3, 4, 5, 7 and 13 each printed their unread tokens (`~/.wk/wk.toml`, `~/.wk/ezhomesteading.toml`, the prose "(As built, P2)", "(sequential overlap with ezhomesteading item 200);" and "`, tests)`") and leased every backtick path. Row 7's cell `` `internal/lease/` (`store.go`, `judge.go`, `bash.go`, tests) `` leases `store.go`, `judge.go` and `bash.go` at the repository root, because the parse reads each backtick token on its own; this is the lossy parse working as D10 says, and the cell's author can write the full paths.

The binary was installed with `GOTOOLCHAIN=local go install ./cmd/wk` after the gate, then `[leases]` with `enabled = false` was added to `~/.wk/ezhomesteading.toml` (with the four D13 L1 paths) and `~/.wk/wk.toml` was made (the P1 defaults, `copy_files = [".claude/settings.local.json"]`, `[leases] enabled = false`). The review of step 10 is this run's independent audit, not a subagent of the row.

Audit 1 (Opus 5.5, of `f0b1eb8`, report `docs/incomplete/autopilot-2026-10-16/reports/row-3-audit-1.md`, which reproduces the live checks; the scratchpad files cited above do not persist). Outcomes, fix round 1 of 2026-10-17:

- B1, a path with the wrong letter case passed on macOS: fixed. On a volume that ignores case (`.git` and `.GIT` are one entry, or the check cannot tell), the hook folds every path, root, lease and single-writer entry to lower case before it judges, and refuses a non-ASCII path below a worktree. Test `TestHookFoldsCaseOnACaseInsensitiveVolume`.
- B2, a cwd outside every worktree could write any worktree: fixed. Such a session may write no path inside a worktree. Test `TestHookDeniesAWriteFromOutsideEveryWorkspace`.
- B3, a dangling symlink to a leased new file passed: fixed. The hook reads each path as the kernel does (`kernelPath` in `internal/wk/lease.go`), following a dangling link to the path it names; a loop or an unreadable link denies. Test `TestHookFollowsLinksAsTheKernelDoes`.
- B4, a symlink followed by `..` hid the real target: fixed. `place` in `internal/lease/bash.go` no longer cleans a target with a `..`, the hook joins a relative `file_path` without cleaning, and `kernelPath` applies each `..` after the link before it. Same test, and a walk row.
- N1, a panic outside the hook's judgment gave no output: fixed; every panic of the verb prints the deny. N2: test `TestHookPanicIsADeny`. N3: the README and the hand-back say that existing workspaces keep the settings they were made with. N4: the README lines on links, the edit entries and the heredoc apostrophe are corrected.
- N5 (any binary named `wk` passes the wk allow) and N6 (one `Bash` command is walked three times): open.

Audit 2 (Opus 5.5, of `1c4a1fd`, report `docs/incomplete/autopilot-2026-10-16/reports/row-3-audit-2.md`) confirmed B1 to B4 closed, B4 included, and found three more silent allows. Outcomes, fix round 2 of 2026-10-17:

- B5, a `cd` through a link and then `..` was cleaned by text: fixed. `moved` in `internal/lease/bash.go` keeps both the cleaned directory and the raw one for a `cd` operand with `..`, and `place` leaves a target uncleaned when its directory holds `..`. Test rows in `TestHookJudgesEverySpellingOfAPath` and two walk rows.
- B6, a link at a leased path was judged at its target only: fixed. The hook judges every path also with its last component not followed (`entryPath`), so `rm`, `mv`, `ln -sf` and `git rm` of the link are refused. Same test.
- B7, an edit path a tool may clean by text: fixed. The hook judges every path also with `..` removed by text first (`cleanedPath`). Same test.
- N7: test `TestLeaseHookPanicOutsideTheJudgmentDenies` covers the recover in `runLeaseHook`. N9: `checkTargets` takes a bool. N5, N6 (now ten walks of one command), N8 and N10: open.

## 3. P3, the landing queue

(Amended 2026-10-16 22:51 EDT under the owner's daemon-or-files rule, decided "files" by `docs/incomplete/integration/KILL-TEST.md`: the landing loop is a detached run-to-completion worker `wk land --loop` started by `wk submit` and by the launchd timer, not a goroutine of a daemon; a `landing` record journals the predicted tree before each CAS. The binding text is row 4's item file, "Decided by the rule".)

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
| 9 (lane F) | `internal/changeset/` (new: `tree.go`, `queue.go`, tests) | `Tree(workspace) (tree string, err)` through a private index that is removed afterwards; `Base(workspace, tip)` by PD-7 with a refusal when there is no merge base; `Head(workspace) (commit, branch, err)`; `Queue` at a path: `Append(record)`, `Load() []Record` folded to the last line per id, ids from the first 12 hex of `sha1(tree + base + time)` (PD-13); states `submitted`, `refused`, `landed`, `gated`, `failed`; a test in a `testrepo` fixture shows an untracked file is in the tree, an ignored file is not, and the workspace's index is byte-identical before and after | Fable 5.1, Fable 5.1 review |
| 10 (lane G) | `internal/landing/` (new: `predict.go`, `land.go`, `sync.go`, `ff.go`, tests, and a `git` shim script the tests put first on `PATH`) | `Predict(repo, base, tip, tree) (result string, conflicts []string, err)`; `Land(repo, ref, tip, result, message) (commit string, err)` by `commit-tree` and the CAS `update-ref`, refusing when the ref moved; `Ensure(repo, ref, main)` for four cases: the ref is missing (create at `main`), the ref is an ancestor of `main` (move it), `main` is an ancestor of the ref (return the tip), diverged (`ErrDiverged`); `Sync(workspace, base, tip)` as scope item 3; `FastForward(primary, branch, to, expect) (blockers []string, err)` and `Push(primary, remote, branch)`. The shim test: every git invocation of the package is logged, and the log shows that the only ref writes are `update-ref <ref> <new> <old>` on the landing ref and `update-ref refs/heads/<branch>` in `Sync`, that `commit-tree` is given exactly the predicted tree, and that nothing but `FastForward` names `main` | Fable 5.1, Fable 5.1 review |
| 4 (lane A) | `cmd/wk/main.go`, `internal/config/config.go`, `internal/wk/landing.go`, `internal/wk/submit.go`, `internal/wk/status.go`, `README.md`, `~/.wk/wk.toml`, `~/.wk/ezhomesteading.toml` | wires 9 and 10: the `submit` and `status` ops, the loop as a goroutine of the daemon, the subcommands, the config section, lease release on `landed`, the README section, the live proof (Amended 2026-10-17: the loop is the detached `wk land --loop` worker, not a goroutine of a daemon; see the note under this section's heading) | Fable 5.1, Fable 5.1 review |

Done when, for each sub-row: its package tests green plus the gate line. For row 4, the gate line plus the proof of `DESIGN.md` section 8, P3, as D14 and D20 amend it, recorded under "As built, P3":

- In a throwaway repository, two workspaces submit overlapping changesets; the first lands, the second is `refused` with the conflicting path named, `wk sync` in the second workspace brings it to the tip with the conflict in the working tree, and a resubmit after the fix lands.
- The landing commit's tree equals the predicted tree byte for byte (`rev-parse <commit>^{tree}` against the `Predict` result), for every landing in the proof.
- The shim log of row 10, run again over the daemon's whole loop, shows no ref write but the CAS on the landing ref, and no write to `refs/heads/main`.
- `wk land` prints the plan; `wk land --apply --expect <hash>` moves `main` by fast-forward and pushes it in the throwaway repository, whose `origin` is a `git init --bare` directory in the session scratchpad; `--expect` with a stale hash is refused; the bare remote's `main` equals the landing commit afterwards.
- No proof touches this repository's `main` or `origin`: under this run only the orchestrating session moves `main`, after an audit.

Gate: the P2 gate line.

Not built, and why:

- A `session_id` on a changeset (option O): nothing reads it. Reversal: one field.
- `queue.path` as a dial (PD-8).
- Predictions run concurrently (section 9): the queue lands in order on one ref, so one goroutine is the whole concurrency. Reversal: none needed.
- `wk submit` from the main checkout (D15 refuses it).
- Re-prediction at apply time: the CAS refuses a moved ref, and the loop predicts again on the next pass. Reversal: none.
- One tree builder: row 10's `Sync` reproduces the private-index sequence that prompt 9 step 1 spells out (`GIT_INDEX_FILE`, `read-tree HEAD`, `add -A`, `write-tree`), because the two rows build beside each other and share no file. Reversal: row 4 passes `changeset.Tree` into `landing.Sync` and deletes the second copy.

## 4. P4, one gate run per batch

(Amended 2026-10-17 from decisions 3, 8 and 20 of `docs/incomplete/integration/DECISIONS.md` and D16's amendment of 2026-10-16: the loop row 5 extends is section 3's detached `wk land --loop` worker, not a daemon; bisection replaces drop-the-last, so a red batch of more than one changeset is split in two and each half re-gated, with path mapping (`batch.Blame`) as a first guess only; and `gates.batch_max` is 1, a serial gate then land, until landings per hour times gate hours passes 0.5. The integration plan's acceptance for row 5 (`docs/incomplete/integration/PLAN.md` section 4) runs a batch with `batch_max = 2`. The binding text is row 5's item file, "Owner amendments, 2026-10-16".)

Scope items:

1. The trial workspace: one workspace at `gates.trial_dir`, detached, marked by PD-9, made by `Env.create` and reused; `read-tree -u --reset` of a batch's final tree into it, held by a lock from the read to the last gate (section 6).
2. The batch (option U, D16): when the trial is idle and the queue holds submitted changesets, take up to `gates.batch_max` in order after `gates.batch_idle_seconds` of quiet; predict each on top of the previous prediction; gate the final tree once; on green, land every changeset of the batch in order (PD-6 moves the land step here) and fast-forward `main` with `landing.ff_main`, then push with `landing.push` (D14, D20). On red, attribute by T3 (paths in the gate's output against each changeset's `diff-tree --name-only base tree`); when exactly one changeset matches it is `failed` with the gate and the paths, and the rest re-gate without it; otherwise T1: drop the last and re-gate. (Amended 2026-10-17: bisection replaces T1's drop-the-last; see the note under this section's heading.)
3. The gate runner (option S): the `[[gates.run]]` list in order, each with `gates.env` added, `gates.fresh_caches` pointed at fresh directories per batch, `pass`, `fail` or `skip` (a gate whose `needs_env` names an unset variable is `skip` with the reason, never `pass`); the pause while free memory is under `gates.min_free_mb` (PD-10).
4. `wk status` shows per changeset `gated`, `failed` (gate and reason), `landed`, and why `main` did not move (PD-11).
5. The `[gates]` keys and `landing.ff_main` (D26).

The keys:

| Key | Default | Meaning | Source |
|---|---|---|---|
| `landing.ff_main` | `true` | fast-forward `main` to the landing ref after a green batch | D14 |
| `gates.trial_dir` | `.claude/worktrees/wk-trial` | the trial workspace | section 3 P4 |
| `gates.batch_max` | `6` | changesets per batch (Amended 2026-10-17: the default is `1` until the busy fraction passes 0.5, decision 20; see the note under this section's heading) | D16 |
| `gates.batch_idle_seconds` | `120` | quiet before a batch forms | D16 |
| `gates.min_free_mb` | `2048` | gating pauses under this | section 5 |
| `gates.fresh_caches` | `["GOCACHE", "GOLANGCI_LINT_CACHE"]` | variables set to a fresh directory per batch | section 6 |
| `[[gates.run]]` | none | `{name, dir, run, needs_env}`; none means no gate and no fast-forward (PD-11); ezhomesteading lists the `CLAUDE.md` gates minus the DB suites in CI's order, wk lists its four | option S |

Sub-rows (rows 11 and 12 build at once; row 5 integrates):

| Row | Owns | Builds | Model, review |
|---|---|---|---|
| 11 (lane H) | `internal/gate/` (new: `run.go`, `paths.go`, `memory.go`, tests) | `Run(root, gates, env, caches, log) ([]Result, error)` with `{Name, Status, Reason, Output, Seconds}`, the last 20 lines of each gate echoed to the log (PD-13); `Paths(root, output) []string`, the repository-relative paths a `tsc`, eslint, `go build`, `go vet`, `gofmt -l`, `golangci-lint` and madge failure prints, with a table test over real output samples written into the test; `FreeMB()` from `vm_stat` (free plus inactive plus speculative pages); `WaitForMemory(min, sample, log)` | Opus 5.5, Opus 5.5 review |
| 12 (lane I) | `internal/batch/` (new: `batch.go`, `blame.go`, tests) | pure logic on records: `Pick(records, max, idle, now)`; `Blame(failedPaths, diffs map[id][]string) (id string, unique bool)`; `Shrink(batch)`; the state transitions of scope item 2, table-tested over the three-changeset case of the proof and the fallback case | Opus 5.5, Opus 5.5 review |
| 5 (lane A) | `cmd/wk/main.go`, `internal/config/config.go`, `internal/wk/trial.go`, `internal/wk/gatekeeper.go`, `internal/wk/landing.go` (the loop P3 wrote), `internal/wk/status.go`, `README.md`, `~/.wk/wk.toml`, `~/.wk/ezhomesteading.toml` | wires 11 and 12: the trial, the batch loop, the land-after-green and the fast-forward and push, the status, the config section, the README section, the live proof | Opus 5.5, Opus 5.5 review |

Done when, for each sub-row: its package tests green plus the gate line. For row 5, the gate line plus the proof of `DESIGN.md` section 8, P4, recorded under "As built, P4":

- In a throwaway repository with three gates in config, a batch of three changesets where one breaks the type-check gate is gated once plus one re-gate; the broken one is `failed` with the gate and the path named; the other two land and `main` is fast-forwarded and pushed to the local bare remote.
- Free memory sampled every second through the proof; the runner paused whenever a sample was under `gates.min_free_mb`, and the samples are recorded. This is how section 8's "memory stays under the dial, measured" is read here: the one memory dial P4 keeps is `gates.min_free_mb`; the other machine limits of option V (`parallel_tsc`, `next_build_concurrency`, the slots' own `min_free_mb`) are row 200's by PD-10, which row 5 reaches only through `wk run --slot`, not by code it owns.
- A gate with `needs_env = ["EZH_TEST_DB"]` shows as `skip` with "EZH_TEST_DB is unset", never as `pass`.
- In the same throwaway repository, with the daemon running: two changesets submitted within `gates.batch_idle_seconds` form one batch, the gates run once in the trial, both land, and `main` is fast-forwarded and pushed to the bare remote by the daemon with no command from anyone. No proof touches this repository's `main` or `origin`. (Amended 2026-10-17: "the daemon" here is the `wk land --loop` worker, which `wk submit` and the launchd timer start, and the batch needs `gates.batch_max` of at least 2; see the note under this section's heading.)

Gate: the P2 gate line.

Review: Opus 5.5 on every P4 row, by Zach's answer. Recommendation he has not taken: a Fable 5.1 review of `internal/batch/blame.go` and of the land-after-green step in row 5, because a wrong attribution lands the wrong two of three.

Not built, and why:

- `gates.parallel_tsc` and `gates.next_build_concurrency` (PD-10): row 200's slots cap them machine-wide.
- A `df`-based disk report in `wk status` (`PLAN.md` section 4): nothing in the proof reads it. Reversal: one line from `df -k`.
- Folding `/land-worktrees` (D13, later row).
- Gating in the article's post-merge order: `DESIGN.md` section 9 states the divergence on purpose.

## 5. Order, dials and the model map

| Row | Phase | Model | Lane | Waits on | Review |
|---|---|---|---|---|---|
| 6, 7, 8 | P2 siblings | Opus 5.5 | C, D, E | 2 | Opus 5.5 |
| 3 | P2 integration | Opus 5.5 | A | 2; 6, 7, 8; ezhomesteading item 200 | Opus 5.5 |
| 9, 10 | P3 siblings | Fable 5.1 | F, G | 2 | Fable 5.1 |
| 4 | P3 integration | Fable 5.1 | A | 3; 9, 10 | Fable 5.1 |
| 11 | P4 sibling | Opus 5.5 | H | 2 | Opus 5.5 |
| 12 | P4 sibling | Opus 5.5 | I | 9 | Opus 5.5 |
| 5 | P4 integration | Opus 5.5 | A | 4; 11, 12 | Opus 5.5 |

A sibling waits only on the rows whose code it imports: rows 9 and 10 import only `internal/gitx` and `internal/testrepo` (row 10 reproduces the private-index sequence of prompt 9 step 1 rather than importing row 9), and row 11 imports no internal package (its tests are shell scripts and a captured `vm_stat`), so all three can start with the P2 siblings; row 12 imports `internal/changeset`, row 9's package, so it waits on 9. Each sibling has its own lane because the board runs one lane's items in order.

Every dial of sections 2 to 4 lives in `~/.wk/<repo>.toml` and nothing is compiled in (D19, D26). The section 5 dials of the design that this plan drops are named under each phase's "Not built, and why". Each integration row installs the binary (`GOTOOLCHAIN=local go install ./cmd/wk`) before it writes any new key, because the hooks run `~/go/bin/wk` and an older binary refuses a file with keys it does not know (`PLAN.md`, "As built, phase 3", the 15:18 incident).

## 6. Session protocol

- One row per worktree. A sub-row edits only the paths in its "Owns" cell; a path it needs outside that cell is a finding for the integration row, written in the hand-back, never an edit.
- The integration row starts by reading this file, the three sibling hand-backs and `main` as it then is, and measures its context after that.
- Absolute dates; a citation on every claim; no contractions and no em dashes in this folder.
- Each phase ends with an "As built, P<n>" note under its section here, with the proof output, and the integration row's commit hash.
- Git: Claude may run any git command (`~/.claude/rules/commits.md`); wk's policy is `agent-pushes`. No `Co-Authored-By:` trailer and no "Generated with" line. A row built under `/autopilot` commits on its branch only; the orchestrating session merges and pushes after the audit.

## 7. Answers

- 2026-10-16, Zach, before this plan, by option label: "Ratify all four (Recommended)" (D22 to D25); "Yes, one file (Recommended)" (D26); "opus review of 3,5" (section 5's review column); "lean, concise", and not over-engineered (the "Not built, and why" lists); "divide up the work for each item to more builders" (the sub-row shape).
