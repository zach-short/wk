# Scope: build slots, a machine-wide cap on heavy gates (`wk run --slot`)

Status: RATIFIED 2026-10-16. Zach chose every recommendation of section 6 ("Answers" at the end): `heavy` 1 and `check` 2, a `PreToolUse` hook that inserts `wk run`, a trim of `web/.next/cache` after each build, a FIFO wait with no limit, machine-wide places, and a hook that fails open. Written 2026-10-16 by an Opus 5.5 session started from the row 196 phase 3 session. It answers the seam "One build at a time across workspaces" of `PLAN.md` section 4, which moves a small piece of `DESIGN.md` option V (P4) earlier. Sections 1 to 6 are the scope as Zach read it; "Answers" records what he chose. Model check: a scope is Default-tier work (`AGENT-PRACTICES.local.md` Part 4), and this session runs Opus 5.5. No code was written. Section 1.8 lists what this session changed on disk.

## The short version

The phase 3 note measured one warm web build in a fresh workspace at 67 s, 2.6 GB peak RSS and 0.9 GB written. Re-measured on 2026-10-16 between 15:31 and 15:45 EDT, under the real load of that afternoon (one-minute load average 10 to 27, 9 to 13 Claude Code sessions, another session's `go test -race` running in each of three process snapshots), the first `bun run build` in a fresh `wk` workspace took 63 s at load 11, 130 s at load 19, and 179 s at load 10 while this session's own text search held a core, peaked at 1.8 to 2.4 GiB in one process, and left 758 MiB of disk that no other file shares, 722 MiB of it webpack's cache. A cold `lintgate`, as `CLAUDE.local.md` tells sessions to run it, peaks at 1.8 GiB and keeps six cores busy for 37 s. Every other gate in the `CLAUDE.md` list peaks near 1 GiB: `tsc` and eslint in web and native, vitest, and `go test -race`.

A slot caps memory and CPU. It does not cap disk. Each build's 758 MiB stays in its workspace until `gc` removes the workspace, whatever the slot count, so 50 workspaces that each build once hold about 37 GiB against 38 GiB free (`df` at 15:43). Only a trim of `web/.next/cache` at the end of the build bounds disk during a burst; option E puts that trim in the slot command, where the process that wrote the bytes knows they are done.

Two facts change the shape of the request. First, the cloned template cache did not make the fresh build warm: the same build with the cloned cache moved aside took 63 s and 112 s of CPU (at a lower load), and a rebuild in a workspace with its own cache took 56 s and 68 s, against 130 s and 162 s with the clone. Second, the agents cost more memory than their builds: the 9 `claude` processes running at 15:34 held 2,251 MiB, about 250 MiB each, so 50 sessions would need about 12 GiB before one gate runs. A slot makes many builds take turns; it does not make 50 sessions fit on 16 GB. Subagents run inside their parent's process and are the unit that scales.

## 1. What exists, verified 2026-10-16

Read between 15:27 and 15:47 EDT, in the main checkout at `65b51149b` and later `77b033a5d`, unless the citation says otherwise. Sizes are MiB (1,048,576 bytes) unless a line says MB.

### 1.1 How the numbers were taken

Each gate ran in a workspace this session made with `wk new` (section 1.8), through a script in the session scratchpad that starts the command in its own process group under `/usr/bin/time -l`, samples the group every 0.5 s with `ps -axo pgid=,rss=` for summed RSS and process count, and reads `vm_stat` for free, inactive and speculative pages. The gates ran one after another, never two at once from this session.

`/usr/bin/time -l` reports the largest single process, not the tree. For `next build` the two agree (1,815 against 1,816 MiB), because one node process does the work. For vitest and `go test -race` they do not (333 against 1,068 MiB; 454 against 1,105 MiB), because many short processes run at once. The 0.5 s sample can miss a short peak, so the tree figures are lower bounds.

`df -k /` cannot attribute disk to one build on this machine today. With no build running it fell by 227 MB in 60 s (15:30:03 to 15:31:03) and by 113 MB in the next idle 60 s (15:34:03 to 15:35:03), and during build 2 it rose by 451 MB because another session freed space. Disk per build is therefore read two ways: the sum of the sizes of files under `web/.next` modified after the build started, and APFS private size (bytes no other file shares), read with `getattrlist(2)` and `ATTR_CMNEXT_PRIVATESIZE` by a read-only C walker in the scratchpad, the method `SLIM-SCOPE.md` section 1.1 calibrated against `df`.

### 1.2 The machine during the measurements

| Claim | Verified state | Citation |
|---|---|---|
| Memory, cores | 16 GB, 10 cores | `sysctl -n hw.memsize hw.ncpu`, 15:27:59 |
| Load at the start | 24.13, 13.64, 9.46; two `go test -race -count=1 ./...` from other sessions at once, in `~/Projects/landing-layer-phase3` and in `agent-a4be01d96a23ad438/backend` | `uptime`; `pgrep -fl 'go test'`, 15:27:59 |
| Swap | 2,805 MB used of 4,096 MB at 15:28, 2,645 to 2,850 MB during the builds; 131,686 pageouts since boot; 40% free by `memory_pressure` | `sysctl vm.swapusage`; `memory_pressure` |
| Free, inactive and speculative memory with no gate of this session running | 3,965 to 5,172 MiB | `vm_stat`, at the start of each run |
| Other sessions' heavy gates during this session | `go test -race` in each of three process snapshots (15:28, 15:36, 15:40); eslint and `next build` only from this session | `ps -axo pid,rss,etime,command` filtered for gate commands |
| Claude Code sessions | 9 `claude` processes at 15:34, 2,251 MiB RSS in all, 158 to 352 MiB each, all the desktop app's bundled 2.1.299; 12 or 13 at other moments | `ps -o rss= -p $(pgrep -x claude)` |
| Free disk | 42,673,992 KB (40.7 GiB) at 15:28; 40,117,044 KB (38.3 GiB) at 15:43 | `df -k /` |

### 1.3 The web build, re-measured

| Run | Where | Start | 1-min load | Wall s | CPU s | Largest process | Tree peak | Memory left, lowest | Written under `web/.next` | Private after |
|---|---|---|---|---|---|---|---|---|---|---|
| Phase 3, measurement 3 | a fresh workspace | 2026-10-16, before 15:20 | not recorded | 67 | not recorded | 2.6 GB | | | 0.9 GB | |
| Build 1 | `wk-9931`, fresh | 15:31:03 | 10.08 | 179.1 | 166.8 | 1,936 | 1,919 | 2,563 of 3,969 | 768 MB, 821 files | |
| Build 2 | `wk-9932`, fresh | 15:36:01 | 19.15 | 130.2 | 162.1 | 1,815 | 1,816 | 2,574 of 3,991 | 755 MB, 820 files | 758, of it `cache` 722 |
| Build 3 | `wk-9931` again, same tree, its own cache | 15:38:11 | 19.47 | 55.6 | 67.8 | 1,457 | 1,591 | 3,009 of 4,630 | 267 MB, 817 files | 801 for both builds, of it `cache` 770 |
| Build 4 | `wk-9933`, fresh, cloned cache moved aside | 15:43:28 | 11.25 | 62.8 | 112.2 | 2,452 | 2,440 | 3,187 of 4,831 | | 758, of it `cache` 722 |

Notes on the table:

- Build 1 is the measurement the task asked for. It overlapped, from about 15:31:40 to 15:33:45, a text search this session ran over a 600 MB file of strings from the Claude Code binary, which held one core; builds 2 and 4 are the clean figures.
- The command is the `CLAUDE.md` one, `bun run build` in `web/`, which runs `next build --webpack` (`web/package.json`, `scripts.build`) with the workspace's own copied `web/.env`.
- Of the 768 MB written by build 1, 734 MB is eight webpack pack files under `web/.next/cache/webpack/`, each rewritten whole; the build output itself is about 36 MB (`server` 21, `static` 5.6, `trace` 8.6, by `du -sk` after build 1).
- The template's `web/.next/cache` is 764 MiB with 0 private, and an unbuilt pool slot's `web/.next` is 765 MiB with 0 private: every clone shares the template's blocks until a build rewrites them.
- The cloned template cache did not make a first build warm. With it, build 2 used 162 s of CPU; with it moved aside, build 4 used 112 s; with a cache the workspace wrote itself, build 3 used 68 s. Builds 2 and 4 ran at loads 19 and 11, so the pair does not isolate the clone's effect, but neither shows a gain from it. The template cache was written at `.claude/worktrees/wk-template/web` at 12:41 (`ls -la` of `wk-template/web/.next/cache/webpack/server-production`), for an older commit and another absolute path; which of the two invalidates it was not tested.
- Peak memory went the other way: 2,452 MiB with no cache (build 4), 1,815 to 1,936 MiB with the clone, 1,457 MiB with the workspace's own cache. Build 4 at load 11 is close to the phase 3 note (67 s, 2.6 GB).
- Every first build left the same 758 MiB private, cache or no cache: webpack writes a new cache of about 722 MiB either way.

### 1.4 The other gates

Each ran once in `wk-9931` after build 3, in the order below, with the `CLAUDE.md` spelling.

| Gate | Command | Start | 1-min load | Wall s | CPU s | Largest process | Tree peak | Processes |
|---|---|---|---|---|---|---|---|---|
| web `tsc` | `bunx tsc --noEmit` in `web/` | 15:39:07 | 21.83 | 45.8 | 30.5 | 950 | 933 | 2 |
| web lint | `bun run lint` in `web/` | 15:39:53 | 27.18 | 43.2 | 48.2 | 1,041 | 1,035 | 3 |
| native `tsc` | `bunx tsc --noEmit` in `native/` | 15:40:37 | 22.52 | 15.4 | 20.6 | 1,018 | 1,011 | 2 |
| native lint | `bun run lint` in `native/` | 15:40:52 | 20.18 | 43.0 | 64.1 | 1,102 | 1,101 | 3 |
| vitest | `bun run test` at the root | 15:41:35 | 16.52 | 18.7 | 101.5 | 333 | 1,068 | 13 |
| backend tests | `go test -race -count=1 ./...` in `backend/`; DB suites skip, `EZH_TEST_DB` is unset | 15:41:54 | 23.83 | 46.0 | 82.6 | 454 | 1,105 | 12 |
| `lintgate`, cold | `GOLANGCI_LINT_CACHE=<new dir> go run ./cmd/lintgate` in `backend/`, as `CLAUDE.local.md` tells sessions to run it; "252 findings across 3 linters, all at ratchet" | 15:44:31 | 10.02 | 37.1 | 236.9 | 1,892 | 1,905 | 12 |

None of these wrote under `web/.next`. The cold `lintgate` left a 23 MB lint cache in its new directory. `go build ./...`, `go vet ./...`, `synchelp`, `bun scripts/codemods/08-verify.ts`, the parity check and `madge` were not measured. `go test` writes to the shared `~/Library/Caches/go-build`, which `df` cannot separate from the noise of section 1.1.

### 1.5 What `wk` already has

| Claim | Verified state | Citation |
|---|---|---|
| No run or slot command | `wk` at `30989b5` has `template refresh`, `new`, `gc`, `viz`, `hive`, `pool`, `hooks` and `hook`; no `run` | `cmd/wk/main.go`, `usage`, at `30989b5` |
| A counting semaphore already exists | `takeCreateSlot` holds one of `new.max_parallel` exclusive `flock(2)` files `create-<n>.lock` under `~/.wk/<repo>/`, tries each with `TryTake`, polls every 100 ms, and says once that it waits; whichever waiter polls first after a release wins, so there is no queue order | `internal/wk/new.go:121-138` at `30989b5` |
| `flock` locks die with their holder | `Take` and `TryTake` lock an open file; `Release` closes it; a process that dies closes its files, so no lock outlives its holder | `internal/flock/flock.go:14`, `:36`, `:68-69` at `30989b5` |
| An unknown config key is an error | `config.Load` refuses a file with any key it does not know | `internal/config/config.go:143` at `30989b5` |
| The design already names these limits | Option V: one `next build` at a time machine-wide, `tsc` runs in parallel up to a dial, gating pauses under a free-memory dial; dials `gates.next_build_concurrency` 1, `gates.parallel_tsc` 2, `gates.min_free_mb` 2048. D16 ratifies "one `next build` at a time machine-wide". None of the three dials is in the code | `DESIGN.md:249`, `:289-291`, `:341`; `git grep -E 'parallel_tsc\|next_build_concurrency\|min_free_mb' 30989b5` finds nothing |
| The word "slot" is taken twice | Pool slots (`pool.dir = ".claude/worktrees/wk-pool-{slot}"`, `wk pool status`) and the create slots of `new.max_parallel` | `README.md:43`, `:130` at `30989b5`; `internal/wk/new.go:121` |
| Hooks `wk` registers | `WorktreeCreate`, `WorktreeRemove`, `SubagentStop`, in the main checkout's untracked `.claude/settings.local.json` (D6, BD-8), copied into every workspace (BD-2) | `.claude/settings.local.json`, `hooks` key |

### 1.6 What Claude Code gives a slot

| Claim | Verified state | Citation |
|---|---|---|
| One `PreToolUse` hook on `Bash` today | personal-config's commit guard, which walks the command's words and refuses with exit 2 | `~/.claude/settings.json`, `hooks.PreToolUse`; `~/.claude/hooks/personal-config/commit-guard.sh:1-15` |
| A `PreToolUse` hook can rewrite the call | `hookSpecificOutput.updatedInput` replaces fields of `tool_input`. In 2.1.301 a rewrite is refused only "for calls that run in a different environment than the hook", with that reason sent to the model | Context7 `/anthropics/claude-code`, `plugins/plugin-dev/skills/hook-development/SKILL.md`, "Format PreToolUse Hook Output"; installed binary 2.1.301, the function that handles `updatedInput` for `PreToolUse` and the message beside it |
| Exit codes | 0 passes, 2 blocks and sends stderr to the model, any other code is a non-blocking error and the call runs. So a hook that crashes fails open unless it exits 2 | Context7, same `SKILL.md`, "Exit Codes" |
| A long foreground command is not killed | In this session (desktop, bundled 2.1.299), a foreground `Bash` call that passed its 120 s timeout returned "Command did not complete within its 120s timeout and was moved to the background (ID: ...) ... You will be notified when it completes." | this session's own call at about 15:33 |
| Versions | The CLI is 2.1.301; desktop sessions run the bundled 2.1.299 | `claude --version`; `ps` of the `claude` processes |

Not verified: whether a subagent's foreground call is moved to the background the same way, and what happens to it if the subagent ends its turn first; how Claude Code stops a background task (signal, and to which process group); whether `updatedInput` from a `.claude/settings.local.json` hook is applied to a subagent's call. Each is a build-time check (section 5).

### 1.7 How agents run gates today

| Claim | Verified state | Citation |
|---|---|---|
| The gate lines are compound commands | `cd web && bunx tsc --noEmit && bun run lint && bun run build`; `cd backend && go build ./... && go vet ./... && go run ./cmd/lintgate && go run ./cmd/synchelp --check && go run ./cmd/sqlgate && go test -race ./...` | `CLAUDE.md:62`, `:65` |
| Two gate runs per delegated row | The `/delegate` builder runs every gate in its worktree, and the auditor "re-run[s] every gate" in its own | `~/.claude/skills/delegate/SKILL.md:84-90`, `:133` |
| Package scripts | web `build` is `next build --webpack`, `lint` is `eslint . --max-warnings 41`; native `lint` is `eslint . --max-warnings 25`; root `test` is `vitest run`. `tsc`, `go test` and `lintgate` are not package scripts | `web/package.json`, `native/package.json`, `package.json`, `scripts` |
| `web/package.json` is owned by open rows | Row 194 lists it in "Files it owns" and waits on row 54, "which owns `web/package.json`" | `PASSOFF.md`, row 194 |
| Skills are generated | The skills carry a personal-config stamp; their source is `~/Projects/personal-config`, not this repository | `~/.claude/skills/scope/SKILL.md:1` |

### 1.8 What this session changed

It made three workspaces with a build of `wk` at `46ecc15` in its scratchpad (the installed binary was unusable at the time, section 5): `wk-9931`, `wk-9932` and `wk-9933` on branches `wk/9931`, `wk/9932` and `wk/9933`, each from a pool slot, each followed by a pool refill. It built web in all three, ran the gates of section 1.4 in `wk-9931` (the cold `lintgate` with its lint cache in a new directory of the scratchpad), and in `wk-9933` moved `web/.next/cache` to `web/.next/cache-aside` before the build. All three are clean in `git status`; their changes are ignored build output, about 0.8 GiB private each, which `gc` frees when it removes them after its quiet window. Before the measurements it sent the parent session one message about the incident of section 5. It wrote this file and nothing else in the repository.

## 2. What this is, and what this is not

### What this is

A `wk run --slot <class> -- <command>` command that runs a command only while it holds one of a fixed number of machine-wide places for its class, waits in a stated way when none is free, passes the command's exit status, output and signals through unchanged, and can trim named build output when the command ends. Also in scope: the way agents are made to go through it (option C), a status view of who holds and who waits, the dials, a self-test, and a README section.

### What this is not

- Not P4. No daemon, no socket, no batch gate, no trial workspace. The slot files are the seam P4's daemon takes later: its trial gates take the same `heavy` and `check` places, so the batch gate and the agents' gates share one cap (option F).
- Not a cap on Claude Code sessions. Section 1.2's 250 MiB per session is outside any gate.
- Not a change to which gates agents run. "The builder and the auditor each build" is `/delegate`'s rule, in personal-config; making only one of them build is a skill change for Zach, and P4 is the long answer.
- Not an edit to a tracked file of this repository: `CLAUDE.md`, `web/package.json`, `ci.yml`, any rule file (DESIGN.md section 2). The hook goes in `.claude/settings.local.json` (D6).
- Not CI, Vercel or EAS. None of them runs `wk`.
- Not the trim of the old worktrees. `SLIM-SCOPE.md` settled that as one-off commands for Zach and `wk slim` as not built (its "Answers", 2026-10-16). Option E here is about bytes a slotted build writes from now on.
- Not Zach's own terminal. A command he types runs unslotted unless he types `wk run`.
- Not leases (P2) or landing (P3).

## 3. Options, with the strongest argument against each

Each option carries a recommendation. The recommendation is a proposal; the decision is Zach's, in section 6.

### A. Which commands take a slot

The measurements of sections 1.3 and 1.4 fall in two groups. About 2 GiB: `next build` (1.8 to 2.4 GiB) and a cold `lintgate` (1.8 GiB, six cores). About 1 GiB: `tsc` and eslint in web and native, vitest and `go test -race`.

- A1. Only `next build`, one class. For: the slowest gate, and the only one with a known collision (two builds over one `.next` print a bare `Failed to compile.`, DESIGN.md section 6). Against: a cold `lintgate` costs as much memory and more CPU, and the five 1 GiB gates run on every gate line; ten agents in `tsc` or eslint at once is about 10 GiB, unslotted.
- A2. Two classes by measured weight: `heavy` for `next build`, `bun run e2e` (which builds and starts servers, `web/playwright.config.ts:32-40`) and `lintgate`; `check` for `tsc`, eslint, vitest and `go test`. The brief's `wk run --slot build` is `heavy` under this name. For: every gate measured above 0.9 GiB is capped, and a heavy command, which holds its place longer, never queues behind checks. Against: native `tsc` (15 s) waits behind 46 s checks in the same class, and the matcher of option C has to know about a dozen spellings, not two.
- A3. One class for all of them, capacity 2 or 3. For: simplest. Against: two builds can run at once, the measured worst case (about 4.8 GiB, more than was free at most samples), or a build waits behind a queue of checks.
- A4. Weighted: each command takes a number of units (heavy 2, check 1) from one pool sized in GiB. For: matches the measurements directly. Against: a take of two units must be all or nothing, so a heavy command can starve while single units free one at a time; it needs the queue of option D to be fair, and it is the most code.
- Recommended: A2.

### B. How many places, on 16 GB

The machine had 3,965 to 5,172 MiB free, inactive or speculative with no gate of this session running and 2.6 to 2.9 GB of swap in use (section 1.2). A build took that down to 2,563 MiB at its lowest.

- B1. `heavy` 1, `check` 2: the design's own numbers (`gates.next_build_concurrency` 1, `gates.parallel_tsc` 2). The worst case is one heavy command and two checks at their peaks together, about 2.4 + 2 × 1.1 = 4.6 GiB, which is more than was free at most of the samples (3,965 to 5,172 MiB); how often the peaks meet was not measured. For: P4 inherits the same numbers. Against: throughput. At one build of 63 to 130 s, the heavy class passes 28 to 57 builds an hour, fewer when `lintgate` runs take turns with them; with two builds per delegated row that is about 14 to 28 rows an hour, and the last of 50 agents waiting for a build waits 51 to 106 minutes.
- B2. `heavy` 1, `check` 3. For: checks are short and about 1 GiB each. Against: 5.7 GiB worst case is more than was free at any measured moment.
- B3. Fixed places plus a free-memory floor: a command starts only when free, inactive and speculative memory is above `slots.min_free_mb`. For: adapts to Xcode, simulators and other load the slot cannot see. Against: the figure moved by 1,644 MiB within one build and by 1,207 MiB between idle samples, so a floor near the measured values flaps, and a floor checked at start reserves nothing.
- Recommended: B1, with the floor of B3 as a dial that is off (0) until a runtime week shows it is needed.

### C. How agents are made to use it

- C1. A `PreToolUse` hook on `Bash` that rewrites the command: before each simple command the matcher recognises, it inserts `wk run --slot <class> --`. The web gate line becomes `cd web && wk run --slot check -- bunx tsc --noEmit && wk run --slot check -- bun run lint && wk run --slot heavy -- bun run build`. For: no agent, skill or prompt changes; the `cd` still runs in the session's shell, so the working directory carries over as before; each command holds its place only for itself; the agent sees the wait in its output. Against: the rewrite parses shell, and the parse is partial by nature, as the commit guard's history shows (`commit-guard.sh:9-14`); a command the matcher misses runs unslotted, and an insertion in the wrong place changes what the command means. A wrong exit status through `wk run` would turn a red gate green, which is a silent failure.
- C2. A `PreToolUse` hook that refuses an unwrapped heavy command with exit 2 and prints the exact wrapped command to run instead, as the commit guard refuses and teaches. For: nothing is rewritten behind the agent's back. Against: every fresh subagent pays one refused call per gate spelling, about two to four calls per subagent, forever; an agent that retypes a compound line by hand may wrap it differently.
- C3. Wrap the package scripts: `"build": "wk run --slot heavy -- next build --webpack"`. For: no hook. Against: `web/package.json` is tracked and owned by rows 194 and 54; Vercel and CI run the same script where there is no `wk`; `tsc`, `go test` and `lintgate` are not scripts (section 1.7).
- C4. Instructions only: the `CLAUDE.md` gate block, `/delegate` and pass-off prompts say to type `wk run --slot`. For: no code beyond `wk run`. Against: `CLAUDE.md` is a tracked rule file this layer may not edit; the skills are personal-config's; a forgotten wrapper fails silently, and a 50-agent fan-out forgets.
- C5. Shims on `PATH` for `next`, `tsc` and `go`. Ruled out: `bun run` puts `node_modules/.bin` first on `PATH`, so a shim never sees `bun run build`.
- Recommended: C1, with these limits. It inserts only where the parse is unambiguous (a plain simple command at the top level of a `&&`, `||`, `;` or `|` list, after any leading `VAR=value` words), and otherwise leaves the command unchanged and appends it to `~/.wk/slots/missed.log`, so the matcher grows from evidence. It never rewrites a command that already contains `wk run`, and it inserts the binary's absolute path, as the three hook entries of section 1.5 do, so a session's `PATH` does not matter. And `wk run` itself is usable by hand, so C4's wording can be added later at no cost.

### D. What an agent sees while it waits

- D1. Block until a place frees. `wk run` prints one line on stderr when it starts to wait (class, the holder's workspace, command and age, and the agent's position in line), and one when it starts the command (time waited). No time limit by default. After 120 s the foreground call moves to the background and the agent is told when it ends (section 1.6). For: no retry loop, no turn spent polling; the agent can do other work. Against: a subagent's behaviour on that timeout is not verified (section 1.6), and an agent that does not wait for the notice may report a gate it never saw finish.
- D2. Fail fast: exit 75 (`EX_TEMPFAIL`) at once with "no build slot free, N waiting". For: the agent decides. Against: agents retry in a loop or run the command without the wrapper, which is the outcome the slot exists to stop.
- D3. Block up to `slots.max_wait_seconds`, then exit 75. For: bounds the worst case. Against: with B1 and 50 agents the honest wait is over an hour, so any limit short of that converts waits into D2's retries.
- Order of service, inside D1: either "any" (the polling of `takeCreateSlot`, whoever polls first after a release wins) or "fifo" (each waiter writes a ticket file under `~/.wk/slots/<class>/queue/` and holds a `flock` on it, so a dead waiter's ticket is seen as dead; only the first k live tickets may take a place). For "fifo": a wait measured in tens of minutes needs a position the agent can read, and "any" lets one waiter lose every race. Against: about twice the code of "any", and a ticket directory to keep clean.
- Recommended: D1 with "fifo", `slots.max_wait_seconds` 0, and a line in the `/delegate` brief (Zach's file) asking subagents to pass the `Bash` tool's longest timeout on gate lines, until the subagent case of section 1.6 is verified.

### E. What happens to disk after a slotted build

- E1. Nothing. The slot bounds memory; disk is freed when `gc` removes the workspace. For: no second destructive path in the tool, the stance `SLIM-SCOPE.md` S0 took for the old worktrees. Against: `gc` keeps every dirty or unlanded workspace with no end date, and a burst of 50 workspaces that each build holds about 37 GiB at once against 38 GiB free (section 1.2).
- E2. A matcher entry may carry a trim list; the two build spellings carry `web/.next/cache`. When such a command exits, with any status, `wk run` deletes the listed paths in the workspace it ran in. About 722 MiB freed per build, about 36 MiB left. For: it bounds disk during a burst, which only this option does; it runs once, at the moment the bytes are known to be done with, in the workspace whose own command wrote them, so it needs no quiet window and no process test. Against: a second build in the same workspace runs with no cache, in build 4's shape (63 s, 112 s of CPU, 2,452 MiB peak) instead of build 3's (56 s, 68 s, 1,457 MiB), so a builder that fixes a build error and rebuilds pays about 44 s more CPU and 1 GiB more peak each time. And `bun run e2e` needs `.next/server`, so its entry carries no trim.
- E3. As E2, but replace the deleted cache with a fresh clone of the template's. For: free on disk while unwritten. Against: section 1.3 shows the cloned cache did not make a first build faster, so the clone buys nothing; and it pins that template generation's 765 MiB after the template's next build (`SLIM-SCOPE.md` section 5).
- Recommended: E2, trimming only `web/.next/cache`, with a flag so a session can keep its cache (`wk run --keep`) when it expects to rebuild.

### F. Where the places live

- F1. Machine-wide: lock files under `~/.wk/slots/`, capacities in a new machine file `~/.wk/machine.toml`; the matcher (which spellings map to which class) in each repository's `~/.wk/<repo>.toml`, because spellings are a repository fact (D19). For: memory is a machine fact, and D19 plans several repositories; an old `wk` binary never reads the machine file, so adding it cannot break the hooks the way section 5's first hazard did. Against: a second config file, and a repository can no longer choose its own capacity.
- F2. Per repository: `~/.wk/<repo>/slots/` and `[slots]` in `~/.wk/<repo>.toml`. For: one config file, as today. Against: two repositories building at once each get the full cap, which is the oversubscription the slot exists to stop; and a new table in the repository file makes any older binary reject the whole file.
- Recommended: F1. P4's daemon takes the same files for its trial gates.

### G. When `wk` itself is broken

- G1. Fail open: if the hook cannot run, cannot read its config or cannot parse a command, it exits 0 with no rewrite and logs why; if `wk run` cannot open the slot directory, it runs the command unslotted with one line on stderr. For: a missed slot costs memory pressure, which is loud and passes; a hook on every `Bash` call that fails closed turns any `wk` bug into a stop of every command of every agent. Against: a broken `wk` stops capping without anyone noticing until the machine swaps.
- G2. Fail closed: the hook exits 2 on any error, as D12 does for leases. For: no silent loss of the cap. Against: D12 protects correctness (two writers on one file); a slot protects capacity, and the 15:18 incident of section 5 would have stopped every `Bash` call of every session for about ten minutes.
- Recommended: G1, with `wk slots` showing the count of misses and errors since the last start, so a broken matcher is visible.

## 4. Dials

Capacities live in `~/.wk/machine.toml` and the matcher in `~/.wk/<repo>.toml` (option F1); nothing below is a constant in code (D19).

| Dial | File | Recommended default | Why |
|---|---|---|---|
| `slots.dir` | machine | `~/.wk/slots` | one directory for every repository |
| `slots.heavy.capacity` | machine | 1 | D16; the measured heavy peaks are 1.8 to 2.4 GiB |
| `slots.check.capacity` | machine | 2 | `gates.parallel_tsc` 2 of the design; about 1 GiB each |
| `slots.order` | machine | `fifo` | option D |
| `slots.max_wait_seconds` | machine | 0, no limit | option D3's argument |
| `slots.min_free_mb` | machine | 0, off | option B3; revisit after a runtime week |
| `slots.match`, class `heavy` | repository | `next build`, `bun run build`, `bun run e2e`, `playwright test`, `go run ./cmd/lintgate` | sections 1.3, 1.4 and 1.7 |
| `slots.match`, class `check` | repository | `tsc`, `bunx tsc`, `eslint`, `bun run lint`, `vitest`, `bun run test`, `go test` | sections 1.4 and 1.7 |
| `trim_after` on a match entry | repository | `["web/.next/cache"]` on `next build` and `bun run build` only | option E2; never `server` or `static`, never on `bun run e2e` |
| `hook.fail_open` | repository | `true` | option G1 |
| `hook.timeout_ms` | repository | 200 | DESIGN.md section 5; the hook runs on every `Bash` call |

## 5. Hazards this work walks into

- A binary and its config can disagree, and every hook goes with them. From 15:18 to about 15:28 on 2026-10-16, `~/go/bin/wk` was a build of `wk` `main` at `5e4368b`, which has no `hook` command and refuses ten keys of `~/.wk/ezhomesteading.toml` ("unknown keys: gc.stopped_quiet_minutes, ... template.follow_base"). `wk hook worktree-create` exited 2, so every `isolation: "worktree"` subagent and desktop worktree session got no worktree (Claude Code has no fallback, `PLAN.md` section 0.1). The phase 3 session reinstalled from the merged tree; the next good hook line in `~/.wk/ezhomesteading/hook.log` is a warm create at 15:28:50 (line 339), and the binary from `30989b5` was installed at 15:33. A `Bash` hook widens the blast radius from worktree creation to every command, which is why options F1 and G1 matter.
- A gate whose status changes through the wrapper is a silent failure. `wk run` must return the command's exit status, pass stdin, stdout and stderr through untouched, and pass signals on. The self-test needs a failing build, a killed build and a build whose output is piped.
- Process groups. If `wk run` starts the command in a new process group, a stop of the agent's shell that signals its own group misses the build, and the build runs on after its place is released. `wk run` must leave the child in the shell's process group (no `Setpgid`) and forward the signals it receives. How Claude Code stops a task is not verified (section 1.6).
- Lock inheritance. If the child inherits the slot's file, the place is held until every descendant exits, including a server a gate started and left running. Go opens files close-on-exec, so the place belongs to `wk run` alone; the cost is that a `wk run` killed with `SIGKILL` frees its place while the build continues.
- Nested runs. A slotted command that calls `wk run` for the same class waits on itself when the capacity is 1. `wk run` sets `WK_SLOT_HELD=<class>` and a nested run of a held class passes straight through.
- Compound and nested shells. `bash -c "..."`, `$(...)`, `eval` and quoted strings are where an insertion goes wrong; option C1 leaves them alone and logs them.
- The subagent case of the 120 s timeout is not verified (option D). Until it is, a subagent waiting for a build may end its turn before its gate result arrives.
- The cloned template cache. Builds 2 and 4 show no gain from D5's clone of `web/.next/cache` for a first build (section 1.3), and a lower peak with it. Whether to keep cloning it is row 196's call, not this row's; with option E2 the clone is replaced by the build's own cache and then deleted, so it matters only for the first build's memory.
- `go test -race` runs up to ten test binaries at once by itself (`-p` defaults to the core count); one `check` place is about 1.1 GiB of them, not one process.
- Sessions outside the hook: Zach's terminal, worktrees made before BD-2 that carry no `.claude/settings.local.json`, and cloud sessions. They run unslotted.
- The word "slot" already names pool slots and create slots (section 1.5). A status line that says "slot" without its class will confuse.
- Throughput, not only safety. Option B1's heavy class passes about 14 to 28 delegated rows an hour. A slot makes a 50-agent fan-out survivable; it does not make it fast. P4's one build per batch is the throughput answer.
- Other sessions write in this folder. `SLIM-SCOPE.md` is another session's file and is not edited here.

## 6. Open questions, the GATE 1 batch

1. Classes and places (options A and B): two classes, `heavy` 1 (build, e2e, cold `lintgate`) and `check` 2 (recommended); `next build` alone, 1 place; one class of 2 for everything.
2. How agents go through it (option C): a `PreToolUse` hook that inserts `wk run` before each recognised command (recommended); a hook that refuses and prints the wrapped command; instructions only.
3. Disk after a build (option E): trim `web/.next/cache` when a build command ends (recommended); leave it to `gc`.
4. The rest as recommended: waiting blocks in a queue with no limit (D1, fifo), places machine-wide with a machine file (F1), the hook fails open (G1), and the dials of section 4. Yes (recommended); or name the ones to change.

## 7. Board row proposal, not yet appended

The next free row on 2026-10-16 at 15:45 is 199 (`PASSOFF.md` ends at row 198; row 197 is the workflow refinement, `0f534e494`). `SLIM-SCOPE.md` section 7 proposes a row for 199 too, so whoever appends second takes 200; re-read the board first.

| # | Task | Status | Model | Lane | Waits on | Files it owns |
|---|---|---|---|---|---|---|
| 199 | `wk run --slot`: a machine-wide cap on heavy gates (`heavy` 1, `check` 2), a `PreToolUse` hook that routes gate commands through it, and a trim of `web/.next/cache` after each build; scope in `docs/incomplete/landing-layer/SLOTS-SCOPE.md` | OPEN | Opus, with a Fable review of the hook's rewrite and of `wk run`'s exit status and signal handling | tooling | no row 196 phase in flight in `~/Projects/wk`; GATE 1 passed 2026-10-16 ("Answers") | `~/Projects/wk` (`cmd/wk/main.go`, a new `internal/slot`, `internal/config`, `internal/settings`, `README.md`); `~/.wk/machine.toml`; `~/.wk/ezhomesteading.toml`; `.claude/settings.local.json` in the main checkout and the template; `docs/incomplete/landing-layer/SLOTS-SCOPE.md` |

Prompt body for `passoff/199-...md`, in Part 3's shape:

```
## 199. Cap heavy gates machine-wide with `wk run --slot`, and route agents through it

**Model: Opus, with a Fable review of the rewrite matcher and of exit status and signals. Lane tooling. Waits on** no row 196 phase being in flight in `~/Projects/wk`; GATE 1 passed 2026-10-16 (`docs/incomplete/landing-layer/SLOTS-SCOPE.md`, "Answers").
Work in `~/Projects/wk` for the code (D21); in this repository write only `docs/incomplete/landing-layer/`.

You are picking up the build slot of the landing layer. Read `SLOTS-SCOPE.md` in full first: section 1 holds every measured number and citation, "Answers" holds Zach's choices, and nothing there is re-derived. Then read `DESIGN.md` option V and D16, `PLAN.md` section 4, and `internal/wk/new.go` `takeCreateSlot` and `internal/flock` in `~/Projects/wk`, the precedent to copy. Session rules: absolute dates, a citation on every claim, never `git commit`, `git push`, `git add -A` or `git add .` while `~/.claude/rules/commits.md` says so, print the two blocks per repository at hand-back, no attribution lines.

**Why this exists.** Zach plans 50 or more parallel agents on a 16 GB machine. Each gate peaks near 1 GiB, and the web build and a cold `lintgate` near 2 GiB, and nothing stops ten of them running at once. Each first build also leaves 758 MiB in its workspace. The slot makes them take turns.

**What is fixed.** The measurements of section 1; the answers; no tracked file of ezhomesteading changes; the hook registers in `.claude/settings.local.json` by `wk hooks install` (D6, BD-8).

Do these, in order:
1. Re-verify section 1.6's open items on the installed Claude Code: a subagent's 120 s timeout, how a stopped task is signalled, and an `updatedInput` rewrite applied to a subagent's `Bash` call. Record each in a PLAN section 0.
2. Write the plan (Stage 4) and stop at GATE 2.
3. Build `wk run`, the queue, `wk slots`, the trim, the hook and its install, with a self-test: exit status, piped output, `SIGINT` and `SIGTERM`, a nested run, a dead waiter's ticket, a dead holder, fail-open on a broken config.
4. Prove it live: four sessions run the web gate line at once in four workspaces; samples every second show at most one build and two checks; each gate's exit status equals the same gate run without the hook; a build with a syntax error is red through the wrapper; private bytes of `web/.next` after a trimmed build are under 50 MiB.

Ask before building: anything the answers leave open.
Not in scope, whoever asks: P4's batch gate, a cap on sessions, which gates `/delegate` runs, `wk slim`, any tracked file of this repository.

Hand back: the `wk` gate line green (`test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...`), the live proof with its samples, and the two commit blocks for each repository.
```

## Answers

Asked 2026-10-16 at about 15:50 EDT in chat, in one batch, answered the same day by Zach. Each answer is quoted by its option label.

1. Classes and places: "heavy 1, check 2 (Recommended)". Options A2 and B1: `heavy` (`next build`, `bun run e2e`, `playwright test`, cold `lintgate`) with 1 place, `check` (`tsc`, eslint, vitest, `go test`) with 2.
2. How agents go through it: "Hook inserts wk run (Recommended)". Option C1, with the limits stated there, and the Fable review of the rewrite and of exit status and signals named in the board row.
3. Disk after a build: "Trim after each build (Recommended)". Option E2: the two build spellings carry `trim_after = ["web/.next/cache"]`; `wk run --keep` skips the trim.
4. The rest: "Yes, all as recommended (Recommended)". Option D1 with "fifo" and no wait limit, option F1 (`~/.wk/slots/`, `~/.wk/machine.toml`), option G1 (the hook fails open), and the dials of section 4 as written.
