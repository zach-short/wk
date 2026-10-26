# Autopilot run 2026-10-26: state

## Run

- Start: 2026-10-26 02:27 EDT. Folder: `docs/incomplete/autopilot-2026-10-26/`. Session runs Opus 5.5.
- Sentinel for `personal-config context`: `wk-autopilot-2026-10-26-sentinel`
- Timer: ScheduleWakeup (through /loop). Usage tool: none, so usage is not measured.
- The owner's words: "just keep iterating on wk and improving it".

## Caps

Answers, 2026-10-26, word for word where typed:
- Budget: "no time limit". No end time, no row cap, usage not measured.
- Agents in flight: "Default: 6, 4 builders". Heavy build pattern `'[g]o test -race'` (heavy-build cap applies only under more: not in force).
- Unattended: "Ready".
- Models: "there is no fable. opus 5.5 is as high as you can go so use it for deep and default work". Deep = Opus 5.5, Default = Opus 5.5, Mechanical = Sonnet 5.5, Light = Haiku 5.5.
- Fix rounds per row 2; wake while agents run 1800 s; wake while holding 3300 s.

## Audit

Not run: no `docs/audit-lanes.md`.

## Standing orders

- Lifted Deep rows: none (no Deep rows). Load-bearing rows: none. Left out: row 19 ("Leave it out": benchmarks need a quiet machine).
- Q8 questions during the run: "Decide by risk". Q9 scope-first rows: "Scope in the run". Q10 audits: "By tier" (all current rows are Default, so all audited). Q11 other boards: not asked (no row waits on another board); default applies.
- Agent C (ezhomesteading per-worktree test database, id aac2e83356b52021f) runs outside this board: its result goes to the owner, not through this run.

## Rows

- 20 | Opus 5.5 | DONE - 2026-10-26 | built in this session | wk-20 | wk/20 | landed d271a4e, released v0.1.6.
- 21 | Opus 5.5 | DONE - 2026-10-26 | builder a542831c0a9a9ea05; auditors a821c190ac054016f, aca78c0ca20b5736e, a0f0da29ad81d9fb0 (row-21-audit-3, worktree /Users/zachshort/Projects/wk/.claude/worktrees/agent-a0f0da29ad81d9fb0) | /Users/zachshort/Projects/wk/.claude/worktrees/agent-a542831c0a9a9ea05 | worktree-agent-a542831c0a9a9ea05 | landed c0449b2 on main (ff from d271a4e, base unmoved), pushed 2026-10-26 03:07 EDT | patch none | relays 0 | reviews 3, fix rounds 2 | verdict 3 clean. Worktrees left to wk gc (wk rule: no hand removal).
- 22 | Opus 5.5 | DONE 505528c, 2026-10-26 12:02 EDT (hold lifted by owner 11:47 EDT; builder pass 4 af9919727a06b6561 row-22-build-4, worktree .claude/worktrees/wk-22; audits 4 blocking, 5 clean a82eac2b6a6fd7926) | was HELD | builder a386db652fd7aa7e4; auditors a7af5c86d9a41e402, ad8c293214ee32be8, a36774a7f36d47608 (row-22-audit-3, worktree /Users/zachshort/Projects/wk/.claude/worktrees/agent-a36774a7f36d47608) | /Users/zachshort/Projects/wk/.claude/worktrees/agent-a386db652fd7aa7e4 | worktree-agent-a386db652fd7aa7e4 | commits 43eaf1e f89c427 33d65e1 8d5cfd6 6c2bc1c 43a52fd 2c7432f, not merged | patch none | relays 0 | reviews 3, fix rounds 2 | verdict 3 blocking (>&/dev/stdout with no fd number); held 2026-10-26 03:15 EDT
- 23 | Opus 5.5 | DONE - 2026-10-26 | builder 1 a76d36991e8808f83 (stopped, no commit), builder 2 a0bc3c6ff2eab3424, auditor a232e77d5f9cf7553 (row-23-audit, worktree /Users/zachshort/Projects/wk/.claude/worktrees/agent-a232e77d5f9cf7553), decider a41e298ba76341991 | /Users/zachshort/Projects/wk/.claude/worktrees/agent-a0bc3c6ff2eab3424 | row-23-build-2 | landed 547b0f6 on main (ff from c0449b2, base unmoved), pushed 2026-10-26 07:00 EDT | patch none | relays 0 | reviews 1 | verdict clean. Worktrees left to wk gc.

## Waits elsewhere

None.

## Queue

Empty. Row 19 left out. Follow-up to write after row 22 lands: the hook writes the create's name on its `claim pool slot` and template clone lines, so `wk stats` can attribute every warm (removes the unattributed group).

## Readings

- 2026-10-26 02:27 EDT | not measured | context not read yet
- 2026-10-26 02:28 EDT | not measured | `personal-config context` found no transcript for any sentinel; context not readable
- 2026-10-26 02:35 EDT | not measured | context not readable (compacted once by the owner's /compact before this wake)
- 2026-10-26 06:31 EDT | session limit hit about 03:16 EDT, reset 06:30 EDT | context not readable

## Reports

### Row 21, builder a542831c0a9a9ea05, report arrived 2026-10-26 02:28 EDT

- Command: `wk stats [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--json]` (a new verb, not `wk slots --stats`: warm times come from `internal/wk`'s hook.log, and `wk slots` is a live view under admit.lock).
- Commit 142d851 on `worktree-agent-a542831c0a9a9ea05`, not merged, pushed or tagged. No `~/.wk` change; match.go and hook.go untouched.
- Files (9, +1333): README.md (+19), cmd/wk/main.go (+45), cmd/wk/stats_test.go (+69, new), internal/slot/stats.go (+242, new), internal/slot/stats_calc.go (+360, new), internal/slot/stats_text.go (+129, new), internal/slot/stats_test.go (+273, new), internal/wk/stats.go (+129, new), internal/wk/stats_test.go (+67, new). Outside the row's cell: `cmd/wk/stats_test.go`, `internal/wk/stats.go`, `internal/wk/stats_test.go` (cell named `cmd/wk/main_test.go` and `internal/slot/stats*.go`): build-level call, audit trigger.
- Gates: full line green as reported (gofmt clean, vet clean, go test -race all ok, golangci-lint 0 issues).
- Live run on 2026-10-17..26 reproduces the review: check 3694 runs, 113.1h waiting inside 8.4h wall clock, peak 26 queued; 118 waits over 10m, 106.4h; holders mostly backend go test with long timeouts. Repeats 896 (223 after a nonzero exit, 3.6h run). missed.log 112 lines (68 already holds wk run, 14 time, 11 $( ), 9 env, 5 sh -c, 4 timeout, 1 parse). errors.log 42 deadline misses, 202 to 599 ms. ezhomesteading warms: pool 346, median 4.1s; template 17, median 28.3s.
- Build-level calls: pool vs template decided by a `claim pool slot` line between WorktreeCreate and warm; subdirectory rules from the path; repeat = next queue time vs previous end; long-wait holders counted over merged spans; missed reason cut at the first colon; dates filter by line timestamp (runs.log = end time), both days included.
- Not done: no HANDOFF line (main checkout's file). Long-timeout go test spellings are not grouped into one row.
- Close-out text: in this report (item file untracked).

### Row 21, auditor a821c190ac054016f, report arrived 2026-10-26 02:35 EDT

- Branch row-21-audit; reproduced by `git merge --ff-only worktree-agent-a542831c0a9a9ea05` (d271a4e to 142d851). Diffstat matches the builder: 9 files, +1333.
- Gate: gofmt and vet clean; go test -race all ok (cmd/wk 13.7s, internal/slot 8.2s, internal/wk 309.0s); golangci-lint 0 issues. Exit 0.
- Slot figures verified by an independent Python count (scratchpad verify.py): 4141 runs, 3715 check, 475 nonzero, 113.1 h agent wait, 8.4 h wall clock, peak 26, 118 waits over 10 min (106.4 h), errors.log 42, missed "already holds wk run" 68. All match.
- B1 blocking: warm split wrong under interleaved creates (`internal/wk/stats.go:83-103`). Report pool 346 / template 17; true 322 / 41. Fixture has no interleaving.
- Non-blocking: N1 bad wait=/run= parses as 0 (old status.go:229-232); N2 duplicate --since after an empty one accepted; N3 range filters runs by end time (stated in README); N4 readWarm and parseStats about 30 lines.
- Verdict: blocking findings.

### Row 22, builder a386db652fd7aa7e4, report arrived 2026-10-26 02:37 EDT

- Commits 43eaf1e (matcher), f89c427 (hook finds repo from files), 33d65e1 (gatekeeper test) on d271a4e. Not pushed, tagged or merged. No ~/.wk edit.
- Files (10, +731/-64): internal/gitx/commondir.go (new, +109), internal/gitx/commondir_test.go (new, +77), internal/slot/hook.go, hook_resolve_test.go (new, +49), hook_test.go, match.go, match_test.go, match_wrappers.go (new, +249), match_wrappers_test.go (new, +162), internal/wk/gatekeeper_test.go (+6/-6). Outside the cell: internal/gitx/*, internal/wk/gatekeeper_test.go (row 23's file).
- Gates: full line green, exit 0. One earlier internal/wk failure (TestSlottedGateTimeoutAfterTakingAPlaceIsAFail) under 24 CPU burners: row 23's flake.
- Numbers claimed: 40 of 42 deadline misses during 2+ slotted gates; git rev-parse 85-95% of hook time; hook time 5-25 ms before, under 1 ms after (30 runs each); CommonDir agrees with git on 67 of 87 dirs, falls back on 20, never disagrees.
- Routing now covers time, $( ), backticks, env (after its options; miss on -i, HOME, WK_*), sh -c / bash -c with one class. Side effect: wk's own gate line from ezhomesteading takes the db place for the whole line.
- Not verified: a live hook call with the new binary; permission prompts for wrapped env or sh -c lines; the memory-pressure cause of 200 ms misses (not reproduced). Recommends keeping timeout_ms = 200.
- Close-out text: in this report (item file untracked).

### Row 23, builder a76d36991e8808f83, report arrived 2026-10-26 02:37 EDT

- No edit, no commit, no patch. Branch row-23-build at d271a4e, clean.
- Before count under load (10 yes, load avg 60-68, -race -count=20): WhileWaiting 4/20 failed, AfterTakingAPlace 6/20 failed. Log: scratchpad before.log.
- Cause: gate.runFor's wall-clock time.AfterFunc (internal/gate/run.go:137-149) kills before two /bin/sh startups print "starting"; production correct, test races the clock. No test-only seam.
- Question: widen the row to internal/gate/run.go, internal/wk/gateplan.go (maybe env.go) for an injectable kill timer. Alternative rejected: longer timeout plus retry (still flaky).
- Close-out text: "2026-10-26, row-23-build: stopped, no edit. Under load (10 yes, load average about 65), -count=20 -race: WhileWaiting failed 4/20, AfterTakingAPlace failed 6/20. Every failure is a case that needs the fake wk run's 'starting' line written within the 1 s wall-clock time.AfterFunc in gate.runFor. Two /bin/sh startups miss it under load. There is no test-only seam (int TimeoutSeconds, timer local to package gate). Owed: an injectable kill timer on gate.Gate, set through an unexported Env field, so the test fires the timeout on a ready signal from the fake. That widens the row to internal/gate/run.go and internal/wk/gateplan.go."

### Row 21, builder a542831c0a9a9ea05 (fix round 1), report arrived 2026-10-26 02:47 EDT

- Commit 2532911 on 142d851: README.md, cmd/wk/main.go, cmd/wk/stats_test.go, internal/wk/stats.go, internal/wk/stats_test.go. No file outside the cell.
- B1: warm matched to create by name; overlapped creates are unattributed; creates also close on reused / plain fallback / cold-after-partial lines; `failed after` closes only when one create is open; an unclosed create drops after 10 minutes. New test TestCrossedCreatesAreUnattributed (fails on old code). N2: parseStats tracks flags seen; test added.
- Gate: full line green; internal/wk 193.1s; golangci-lint 0 issues.
- Diffstat vs main: 9 files, +1453.
- Real log 2026-10-17..26: ezhomesteading pool 165 (median 3.0s, p90 8.1s), template 0, unattributed 200 (median 5.9s, p90 21.5s). Template 0 because every template clone overlaps another create. Exact split needs hook.go to name the create on claim and clone lines (row 22's file): follow-up row after 22 lands.

### Row 22, auditor a7af5c86d9a41e402, report arrived 2026-10-26 02:49 EDT

- Branch row-22-audit; ff d271a4e to 33d65e1. Diffstat matches: 10 files, +731/-64. Gate exit 0 (internal/wk 161.8s, lint 0 issues). Throwaway tests deleted, nothing committed.
- B1 blocking: gitx.CommonDir (commondir.go:32-41, :54-71, :101-102) gives a different answer from git with ok=true for a bare repo inside a working tree, a submodule git dir as cwd, and a symlinked .git/worktrees (relative commondir joined as text).
- B2 blocking: inside $( ) or backticks with 2>&1, wk run's stderr lines enter the captured text (match.go:172-198); comment at match.go:175 is false.
- Non-blocking: env -u PATH / PATH= rewritten (refuse like HOME); `sh -c 'echo go test'` waits for a slot (safe); a silent miss with backslash in backticks; permission twins may not match env or merged sh -c prefixes; ezh db place for wk's whole gate line is correct by design (run wk's gate from wk's cwd); hook_resolve_test.go not in cell; four functions over L1 length; long comment at match.go:23.
- Correct: time, time -p, !, &, quoted heredoc, backtick escapes, sh -c with --, -e, -xc, -o pipefail, "$@", redirections, &&; exec, env -i, -S, env X=1 -u Y are misses; existing wk run untouched.
- Verdict: blocking findings.

### Row 21, auditor aca78c0ca20b5736e, report arrived 2026-10-26 02:56 EDT

- Branch row-21-audit-2; ff d271a4e to 2532911. Diffstat matches: 9 files, +1453. Gate exit 0, lint 0 issues. Nothing committed.
- Attribution correct: independent script (scratchpad indep.py) gives pool 165 (665.03s), template 0, unattributed 200 (192 mixed or claim evidence with overlap, 8 template-only with overlap). 0 `failed after` lines in range. Drop and failed-after rules can only grow unattributed.
- B1 blocking (wording): stats.go:191, :196-197, README.md:127 do not say unattributed holds pool or template creates of unknown source; "template: none" reads as no clones (41 happened).
- N1: header omits "both sources" case (stats.go:169). N2: no tests for failed-after (stats.go:132), end lines (:127), 10-minute drop (:145). Earlier N2 (parseStats) fixed and tested.
- Verdict: blocking findings (wording).

### Row 22, builder a386db652fd7aa7e4 (fix round 1), report arrived 2026-10-26 03:00 EDT

- Commits 8d5cfd6 (B1: commondir.go, commondir_test.go) and 6c2bc1c (B2 and PATH: match.go, match_wrappers.go, match_wrappers_test.go). No file outside the cell.
- B1: a level with no .git that holds HEAD, objects and refs goes to git; relative gitdir/commondir resolved with EvalSymlinks; the common dir must hold HEAD, objects and refs. TestCommonDirAgreesWithGitOnGitDirectoriesAndLinks; inner.git, inner.git/objects, .git/modules/sub, symlinked .git/worktrees failed before. Real dirs: 90, 70 git's answer, 20 to git, 0 differ.
- B2: a substitution that captures stderr (2>&1, &>, &>>, 2>/dev/stdout, /dev/fd/1, |&, nested) makes every gate inside a miss. Seven must-stay-untouched cases failed before.
- PATH: env PATH= and env -u PATH are misses; two tests failed before.
- Gate: one run, exit 0, internal/wk 184 s, lint 0 issues. Diffstat vs main: 10 files, +882/-64.
- Not verified: live hook call; hook not re-timed this round.

### Row 21, builder a542831c0a9a9ea05 (fix round 2), report arrived 2026-10-26 03:05 EDT

- Commit c0449b2: README.md, internal/wk/stats.go, internal/wk/stats_test.go only.
- B1: header says unattributed creates came from pool or template and the log cannot separate them; pool/template lines print "none attributed" when unattributed > 0. README sentence added. N1: header names all three causes. N2: TestWarmCreateWindowRules (9 cases), TestWarmTextSaysNoneAttributed.
- Gate: one run, all ok (internal/wk 210.0s), lint 0 issues. Diffstat vs main: 9 files, +1534.

### Row 22, auditor ad8c293214ee32be8, report arrived 2026-10-26 03:10 EDT

- Branch row-22-audit-2; ff d271a4e to 6c2bc1c. Diffstat matches: 10 files, +882/-64. Gate exit 0 (internal/wk 151.6s), lint 0 issues. Throwaway tests deleted; nothing committed.
- A blocking: matchStderrToStdout (match.go:231-243) misses 3>&1 2>&3, exec 3>&1, 2<&1, 2>&1-, 2>&"1", 2> >(cat): rewrite still happens and wk run's stderr enters the capture.
- B non-blocking: CommonDir accepts an invalid inner .git (garbage HEAD, HEAD dir, ref: foo/bar, refs a file) that git skips for the outer repo (commondir.go:97, :128; comment :127 wrong).
- C: wrong-case cwd on APFS gives a differently cased answer. D notes: no filesystem-boundary stop, no safe.directory check, GIT_CEILING_DIRECTORIES sends every call to git.
- Holds: B1 cases (symlinks, relative-paths, CRLF, separate-git-dir, bare worktree, legacy HEAD) agree with git; listed B2 forms are misses; outer-stderr forms correctly rewritten; first audit's correct cases hold; the hook starts no child process on the common path.
- Verdict: blocking findings.

### Row 21, auditor a0f0da29ad81d9fb0, report arrived 2026-10-26 03:06 EDT

- Branch row-21-audit-3; ff to c0449b2. Diffstat matches: 9 files, +1534. Gate one run, all ok (internal/wk 220s), lint 0 issues. Nothing committed.
- Text and --json cannot mislead (unattributed sits beside pool and template in JSON). Pool now 166 (log grew). Each of 9 window-rule cases fails when its rule is mutated; TestWarmTextSaysNoneAttributed fails without the rule. c0449b2 touches only README.md, stats.go, stats_test.go. README matches code.
- Non-blocking: text header omits the "warm line with no create" cause (stats.go:191-193, :170); 10-minute boundary untested at exactly 10:00.
- Verdict: clean. Landed: see the row line. Close-out written at the end of passoff/21-*.md (untracked).

### Agent C (outside the board), ezhomesteading per-worktree test database, aac2e83356b52021f, report arrived 2026-10-26 03:06 EDT

- ezhomesteading worktree .claude/worktrees/wk-298, branch wk/298, commit 21808dde0, not merged or pushed. 20 files, +494/-157.
- New package backend/testdb: each of 8 DB test packages has TestMain calling testdb.Main, which copies EZH_TEST_DB with CREATE DATABASE ... TEMPLATE, points the env var at the copy, runs, drops it WITH (FORCE), and drops leaked copies. All 5 suite advisory locks removed. Guard test backend/testdb_main_test.go.
- Timings: main alone 215 s; change alone 226 s; change, two at once 232 s and 232 s; main, two at once on one database 705 s and 708 s.
- Gates green; full suite green with the same 15 skips. One payments timeout under load in one concurrent run (no DB), passed 3 of 3 alone.
- Not verified: CI. Owner decisions: merge wk/298; move backend go test from slot class db (capacity 1) to a parallel class; container ezh-testdb-298 still running (docker rm -f ezh-testdb-298). Noted: `/usr/bin/time -p go test` is not routed (row 22 routes bare `time` only).
- Goes to the owner, not through this run.

### Row 22, builder a386db652fd7aa7e4 (fix round 2), report arrived 2026-10-26 03:20 EDT

- Commits 43a52fd (A: match.go, match_wrappers_test.go) and 2c7432f (B: commondir.go, commondir_test.go). Only cell files. C and D left.
- A: inside a substitution, >& or <& on fd 2 not to `-`, non-literal fd-2 targets, /dev/stdout or /dev/fd/*, exec with any fd dup all count as capture; 2>/dev/null and 2>&- still rewrite. Eight cases failed before.
- B: HEAD must be a regular file with ref: refs/... or 40/64-hex; objects and refs must be dirs; else ok=false. Four cases failed before.
- Gate: full line failed only on TestSlottedGateTimeoutWhileWaitingIsAStartError (row 23 flake); internal/wk alone ok 113.8 s; golangci-lint alone 0 issues. Diffstat vs main...HEAD: 10 files, +989/-64.
- Not verified: live hook call; no re-timing; no real-directory re-run.

### Row 22, auditor a36774a7f36d47608, report arrived 2026-10-26 03:14 EDT

- Branch row-22-audit-3; main c0449b2 merged with the builder branch: merge 2047ee6, no conflict. Diffstat 10 files, +989/-64. Full gate twice, all ok (internal/wk 108 s and 123 s), lint 0 issues, no flake.
- B clean: 14 of 15 real directories answer without git, all agree with git, 0.1 to 2 ms; 6 of 6 synthetic repos (unborn branch, packed refs, sha256, detached worktree) agree.
- Blocking: match.go:242 misses `>&/dev/stdout` and `>&/dev/fd/1` (no fd number, N == nil). Non-blocking: no path.Clean at :252; function or eval before the gate; `command exec` (:257); stderr to a file read back; /proc/self/fd/1; validHead rejects `ref:` with no space.
- Verdict: blocking findings. Third blocking verdict after two fix rounds: the row is held.

### Row 23, builder a0bc3c6ff2eab3424, report arrived 2026-10-26 06:55 EDT

- Branch row-23-build-2, commit 547b0f6 on c0449b2. Files (4, +92/-9): internal/gate/run.go, internal/wk/env.go, internal/wk/gatekeeper_test.go, internal/wk/gateplan.go. No file added. TestSlottedRoutesOnlyAMatchedGate unchanged.
- Design as decided: gate.Timer, Gate.AfterFunc (nil = time.AfterFunc), runFor nil check after limit <= 0; Env.gateAfter copied by gateList; test killWhenReady polls $0.ready every 10 ms, 30 s backstop; Stop mutex-safe.
- Counts: before 20/20 pass for both under 10 yes (load ~25) and 30 yes (load ~47); after 20/20 under 10 yes (load ~47). The race did not show on the before run. Probe: `sleep 2` at the fake's start fails both tests on the wall clock and passes with the seam; probe reverted.
- Gate: full line green; internal/wk 87.5 s, internal/gate 7.2 s, lint 0 issues.
- Close-out text: "Closed 2026-10-26 on branch row-23-build-2, commit 547b0f6. The 1 s wall-clock kill raced two /bin/sh starts. Now the kill is driven by a test seam: gate.Gate.AfterFunc (nil is time.AfterFunc) and the unexported Env.gateAfter, which gateList copies into each gate. The two tests fire the kill when the fake wk run writes $0.ready after its lines, and a 30 s backstop fails the test if the file never comes. The assertions are unchanged. Under 10 yes, -race -count=20 passed 20/20 for each test before and after. The before count did not show the race on this machine. A probe that adds a 2 s delay to the fake reproduces both failures on the wall clock and passes with the seam. The full gate is green. The row's files grew, by the Default decider's call, to internal/gate/run.go, internal/wk/env.go and internal/wk/gateplan.go. Production code changed only at the seam."

### Row 23, auditor a232e77d5f9cf7553, report arrived 2026-10-26 06:59 EDT

- Branch row-23-audit; ff to 547b0f6. Diffstat matches: 4 files, +92/-9. Gate exit 0 (internal/wk 86.5 s), lint 0 issues. Nothing committed.
- Nil default unchanged (run.go read line by line; run_test.go:128 and :164 run the default and pass). trial.go:235 sets no Timeout. Fake timer semantics match time.Timer; no leak; race-safe. Assertions unchanged. Probe reproduced (sleep 2: seam passes, wall clock fails at gatekeeper_test.go:559, :561, :586). -race -count=20: ok, 137.1 s.
- Non-blocking: stop-before-fire branch and 30 s backstop checked by reading only; AfterFunc exported for tests, as decided.
- Verdict: clean. Close-out written at the end of passoff/23-*.md (untracked).

## Judgment calls

- Rows 21 and 22 were started as plain subagents before the run, not by /delegate step 2: their builders did not rename branches to row-N-build and were not given the relay contract. Their reports are routed to auditors as /delegate step 3 says. Reversal: none needed; the audit is the check.
- Row 20 was built and landed in this session with no separate auditor, before the run. Reversal: audit d271a4e later if the owner asks.
- Autonomy Q11 not asked (no other-board waits); recorded as the default.

- 2026-10-26 02:29 EDT, row 21: the builder added `cmd/wk/stats_test.go`, `internal/wk/stats.go`, `internal/wk/stats_test.go`, outside its cell; no other open row lists them, so this session added them to the row's "Files it owns" (caught at once). Reversal: restore the cell to `cmd/wk/main.go`, `cmd/wk/main_test.go`, `internal/slot/stats*.go` (new), `README.md`.

- 2026-10-26 02:36 EDT, row 21 B1: the exact fix needs the hook to log pool or template on the warm line, in hook.go, which row 22 owns. Decided (caught at once): attribute only unambiguous creates and report the rest as "unattributed"; add an interleaved fixture; fix N2. Follow-up for after row 22 lands: a new row to log the source on the warm line. Reversal: drop the unattributed group once the hook logs the source.
- 2026-10-26 02:38 EDT, row 22: the builder added internal/gitx/* (no other open row lists it) and edited internal/wk/gatekeeper_test.go, which row 23 owns. Row 23 had made no edit, so no work collides. Decided (caught at once): add both to row 22's cell; row 23's build waits until row 22 lands, then builds from the new base. Reversal: drop the gatekeeper_test.go change from row 22 and let row 23 make it.
- 2026-10-26 02:37 EDT, row 23 question (widen to internal/gate/run.go, internal/wk/gateplan.go for an injectable timer): a shared primitive, so a Default decider (Opus 5.5, a41e298ba76341991) decided 02:40 EDT: approve with design changes: exported `gate.Timer` interface{ Stop() bool } and `Gate.AfterFunc func(time.Duration, func()) Timer` (comment "nil is time.AfterFunc; tests only"); nil check only inside runFor after the limit <= 0 return; unexported Env.gateAfter in env.go copied by gateList, not set by Open, trial.go postCloneGates untouched; test fake fires the kill on "$0.ready" with a 30 s backstop, Stop race-safe, Submitted cases still go through the timeout path; existing run_test.go:128 and :164 prove the nil default. Not load-bearing. Cell widened to internal/gate/run.go, internal/gate/run_test.go, internal/wk/gateplan.go, internal/wk/env.go; Waits on 22. Reversal: drop the AfterFunc field, Timer type and Env field.

- 2026-10-26 03:15 EDT, row 23: row 22 is held, not in flight, so row 23 no longer waits on it; its Waits on is back to none and it builds from main c0449b2. Row 22's branch also edits internal/wk/gatekeeper_test.go (another test function), so row 23 was told not to change TestSlottedRoutesOnlyAMatchedGate. Reversal: hold row 23 until row 22 lands.

- 2026-10-26 06:33 EDT, row 23: builder 1 stopped on the session limit and its clean worktree was removed by wk gc; a fresh builder (same model, full brief and the decision) starts from main c0449b2. Reversal: none needed; nothing was committed.

- 2026-10-26 07:01 EDT, release: `git tag -a v0.1.7` and `git push origin v0.1.7` were denied by the auto mode classifier (Create Public Surface). Not retried. main at 547b0f6 holds rows 21 and 23 unreleased; the owner tags v0.1.7 and runs `brew upgrade zach-short/tap/wk`.

## Held

- Row 22: blocking finding after two fix rounds. The note is in passoff/22-find-and-fix-the-hook-deadline-misses-and-route.md ("Held, 2026-10-26"). The owner lifts the hold.

### Row 22, builder pass 4 (af9919727a06b6561), 2026-10-26 11:52 EDT
- First send stopped at setup: the brief's ff-only merge of the held branch could not work from a main-based worktree (brief error). Resent with a plain merge. The agent's first worktree was removed by gc while clean; it rebuilt in `.claude/worktrees/wk-22` via `wk new 22` and switched to `row-22-build-4`.
- Merge 43cffb2 (held branch into main b0a1368), no conflicts. Fix 2942187: internal/slot/match.go +30 -2, internal/slot/match_wrappers_test.go +31. Branch diffstat vs main: 10 files, +1046 -64.
- Gates green: gofmt, vet, go test -race (internal/wk 82.3 s, first run), golangci-lint 0 issues. status clean.
- Judgment calls: `>&/dev/stderr` counts as a capture (reversal: exempt cleaned /dev/stderr, /dev/fd/2 in the no-fd branch); tests in match_wrappers_test.go beside the B2/A cases (reversal: move to match_test.go).
- Not verified: no real bash comparison. Non-blocking items of the held note stay open; `command exec` is now cheap.

### Row 22, audit 4 (a4a9191f18d0f6b40, branch row-22-audit-4), 2026-10-26 12:05 EDT
- Reproduced 2942187 by ff; diffstat 10 files +1046 -64 matches. Gates green (internal/wk 54.7 s, lint 0 issues). status clean.
- Audit 3 item fixed (39 bash forms tried against /bin/bash 3.2.57 and mvdan/sh). Merge 43cffb2 clean; nothing lost from 547b0f6 or 2c7432f.
- BLOCKING B1: match.go:249 compares fd as a string, so `02>&1`, `002>&1`, `02>/dev/stdout` are rewritten while bash puts stderr in the capture (zsh does not). Fix: numeric compare.
- Non-blocking: N1 `>&/dev/stderr`, `>&/dev/null` count as a capture (accepted); N2 held-note items unchanged; N3 test placement right.
- Routed to the builder as fix round 1 after the lift, 2026-10-26 12:05 EDT.

### Row 22, builder fix round 1 after lift, 2026-10-26 11:59 EDT
- 505528c: match.go +9 -1 (matchFDIs, numeric fd compare), match_wrappers_test.go +14. Gates green (internal/wk 51.5 s). status clean. 3 new cases fail on old code.

### Row 22, audit 5 (a82eac2b6a6fd7926), 2026-10-26 12:02 EDT
- Clean. Reproduced 505528c; diffstat 10 files +1068 -64. Gates green (internal/wk 48.6 s, lint 0 issues). Numeric fd rule checked in every capture path; bash 3.2.57 experiments agree; overflow fds are rejected by bash, so no capture is missed.
- Landed: main b0a1368..505528c fast-forward, pushed 12:02 EDT. Board DONE, log line written.

## After the run: rows 24 and 25, opened 2026-10-26 13:19 EDT
- 24 | Opus 5.5 | IN FLIGHT | builder a197c885e59552959, branch row-24-build | warm line names its source; stats reads it
- 25 | Opus 5.5 | IN FLIGHT | builder a4558eee9bf1c618f, branch row-25-build | replay missed.log; route /usr/bin/time and compound time forms

### Row 24, builder (a197c885e59552959, row-24-build), 2026-10-26 13:23 EDT
- 46494ee on 405ec7e: 6 files +118 -22 (README, hook.go, hook_test.go, new.go, stats.go, stats_test.go). Gates green (internal/wk 53.0 s, lint 0). Real-log warm numbers equal before and after (ezh pool 175, template none, unattributed 202).
- Calls: createFrom beside create (trial.go untouched); sourced warm line with no create line counts at its source; sourced creates still cross old-format ones.
- Found: wk stats --json order unstable on equal held_seconds in internal/slot (new row later).
- Judgment call (this session, caught at once): update help text at cmd/wk/main.go:135; main.go added to row 24's cell. Reversal: revert that hunk and drop it from the cell.

### Row 25, builder (a4558eee9bf1c618f, row-25-build), 2026-10-26 13:26 EDT
- 305ea4d on 405ec7e: match.go, match_wrappers.go, match_wrappers_test.go, +227 -20. Gates green. Replay of ~/.wk/slots/missed.log (112, ezh): main 98 routed, 9 stderr-capture misses, 4 timeout misses, 1 zsh parse; after 102/9/0/1. No /usr/bin/time lines in the log; routed with tests anyway.
- Judgment call (this session, by risk: a wrong answer passes every gate): do not route timeout/gtimeout; log it as a miss with a reason. Reason: 118 waits over 10 min, so the wait would spend the timeout's clock and exit 124 before the gate starts. Reversal: re-add timeout to matchWrapperReaders. Sent back to the builder.
- Open: timeout ... sh -c not routed or logged.

### Row 24, audit (a8b3be3231d063483), 2026-10-26 13:28 EDT
- Clean. Reproduced d3f834c; 7 files +121 -24. Gates green. Real-log warm numbers equal with both binaries. Non-blocking: repeats sort has no tie-break (internal/slot).
- Landed: main 405ec7e..d3f834c, pushed. Board DONE, log line written.

### Row 25, builder rework (a4558eee9bf1c618f), 2026-10-26 13:30 EDT
- 6fce4fb on 305ea4d: timeout/gtimeout not routed, logged "after the wrapper timeout: the wait for a place would count against its clock". 3 files +202 -22. Gates green. Replay: 98 routed (same as main), 9 stderr, 1 zsh, 4 timeout (miss, new reason). /usr/bin/time, gtime, command time routed with tests.
- Correction: builder text said the owner decided timeout; this session decided it. Sent: merge main d3f834c, re-gate, before audit.

### Row 25, audit (a2b8dea9f59de0c1f), 2026-10-26 13:39 EDT
- Non-blocking only. Reproduced 32286e6, replay equal, gates green. Findings N1-N6 in the item close-out.
- Landed: main d3f834c..32286e6, pushed 13:39 EDT. Board DONE, log line written.

## Rows 26 and 27, opened 2026-10-26 13:40 EDT
- 26 | Sonnet 5.5 | IN FLIGHT | builder ad2f47055f59d8161, row-26-build | stats tie-breaks; audit by tier unless a trigger fires
- 27 | Opus 5.5 | IN FLIGHT | builder a664d17ecafe2898c, row-27-build | wrapper reader findings from the row 25 audit

### Row 26, builder (ad2f47055f59d8161, row-26-build), 2026-10-26 13:45 EDT
- 54c6eba on a02a87b: stats_calc.go, stats_test.go, +169 -7. Gates green. JSON hash stable over 7 runs (main unstable). Extra fix: repeats sums floats in fixed key order.
- Audit trigger: numbers claimed (hashes). Auditor add4cd51e215b762e launched.

### Row 26, audit (add4cd51e215b762e), 2026-10-26 13:48 EDT
- Clean. Hash reproduced 5/5; main 5 hashes. New tests fail 5/5 on old code. Landed main a02a87b..54c6eba, pushed. Board DONE, log written.

### Row 27, builder (a664d17ecafe2898c, row-27-build), 2026-10-26 13:49 EDT
- 6caf696 on a02a87b: match.go, match_wrappers.go, match_wrappers_test.go, +174 -15. Gates green. Replay byte-identical to main (98 routed). 31 cases; 20 fail on main.
- Calls: quoted fixed text accepted as time arg (reversal matchLiteral); "$F" refused; other wrappers over sh -c now logged (reversal: only timeout); path.Base kept (reversal: bare name or absolute path).
- Sent: merge main 54c6eba, re-gate, before audit.

### Row 27, audit (a047a81c66fb41c03), 2026-10-26 13:56 EDT
- Non-blocking N1-N4 (in close-out). Replay byte-identical. 20/31 confirmed. Landed main 54c6eba..d08f2cc, pushed. Board DONE, log written.

## Row 28, opened 2026-10-26 14:11 EDT
- 28 | Opus 5.5 | IN FLIGHT | builder a19195291b602c211, row-28-build | row 27 audit N1-N4

### Row 28, builder (a19195291b602c211, row-28-build), 2026-10-26 14:20 EDT
- 0fc095b on 16834e0: 3 files +224 -20. Gates green. Replay byte-identical incl. reasons. 22/36 new cases fail on main.
- Calls: capture reason wins over every gate reason (reversal: timeout only); $ refused in time args too (reversal: drop from matchFixedWord); ./timeout keeps timeout reason; ~/bin/time relative.
- Audit a4868330748d552b7 launched.

### Row 28, audit (a4868330748d552b7), 2026-10-26 14:28 EDT
- Non-blocking N-A..N-D (in close-out); zsh not verified (guard refused zsh). Replay identical. Landed main 16834e0..0fc095b, pushed. Board DONE, log written.

- 2026-10-26 16:52 EDT row 29 opened from the row 26 and row 28 open findings (owner said "continue"), claimed, builder Opus 5.5 launched in its own worktree, branch row-29-build. Judgment call: N-A refuses `$=` on the safe side when zsh cannot be run (reversal: drop the refusal if a zsh check shows no split).
- 2026-10-26 17:07 EDT row 29 builder reported: branch row-29-build, commit 1906cdb, 5 files +277 -47, all gates green on the second run (the first run failed in internal/landing and internal/setup with "No space left on device", 363 MiB free). Replay of missed.log byte-identical to main; 20 new cases fail on main; stats hash old = new (8f430eb). Build-level calls: N-A refused on the safe side (zsh refused by the guard); a one-class string that mentions another class is wrapped, not refused (reversal: matchMentioned for the class in shellGate). This session ran `go clean -cache` (65 GB Go build cache), and free space went from 360 MiB to 65 GiB. Auditor Opus 5.5 launched, branch row-29-audit.
- 2026-10-26 17:15 EDT row 29 audit 1 (Opus 5.5, row-29-audit): non-blocking. Gates green, replay byte-identical (102f57d), stats hash main = HEAD on --until 2026-10-25 (3416e2a), 20 cases fail on main, zsh refused by the guard again. F1: a one-class sh -c string can run another class it only mentions, and that gate is no longer logged. Judgment call (this session, by risk): restore main's class rule (matchMentioned), keep nestedGates for whether to wrap; reversal: take the class from nestedGates. Sent to the builder as fix round 1. F2 (pipe into sh not wrapped), F3 (wk run later words, conservative label), F4 (runBefore Equal ignores zone) open.
- 2026-10-26 17:21 EDT row 29 audit 2 (row-29-audit-2) did no audit: its fast-forward to row-29-build was refused because main had two STATE.md commits after the build's base 9f0c8f3, and its clean worktree was then removed by Claude Code's own cleanup of an unchanged agent worktree (not by wk gc: gc.log shows no run after 11:10). The builder merged main (df54055, STATE.md only). Audit 3 (Opus 5.5, row-29-audit-3) launched on df54055. Seen: every wk gc launchd agent but ezhomesteading's shows last exit 78 (EX_CONFIG) and no gc.log entry after 11:10 EDT; probable cause the full disk (360 MiB free until this session's go clean -cache). Check at the next hourly run.
- 2026-10-26 17:25 EDT row 29 audit 3 (Opus 5.5, row-29-audit-3) on df54055: non-blocking. Gates green, missed.log replay identical on 112 lines, stats hash main = HEAD (3416e2a), F1 forms refused with main's reason. F2 raised: `sh -c 'echo go test | sh'` was slotted on main and is now unslotted and unlogged. Judgment call (this session, by risk): fix round 2 logs a miss when a string names a gate and holds a form that can run text as a command; reversal: remove the check. Last fix round for the row.
- 2026-10-26 17:37 EDT row 29 audit 4 (Opus 5.5, row-29-audit-4) on 899b9e3: blocking. Gates green, replay identical, stats hash 3416e2a. Item 5 still leaves forms silent that main slotted (pipe into a wrapper-prefixed runner, expansion behind exec or command, pipe into a subshell or block, a file written then run, another interpreter). Two fix rounds used, so row 29 is HELD; the note is in the item file under "Held, 2026-10-26". The ways on (take item 5 out, or turn it to a provably-inert allow list) go to the owner.
- 2026-10-26 18:40 EDT owner ("your recommendations"): row 29 way (a), item 5 out, and release. Builder took item 5 out (5b7b1b8); audit 5 (Opus 5.5, row-29-audit-5) clean with non-blocking findings (quoted shell name hides a gate, on main too; F4). Row 29 landed on main by fast-forward at 5b7b1b8 and pushed. DONE.
- 2026-10-26 19:34 EDT row 30 (gc agent across brew upgrades) opened from the exit-78 evidence, built by Opus 5.5 (row-30-build, 6182552, fix round 1 bcaebbf), audited twice by Opus 5.5 (non-blocking both), landed on main by fast-forward at bcaebbf and pushed. DONE. The tag push and the agent reinstall were refused by the classifier and are the owner's.
