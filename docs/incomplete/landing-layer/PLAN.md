# Plan: landing layer, P1 cheap workspaces

Status: PLANNED (Stage 4), written 2026-10-16 by an Opus 5.5 session in the worktree `focused-bassi-248015` at `fef977f48`, started from the session that ratified `DESIGN.md`. GATE 2 passed 2026-10-16: Zach approved the phases, the order and the dials in chat; section 7 records the answers. Phases 1 to 3 are BUILT as of 2026-10-16; phase 3 (`46ecc15`) is merged into `wk` `main` at `30989b5`, and its answers and BD-15 to BD-23 amend D7, D8, D9 and section 8 of the design (section 7, "Phase 3 answers"). This plan covers P1 only (D1); P2 to P4 get their own plans when P1 is BUILT. The design is `DESIGN.md` in this folder, frozen at D1 to D20; this file changes nothing in it and records build-level calls as BD-1 to BD-n. (Amended 2026-10-17: the design now holds D1 to D26 and builds no daemon; see the note under its header.) Model check: D17 assigns P1 to Opus; this session runs Opus 5.5.

## 0. Facts verified 2026-10-16 (this table supersedes DESIGN.md where they differ)

Read between 12:09 and 12:25 EDT, in the main checkout at `fef977f48` unless the citation says otherwise. Where a value differs from `DESIGN.md` section 1 (read 10:43 to 10:54 EDT at `d17444591`), this table wins and the third column says what moved.

### 0.1 The `WorktreeCreate` hook contract (D4, task step 1)

Verified against the installed Claude Code 2.1.301 (`claude --version`; binary `/Users/zachshort/.local/share/claude/versions/2.1.301`, dated 2026-10-15 20:05). The documentation reachable through Context7 (`/anthropics/claude-code`) still lists only the common fields, so the event-specific fields below come from the zod schemas and the dispatcher inside the installed binary (`strings` of the binary, searched for `"WorktreeCreate"`). Minified names such as `n0` and `pLn` change between versions; the build session re-reads them on its own installed version before relying on them.

| Claim | Verified state | Citation |
|---|---|---|
| Input fields | The common hook input (`session_id`, `transcript_path`, `cwd`, optional `permission_mode`, `agent_id`, `agent_type`) plus `hook_event_name: "WorktreeCreate"` and `name` (a string). There is no branch, base commit or target directory in the input | binary schema `n0=...N().and(u({hook_event_name:A("WorktreeCreate"),name:o()}))`; dispatcher `{...tu(e,de()),hook_event_name:"WorktreeCreate",name:n}`; common fields from Context7 `mods/types/claude-code.d.ts`, `BaseHookInput` |
| What `name` is | The worktree name Claude Code would use itself: `agent-<17 hex>` for a subagent with `isolation: "worktree"`, the `--worktree <name>` argument, or a generated `<adjective>-<noun>-<4 chars>` such as `swift-fox-ab12` | binary: the three callers of the dispatcher (session `--worktree`, agent worktree, tmux start); worktree names in `git worktree list` |
| Output | A command hook prints the worktree path on stdout. An HTTP hook returns `hookSpecificOutput: {hookEventName: "WorktreeCreate", worktreePath}`. A relative path is resolved against the input `cwd` | binary schema `x0` ("Command hooks print the path on stdout instead"); dispatcher `isAbsolute(b) ? b : resolve(cwd, b)` |
| Path checks | The path must exist as a directory ("The hook must create the directory before echoing its path"), must hold no dot segments, and no component below the checkout root may be a symlink | binary error strings `Snn`, "emitted a path with dot segments", "is a symlink" |
| Failure has no fallback in Claude Code | When a `WorktreeCreate` hook is configured and fails, or succeeds with no path, Claude Code throws ("WorktreeCreate hook failed: ...") and the session or subagent gets no worktree. It does not fall back to `git worktree add` | binary dispatcher `pLn`: `throw Error("WorktreeCreate hook failed: ...")` |
| The hook replaces git's creation | With the hook configured, Claude Code uses the hook's path and records the worktree as `hookBased: true`; it does not run `git worktree add` itself | binary `WYt`: `if (w) {... hookBased: !0}` else the git path |
| Removal of a hook-made worktree | With no `WorktreeRemove` hook, a hook-made agent worktree is kept, not removed ("no WorktreeRemove hook is enabled here (hooks are disabled, or the repo defines only WorktreeCreate)"; "kept hook-based worktree") | binary strings near `removeAgentWorktree` |
| `WorktreeRemove` input | Common fields plus `worktree_path` | binary schema `r0` |
| Timeout | A constant `Su=600000` (10 minutes) sits beside the other hook timeouts (`s9e=30000`) and is passed as `timeoutMs` to the dispatcher. Read from minified code; treat as likely, not certain | binary `var Su=600000,s9e=30000,...` |
| A restriction that can refuse the hook | Under `CLAUDE_CODE_RESTRICT_PERSONAL_CONFIG`, a `WorktreeCreate` hook "from your own Claude Code files" runs but its folder is not used | binary error string |
| Desktop app worktree sessions | Not verified. This session's worktree (`focused-bassi-248015`, branch `claude/jovial-moser-609a96`) has a directory name and a branch name that differ, which no CLI caller above produces; the desktop app may create its worktrees without the CLI hook. Proven or disproven in phase 3 | `git worktree list`; this session's environment |

Consequence for D4: A3 stands, with two changes in how it is built, both recorded as BD-6 and BD-7. The fallback to a plain worktree must live inside `wk`'s hook command, because Claude Code has none. And the hook must create the branch itself, because the input carries no branch.

### 0.2 The machine, the repository and its worktrees

| Claim | Verified state | Moved from DESIGN.md section 1 | Citation |
|---|---|---|---|
| Memory, cores | 16 GB, 10 cores | no | `sysctl -n hw.memsize hw.ncpu` |
| Free disk | 43 GiB available of 460 GiB | was 53 GiB at 10:45 | `df -h /` at 12:12 EDT |
| git, Go, bun | git 2.49.0; go1.24.1 darwin/arm64; bun 1.2.9 | no | `git --version`; `go version`; `bun --version` |
| Tracked files | 5,537 | was 5,519 | `git ls-files \| wc -l` |
| `.git` | 1.0 GB; 16 packs, 121,269 packed objects; 2,007 loose objects (21.93 MiB); one `tmp_obj` garbage file | loose count was 1,833 | `du -sh .git`; `git count-objects -vH` |
| Worktrees | 18 entries: the main checkout and 17 under `.claude/worktrees/` | was 22 entries, 20 under `.claude/worktrees/` | `git worktree list` |
| Locked worktrees | 0 | was 3 | `git worktree list --porcelain \| grep -c '^locked'` |
| Live sessions are not locked | Two worktrees hold a live `claude` process (`focused-bassi-248015`, this session, and `nifty-lehmann-452d6e`, the sibling workflow-refinement session); neither is locked | new | `lsof -d cwd` through `lib.sh` `live_in`; `git worktree list --porcelain` |
| Worktree disk | 39 GB under `.claude/worktrees/`: one at 479 MB, eight at 2.0 GB, seven at 2.7 GB, one at 4.0 GB | was 42 GB | `du -sh .claude/worktrees` and `.claude/worktrees/*` |
| Branches | 19 local; 12 merged into `main`; 7 not (`claude/focused-bassi-248015` and six `worktree-agent-*`) | was 21, 17, 8 | `git branch`, `--merged main`, `--no-merged main` |
| The gc trivial case today | Of the 17 worktrees: 6 are not absorbed by `main` (clean, with unmerged commits); 9 are absorbed and dirty (2 to 18 paths each); 2 are absorbed and clean, and both are the live sessions above. So a correct `wk gc` today removes nothing, and the only thing between it and removing two live sessions is the process-inside test | new; supersedes the "17 absorbed, 3 locked" of DESIGN.md section 8 | `lib.sh` `is_absorbed`, `dirty_paths`, `live_in` run over every row of `worktree_rows`, 12:20 EDT |
| The quiet-window test can read a new worktree as old | `nifty-lehmann-452d6e`, made at 12:09, has an empty HEAD reflog, so `lib.sh` `head_moved_at` prints nothing and the age computes as about 56 years | new | `git -C .claude/worktrees/nifty-lehmann-452d6e reflog -1` prints nothing |
| Nested `node_modules` | `web/node_modules` is 1.2 GB in 121,376 files (614 top-level entries); `native/node_modules` is 7.3 MB (`react`, `react-dom`, `.cache`); no other workspace has one | new; DESIGN.md 1.2 and D5 name only the root `node_modules` | `du -sh`; `find web/node_modules -type f \| wc -l` |
| Root `node_modules`, `.next` | `node_modules` 2.3 GB; `web/.next` 2.6 GB, of which `cache` 1.5 GB; `docs/incomplete` 440 MB | no | `du -sh` |
| `web/.next` holds a build | `cache`, `server`, `static`, `types`, `trace`, `dev`, `diagnostics` | no | `ls web/.next` |
| Next.js and `typegen` | `next` 16.3.8; CI runs `bunx next typegen` and writes `native/expo-env.d.ts` with one `printf` | no | `web/package.json:46`; `.github/workflows/ci.yml:188`, `:210`, `:325`, `:327` |
| `bun.lock` on `main` | blob `027889f88b7f141173a3a8233809421806ca38cb` | new | `git rev-parse main:bun.lock` |
| `extensions.worktreeConfig` | `true` | no | `git config --get extensions.worktreeConfig` |
| The main checkout is dirty | 65 status entries | was 69 | `git status --short \| wc -l` |
| No `wk`, no repository, no config yet | `wk` not on PATH; `~/Projects/landing-layer` and `~/.wk` do not exist | no | `which wk`; `ls -d` |

### 0.3 `.claude/settings.local.json` (D6)

| Claim | Verified state | Moved | Citation |
|---|---|---|---|
| Content | The main checkout's file is 106 lines with a `permissions.allow` list and no `hooks` key, last modified 2026-10-04 15:07 | DESIGN.md 1.4 said "untracked and empty"; it is untracked and not empty | `wc -l`; `grep -c hooks`; `stat` |
| Ignored | `.gitignore:53` | no | `git check-ignore -v .claude/settings.local.json` |
| Copied into new worktrees | The two worktrees made at 12:09 hold a copy (5,820 bytes, 12:09); the older `agent-*` worktrees hold none | new | `ls -la .claude/worktrees/*/.claude/settings.local.json` |
| personal-config also writes it | `personal-config setup` merges a tier-ceiling `availableModels` key into it with strategy `merge-json`, never an overwrite | new | `~/Projects/personal-config/src/render/ceiling.ts:5-8`, `:55-62` |

### 0.4 Not measured in this session

The time of a directory `clonefile` of `web/node_modules` (121,376 files) and `native/node_modules` was not measured: the measurement command was refused by the permission check on 2026-10-16. By proportion to the 4.48 s measured for the root's 143,199 files (DESIGN.md 1.3), it is about 3.8 s. Neither the time of `bun install --frozen-lockfile` over a cloned tree nor the time of `bunx next typegen` was measured. Phase 1 measures all four before any timing claim is made.

The budget that follows is an estimate: 2.24 s checkout, 4.48 s root clone, about 3.8 s `web/node_modules` clone, 0.07 s `.next/cache` clone, plus install and `typegen`. That is about 10.6 s before the last two, over the 8 s target in DESIGN.md section 8 if the steps run one after another. BD-4 runs them in parallel; GATE 2 question 3 asks what happens if the measured time still misses.

## 1. Decisions taken since ratification

Each build decision has a one-line reversal. A later deviation cites the number.

- **BD-1. "node_modules" in D5 means all three.** The clone list is `node_modules`, `web/node_modules`, `native/node_modules` and `web/.next/cache`, from config, not code (D19). Without `web/node_modules`, `bun install --frozen-lockfile` writes 1.2 GB of real disk per workspace and the `df` proof fails. Reversal: drop the two nested entries from `template.clone_dirs`.
- **BD-2. The copied files are the five of D5 plus `.claude/settings.local.json`.** The sixth file carries the hook registration (BD-8) into every workspace, so a session working in a `wk` workspace also has the hook; the desktop app already copies it into new worktrees (section 0.3). Reversal: drop it from `template.copy_files`.
- **BD-3. The template is a detached worktree at `main`'s commit.** A branch named `main` is checked out in the main checkout and git refuses a second checkout of it, so the template holds `main`'s commit detached, and a marker file in its private git directory (the `trial_marker` pattern of `lib.sh`) tells `gc` never to touch it. Reversal: a branch `wk/template` moved by `wk template refresh`.
- **BD-4. `wk new` runs the slow steps at the same time.** `git worktree add --no-checkout`, then the checkout and the four clones as concurrent goroutines (each clone target's parent directory is made first; git does not mind an existing untracked directory), then the five file copies, then `bun install --frozen-lockfile` and `bunx next typegen`. Reversal: run them in sequence, if phase 1 shows parallel clones are slower on APFS.
- **BD-5. One lock file guards the template.** `~/.wk/<repo>/template.lock`, taken shared by `wk new` for the length of its clones and exclusive by `wk template refresh` and by `gc --apply` (which must never prune during a `wk new`, DESIGN.md section 6). `flock(2)` through `golang.org/x/sys/unix`. Reversal: none needed for P1; P2's daemon may own the lock instead. (Superseded 2026-10-17: no daemon is built, so the lock stays as written; `docs/incomplete/integration/KILL-TEST.md`.)
- **BD-6. The hook command never fails.** `wk hook worktree-create` reads the input, runs the `wk new` path under the given `name`, and on any error falls back to `git worktree add -b worktree-<name> <dir> <base>` itself; if that also fails it prints the reason on stderr and exits 2. It prints exactly one line on stdout, the absolute, dot-free path. A warm workspace that half-failed (checkout done, a clone failed) is still printed, cold, with the failure on stderr and in `~/.wk/<repo>/hook.log`. Reversal: exit non-zero on any `wk` error, which Claude Code turns into a session with no worktree (section 0.1).
- **BD-7. The hook keeps Claude Code's names.** Directory `.claude/worktrees/<name>`, branch `worktree-<name>`, as Claude Code names its own today (DESIGN.md 1.2), so `/land-worktrees` and `gc` see hook-made and git-made worktrees the same way. The base is the `HEAD` of the input `cwd`; phase 3 checks that against a worktree Claude Code makes with the hook removed. `wk new <row>` keeps `wk-<row>` and `wk/<row>` (dials `workspace.dir`, `workspace.branch`). Reversal: name hook workspaces by row once P2 knows the row.
- **BD-8. `wk hooks install` merges, never overwrites.** It adds one `hooks.WorktreeCreate` entry to `.claude/settings.local.json` in the main checkout and in the template, keeps every other key (the 106-line permissions list, personal-config's `availableModels`), refuses if the file is not valid JSON, and is idempotent. `wk hooks uninstall` removes only its own entry. Reversal: none; overwriting would delete personal-config's merged key.
- **BD-9. No `WorktreeRemove` hook in P1.** Hook-made agent worktrees are then kept by Claude Code (section 0.1) and `wk gc` collects them under its own tests. Reversal: register `WorktreeRemove` calling a `wk rm --if-clean`, a seam in section 4.
- **BD-10. The quiet window reads more than the reflog.** A worktree's last activity is the newest of: the HEAD reflog time, the mtime of its private git directory's `HEAD` and `index`, and the mtime of the worktree's top directory. An empty reflog never means "old" (section 0.2). Reversal: reflog only, as `lib.sh` does, which today misreads a 12-minute-old worktree as 56 years old.
- **BD-11. `gc` deletes a branch only when it is an ancestor of `main`.** After removing a worktree, `gc --apply` runs `git branch -d` on its branch only when `merge-base --is-ancestor` holds; a branch absorbed by content (squash, copy) is kept and listed. Reversal: keep every branch and leave deletion to `/land-worktrees`.
- **BD-12. Config is TOML through `github.com/BurntSushi/toml`.** The Go standard library has no TOML reader. Phase 1 pins the version it finds current and records it. Reversal: JSON at `~/.wk/<repo>.json`, which the standard library reads.
- **BD-13. The module targets the Go on PATH.** `go 1.24` in `go.mod`, matching go1.24.1 (section 0.2), so no toolchain download runs. The app's backend toolchain is separate and untouched. Reversal: raise the directive when PATH moves.
- **BD-14. Every path `wk` takes is absolute.** It refuses a relative path argument and resolves the repository from `git rev-parse --path-format=absolute --git-common-dir`, never from the shell's cwd alone (DESIGN.md section 6). Reversal: none.

Phase 3 added BD-15 to BD-23 (section 7, "Phase 3 answers").

- **BD-15. A warm pool, sized by dials.** `pool.size` full and `pool.sparse_size` sparse slots live under `pool.dir` as detached worktrees with a `wk-pool` marker in their private git directory, which holds the slot's state, shape, commit and template blob; gc skips a marked worktree. A claim, under `~/.wk/<repo>/pool.lock`, removes the marker, then runs `git worktree move` to the workspace's path, then puts it on its branch; a detached `wk pool fill`, started from `/` in a session of its own, tops the pool up after every create. A slot whose template blob or sparse shape no longer matches is never claimed, and the next fill removes it. Defaults 0 and 0; ezhomesteading runs 0 full and 4 sparse, because the burst measurement passed 30 s (As built, phase 3). Reversal: both sizes 0.
- **BD-16. Pattern-sparse by default (amends D8).** `workspace.sparse = true`, and an entry of `workspace.sparse_exclude` with a glob character is a non-cone pattern as written, so the default leaves out only the media under `docs/incomplete` and keeps every markdown. `wk new --full` checks out everything, `--sparse` forces the sparse shape. D8's objection to sparse (a grep for a design decision finds nothing) was about the markdown, which stays. Reversal: `sparse = false`.
- **BD-17. The template follows the base (amends D7, D5).** With `template.follow_base`, a base that moved without a `refresh_on` change gets a light refresh under the exclusive lock: checkout, copies and the post-clone commands, no build. A create, a claim or a pool fill skips its own post-clone commands when it sits at the commit its clones were installed and generated at, and `web/.next/types` (typegen's output, 0.6 MB, relative imports; its absolute paths are comments only) is cloned so the skip is safe. Reversal: `follow_base = false`, which runs the post-clone commands in every workspace not at the template's commit.
- **BD-18. At most `new.max_parallel` creates clone at once.** Each cold create and each pool slot takes one of `create-<n>.lock` for its `worktree add`, checkout and clones; others poll every 100 ms and say once that they wait. Eight unthrottled creates all finished at 46 to 61 s; throttled to 2, pairs finished about every 5.5 s (As built, phase 3). Reversal: a large `max_parallel`.
- **BD-19. Two gc classes for subagent worktrees (amends D9).** A worktree named as Claude Code names a subagent's (`agent-a<16 hex>`) that carries the `wk-agent-stopped` marker of the `SubagentStop` hook is removable when it is clean, unlocked, has no process inside and has been quiet `gc.stopped_quiet_minutes`, whether or not the base holds its commits; its branch is kept unless it is an ancestor of the base (BD-11). One without the marker needs `gc.unmarked_agent_quiet_hours` of quiet, because a running subagent leaves no process in its worktree between tool calls. Every other worktree keeps the E2 rule. `wk hook subagent-stop` marks only the worktree at the input `cwd` whose name is `agent-` and the input `agent_id`, so a subagent without isolation never marks its parent's checkout, and it always exits 0, because exit 2 would keep the subagent running. Reversal: drop the hook; every worktree then follows E2.
- **BD-20. A `WorktreeRemove` hook for explicit discards (reverses BD-9).** Claude Code 2.1.301 runs it only when the owner discards a worktree (Fable finding A, binary string "only an explicit discard may dispatch the remove hook"), so it is never the cleanup for subagents; BD-19 is. `wk hook worktree-remove` removes the worktree with `--force`, since the owner asked for it gone, deletes its branch only when the base holds it, and refuses the main checkout, a path that is not a worktree, the template and pool slots. Reversal: drop the entry; Claude Code then deletes the directory itself.
- **BD-21. `gc --apply` takes the template lock only to prune (amends BD-5).** Removing a worktree does not race a create; only `git worktree prune` can, so holding the lock for the whole pass would stall every create while gc runs. Reversal: take it for the pass.
- **BD-22. Disk-first targets (amend DESIGN.md section 8).** Under 120 MB of `df` per workspace, one cold create under 10 s, and ten creates through the hook at once all warm, with no plain fallback, within 60 s. They replace full under 8 s and sparse under 5 s. Reversal: the old dials.
- **BD-23. A claim at the slot's own commit only attaches the branch.** `git branch` and `git symbolic-ref HEAD`, no `git checkout -b`, which re-stats every file and waited about 5 s per claim behind other creates' clones in the first ten-create burst. At any other commit the claim runs `git checkout -b` and the post-clone commands. Reversal: always `checkout -b`.

## 2. Phases

Driver ceiling for Opus is about 400k, landing from about 300k (`AGENT-PRACTICES.local.md` Part 5). No phase is planned above `full`.

| # | Phase | Driver | Subagents | Est. context | Why that shape |
|---|---|---|---|---|---|
| 1 | Repository, config, template, `wk new` (A1), measurements | Opus | none | comfortable | The CLI is the implementation (D4); everything after calls it. The four unmeasured timings decide whether BD-4 is enough, so they come first |
| 2 | `wk gc`, dry run and `--apply`, and its schedule | Opus | none | comfortable | Removal is the one destructive operation; it gets a phase and a self-test of its own, after workspaces exist to remove |
| 3 | `WorktreeCreate` hook, `wk hooks install`, README, the live proofs in ezhomesteading | Opus | one Sonnet gate runner | full | The hook depends on `wk new`; the live proofs need all three parts. Gate output is long, so a subagent runs the gate list and reports per gate |

D17 assigns P1 to Opus with no review, because a bad workspace fails loudly at the first gate. Order is fixed by dependency: the hook calls `wk new`, and `gc` must exist before the hook starts making worktrees nobody removes.

### Phase 1: repository, config, template, `wk new`

Status: BUILT 2026-10-16, `~/Projects/landing-layer` commit `a0ed906` ("wk: config, template refresh and wk new (p1 phase 1)"), by an Opus 5.5 session (board row 196).

Scope:

1. `git init` a new repository at `/Users/zachshort/Projects/landing-layer` (D3); `go mod init` with the `go 1.24` directive (BD-13); one binary, `cmd/wk`. Zach makes every commit there, as here.
2. Config loader for `~/.wk/<repo>.toml`, where `<repo>` is the base name of the common git directory's parent (`ezhomesteading`), with the dials of section 3 and their defaults; an unknown key is an error, a missing file uses the defaults and says so once. Write `~/.wk/ezhomesteading.toml` with the ezhomesteading values (BD-1, BD-2, the post-clone commands, the placeholder env).
3. `wk template refresh`: take the template lock exclusive (BD-5); make or move the detached template worktree to `main`'s commit (BD-3); copy the five untracked files from the main checkout inside one process and never print them (DESIGN.md section 6); `bun install --frozen-lockfile`; `next build` in `web/` with `gates.env`, so `web/.next/cache` exists; record the `bun.lock` blob hash in `~/.wk/<repo>/template.json`. `wk new` refreshes first when `main:bun.lock` differs from the recorded hash (D7).
4. `wk new <row> [--sparse]`: refuse a row that is not a positive integer, an existing directory or branch, a relative path; then BD-4's order; the sparse variant excludes `workspace.sparse_exclude` through `git sparse-checkout set --no-cone` with negated patterns (D8); print the absolute path and nothing else on stdout.
5. If the full `wk new` of scope item 4, timed end to end with `/usr/bin/time -p`, is over 8 s, build the warm pool (section 7, question 3): `pool.size` workspaces made ahead under neutral names, `wk new` renames one (`git worktree move` and `git branch -m`) and refills the pool in the background; the pool is never cloned while the template lock is held exclusive.
6. Measure, with `/usr/bin/time -p`, each step on its own on this machine: directory `clonefile` of each of the four clone targets, `bun install --frozen-lockfile` over the clone, `bunx next typegen`, and the D3 variant (clone the template's working tree, then `git checkout` the delta, then compare `git status` with a fresh checkout). Record the numbers in this file under phase 1 as an `As built:` note. D3 is built only if its `git status` matches a fresh checkout exactly and the time beats BD-4 (D8).
7. Self-test in Go (`go test`), in the style of `selftest.sh`: a throwaway repository with a tracked tree, a fake `node_modules` with a relative workspace symlink, nested `node_modules`, and ignored files; cases for a full and a sparse `wk new`, a refused relative path, a refused existing row, a refresh triggered by a lockfile change, and a `wk new` that waits on a held refresh lock.

Subagents: none.

Done when:

```bash
cd /Users/zachshort/Projects/wk && test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...
```

plus these proofs a green gate cannot supply:

- The measurements of scope item 6 are written in this file with their command and time. If the full time was over 8 s, the pool of scope item 5 exists and a full `wk new` from it is timed the same way.
- In the self-test repository, a cloned `node_modules` symlink such as `node_modules/@ezh/core` resolves inside the new workspace, not the template (DESIGN.md 1.2, relative links).
- `wk template refresh` run twice leaves one template worktree and one marker.

Watch for: a directory `clonefile` of a tree another process is writing is consistent in shape and inconsistent in content (DESIGN.md section 6), so no clone runs without the shared template lock. And `bun install --frozen-lockfile` over a clone rewrites only changed packages; a lockfile far from the template's means a slow install, which is what the refresh trigger is for.

#### As built, phase 1 (2026-10-16)

**Section 0 re-read at 12:34 EDT.** Free disk 44 GiB (43 at 12:12). `main:bun.lock` is still blob `027889f88b7f141173a3a8233809421806ca38cb`. `node_modules` 2.3 GB, `web/node_modules` 1.2 GB, `native/node_modules` 7.3 MB and `web/.next/cache` 1.5 GB are unchanged. `git worktree list` had 20 entries (18 at 12:20) and 7 `claude` processes were running. Nothing that phase 1 depends on had moved.

**The repository.** `~/Projects/landing-layer`, module `github.com/zach-short/landing-layer`, one binary `cmd/wk`, packages `internal/config`, `internal/gitx`, `internal/clone` (darwin only; other systems refuse), `internal/flock`, `internal/wk` and `internal/testrepo` (the self-test fixture). `go mod tidy` writes the directive as `go 1.24.0`, which is BD-13's `go 1.24`. BD-12: `github.com/BurntSushi/toml` v1.6.0, the current release. `golang.org/x/sys` is pinned at v0.41.0, not the current v0.49.0: v0.42.0 and later require Go 1.25 and v0.49.0 requires 1.26, so a newer pin would break BD-13. Probing v0.42.0 made `go` fetch a go1.26.9 toolchain into the module cache once; every later command ran with `GOTOOLCHAIN=local`. `wk` is installed at `~/go/bin/wk` by `go install ./cmd/wk`.

**The config.** The compiled defaults are generic (D19): `clone_dirs` is `node_modules` alone, and there are no copied files, no commands and no gate env. Every ezhomesteading value is in `~/.wk/ezhomesteading.toml`. `template.refresh_on` is a path (`bun.lock`); `wk new` compares its blob on `workspace.base` with the blob recorded in `~/.wk/ezhomesteading/template.json`. Commands are `{dir, run}` tables with `run` as an argument list. `template.post_clone` runs in the template at refresh and in every new workspace. `template.build` runs at refresh only, with `gates.env` added to the environment. An unknown key, an absolute path, a path with `..`, an empty command, and a `workspace` or `hook` pattern without its `{row}` or `{name}` are all errors. `WK_HOME` moves `~/.wk`, for the self-test.

**How `wk new` fills the worktree.** `git worktree add --no-checkout -b <branch> <dir> <base commit>`, then at the same time: the clones, and either `git read-tree -m -u HEAD` (full) or `git sparse-checkout set --no-cone '/*' '!/docs/incomplete/'` followed by the same `read-tree` (sparse). The self-test and both live workspaces show a clean `git status` after it. The shared template lock is released after the clones, before the copies and the post-clone commands (BD-5). On a failure after `worktree add`, `wk new` leaves the workspace in place and says so. It never removes anything.

**A fact that moved: a fresh install writes no `web/node_modules`.** The template's `bun install --frozen-lockfile` over a fresh checkout wrote a root `node_modules` of 96,147 files (143,199 in the main checkout) and no `web/node_modules` at all. The template's `next build` passed without it. So the 1.2 GB `web/node_modules` in the main checkout is left over from an older install, and BD-1's premise does not hold for a fresh one. The entry stays in `template.clone_dirs`, because a later bun that writes it gets it cloned, and `wk new` prints `skip clone of web/node_modules: the template has none`. Nothing in the design changes.

**Measurements.** All with `/usr/bin/time -p` on this machine, on the template at `0208c8e84`. The single-step clones used a one-call `unix.Clonefile` helper in the session scratchpad on the same APFS volume (`disk3s5`). The directory clonefile was not refused this time.

| Step | Command | real |
|---|---|---|
| First `wk template refresh` (worktree add 2.54 s, install 3.42 s, typegen 1.98 s, build 36.12 s) | `wk --repo <main checkout> template refresh` | 44.37 s |
| Second refresh, no checkout (install 0.27 s, typegen 0.44 s, build 16.81 s) | same | 17.61 s |
| Full checkout alone, into a private index | `GIT_INDEX_FILE=<tmp> git --git-dir=<.git> --work-tree=<tmp> read-tree -m -u main` | 2.42 s |
| Sparse checkout alone | not re-measured; DESIGN.md 1.3 | 0.35 s |
| Clone `node_modules` alone (96,147 files) | `clonebench <template>/node_modules <tmp>` | 4.27 s |
| Clone `native/node_modules` alone | same | 0.00 s |
| Clone `web/.next/cache` alone | same | 0.00 s |
| Clone `web/node_modules` | the template has none | none |
| `bun install --frozen-lockfile` over the clone | step inside `wk new 9901` | 0.40 s |
| `bunx next typegen` | step inside `wk new 9901` | 1.86 s |
| **Full `wk new`**, the first after the refresh, 8 `claude` processes, 5.7 GB free plus inactive memory | `wk --repo <main checkout> new 9901` | **7.05 s** |
| **Sparse `wk new`** | `wk --repo <main checkout> new 9902 --sparse` | **7.29 s** |
| D3: clone the whole template working tree in one call | `clonebench <template> <tmp>` | 4.26 s |
| D3: first `git status` over that clone with the template's index copied | `GIT_INDEX_FILE=<copy> git --git-dir=<.git> --work-tree=<tmp> status --porcelain` | 2.43 s, 0 lines |

Real disk by `df -k /`: the first refresh used 1.26 GB (45,649,040 to 44,331,440 KB available). The full `wk new` used 521 MB (44,296,380 to 43,762,532), under the 600 MB of the phase 3 proof. The sparse one used 97 MB (43,756,332 to 43,656,748).

**What the numbers decide.**

- The warm pool is not built. The full `wk new` measured 7.05 s, under the 8 s of GATE 2 question 3. It stays a seam (section 4).
- BD-4 holds, with less gain than planned. Run in parallel, every clone and the checkout finished at the same moment (4.68 to 4.69 s in `wk new 9901`), including the 7 MB `native/node_modules` that clones in 0.00 s alone, so APFS appears to serialise concurrent clonefile calls on one volume. The sum of the steps run one after another is 9.0 s full and 7.6 s sparse, against 7.05 s and 7.29 s measured.
- D3 is not built (D8). Its `git status` matched a fresh checkout exactly (0 lines in both), but clone plus first status is 6.69 s against 4.73 s for the same part under BD-4. It also has no supported way to register a cloned directory as a worktree, because `git worktree add` needs an empty or missing directory and the clone needs a missing one.
- **The sparse target of under 5 s is out of reach for P1 as built.** The root `node_modules` clone (4.27 s) plus `typegen` (1.86 to 2.56 s) is already over 5 s, whatever the checkout costs. Only the pool reaches it. GATE 2 question 3 builds the pool on the full time only, so this is a question for Zach before phase 3, whose done-when holds both thresholds. **Open question, by Zach's answer in chat on 2026-10-16:** "Leave the sparse 5 s miss as an open question in PLAN.md; I decide before phase 3." Phase 3 does not start its proofs until he has decided.

**Proofs.**

- A cloned `node_modules/@ezh/core` resolves inside the new workspace. The self-test `TestClonedWorkspaceLinkResolvesInsideWorkspace` checks the link is still `../../packages/core` and resolves to the workspace's own `packages/core`, not the template's. Live: in both `wk-9901` and `wk-9902`, `readlink node_modules/@ezh/core` printed `../../packages/core` and `pwd -P` inside it printed the workspace's own `packages/core`.
- `wk template refresh` run twice leaves one template and one marker. Self-test `TestRefreshTwiceLeavesOneTemplateAndOneMarker`. Live, after the second refresh: one `wk-template` entry in `git worktree list --porcelain`, one marker at `.git/worktrees/wk-template/wk-template`, and 0 lines of `git status --porcelain` in the template.
- The gate line of this phase is green: `gofmt -l .` empty, `go vet` clean, `go test -race -count=1 ./...` ok for `cmd/wk`, `internal/config` and `internal/wk`, and `golangci-lint run ./...` (v2.12.2, default linters, no config file) with 0 issues.

**The self-test.** Throwaway repositories in `t.TempDir()`, git isolated from the owner's config by `GIT_CONFIG_GLOBAL=/dev/null` and `GIT_CONFIG_NOSYSTEM=1`, a tracked tree with `packages/core`, `web/` and `docs/incomplete/`, ignored `.env` files, and a fake install that writes a relative workspace symlink, a nested `web/node_modules` and a record of the lockfile it read. Cases: full and sparse `wk new`; stdout holds only the path and no copied secret reaches stderr; bad rows (`abc`, `0`, `-3`, `07`, `../x` and others); relative `--repo`, `WK_HOME` and `Open` paths; an existing row and an existing branch; a refresh triggered by a lockfile change on `main`, and none when it did not change; a `wk new` that waits on a held refresh lock and does not touch the repository until the lock is free; two refreshes; a dirty template and a foreign directory at the template path, both refused; config defaults, overrides, and nine kinds of bad file. The self-test makes its fixture commits with `git commit` inside `go test`. The commit guard refused the same command when this session ran it in a scratch repository from `Bash`, so no experiment was run that way.

**Left on disk for Zach.** The template `.claude/worktrees/wk-template` (kept on purpose). The two measurement workspaces `.claude/worktrees/wk-9901` and `wk-9902`, on branches `wk/9901` and `wk/9902`: the classifier refuses worktree removal from a session, and `wk gc` does not exist yet. Remove them with `git worktree remove` and `git branch -D` when convenient.

### Phase 2: `wk gc`

Status: BUILT 2026-10-16, `~/Projects/landing-layer` commit `5eca622` ("wk: gc dry run, apply and launchd schedule (p1 phase 2)"), by an Opus 5.5 session (board row 196).

Scope:

1. `wk gc` (dry run, the default): for every worktree but the main checkout, the template and any marked trial, print one line with the verdict and the reason. Removable only when all hold: the branch head is absorbed by `main` (the `is_absorbed` test of `lib.sh`: ancestor, or `merge-tree --write-tree main <head>` gives `main`'s own tree); `dirty_paths` is empty (tracked changes and untracked non-ignored files); not locked; no process has its cwd inside (one `lsof -d cwd` snapshot per run, as `lib.sh` `lsof_snapshot`); last activity older than `gc.quiet_minutes` by BD-10. Everything else is "kept" with its reason, and an absorbed but dirty worktree is "kept, run /land-worktrees" (D9, E2).
2. `wk gc --apply`: take the template lock exclusive (BD-5), re-check every test for each candidate at the moment of removal, `git worktree remove` without `--force`, then BD-11 for the branch. Never `git worktree prune` while any `wk new` holds the lock.
3. The schedule of D9, as GATE 2 question 2 decided (section 7): before P2's daemon exists, a launchd agent `~/Library/LaunchAgents/<label>.plist` written by `wk gc schedule install`, `StartInterval` from `gc.apply_interval_minutes`, running `wk gc --apply` with output to `~/.wk/<repo>/gc.log`. P2's daemon replaces it (section 4). (Superseded 2026-10-17: the launchd agent stays, by PD-5 of `PLAN-P2-P4.md`, and no daemon is built, by `docs/incomplete/integration/KILL-TEST.md`.)
4. Self-test cases: absorbed by ancestry, absorbed by content only (branch kept by BD-11), not absorbed, dirty tracked, dirty untracked, ignored-only changes (removable), locked, a process standing inside (a child `sleep` with its cwd there), an empty reflog with recent files (BD-10), the template and a trial marker, and a removal that races a `wk new` holding the lock.

Subagents: none.

Done when: the phase 1 gate line, green, plus:

- `wk gc` in ezhomesteading names, as removable, exactly the set that the `lib.sh` reference classification gives in the same minute (`is_absorbed`, `dirty_paths`, `live_in`, BD-10 quiet window; the shell loop in section 0.2's citation), and names every absorbed but dirty worktree as "kept, run /land-worktrees". On 2026-10-16 at 12:20 that set was empty: the two absorbed and clean worktrees held live sessions.
- Neither this session's worktree nor any other worktree with a live `claude` process is ever listed as removable, checked while at least two sessions are open.

Watch for: live desktop sessions are not locked today (section 0.2), so the process-inside test is the only guard for them; a dry run that lists one as removable is a stop, not a warning.

#### As built, phase 2 (2026-10-16)

This note was written by a later Opus 5.5 session than the one that built the phase. Every check below was run again by that session at 13:44 to 13:52 EDT, on a `git archive` of `5eca622` in its scratchpad, with `GOTOOLCHAIN=local`. The binary for the live run was built from that copy. The installed `~/go/bin/wk` is still the phase 1 binary and has no `gc`.

**The code.** `internal/wk/gc.go` holds the classifier, the dry run and `--apply`. `internal/procs` takes the one `lsof -d cwd` snapshot per run. `internal/gitx` gained the absorption, dirty-path, lock and prune calls. `internal/wk/schedule.go` writes and loads the launchd agent. The config gained a `[gc]` table with `quiet_minutes` and `apply_interval_minutes`, both 60 by default, and a value of 0 for either is an error, because a quiet window of 0 lets `gc` remove a worktree made a second ago. The agent label is `com.github.zach-short.wk.gc.<repo>`, its log is `~/.wk/<repo>/gc.log`, and `wk gc schedule install` refuses a binary that lives in a temporary directory, so a `go run` or test binary never becomes the scheduled one.

**The self-test.** 17 `gc` cases in `internal/wk/gc_test.go` cover every case of scope item 4: removal by ancestry with its branch, absorption by content only with the branch kept (BD-11), unlanded commits, dirty tracked and untracked, ignored-only changes (removable), a locked worktree, a child process standing inside, an empty reflog with recent files (BD-10), a worktree made just now, the template and a trial marker, and an `--apply` that waits on a `wk new` holding the lock. Beyond the scope list: the main checkout is never judged, the reads never touch a worktree's index, a detached HEAD is removed with no branch, an entry whose directory is gone is pruned, `--apply` judges again at the moment of removal, and the dry run changes nothing. Two schedule cases cover the agent install and the refused transient binary.

**Gate.** The phase 1 gate line is green on `5eca622`: `gofmt -l .` empty, `go vet` clean, `go test -race -count=1 ./...` ok for `cmd/wk`, `internal/config`, `internal/procs` and `internal/wk` (129 s for `internal/wk`), and `golangci-lint run ./...` with 0 issues.

**Proof: the dry run matches the reference classification.** `wk --repo <main checkout> gc` ran at 13:51:52 and a shell loop over `lib.sh` (`worktree_rows`, `is_absorbed`, `dirty_paths`, `lsof_snapshot` and `live_in`, with the BD-10 newest-of rule and a 60-minute window) ran at 13:51:59. Of 24 worktrees besides the main checkout, both named the same two as removable, `wk-9901` and `wk-9902` (the phase 1 measurement workspaces, absorbed by ancestry, clean, idle 68 minutes). Both named the same ten absorbed but dirty worktrees as "kept, run /land-worktrees" (2 to 41 paths each, `row-195` among them). Both skipped the template by its marker and one `/land-worktrees` trial by its marker. The remaining twelve were kept by both: six for unlanded commits, the rest for activity inside the 60-minute window.

**Proof: no live session is removable, partly met.** Six `claude` processes were running at 13:52 (seven a minute later), so at least two sessions were open. None of them stood inside an ezhomesteading linked worktree (their working directories were the main checkout and other repositories), so this run could not show the process guard holding back a live `claude` session. The guard did fire on other processes: `friendly-mahavira-e3d45e` (a `git` process) and `row-195` (two `zsh` and a `node`) were both kept with the processes named. The self-test `TestGCKeepsAWorktreeWithAProcessInside` covers a process standing inside. **Left owed:** a dry run while a `claude` session stands in a linked worktree that is absorbed, clean and quiet. Phase 3's last proof re-runs the dry run and can carry it.

**Not done in this phase.** `wk gc schedule install` was not run on this machine: there is no `com.github.zach-short.wk.gc.*` agent in `~/Library/LaunchAgents` and no `gc.log`. `wk gc --apply` was not run against ezhomesteading, so `wk-9901` and `wk-9902` are still on disk. Install the new binary with `go install ./cmd/wk` before either.

**Outside the plan.** Two later commits extend `gc` beyond scope: `b49b8d7` adds `wk gc --json` and a hive page, and `500f7ce` adds `wk viz`, which serves that page with live `gc` data. Neither is part of phase 2, and neither was checked by this note.

**Facts found while building, from the first note.** The session that built phase 2 wrote its own note at 13:04 to 13:05 EDT, in commit `dbcc06ddc` on the detached HEAD of the worktree `nifty-lehmann-452d6e`. That commit never reached `main`; the note above replaced it. Its list of facts was on no other page, so board row 199 copied it here word for word on 2026-10-16, before Zach discarded that worktree. Since then the schedule has run (`SLIM-SCOPE.md` section 1.5), and BD-19 gives subagent worktrees their own quiet rule for the finding in the last item.

- `git diff HEAD` rewrites the worktree's `index`. A throwaway `go test` in the session scratchpad reset an index mtime to two days back and ran each read: `git diff --name-only HEAD` and `git status --porcelain` moved it, `git ls-files --others` did not, and with `GIT_OPTIONAL_LOCKS=0` neither of the first two moved it. The index mtime is one of BD-10's clocks, so without the variable each gc pass would mark every worktree as active and an hourly `--apply` would never remove anything. `gitx.Run` now sets `GIT_OPTIONAL_LOCKS=0` for every git call `wk` makes, and the self-test `TestGCReadsDoNotTouchTheIndex` holds it. `lib.sh` `dirty_paths` makes the same write; `plan.sh` does not read the index mtime, so the skill is not misled by it, but a `/land-worktrees` run resets every worktree's BD-10 clock.
- `lib.sh` ignores an `lsof` failure, and an empty snapshot makes every live session look idle. `procs.Snapshot` refuses any snapshot that does not list `wk`'s own process, so gc stops with an error in place of removing a worktree it cannot see into. The exit code alone is not the test, because `lsof` exits 1 when it cannot read another user's process.
- BD-10 also takes the mtime of every changed path, as `plan.sh`'s busy test does. That changes only the reason printed: a worktree with a changed path is never removable.
- A worktree on a branch whose `logs/HEAD` is deleted still has a HEAD reflog: git falls back to the branch's own log. An empty HEAD reflog occurs on a detached worktree, or on a branch made with no reflog (section 0.2). The self-test for BD-10 uses a detached worktree for that reason.
- **Finding for Zach: an idle session holds no process.** At 13:05 EDT, `lsof -d cwd` listed a `claude` process in the folder of each of the three sessions running a turn (the main checkout, `~/Projects/personal-config`, `~/Projects/kessik`) and in this session's worktree. It listed none for "Alternate Timeline Decision-Support App", an open session that was not running, in its scratch folder. So the process-inside test protects a session only while it runs a turn. A session left idle for `gc.quiet_minutes` in a clean worktree whose commits `main` holds loses that worktree at the next `--apply`. Git loses nothing (every commit is in `main` and no path is changed), but the folder is gone when Zach returns to the session. `wk gc schedule install` was therefore not run. **Open question, before the install:** accept this, or have `wk` read the desktop app's list of open sessions, or lock the worktrees of open sessions, or raise `gc.quiet_minutes`.

### Phase 3: the hook, its install, the README, the live proofs

Status: BUILT 2026-10-16, `~/Projects/wk` commit `46ecc15` ("wk: hooks, pool, pattern-sparse default and subagent gc (p1 phase 3)") on branch `p1-phase-3` (worktree `~/Projects/landing-layer-phase3`, from `e757aa9`), by an Opus 5.5 session (board row 196). Merged into `main` as `30989b5` (merges `4a2f7fb` and `30989b5` bring in `main`'s guided setup `5e4368b` and README `29cb987`; the conflicts in `cmd/wk/main.go`, `internal/hive/style.go` and `README.md` were resolved by this session; the full gate line was green after the first merge, and the second changed only `README.md`); `~/go/bin/wk` rebuilt from `30989b5`. Scope grew by Zach's answers (section 7, "Phase 3 answers"): BD-15 to BD-23.

Scope:

1. `wk hook worktree-create`: read the JSON input from stdin; take `name` and `cwd`; run BD-7's names through the `wk new` path; BD-6's fallback; one absolute, dot-free path on stdout; log to `~/.wk/<repo>/hook.log`.
2. `wk hooks install` and `wk hooks uninstall` by BD-8, writing the main checkout's and the template's `.claude/settings.local.json`. Never `~/.claude/settings.json` and never the tracked `.claude/settings.json` (D6).
3. README with the provenance section of DESIGN.md section 9 as D18 amends it, the article cited at `https://www.anthropic.com/engineering/redesigning-source-control`, and the note that `du` reports a clone at full size while `df` tells the truth (DESIGN.md section 6).
4. The live proofs below, in ezhomesteading, on a refreshed template.

Subagents: one Sonnet subagent runs the gate list in the new workspace and returns pass, fail or skip per gate with the last 20 lines of any failure. It runs inside the workspace `wk new` made, which is its own worktree, and edits nothing; the diffstat of the workspace is checked when it returns (`~/.claude/rules/model-routing.md`, subagent rules).

Done when: the phase 1 gate line, green, plus every proof below, each recorded in this file with its output:

- `wk new <row>` on a cold machine returns in under 8 s full and under 5 s with `--sparse`, measured with `/usr/bin/time -p` (DESIGN.md section 8). Cold means: the first `wk new` after `wk template refresh`, with no earlier `wk new` in that boot's file cache; the `claude` process count and free memory at the time are recorded beside the number.
- `df -k /` moves by under 600 MB between before and after one full `wk new`, with the workspace kept.
- Every `CLAUDE.md` gate runs in the new workspace with no manual step, and each result equals the same gate's result on a clean `git archive` copy of the same commit set up by the fresh-checkout recipe. `sqlgate` and the DB suites are recorded as `skip` with the reason `EZH_TEST_DB is unset`, never as `pass`; a gate red on HEAD itself (such as the `packages/core` typecheck of 2026-10-15) is red in both and named.
- `wk gc` dry run, run at the end, names exactly the absorbed and clean worktrees that the reference classification names in the same minute (phase 2), and no live session.
- A subagent started with `isolation: "worktree"` in a session in the main checkout gets a warm worktree from the hook: `node_modules` and `web/node_modules` exist without an install, and `df` moved by under 600 MB.
- With `wk` made to fail (an environment variable the hook command honours for this test only), the same subagent still gets a plain worktree, and the failure is in `hook.log`.
- A desktop-app worktree session is started after the install, and the result is recorded: either its worktree came from the hook, or the desktop app bypasses it, in which case A1 and `gc` cover it and an `As built:` note under D4 says so.

Watch for: a hook that fails leaves a session with no worktree, because Claude Code has no fallback (section 0.1); test the fallback before the install touches the main checkout's file. And `CLAUDE_CODE_RESTRICT_PERSONAL_CONFIG` makes Claude Code ignore a hook's folder from personal files; if a session runs under it, the hook is refused there, and that is a config fact to record, not a bug to work around.

#### As built, phase 3 (2026-10-16)

The session started on Haiku 5.5, saw the Opus assignment in section 2, did no work, and was switched to Opus 5.5 before the build. Every number below was taken by this session on this machine between 14:08 and 15:20 EDT, with `GOTOOLCHAIN=local`. The raw proof log is in the session's scratchpad (`phase3-proofs.md`); everything that matters from it is here.

**The code.** On top of phases 1 and 2: `internal/wk/pool.go` (BD-15, BD-23), `internal/wk/hook.go` (`wk hook worktree-create`, BD-6 and BD-7), `internal/wk/agent.go` (`wk hook subagent-stop` and `wk hook worktree-remove`, BD-19 and BD-20), `internal/wk/hooks.go` and `internal/settings` (`wk hooks install|uninstall`, BD-8, three events), `flock.TryTake` (BD-18), the light refresh in `template.go` (BD-17), the pattern excludes and the `Checkout` shape in `new.go` (BD-16), the two gc classes and the narrower lock in `gc.go` (BD-19, BD-21), the new dials in `internal/config`, `README.md`, and the module rename of D21 (`github.com/zach-short/wk`), which Zach asked for in this session. The binary is installed at `~/go/bin/wk` from this branch.

**The self-test.** New cases: settings (key order and the file mode kept, idempotent install, an old wk entry replaced, refusals for bad JSON, a non-object, trailing data and another tool's `WorktreeCreate` or `WorktreeRemove` hook, `SubagentStop` shared with another tool, uninstall that removes only wk's entries and an emptied `hooks` key); the pool (each kind filled and skipped by gc, a claim at the slot's commit and after main moved, stale and old-shape slots never claimed and replaced, a stopped fill's slot removed, a pool trimmed to a lower size, two claims at once, status); the hook (warm from the session's HEAD, the `WK_HOOK_FAIL` fallback, a broken config, reuse of its own worktree and of a left-over branch, bad names); the stop marker (its own worktree only, never the parent's or another agent's); gc (a stopped subagent removed after 15 minutes with its branch kept, a dirty one kept, an unmarked one kept for a day, removals that do not wait for a create, a prune that does); the remove hook (forced removal, branch kept or deleted by BD-11, refusals); creates (pattern excludes keep markdown, `--full`, the light refresh, `follow_base = false`, the create slot wait); and the command line (`--sparse` with `--full` refused, the stop hook exits 0 on any input). Every refusal test now runs from an empty directory, after one run of the old `--full` refusal case made a template and `wk-1` in `~/Projects/wk`, the test's own repository; this session removed both and branch `wk/1`.

**Gate.** Green on the final tree: `gofmt -l .` empty, `go vet` clean, `go test -race -count=1 ./...` ok for `cmd/wk`, `internal/config`, `internal/hive`, `internal/procs`, `internal/settings`, `internal/viz` and `internal/wk` (39.7 s), and `golangci-lint run ./...` with 0 issues.

**Measurements.** All with `/usr/bin/time -p`; `df` is `df -k /`. Seven to eleven `claude` processes ran throughout, with 4.0 to 4.7 GB free plus inactive memory.

| What | Result |
|---|---|
| Cold full `wk new` before BD-16 to BD-18, pool off, first after `wk template refresh` | 9.93 s; clones and checkout all done at 6.3 s; 523 MB |
| Pool claim, before BD-16: full, sparse; and from a worktree 7 commits behind main | 0.20 s, 0.29 s; 2.88 s |
| Eight cold sparse creates at once, no throttle (Fable measurement 1) | 46 to 61 s each, 61 s wall, 111 MB each. One sparse checkout alone took 39.75 s and one `worktree add` 10 s: clones and file writes on one APFS volume queue behind each other |
| One warm `bun run build` in a fresh workspace (measurement 3) | 67 s, 2.6 GB peak RSS, 0.9 GB written |
| Cold pattern-sparse create, the first after main moved (with the light refresh) | 7.92 s, 100 MB |
| Cold pattern-sparse create at the template's commit | 5.59 s, 99 MB; 0 images and all 158 markdown files under `docs/incomplete`; status clean |
| Pool fill of 4 pattern-sparse slots, removing 3 old-shape slots | 52.11 s; 742 MB freed net |
| Ten hook creates at once (BD-22), before BD-23 | all warm, 0 fallbacks, 4 claims; slowest 45.56 s; 978 MB for 10 plus the refill of 4 |
| The same, after BD-23, with main moved past the slots' commit | all warm, 0 fallbacks, 4 claims; slowest 47.92 s; branch attach 0.13 to 1.07 s |

**Proofs against the done-when.** Section 7 and BD-22 replace the first two bullets' numbers.

- Timing: one cold create under 10 s holds (5.59 s; 7.92 s with a light refresh). Ten at once within 60 s holds twice (45.56 s and 47.92 s). The original 8 s full cold target failed before the redesign (9.93 s), which is why it was replaced.
- Disk: under 120 MB per workspace holds (99 to 111 MB). The original 600 MB bound held for full workspaces too (512 to 536 MB with the refill).
- Gates: every `CLAUDE.md` gate ran in `wk-9904` with no manual step and matched a clean `git archive` copy of `d534a262c` set up by the fresh-checkout recipe, run by a Sonnet subagent (its own worktree; status and diff empty afterwards, checked by this session). `sqlgate` and the DB suites were `skip`, "EZH_TEST_DB is unset", in both (326 `--- SKIP` each). The `packages/core` typecheck that was red on 2026-10-15 is green at this commit. One difference: the archive's web build failed, because a deny rule refused the recipe's `cp` of `web/.env`; with the six CI placeholder values of `gates.env` the build passed in both trees.
- gc: the final dry run and the `lib.sh` reference classification, run 20 s apart at 15:16, agreed on 70 of 71 worktrees. The one difference is BD-19 working: `agent-a02d20bbe0ab981c8`, absorbed, clean and idle 272 minutes, is removable by E2 and kept by wk ("a subagent worktree with no stop marker waits 24 hours"). Two more rows differed only because another session's subagent claimed pool slot `wk-pool-bb0ae5e2` between the two runs; wk judged the old path after the move and kept it, which is the safe side.
- The phase 2 debt: while the probe session "gc-probe" worked inside `wk-9902` (absorbed, clean, quiet since phase 1, removable a minute earlier), `wk gc` kept it with one reason, "a process is working in it: 9078 disclaimer, 9080 claude, 9321 node and 7 more". The real `gc --apply` that followed kept it the same way.
- Subagent: a Haiku subagent with `isolation: "worktree"` got `agent-a99035391e1eee7e2` warm from the hook in 0.26 s, with `node_modules`, `native/node_modules`, `web/.next/cache` and the `.env` files and no install; `df` moved 512 MB with the refill. `web/node_modules` was absent, as phase 1 found: a fresh install writes none, so the template has none. After BD-16, `agent-a0e4b4b91ae6b9fae` came warm in 0.24 s, pattern-sparse, and the `SubagentStop` hook marked it at 15:12:55.
- Fallback: with the hook command prefixed `WK_HOOK_FAIL=1`, a subagent got the plain worktree `agent-a5e981e1d4b3959b6` in 2.33 s, and `hook.log` says "warm workspace failed: WK_HOOK_FAIL is set" and "plain fallback". The first try, spawned in the same message as the settings edit, still ran the old command: Claude Code reloads settings with a short delay.
- Desktop app: a desktop worktree session started after the install ("Landing layer: desktop worktree probe") got `heuristic-lederberg-e9891e` from the hook, warm, in 0.80 s. The desktop app does not bypass the hook. Its sessions run the bundled Claude Code 2.1.299, not the 2.1.301 that section 0.1 was read from.
- `CLAUDE_CODE_RESTRICT_PERSONAL_CONFIG` was not set in this session.

**Hook inputs, measured (Fable measurements 4 and 5).** The `WorktreeCreate` input holds `session_id`, `transcript_path`, `cwd`, `scratchpad_dir`, `prompt_id`, `hook_event_name` and `name` (`agent-<agent id>`): no `agent_id` and no `agent_type`, so a hook cannot tell a review agent from a builder. The `SubagentStop` input holds `agent_id`, `agent_type`, `cwd` (the subagent's own worktree when it ran with isolation), `permission_mode`, `effort`, `stop_hook_active`, `agent_transcript_path`, `background_tasks` and `session_crons`.

**Installed and applied.** `wk hooks install` wrote the three entries into the main checkout's and the template's `.claude/settings.local.json`; the first install appended 13 lines and left the 105 before them unchanged, and a second changed nothing. `wk gc schedule install` loaded `com.github.zach-short.wk.gc.ezhomesteading`, every 900 s, log `~/.wk/ezhomesteading/gc.log`. One `wk gc --apply` at 15:17 removed 10 of 72 worktrees (`affectionate-lehmann-405ffd`, `focused-bassi-248015`, `heuristic-lederberg-e9891e`, the three `hooktest-*`, `wk-9901`, `wk-9903`, `wk-9904`, `wk-9905`), deleted the eight branches that `main` holds, and freed 5.4 GB by `df`.

**Left on disk.** This session's test workspaces, all clean, which the schedule removes when their windows pass: `burst-0` to `burst-9` and `burstb-0` to `burstb-9` (hook-made, 60 minutes), `wk-9881` to `wk-9888`, `wk-9910`, `wk-9911` and `wk-9902` (60 minutes), `agent-a0e4b4b91ae6b9fae` (stopped, 15 minutes), and five subagent worktrees made before the stop hook existed, which wait 24 hours: `agent-afd0496a90e310fde`, `agent-a99035391e1eee7e2`, `agent-a6afb7f1cf76d335f`, `agent-a5e981e1d4b3959b6`, `agent-ae960b7fa91d4b492`. The pool holds four sparse slots.

**Incident, 15:18 to 15:28.** A `go install ./cmd/wk` from `wk` `main`, before this branch was merged, replaced `~/go/bin/wk` with a build that had no `hook` command, so every hook call failed (a session or subagent asking for a worktree got none) and the scheduled gc stopped on the unknown config keys. The "wk: scope a one-build-at-a-time gate semaphore" session reported it; this session's reinstall from the merged tree at 15:28 ended it. The hooks run whatever binary sits at `~/go/bin/wk`, so it must always be built from a commit that has them; the README now says so.

**Owed.** Zach's ratification in `DESIGN.md` of the D7, D8 and D9 amendments and of section 8's targets; a runtime week of the schedule (read `gc.log`); and the follow-up rows of section 4 (the build semaphore, `wk slim`). The Fable review also proposed taking the screenshots out of git (436 MB of the 845 MB pack is `docs/incomplete` blobs); that is Zach's call and not in this plan.

## 3. Dials

Every dial lives in `~/.wk/<repo>.toml`; nothing below is a constant in code (D19). The P1 dials are the subset of DESIGN.md section 5 that P1 reads, plus the ones BD-1, BD-2 and BD-4 need. P2 to P4 dials stay in DESIGN.md section 5 until their plans.

| Dial | Default for ezhomesteading | Source |
|---|---|---|
| `template.path` | `.claude/worktrees/wk-template` | DESIGN.md section 5 |
| `template.refresh_on` | the blob hash of `bun.lock` on `main`, or `wk template refresh` | DESIGN.md section 5; D7 |
| `template.follow_base` | `true`: a moved base gets a light refresh (checkout, post-clone, no build) | BD-17 |
| `template.clone_dirs` | `node_modules`, `web/node_modules`, `native/node_modules`, `web/.next/cache`, `web/.next/types` | D5; BD-1; BD-17 |
| `template.copy_files` | `web/.env`, `backend/.env`, `native/.env`, `web/next-env.d.ts`, `native/expo-env.d.ts`, `.claude/settings.local.json` | D5; BD-2 |
| `template.post_clone` | `bun install --frozen-lockfile` at the root; `bunx next typegen` in `web/` | D5; DESIGN.md section 3 |
| `template.build` | `bun run build` in `web/` with `gates.env`, at refresh only | D7 |
| `gates.env` | the six CI placeholder values of the `web` job | DESIGN.md 1.4, section 5 |
| `workspace.dir` | `.claude/worktrees/wk-<row>` | DESIGN.md section 5 |
| `workspace.branch` | `wk/<row>` | DESIGN.md section 5 |
| `workspace.base` | `main` | DESIGN.md section 3 |
| `workspace.sparse` | `true`; `wk new --full` overrides it | D8; BD-16 |
| `workspace.sparse_exclude` | `docs/incomplete/**/*` for `png`, `svg`, `json`, `jpg`, `jpeg`, `gif`, `webp`, `riv`; a directory entry still excludes the whole directory | D8; BD-16 |
| `hook.dir` | `.claude/worktrees/<name>` | BD-7 |
| `hook.branch` | `worktree-<name>` | BD-7 |
| `gc.quiet_minutes` | 60 | DESIGN.md section 5 |
| `gc.stopped_quiet_minutes` | 15 | BD-19 |
| `gc.unmarked_agent_quiet_hours` | 24 | BD-19 |
| `gc.apply_interval_minutes` | 15 (generic default 60) | D9; BD-19 |
| `new.max_parallel` | 2 | BD-18 |
| `new.max_seconds_cold`, `new.max_mb_per_workspace`, `new.burst_size`, `new.burst_max_seconds` | 10, 120, 10, 60; proof thresholds, never timeouts | BD-22, replacing `new.max_seconds_full` 8 and `new.max_seconds_sparse` 5 |
| `pool.size`, `pool.sparse_size` | 0 full, 4 sparse (generic default 0 and 0) | BD-15 |
| `pool.dir` | `.claude/worktrees/wk-pool-<slot>` | BD-15 |

## 4. Seams reserved, deliberately not built

- The warm pool and the `WorktreeRemove` hook were seams here and are built in phase 3 (BD-15, BD-20).
- D3, clone the template's working tree and let `git checkout` write the delta. Measured in phase 1, built only if it holds (D8).
- One build at a time across workspaces (`wk run --slot build <cmd>`, P4 option V moved early). Phase 3 measured one warm web build in a fresh workspace at 67 s, 2.6 GB peak RSS and 0.9 GB written to disk, so 50 agents that each build need about 45 GB and more memory than the machine has. A follow-up row, not phase 3.
- `wk slim`: not built, by Zach's answer of 2026-10-16 (`SLIM-SCOPE.md`). The 2.2 GB each and 37 GB over 17 worktrees were `du` figures. Measured by APFS private size, every `node_modules` is a clone of `~/.bun/install/cache` and holds 0 private bytes; the 17 old worktrees held 14,409 MiB, all in the checkout (478 MiB each, 419 of it tracked images under `docs/incomplete`) and in `web/.next/cache` (5,990 MiB over 8). Two one-off commands for Zach, a cache trim and a pattern-sparse checkout, replace the command; `SLIM-SCOPE.md` "Answers" holds both.
- A shared read-only checkout for review agents. The `WorktreeCreate` input carries no `agent_type` (phase 3 measurement 5), so it would be a skill convention, not a hook feature.
- The daemon, the unix socket and `wk up` (P2); P1 runs as a plain CLI, and P2's daemon takes over the `gc` schedule from the launchd agent. (Superseded 2026-10-17: none of the three is built at any phase, by `docs/incomplete/integration/KILL-TEST.md`, and the launchd agent keeps the schedule, by PD-5 of `PLAN-P2-P4.md`.)
- Leases, `wk claim` and the `PreToolUse` hook (P2). P2's plan must also decide how a hook registered in one checkout's `.claude/settings.local.json` reaches a session working in another worktree; BD-2's copy is P1's answer for the `WorktreeCreate` hook only.
- `wk status` with a `df`-based disk report (P4 owns `wk status`).
- The four-way sort of dirty files (`base`, `gone`, `past`, `new`) in Go (option E1); folding `/land-worktrees` is a later row (D13).
- Any repository but ezhomesteading proven; the config shape allows it (D19).
- Linux and Windows: `clonefile(2)` is APFS only; `wk` refuses on other systems (DESIGN.md section 2).

## 5. Repo hazards, with live numbers

From DESIGN.md section 6, filtered to P1, with the numbers of section 0.

- The main checkout is dirty (65 entries) and shared; it is the source of the five copied files and nothing else. The template is never the main checkout (D7).
- Live sessions are not locked (0 locked of 17 worktrees, 2 live). `gc` depends on the process-inside test and BD-10.
- 9 worktrees are absorbed but dirty; `gc` keeps all 9 and names `/land-worktrees` (D9).
- `du` on a clone reports full size; 39 GB under `.claude/worktrees/` by `du` is not 39 GB of real disk once clones exist. Only `df` measures the proof.
- Reading or printing a `.env` file is refused by the auto-mode classifier; `wk` copies the three inside one process and never prints them (DESIGN.md section 6).
- The golangci-lint cache is shared; in the gate proof, run `lintgate` with `GOLANGCI_LINT_CACHE=$(mktemp -d)` (`CLAUDE.local.md`, "Gates that lie").
- `go test` prints `ok` with DB suites skipped; record `skip`, never `pass`.
- Two `next build` runs over one `.next` print a bare `Failed to compile.`; the template's build runs only under the exclusive template lock, and never in the main checkout.
- Free disk is 43 GiB (section 0.2); a refreshed template adds one more full `node_modules` set until clones replace the cold worktrees.

## 6. Session protocol

Follow `AGENT-PRACTICES.local.md` (Stage 5 onward, Parts 5 to 7) and `CLAUDE.local.md`. Specific to this project:

- Work in `/Users/zachshort/Projects/wk` for code (DESIGN.md D21); in ezhomesteading, write only `docs/incomplete/landing-layer/` and the untracked files `wk` itself manages (BD-2, BD-8). No tracked rule file changes (DESIGN.md, "Rules that survive unchanged").
- Never `git commit`, `git push`, `git add -A` or `git add .` in either repository while `~/.claude/rules/commits.md` says so; print the two blocks per repository at hand-back. No attribution line anywhere.
- Each phase ends with an `As built:` note in this file and the phase status set to `BUILT` with the commit hash Zach gives.
- Cite no ledger step numbers in this folder; cite `DESIGN.md` sections and commands instead. The workflow refinement (`docs/incomplete/workflow-refinement/DESIGN.md`, D5) freezes the ledger's step log, and every "step N" citation must resolve after that freeze.

## 7. GATE 2 answers

Asked 2026-10-16 in chat in one batch, answered the same day by Zach. Each answer is quoted by its option label.

- Phases and order: "Approve 3 phases (Recommended)". Phase 1, then 2, then 3, as in section 2.
- What runs `gc --apply` before P2's daemon exists: "launchd agent (Recommended)". `wk gc schedule install` writes the LaunchAgent in phase 2; P2's daemon replaces it. (Superseded 2026-10-17: the LaunchAgent stays, by PD-5 of `PLAN-P2-P4.md`; no daemon is built.)
- If phase 1 measures a full `wk new` over 8 s: "Build a warm pool (Recommended)". Phase 1 scope item 5; dial `pool.size`, default 2.
- The P1 dials of section 3, with BD-1 and BD-2: "Accept all (Recommended)".

### Phase 3 answers, 2026-10-16

Asked in the phase 3 session (board row 196), in chat. Quoted as given.

- The sparse "under 5 s" miss of phase 1: "Build the pool now". Then the schedule: "Install it (Recommended)". Then one `wk gc --apply` from the session: "Yes, run --apply once".
- A peer session ("Next item") later relayed a second answer it said Zach typed there: "Use 8 s for sparse and drop the pool. Send my answer to the phase 3 session so it stops the pool work." Asked to choose between the two in the phase 3 session, Zach asked for the tradeoffs instead, then said storage is his largest problem ("saving 7s is cool and all but is this 1GB per worktree"), that he will run 50 or more agents at once, and asked for a Fable subagent to decide, with the cost of a refactor ruled out as a reason.
- On the Fable decision (BD-16 to BD-22) and its six measurements: "go". That answer supersedes both pool answers above: the pool stays as dials, sized by the burst measurement (BD-15).
- The module rename of D21 in the phase 3 branch: "Yes, apply the github.com/zach-short/wk module rename in landing-layer-phase3 and fix the README install line."

D7, D8 and D9 of `DESIGN.md` are amended by BD-16, BD-17 and BD-19, and section 8's timing targets by BD-22. `DESIGN.md` is not edited by this phase; Zach ratifies the amendments there.
