# Plan: build slots, a machine-wide cap on heavy gates (`wk run --slot`)

Status: PLANNED (Stage 4), written 2026-10-16 by an Opus 5.5 session started from the session that wrote and ratified `SLOTS-SCOPE.md` (board row 200). GATE 2 passed 2026-10-16: Zach chose every recommendation (section 7), so SB-15 to SB-17 stand as written and SB-4, SB-11 and SB-12 amend the scope. The scope is `SLOTS-SCOPE.md` in this folder, ratified at GATE 1 ("Answers"); this file changes nothing in it and records build-level calls as SB-1 to SB-n, so they never collide with `PLAN.md`'s BD numbers. Model check: the row assigns the build to Opus with a Fable review of the rewrite matcher and of `wk run`'s exit status and signal handling; this session runs Opus 5.5.

This session measured its own context at 303,000 tokens after the reading and the probes of section 0 (the transcript method of `AGENT-PRACTICES.local.md` Part 5). That is the Default tier's landing line, so this session builds nothing: each phase below runs in a fresh Opus session, and each phase section is written to be executed without reading this session.

## 0. Facts verified 2026-10-16 (this table supersedes SLOTS-SCOPE.md where they differ)

Taken between 15:55 and 16:16 EDT. Binary citations are the minified names in the desktop app's bundled Claude Code 2.1.299 (`~/Library/Application Support/Claude/claude-code/2.1.299/4fa9694b7975/claude.app/Contents/MacOS/claude`), read with a Python `mmap` search in the session scratchpad; these names change between versions. Probe logs are in this session's scratchpad (`probe/t1a-async.log`, `probe/t1b-sync.log`, `probe/t2-stop.log`, `hooktest-hook.jsonl`). Times in the logs are Unix seconds.

The probe script (`probe/probe.sh`) logs its pid and process group, traps SIGTERM, SIGINT, SIGHUP, SIGQUIT, SIGUSR1, SIGUSR2 and SIGPIPE, starts a child that moves itself into its own process group and traps SIGTERM and SIGINT, and starts a watcher that double-forks and calls `setsid`, so it leaves the process tree and can log the moment the probe dies, which is how an untrappable SIGKILL would show.

### 0.1 The three open items of SLOTS-SCOPE.md section 1.6

| Claim | Verified state | Citation |
|---|---|---|
| An async subagent's foreground call at the 120 s timeout | Moved to the background, and it keeps running after the subagent's final response; the subagent is resumed when it exits. The probe ran its full 150 s with no signal | Haiku subagent, `run_in_background: true` (the desktop default): `t1a-async.log` start 1792180699.043, end 1792180849.448, no `sig-` line; its tool result, as the subagent quoted it later: "Command did not complete within its 120s timeout and was moved to the background (ID: bzo2f8ukh). ... You will be notified when it completes."; the completion notice "This agent stopped with background work of its own still running. It may resume on its own when that work completes"; the subagent then reported the task "completed with exit code 0", but only about 19 minutes after the probe ended: the agent ran 1,286,600 ms from its start at 15:58:16, and the probe ended at 16:00:49. The cause of the delay was not measured; this session was inside one long turn the whole time. Phase 3 times a resume again with the parent session idle |
| A synchronous subagent's foreground call at the 120 s timeout | Moved to the background with the warning that it "is terminated when you give your final response and no notification can follow that", and it was: SIGTERM 147.3 s after its start, when the subagent's final response landed, to the probe and to its own-group child | Haiku subagent, `run_in_background: false`: its quoted tool result; `t1b-sync.log` start 1792180724.971, `sig-TERM` 1792180872.267; binary `Mo=Be.backgroundTaskId!==void 0&&(Lpt(s.agentContext)\|\|s.agentId===void 0&&Pkt())`, with `Lpt(e)` true only for `agentType==="subagent"&&isAsync===!1`; schema text of `backgroundEndsWithFinalResponse`: "a synchronous subagent's command, or a headless session that takes no further input" |
| A deadline on backgrounded commands | Desktop sessions have none. A deadline exists only when `P7t()` holds; its default is 10 minutes in a headless `-p` session and 30 minutes otherwise, its maximum 2 hours. This session's `Bash` tool offers the short `run_in_background` text that `uLn()` returns when `P7t()` is false, and the async subagent's timed-out call above got no "it will be stopped" sentence, which `$dr` adds whenever a deadline applies. Not verified: whether an interactive CLI session in a terminal has the deadline | binary `function d_r(e){return Fco()?$co(e):void 0}`, `Fco()` is `P7t()&&k("tengu_cosmic_shore",!0)`, `I7t()` is `rz()?Math.max(VB,Coe()):A_s()` with `VB=600000` and `A_s` `Math.max(1800000,Coe())`, `gue()` `Math.min(Math.max(7200000,DRe()),2147483647)`; this session's tool schema |
| How a stopped background task is signalled | SIGTERM to the shell's process group and, separately, to every descendant found by a `ps` parent walk, whatever its process group; SIGKILL to both 1,500 ms later if anything is left. The same path ends a synchronous subagent's command at its final response | `TaskStop` on a probe task: `t2-stop.log`, the probe (group 66660, the shell's) `sig-TERM` 1792180696.261 and its own-group child (group 66716) `sig-TERM` in the same second; binary `vE(r,"SIGTERM")`, whose `h(n,r)` runs `process.kill(-n,r)` and then `process.kill(e,r)` for each pid of `QCo(n)`; `M8n=1500` |
| Which process group a command runs in | The shell Claude Code starts (`/bin/zsh -c source <snapshot> ...`) leads its own group, and the command runs in it | every probe log: `pgid` equals the shell's pid |
| An interrupted turn | A running foreground command is moved to the background, not killed. From the code only | binary `turnAbortBackgrounds:fo` with `fo=!xt`; result field `backgroundedByTurnAbort` |
| `updatedInput` from a project's `.claude/settings.local.json` reaches a subagent's `Bash` call | Yes, on both installed versions, with no `permissionDecision`. The subagent's hook input carries `agent_id` and `agent_type` | headless `claude -p` in a scratch project whose `.claude/settings.local.json` held a `PreToolUse` hook that rewrote `echo PROBE_ORIGINAL` to `echo PROBE_REWRITTEN`: CLI 2.1.301 printed `main: PROBE_REWRITTEN main` and `sub: PROBE_REWRITTEN sub`, hook input `agent_id` `aaddc5bcb34e3e9d2`, `agent_type` `general-purpose`; the bundled 2.1.299 run the same way printed the same, `agent_id` `a905a1fcf00ed00ea`. A desktop (stream-json) session was not tested in isolation; phase 3's live proof is that test |

### 0.2 New facts the scope did not have

| Claim | Verified state | Citation |
|---|---|---|
| Permission rules are matched against the rewritten command | With the allow rule `Bash(echo PROBE_ORIGINAL:*)` and a hook that rewrote the call to `/bin/echo PROBE_REWRITTEN main`, the call was refused with "This command requires approval"; with `Bash(/bin/echo PROBE_REWRITTEN:*)` it ran. Same on 2.1.301 and 2.1.299 | four headless runs in the scratch project, 2026-10-16 at 16:03 to 16:04 |
| The allow rules that cover gates today | `Bash(bun *)` and `Bash(bunx *)` in `~/.claude/settings.json`; `Bash(go test *)`, `Bash(go vet *)`, `Bash(go build *)` and `Bash(npx tsc *)` in the main checkout's `.claude/settings.local.json` (87 allow and 3 deny rules). A wrapped gate matches none of them | the three settings files, read with `json.load` |
| What sessions do with a command no rule matches | Desktop sessions run with `--permission-mode=auto`, so it goes to the auto-mode classifier: an extra model call per wrapped gate, and a possible refusal. A session in the default mode would prompt | `ps` of the desktop `claude` processes |
| Models notice a rewrite whose output differs from the command they wrote | Both headless runs said so ("Something altered the output before it reached me") | `t3-cli-301.out`, `t3-299.out` |
| The `PreToolUse` input | `session_id`, `transcript_path`, `cwd`, `scratchpad_dir`, `prompt_id`, `permission_mode`, `effort`, `hook_event_name`, `tool_name`, `tool_input` (`command`, `description`, and `timeout` or `run_in_background` when set), `tool_use_id`; plus `agent_id` and `agent_type` in a subagent's call | `hooktest-hook.jsonl` |
| The commit guard sees every `Bash` call | `~/.claude/hooks/personal-config/commit-guard.sh` refused a `git commit` in the scratch project, so it runs beside any hook this row adds | this session's refused call, 2026-10-16 |
| Shell aliases or functions on gate words | None for `bun`, `bunx`, `go`, `tsc`, `eslint`, `vitest`, `next`, `playwright` in the newest zsh snapshot (422 lines), so moving the word out of first place changes no lookup | `grep` of `~/.claude/shell-snapshots/snapshot-zsh-1792180416147-2x4dh1.sh` |
| A shell parser that builds on `wk`'s Go | `mvdan.cc/sh/v3` v3.14.1 needs go 1.26 and v3.13.x go 1.25.0; v3.12.0 (2025-07-06) needs go 1.23.0, so it builds on go1.24.1 under `GOTOOLCHAIN=local` (BD-13) | `go list -m -json mvdan.cc/sh/v3@<v>`; `https://proxy.golang.org/mvdan.cc/sh/v3/@v/v3.13.0.mod` |
| `gitx.Resolve` costs two `git` runs | `rev-parse --git-common-dir` and `worktree list --porcelain`; the hook needs only the first | `internal/gitx/gitx.go:48-65` at `30989b5` |
| The root `package.json` has no `build` script | `test`, `test:coverage`, `test:watch` only, so `bun run build` is always the web build | `package.json`, `scripts` |
| The wk repository is free | Row 196 is OPEN with P2 waiting on Zach; `~/Projects/landing-layer-phase3` is clean at `30989b5`; the one other `wk` session ("README and repo for wk") is idle and its subagent worktree is clean at `29cb987` | `PASSOFF.md` row 196; `git status`; the desktop session list, 15:55 |

### 0.3 What this session changed

`PASSOFF.md` row 200 and `passoff/200-wk-run-slot-a-machine-wide-cap-on-heavy-gates.md` (row 199 went to the land-or-discard row, appended by its own session minutes later), this file, and scratch files in its own scratchpad. Nothing in `~/Projects/wk`, `~/.wk/` or any settings file.

## 1. Decisions taken since ratification

Each decision has a one-line reversal. SB-15 to SB-17 were the GATE 2 questions; Zach chose each as written (section 7).

- **SB-1. A worktree of `wk` for the build.** `~/Projects/wk-slots` on branch `slots` from `wk` `main` at `30989b5`, as phase 3 used `~/Projects/landing-layer-phase3`. Reversal: build in the main checkout of `wk`.
- **SB-2. One new package, `internal/slot`.** The machine file, the queue, `wk run`, `wk slots` and the matcher. Commands in `cmd/wk/main.go`: `run`, `slots [--json]`, `hook pre-tool-use`. Reversal: none needed.
- **SB-3. The machine file, `~/.wk/machine.toml` (F1).** `[slots]` holds `dir`, `order`, `max_wait_seconds`, `min_free_mb`; each class is a table `[slots.<name>]` with `capacity`. No class name is compiled in (D19). A missing file means no classes, so every run passes through unslotted with one line; an unknown key is an error, as in `config.Load`. Reversal: classes as fixed fields.
- **SB-4. The repository's slot keys go in a file of their own, `~/.wk/<repo>.slots.toml`.** It holds `[[slots.match]]` and `[hook]` (`fail_open`, `timeout_ms`). An older binary never reads it, so a reinstall from a commit without this row (the 15:18 incident's cause) cannot make `config.Load` refuse `~/.wk/<repo>.toml` and stop every `WorktreeCreate`. The scope's F1 puts these keys in `~/.wk/<repo>.toml`; its own F2 argument ("a new table in the repository file makes any older binary reject the whole file") is the reason to move them. Reversal: a `[slots]` table in `~/.wk/<repo>.toml`, after `wk` `main` holds this row.
- **SB-5. The FIFO queue.** Under `<dir>/<class>/`: `place-<i>.lock` for each place, `admit.lock`, a `seq` counter and `queue/<seq>.ticket`. A waiter makes its ticket in `tmp/`, takes an exclusive `flock` on it, then renames it into `queue/`, so a ticket in `queue/` is locked by a live waiter or its waiter is dead; a ticket another process can lock is dead and is removed. Every admission happens under the exclusive `admit.lock`: a waiter takes a place only when fewer live tickets are ahead of it than there are free places, so no later waiter can win a race. The holder writes its record (pid, workspace, command, start) into its place file, which a reader trusts only while the place is locked. Waiters within reach of a place poll every 100 ms (`takeCreateSlot`'s interval), the rest every second. `order = "any"` skips the tickets and keeps the poll. Reversal: `order = "any"`.
- **SB-6. `wk run` keeps the command's process context.** The child gets wk run's own three file descriptors (no copying, so a pipe, a terminal or binary output is untouched), stays in the shell's process group (no `Setpgid`), and inherits the environment plus `WK_SLOT_HELD=<class>`. The place's lock file is close-on-exec (Go's default), so the place belongs to `wk run` alone and a server the gate leaves running never holds it. Reversal: none; each alternative is a scope hazard.
- **SB-7. Signals.** `wk run` catches SIGINT, SIGTERM, SIGHUP, SIGQUIT, SIGUSR1 and SIGUSR2, but only those not ignored when it started (`signal.Ignored`), so a disposition the shell set to ignored still reaches the child unchanged; it forwards each to the child's pid, never to the group. It does not forward SIGINT or SIGQUIT when its stdin is a terminal whose foreground group is its own, because the terminal already sent them to the child and a second SIGINT means "force quit" to some tools. It catches SIGPIPE without acting, so a write to a closed stderr returns an error instead of killing `wk run` while its child runs on, and the child still starts with the default disposition. It leaves SIGTSTP, SIGCONT, SIGTTIN, SIGTTOU and SIGWINCH alone, so job control stops and continues both processes together. A signal that arrives while it waits in the queue removes the ticket and ends `wk run` by that signal. Under Claude Code the child also gets the signal directly (section 0.1), so it may see SIGTERM twice; that changes no exit status. Reversal: forward nothing and rely on group delivery.
- **SB-8. Exit status.** The child's exit code passes through unchanged. A child that died of SIGINT, SIGTERM, SIGHUP or SIGKILL makes `wk run` reset its handler and kill itself with the same signal, so the shell sees the same death; bash and zsh stop a script on a child's SIGINT death and do not on an exit of 130. Any other signal gives 128 plus its number, the code the shell reports, because Go cannot restore the default action for SIGQUIT or SIGSEGV without its own crash dump. A command that is not found gives 127, and one that is not executable gives 126, with one line on stderr, as the shell does. A bad `wk run` argument list gives 2 with the usage text, which is a red gate, never a green one. Reversal: 128 plus the number for every signal.
- **SB-9. Fail open (G1).** An unreadable or invalid machine file, an unwritable slot directory, an unknown class or any queue I/O error makes `wk run` run the command unslotted, with one stderr line "wk run: running unslotted: <reason>" and a line in `<dir>/errors.log`. Reversal: exit 1.
- **SB-10. What `wk run` prints.** Nothing when a place is free at once, so the output is the unwrapped command's. When it waits: one line as it starts to wait (class, each holder's workspace, command and age, and how many are ahead) and one as the command starts (time waited). One line after a trim. Each run appends one line to `<dir>/runs.log`: time, class, pid, workspace, command, wait, run time and exit status, which `wk slots` and phase 3's proof read. Reversal: no `runs.log`.
- **SB-11. A nested run of any class passes straight through.** While `WK_SLOT_HELD` is set, `wk run` takes no place, whatever the class: a heavy holder that waits for a check place while a check holder waits for heavy would deadlock. The scope said "a nested run of a held class". Reversal: same class only.
- **SB-12. The trim.** `--trim <path>` (repeatable, relative to the git top level of `wk run`'s working directory, never absolute, never with `..`) is removed after the child exits with any status, while the place is still held, unless `--keep` is given. It is skipped in the main checkout, the template and pool slots: the scope's purpose is the disk of a burst of workspaces, the template's cache is what every workspace clones, and the main checkout is Zach's. The hook writes `--trim` from the match entry's `trim_after`, so a hand-typed `wk run` trims nothing unless told to. Reversal: trim in the main checkout too.
- **SB-13. The hook, `wk hook pre-tool-use`.** Registered on `PreToolUse` with matcher `Bash`. It reads the input, resolves the repository from `cwd` with one `git rev-parse --path-format=absolute --git-common-dir`, loads `~/.wk/<repo>.slots.toml`, and prints nothing when there is no file or no match. A rewrite prints one JSON document whose `updatedInput` is the whole `tool_input` with `command` replaced (and `timeout` per SB-16), and never a `permissionDecision`. `hook.timeout_ms` (200) is an internal deadline: past it the hook prints nothing and exits 0; the single write of the JSON and the deadline share one lock, so a half-written document cannot happen. The settings entry's own `timeout` is 5 s. Reversal: none needed.
- **SB-14. The settings entry fails open even for a binary that lacks the verb.** Its command is `<wk> hook pre-tool-use || true`. An old binary prints the usage text and exits 2, and exit 2 from a `PreToolUse` hook blocks the call, so without the guard a reinstall like the 15:18 one would stop every `Bash` call of every hooked session. A deliberate block (`hook.fail_open = false`) is therefore a JSON `permissionDecision: "deny"` with exit 0, never exit 2. Phase 3 checks that Claude Code runs the entry through a shell. Reversal: a plain command.
- **SB-15. The matcher (GATE 2 question 2).** The command is parsed with `mvdan.cc/sh/v3` v3.12.0, bash dialect; text that does not parse is left alone and logged. A simple command is rewritten when its first argument words are unquoted literals equal to a spelling of `slots.match` (after any `VAR=value` words, whose values may hold expansions such as `$(mktemp -d)`), it sits in a statement position (recommended: at any depth of `&&`, `||`, `;`, `|`, `!`, `&`, `( )`, `{ }`, `if`, `while`, `until`, `for` and `case`, never inside a word, which is where `$( )`, backticks, `<( )` and the string of `bash -c` live, and never in a function body), and the command line holds no `wk run` yet. The insertion is `<absolute wk> run --slot <class> [--trim <path>]... -- ` at the byte offset of the first argument word; no other byte changes. A literal sequence equal to a spelling that was not rewritten appends the command and the reason to `<dir>/missed.log`. Reversal: top level only, as the scope's C1 limit says.
- **SB-16. The hook raises the `Bash` timeout on a rewritten call (GATE 2 question 3).** When `tool_input.timeout` is absent or under 600,000 ms, the rewrite sets it to 600,000 ms, the foreground maximum (`DRe()`, `BASH_MAX_TIMEOUT_MS` unset). A synchronous subagent then waits up to ten minutes before its gate is moved to the background, where its final response would kill it (section 0.1). This replaces the scope's proposed line in the `/delegate` brief, which is Zach's file. Reversal: leave `timeout` alone.
- **SB-17. Permission twins (GATE 2 question 1).** `wk hooks install` writes, into the same `.claude/settings.local.json` files, a wrapped twin of each `Bash` allow, ask and deny rule in the user, project and local settings that covers a spelling: `Bash(bun *)` gains `Bash(<wk> run --slot heavy --trim web/.next/cache -- bun *)`, `Bash(<wk> run --slot heavy -- bun *)` and `Bash(<wk> run --slot check -- bun *)`. A twin allows exactly what its original allows, because `wk run` runs the words after `--` and nothing else, and the trim path in it is fixed text. Uninstall removes every rule that starts with `Bash(<path>/wk run --slot `. A rule added later gets its twin at the next install. Phase 2 tests the twin syntax in the scratch project on both versions. Reversal: no twins; auto mode sends wrapped gates to the classifier.
- **SB-18. `wk slots`.** Per class: capacity, each place (free, or pid, workspace, command, age), the waiters in queue order (pid, workspace, command, time waited), the last runs from `runs.log`, and the count and newest line of `missed.log` and `errors.log` in the last 24 hours. `--json` prints the same as one document for phase 3's sampler. It probes places under `admit.lock`, so it never takes a place from a waiter. Reversal: none needed.
- **SB-19. The other machine dials.** `max_wait_seconds` above 0 ends a wait with exit 75 (`EX_TEMPFAIL`) and one line. `min_free_mb` above 0 makes the head of the queue also wait until free, inactive and speculative pages from `vm_stat` reach it; 0 skips the read. Both are 0 in the file (the scope's dials). Reversal: drop the dial.
- **SB-20. The logs are never rotated in this row.** `runs.log` gains about 150 bytes per gate run. Reversal: rotation in P4's daemon.

## 2. Phases

| # | Phase | Driver | Subagents | Est. context | Why that shape |
|---|---|---|---|---|---|
| 1 | `wk run`, the queue, `wk slots`, the trim, the machine file, and their self-test | Opus, fresh session | none | comfortable | Everything else calls `wk run`; exit status and signals are where a silent failure lives, so they are built and tested before anything routes gates through them |
| 2 | The matcher, the hook, the repository slots file, the `wk hooks install` entry and twins, README, and the Fable review of both phases | Opus, fresh session | one Fable reviewer | full | The hook depends on `wk run`'s command line; the review needs both halves finished and nothing installed yet |
| 3 | Install, configure, and the live proof | Opus, fresh session | four Haiku gate runners | comfortable | Install order is a hazard of its own (section 5); the proof needs the installed binary and four agents |

Order is fixed by dependency and by the install hazard: nothing is installed until the review's findings are fixed.

### Phase 1: `wk run`, the queue, `wk slots`, the trim

Status: PLANNED.

Scope:

1. Make the worktree (SB-1): `git -C ~/Projects/wk worktree add -b slots ~/Projects/wk-slots main`, then confirm `git -C ~/Projects/wk-slots log -1` is `30989b5` or a later `main`. Work only there, with `GOTOOLCHAIN=local`.
2. `internal/slot`: the machine file loader (SB-3), the queue (SB-5), `Run` with SB-6 to SB-12 and SB-19, the status of SB-18. Copy `takeCreateSlot`'s poll and `internal/flock` (`TryTake`, `Take`, `Release`).
3. `cmd/wk/main.go`: `wk run [--slot <class>] [--keep] [--trim <path>]... -- <command> [args...]` and `wk slots [--json]`, in `usage`.
4. The self-test, each case in `go test -race`: exit codes 0, 1, 3, 42 and 255 pass through; a child killed by SIGINT, SIGTERM, SIGHUP and SIGKILL makes `wk run` die of the same signal (`ProcessState`), SIGQUIT and SIGSEGV give 128 plus the number, a missing command 127, a non-executable file 126; 1 MiB of random bytes on stdout and on stderr through a pipe arrive byte for byte (hash), stdin reaches the child, and nothing is added to stderr when no wait happened; SIGINT and SIGTERM sent to `wk run`'s pid reach the child once, and sent to the group (as Claude Code does) the run still ends with the child's status; a signal during a wait removes the ticket and leaves the places as they were; a nested run passes through while the outer run holds capacity 1; a waiter killed with SIGKILL leaves a ticket that the next waiter removes, and order holds; a holder killed with SIGKILL frees its place; five waiters behind one holder start in arrival order; six runs at capacity 2 never overlap more than two; a broken machine file, an unknown key, an unknown class and an unwritable slot directory each run the command with its own status, one stderr line and one `errors.log` line; the trim after exit 0 and exit 1, `--keep`, a refused `..` or absolute path, and no trim in the main checkout, the template or a pool slot; `max_wait_seconds`, `order = "any"`, `min_free_mb` through an injected memory reader; `wk slots` text and JSON with two holders and three waiters.
5. Every test runs from a temporary `WK_HOME` and an empty working directory (the phase 3 lesson in `PLAN.md`, "As built, phase 3").

Subagents: none.

Done when: from `~/Projects/wk-slots`, `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'` is green; and by hand, `wk run --slot heavy -- sh -c 'exit 7'; echo $?` prints 7, `wk run --slot heavy -- sh -c 'kill -TERM $$'` dies of SIGTERM in zsh ("terminated"), and two `wk run --slot heavy -- sleep 5` started together finish about 5 s apart, with `wk slots` during the overlap showing one holder and one waiter. This phase installs nothing and writes no config file outside a test's `WK_HOME`. An `As built` note in this file.

Watch for: a test that signals the wrong process group kills the test runner; start every child under test in a process group of the test's own making and signal only that group or a pid. And `os/exec` with the three descriptors set to `os.Stdin`, `os.Stdout` and `os.Stderr` passes them straight through only when they are `*os.File`; a wrapper type makes Go copy through a pipe, which changes buffering and can lose bytes at exit.

### Phase 2: the matcher, the hook, the install entry, the review

Status: PLANNED.

Scope:

1. In `~/Projects/wk-slots`, on phase 1's tree: `go get mvdan.cc/sh/v3@v3.12.0` (section 0.2). Read its `syntax` package API through Context7 or the module source before writing the walker.
2. The repository slots file and its loader (SB-4), with the dials of section 3 as its generic defaults only where the scope gives one (`hook.fail_open` true, `hook.timeout_ms` 200); the spellings are ezhomesteading's and live only in its file.
3. The matcher (SB-15, as GATE 2 answers it) and `wk hook pre-tool-use` (SB-13, SB-16).
4. `internal/settings`: an `Entry.Matcher` field, the `PreToolUse` entry (not sole, timeout 5, the `|| true` guard of SB-14), `IsWK` for the guarded command, and the twins of SB-17 (as GATE 2 answers it), read from `~/.claude/settings.json`, the tracked `.claude/settings.json` and the file being written, written only into the main checkout's and the template's `.claude/settings.local.json`. Never write `~/.claude/settings.json` or the tracked file (D6).
5. README: a section on `wk run`, `wk slots`, the hook, the two config files, fail-open, and the warning that `~/go/bin/wk` must always be built from a commit with this row.
6. The self-test: a table of at least 40 command lines with their exact expected output text, covering every spelling, arguments after a spelling, `VAR=$(mktemp -d)` before one, each list operator, each statement position of SB-15 (or only the top level), the untouched cases with their `missed.log` reason (`$( )`, backticks, `bash -c`, `eval`, a function body, a quoted spelling, a line that already holds `wk run`, text that does not parse); the hook's JSON (whole `tool_input` kept, `command` and `timeout` replaced), a non-`Bash` tool, no slots file, a broken slots file with `fail_open` true (no output, `errors.log`) and false (a deny document, exit 0), the deadline passed (an injected slow step: no output, exit 0), input that is not JSON; the settings entry, idempotent, uninstall, the twins and their removal.
7. The Fable review. Make `~/Projects/wk-slots-review` with `git -C ~/Projects/wk worktree add --detach ~/Projects/wk-slots-review main`, apply this phase's whole diff there (`git -C ~/Projects/wk-slots diff --binary main > <scratch>/slots.patch`, after `git add -N` of each new file; then `git -C ~/Projects/wk-slots-review apply <scratch>/slots.patch`), and spawn one Agent with `model: "fable"` whose brief is: review the matcher (`internal/slot` match code and its tests) for any rewrite that changes what a command means or a gate spelling it misses, and `wk run` for any path where the exit status, the output or a signal differs from the unwrapped command, or where a place outlives its command or is lost while the command runs; work only in `~/Projects/wk-slots-review`, edit nothing, run the gate line there; return findings with file and line, each marked blocking or not. When it returns, check `git -C ~/Projects/wk-slots-review status --short` against the applied patch and `git -C ~/Projects/wk-slots status --short` against this phase's own list.
8. Fix every blocking finding in `~/Projects/wk-slots`, rerun the gate line, record the review in this file.

Subagents: one Fable 5.1 reviewer, read-only, in its own worktree (row 200's assignment; `~/.claude/rules/model-routing.md`, "never let a subagent inherit the session's tier", and the shared-worktree rule).

Done when: the gate line of phase 1, green; the matcher table passes with every expected string written out in the test; `wk hook pre-tool-use` fed the `PreToolUse` input of section 0.2 with the command `cd web && bunx tsc --noEmit && bun run lint && bun run build` prints exactly one `updatedInput` whose command is `cd web && <wk> run --slot check -- bunx tsc --noEmit && <wk> run --slot check -- bun run lint && <wk> run --slot heavy --trim web/.next/cache -- bun run build`, in under 50 ms of wall time measured with `/usr/bin/time -p` over 20 runs; the Fable review is recorded with each finding's outcome. Still nothing installed.

Watch for: `mvdan.cc/sh` positions are byte offsets into the input as given; a command with a multi-byte character before the insertion point must still come out byte-exact, so the table holds one. And the reviewer subagent starts in this session's working directory, which may be the ezhomesteading main checkout; its brief must name absolute paths, and the diffstat check after it returns is not optional.

### Phase 3: install, configure, prove it live

Status: PLANNED.

Scope:

1. Install the binary before any new config key: `GOTOOLCHAIN=local go install ./cmd/wk` from `~/Projects/wk-slots`; check `~/go/bin/wk run --slot heavy -- true` and `~/go/bin/wk hook worktree-create </dev/null` (the existing hook still answers; it exits 2 on empty input, which is its old behaviour).
2. Write `~/.wk/machine.toml` and `~/.wk/ezhomesteading.slots.toml` with the values of section 3. Run `wk slots`.
3. Before touching the main checkout's settings, prove the entry in the scratch project of section 0 (`hooktest`, or a new one): a headless `claude -p` on each version runs `bun --version` through the hook with the twin rules in place and no prompt; the `|| true` guard with a fake binary that prints usage and exits 2 lets the call run.
4. `wk hooks install`. Within a minute, this session's own `Bash` calls and a subagent's are rewritten (Claude Code reloads settings after a short delay; `PLAN.md`, "As built, phase 3", fallback proof).
5. The live proof. Start a sampler in the background that writes, every second, `wk slots --json` and a `ps -axo pid=,command=` count of `next build`, `tsc --noEmit` and `eslint` processes to a file. Spawn four Haiku subagents at once with `isolation: "worktree"`, so the `WorktreeCreate` hook gives each a workspace; each runs, as one `Bash` call, `cd web && bunx tsc --noEmit && bun run lint && bun run build; echo "gate line exit=$?"` and returns the last 30 lines. Then run the same three gates unwrapped in one of the four workspaces through a script file (`bash <scratch>/web-gates.sh <workspace>`, which the matcher leaves alone, so nothing in it is slotted) and compare each gate's status with `runs.log`.
6. Red through the wrapper: in one workspace, copy a web source file aside, add a syntax error, run `cd web && bun run build` through the hook and then unwrapped through the script, compare the two statuses, and restore the file with `cp`.
7. Disk: after a trimmed build in a fresh workspace, sum `ATTR_CMNEXT_PRIVATESIZE` under its `web/.next` with a read-only `getattrlist` walker (`SLIM-SCOPE.md` section 1.1 gives the method).

Subagents: four Haiku 5.5 gate runners, each in its own hook-made workspace, each running one command and reporting its output; the diffstat of each workspace is checked when they return.

Done when: every sample shows at most one heavy holder and at most two check holders in `wk slots`, and `ps` never shows more than one `next build` and two of `tsc` or eslint among slotted workspaces (any unslotted process found is named with its working directory); each gate's exit status in `runs.log` equals the unwrapped run's; the syntax-error build exits non-zero through the wrapper with the same status as unwrapped; the private bytes under `web/.next` after a trimmed build are under 50 MiB; `missed.log` and `errors.log` are read and every line explained. Plus the `wk` gate line, green, on the installed commit. The `As built` note records each number, the sampler file's path, and the commit blocks for both repositories.

Watch for: from step 4 on, every hooked session in the main checkout and in new workspaces is routed through this binary, including sibling sessions (section 5); `wk hooks uninstall` is the rollback and must be run at once if any session reports a refused or changed command. And four web gate lines under the cap take about 4 × 63 to 130 s for the builds alone (`SLOTS-SCOPE.md` 1.3), so the subagents' calls run past 120 s and go to the background (they are async, which section 0.1 shows is safe).

## 3. Dials

Nothing below is a constant in code (D19). The values are the scope's section 4, in the files of SB-3 and SB-4.

| Dial | File | Value | Source |
|---|---|---|---|
| `slots.dir` | `~/.wk/machine.toml` | `<WK_HOME>/slots`, written out as an absolute path | scope section 4 |
| `slots.order` | machine | `fifo` | option D |
| `slots.max_wait_seconds` | machine | 0, no limit | option D3 |
| `slots.min_free_mb` | machine | 0, off | option B3 |
| `slots.heavy.capacity` | machine | 1 | D16; option B1 |
| `slots.check.capacity` | machine | 2 | option B1 |
| `slots.match`, class `heavy` | `~/.wk/ezhomesteading.slots.toml` | `next build`, `bun run build`, `bun run e2e`, `playwright test`, `go run ./cmd/lintgate`; added here: `bunx next build`, `bunx playwright test` | scope sections 1.3, 1.4, 1.7; the additions are the same programs through `bunx` |
| `slots.match`, class `check` | repository slots file | `tsc`, `bunx tsc`, `eslint`, `bun run lint`, `vitest`, `bun run test`, `go test`; added here: `bunx eslint`, `bunx vitest` | as above |
| `trim_after` | on the entries `next build`, `bun run build` and `bunx next build` | `["web/.next/cache"]` | option E2; never on `bun run e2e` |
| `hook.fail_open` | repository slots file | `true` | option G1 |
| `hook.timeout_ms` | repository slots file | 200 | DESIGN.md section 5 |
| the settings entry's `timeout` | `.claude/settings.local.json` | 5 s | SB-13 |
| the raised `Bash` timeout | code, from `BASH_MAX_TIMEOUT_MS`'s default | 600,000 ms | SB-16; Claude Code's own maximum, not a choice of this row |

## 4. Seams reserved, deliberately not built

- P4's daemon takes the same place files for its trial gates, so the batch gate and the agents' gates share one cap (scope option F).
- A cap on Claude Code sessions, and their memory (scope section 1.2).
- Which gates `/delegate`'s builder and auditor run; it is Zach's skill. SB-16 removes the need for the scope's proposed line in its brief.
- `wk slots` in `wk hive` and `wk viz`.
- Log rotation (SB-20).
- Slots for Xcode, EAS or simulator builds; none of them runs through `Bash` gate spellings here.
- An interactive CLI session's 30-minute background deadline (section 0.1, not verified). If it holds, a gate that waits more than 30 minutes in a terminal session is stopped; desktop sessions are not affected.
- Linux and Windows (DESIGN.md section 2).

## 5. Repo hazards, with live numbers

- Memory: 3,965 to 5,172 MiB free, inactive or speculative with no gate running, and 2.6 to 2.9 GB of swap in use (`SLOTS-SCOPE.md` 1.2). Phase 3's four gate lines under the cap peak near one build and two checks, about 4.6 GiB.
- The installed binary is shared by every hook. From phase 3 step 4 on, a bad `~/go/bin/wk` reaches every `Bash` call in every hooked session, on top of every worktree creation. SB-4 and SB-14 keep an old binary harmless; a new binary with a bug is caught only by fail-open and by the rollback.
- Sibling sessions run in the main checkout now (the land-or-discard row's session was running at 16:16). After the install their gate commands are slotted too, which is the intent, and they may wait behind phase 3's proof.
- `go test -race` runs up to ten test binaries at once (`-p` defaults to the core count), so one check place is about 1.1 GiB of them (scope section 5).
- Sessions outside the hook run unslotted: Zach's terminal, worktrees made before the hook carried a `.claude/settings.local.json`, cloud sessions.
- The word "slot" also names pool slots and create slots; every line `wk run` and `wk slots` print says "place" with its class.
- `du` overcounts clones; the disk proof uses private size (`SLIM-SCOPE.md` 1.1).
- `wk`'s gate line uses `golangci-lint` with the shared cache; a red lint may be a sibling worktree's (`CLAUDE.local.md`, "Gates that lie").

## 6. Session protocol

Follow `AGENT-PRACTICES.local.md` (Stages 5 to 8, Parts 5 to 7) and `CLAUDE.local.md`. Specific to this row:

- One phase per session. Each phase session starts by reading this file in full, `SLOTS-SCOPE.md` sections 1.5 and 5, and the code of `internal/flock`, `internal/wk/new.go` `takeCreateSlot`, `internal/settings` and `cmd/wk/main.go`, and measures its own context after that reading.
- Code in `~/Projects/wk-slots` only (D21); in ezhomesteading, write only `docs/incomplete/landing-layer/` and the untracked files `wk` itself manages.
- Never `git commit`, `git push`, `git add -A` or `git add .` in either repository while `~/.claude/rules/commits.md` says so; print the two blocks per repository at hand-back. No attribution line anywhere. Run `git add -N` on every new file before printing the blocks.
- Never edit a tracked rule file of ezhomesteading.
- Install the binary before writing any key it alone reads (phase 3 step 1 before step 2).
- Each phase ends with an `As built` note here and its status set to `BUILT` with the commit hash Zach gives.

## 7. GATE 2 answers

Asked 2026-10-16 at about 16:17 EDT in chat, in one batch, answered the same minute by Zach. Each answer is quoted by its option label.

1. Permission rules for wrapped commands (section 0.2, SB-17): "Twin rules (Recommended)". `wk hooks install` writes a wrapped twin of each allow, ask and deny rule that covers a gate spelling.
2. Where the matcher may rewrite (SB-15): "Every statement (Recommended)". The parser is `mvdan.cc/sh/v3` v3.12.0, and gates inside `( )`, `{ }`, `if`, `while`, `until`, `for` and `case` bodies are rewritten; never inside a word or a function body.
3. The `Bash` timeout of a rewritten call (SB-16): "Raise to 10 min (Recommended)". The hook sets `timeout` to 600,000 ms when the call sets less.
4. The plan: "Approve all (Recommended)". Three phases, each in a fresh Opus session, with the amendments SB-4 (the repository's slot keys in `~/.wk/<repo>.slots.toml`), SB-11 (a nested run of any class passes through) and SB-12 (no trim in the main checkout, the template or a pool slot).
