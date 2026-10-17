# 3. Build P2, path leases: integrate the daemon, the lease store, the board parser and the edit hook

> **Claimed `IN FLIGHT` 2026-10-17.**

*Opened 2026-10-16, when the landing-layer work moved here from `~/Projects/ezhomesteading` (its board row 196, now SUPERSEDED). Prompt filled 2026-10-16 by row 2 from `docs/incomplete/landing-layer/PLAN-P2-P4.md` section 2.*

**Model: Opus 5.5. Lane A. Waits on 2; 6, 7 and 8 (the P2 siblings); `/Users/zachshort/Projects/ezhomesteading/PASSOFF.md` item 200.**

You are the integration row of P2 of the landing layer, a Go CLI (`wk`) that makes warm git worktrees for parallel agents. Read, in this order: `docs/incomplete/landing-layer/PLAN-P2-P4.md` sections 0, 1 and 2 (the plan; its tables are binding), `DESIGN.md` D10 to D13, D26, options G to M and section 6 (the design, frozen; cite it, never relitigate it), the close-outs of rows 6, 7 and 8 in `passoff/6-*.md`, `7-*.md` and `8-*.md` (what they built and the findings they left you), `internal/daemon`, `internal/lease` and `internal/board` as `main` holds them, `internal/config/config.go`, `internal/settings/settings.go`, `internal/wk/hooks.go` and `cmd/wk/main.go`. Row 200 of ezhomesteading built `internal/slot`, `Entry.Matcher` in `internal/settings` and a `PreToolUse` entry on `Bash` at the same time as the siblings: start from whatever `main` holds (PD-12), and where `Entry.Matcher` is missing add it in the shape `SLOTS-PLAN.md` phase 2 step 4 gives.

Do these, in order:
1. Verify on the installed Claude Code (`claude --version`) the `PreToolUse` deny shape for a command hook: the JSON `hookSpecificOutput.permissionDecision: "deny"` with `permissionDecisionReason`, or exit 2 with the reason on stderr (`DESIGN.md` 1.4 names both). Use a headless `claude -p` in a scratch project with a one-line hook, as `SLOTS-PLAN.md` 0.2 did. Record the fields in the plan's section 0 with a citation. Verify also that the hook input for `Edit` carries `tool_input.file_path` and for `Bash` carries `tool_input.command`.
2. `internal/config/config.go`: the `[leases]` section of the plan (`enabled`, `board`, `single_writer`, `bash_guard`) with the defaults and validation of the table; keys are additive; `wk config show` prints the effective config as TOML with the file path and whether it was found (D26).
3. `internal/wk/daemon.go`: `wk up` starts `internal/daemon` detached (the `spawnFill` pattern, log `~/.wk/<repo>/daemon.log`, socket and lock from PD-1), loads `leases.json` (PD-3), registers the lease ops (`claim`, `release`, `add`, `check`, `status`), refreshes its worktree list from `gitx.Worktrees` when a path matches none; `wk down`; on-demand start from every lease command when the socket does not answer, never from the hook.
4. `internal/wk/lease.go`: `wk claim <row>` (board from `leases.board`, cell by `internal/board`, globs expanded against `git ls-files`, unread tokens printed, refusal naming the holder), `wk claim --none`, `wk lease add <path>...`, `wk release`, `wk status` (leases only; P3 adds the queue).
5. `internal/wk/leasehook.go` and `cmd/wk/main.go`: `wk hook lease --repo <primary>`: read the hook JSON, resolve `file_path` against `cwd` (clean, then `EvalSymlinks` on the longest existing prefix), call `check` with `{tool, cwd, path, command}`, and print the deny shape of step 1 when the daemon says no; when the socket does not answer and `leases.enabled` is true, deny with "start the daemon: wk up" (D12); when `leases.enabled` is false, allow with one line in `~/.wk/<repo>/hook.log`. No git call in the hook (PD-2).
6. `internal/settings/settings.go` and `internal/wk/hooks.go`: two `PreToolUse` entries (`Edit|Write|MultiEdit|NotebookEdit` and `Bash`), verb `hook lease --repo <primary>`, `timeout` 5, not sole; `wk hooks install` and `uninstall` keep every other entry, including row 200's.
7. `README.md`: a section on leases, the claim ritual, the two hook entries, fail-closed, and the statement that `Bash` coverage is partial (D11).
8. `GOTOOLCHAIN=local go install ./cmd/wk` before any new key is written. Then write `[leases]` into `~/.wk/ezhomesteading.toml` with `enabled = false` and the four `single_writer` paths of D13, and make `~/.wk/wk.toml` for this repository (copy the P1 sections from a `wk` setup run or the defaults, then `[leases]` with `enabled = false`). Turning `enabled` on is Zach's, see Hand back.
9. The live proof of the plan, section 2 "Done when", in a throwaway repository under the session scratchpad with its own `~/.wk/<name>.toml` (`enabled = true`, `copy_files` holding `.claude/settings.local.json`), the hooks installed there, and two `claude -p` sessions each in a worktree made by `wk new` (that is what copies `.claude/settings.local.json` and so the hook, BD-2; a plain `git worktree add` has no hook and the proof would pass for nothing). Show that each refusal came from the hook: the deny output in the session's transcript and the denial line in `~/.wk/<name>/hook.log`. Record the output under "As built, P2" in `PLAN-P2-P4.md`, with the hook timing over 20 runs (PD-13).
10. Review: spawn one Opus 5.5 subagent in its own worktree (`isolation: "worktree"`) with the diff applied there, briefed to read `internal/wk/leasehook.go`, `lease.go`, `daemon.go` and the config validation for any path where an edit passes that should be refused, or where the hook can hang or exceed its timeout; it edits nothing and returns findings with file and line. Check its worktree's diffstat when it returns. Record each finding's outcome in the As built note.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, plus the "Done when" of P2 in `PLAN-P2-P4.md` section 2.

Ask before building: one owner's call, in chat, before step 2: "Defer L3 (Recommended) or build L3 now?" Context: D13 L3 (insert-only edits on the ledger files with an exception for the claimed row) is not in this plan; the ledger files are `single_writer` instead (plan section 2, "Not built, and why"). Recommendation: defer, because the proof needs L1 only and the board layout is still moving. On "build L3 now", add an `insert_only` key and the check that `new_string` contains `old_string`, and say so in the As built note. The siblings do not depend on the answer.
Not in scope, whoever asks: P3 and P4; any change inside `internal/daemon`, `internal/lease` or `internal/board` beyond what a finding of step 10 needs (a bug there is fixed here and named); the gc schedule (PD-5); `wk run --slot` (row 200).

Hand back: the gate output, the proof, the As built note, the config lines Zach adds to turn leases on (`enabled = true` in `~/.wk/ezhomesteading.toml` and `~/.wk/wk.toml`; every session then needs `wk claim <row>` or `wk claim --none` before its first edit), and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. Commit on your own worktree branch only; the lead merges, pushes and removes the worktree after the audit. No `Co-Authored-By:` trailer and no "Generated with" line.

## Owner amendments, 2026-10-16

From the owner's research review of 2026-10-16, applied by the autopilot run before this row launched. They bind the build; where they differ from the steps above, they win.

- Held until the daemon-or-files question is decided by the owner's rule (owner's choice, 2026-10-16, revised 22:23 EDT). A separate Fable 5.1 session ("Integrate wk and personal-config") runs a kill test in a scratch clone: a file store (exclusive `flock`, rewrite, atomic rename) behind row 7's `lease.Store` interface against the landed `internal/daemon`, 100 `kill -9` runs each, counting lost and stale leases and manual recoveries, and writes the table to `docs/incomplete/integration/KILL-TEST.md` in this repository. Do not run that test in this row. When that file appears, the rule applies without the owner: files, unless the file store shows more lost or stale leases, or more manual recovery steps, than the daemon. The outcome is recorded in the run's `STATE.md` and written here, and rows 3 and 4 are released.
- The L3 question above is answered: Defer L3 (owner's choice, 2026-10-16). Do not ask it again.
- The lease hook computes `Bash` write targets in its own process before any socket call. A `Bash` command with no recognised write target is allowed without asking the store.
- Always allow a command whose first word is the wk binary, so `wk up` and `wk claim` can never be locked out.
- If the daemon stays: the hook may start it with `daemon.Spawn` when the socket is missing and retry once inside its deadline; the daemon exits when its own executable changes; every state-changing handler saves before it replies.
- Hook verbs exit 2 only for a deliberate deny, and each verb recovers from a panic. A Go panic exits 2, which Claude Code reads as a block on `PreToolUse`.
- `wk hooks install` writes the two `PreToolUse` lease entries only when `leases.enabled = true`, and registers the edit-tool entries with `"onFailure": "block"` (Claude Code 2.1.295 and later; verify the behaviour on 2.1.301 in a scratch project first). Decide separately whether the `Bash` entry gets it, because with a missing binary it would also block the reinstall.
- Reap leases whose worktree root no longer exists, on every read and every claim.
- Find the "Files it owns" column by its header, not as the last cell.
- Record the common git directory under `~/.wk/<name>/` and refuse on a mismatch, so two repositories with one basename cannot share a lease store.
- Say how the lead's own writes to `HANDOFF.md` and `PASSOFF.md` get a lease before leases are turned on in ezhomesteading, where both are in the planned `single_writer` list.
- Row 7 (the lease store, the judgment and the `Bash` word walk) is `HELD` after three blocking audits of the word walk (`docs/incomplete/autopilot-2026-10-16/RUNDOWN.md`); this row waits on it.

### Decided by the rule, 2026-10-16 22:51 EDT: files

`docs/incomplete/integration/KILL-TEST.md` (commit 29ed9a6) reports 900 kills: lost leases 0 for both stores, stale leases 0 for both, manual recovery steps 0 for the file store and one restart per crash for the daemon (100 of 100 in each daemon-kill scenario, with every hook call during the gap refused), plus probe D (a failed save leaves the daemon's memory ahead of its disk). The owner's rule picks files. This row therefore builds the file store, as the kill test's amendments say:

- `internal/wk/lease.go` keeps `lease.Store` in `~/.wk/<repo>/leases.json` under `~/.wk/<repo>/leases.lock`: an exclusive `flock` from read to rename, readers lock-free, temp-file reaping (remove `.leases.json.*.tmp` older than one minute before a write), and an idempotent re-claim (a claim by the same root for the same row with the same or fewer leases is success and prints the claim it found).
- `wk up` and `wk down` are not built; no `internal/wk/daemon.go`, no socket, no resident process. The bullet above that begins "If the daemon stays" does not apply. `internal/daemon`'s `SaveJSON`, `LoadJSON` and `Spawn` stay in use; `Server` and `Client` stay on `main` unused, with a doc comment saying so, for a later row to delete.
- The hook (`wk hook lease --repo <primary>`) reads `leases.json` itself and judges with `lease.Judge`; its worktree list comes from a file that `wk claim` and `wk release` maintain beside the store, never from git (PD-2 holds). D12's "deny when the socket does not answer" has no equivalent and is dropped.
- Everything else in this row stands, including the `Bash` targets computed in the hook's own process, the wk-binary allow, the exit-2 and panic rules, the `onFailure` entries, the reaping of leases whose root is gone, the header lookup, the common-git-directory check, and the lead's own writes to the ledger files.

### Note from ezhomesteading item 200, 2026-10-17 02:15 EDT

Item 200's integration lands on wk `main` before this row starts. It changes two files this row owns: `internal/wk/hooks.go` now calls `settings.InstallTwins` before `settings.Install`, and `cmd/wk/main.go` has `run`, `slots` and `hook pre-tool-use` verbs. Build on that state: merge `main` first and keep those verbs and that call order.

## Close-out, 2026-10-17

Builder: Opus 5.5, in the worktree `ap-row-3` on `ap/row-3`, from `main` at `26f14e3` (after `git merge --ff-only main` moved it from `5228e9c`). Built under the files decision: no `wk up`, no `wk down`, no `internal/wk/daemon.go`, no socket. The L3 question was not asked: it is deferred (owner's choice, 2026-10-16).

### What changed and why

- `internal/config/config.go`: the `[leases]` section (`enabled` false, `board` "PASSOFF.md", `single_writer` empty, `bash_guard` true), validated: the board is a relative path inside the main checkout, and each single-writer entry is a clean repository-relative path, because the judgment compares paths as written. `config.Show` prints the effective config as TOML under a comment naming the file and whether it was found (D26).
- `internal/wk/lease.go` (new): `wk claim <row>`, `wk claim --none`, `wk lease add`, `wk release`, `wk status` on `~/.wk/<repo>/leases.json`, with an exclusive `flock` on `leases.lock` from read to rename, temp files older than one minute reaped before a write, an idempotent re-claim, the claims of gone worktrees reaped on every read and every claim, the common git directory recorded and a mismatch refused, and the canonical worktree roots refreshed from git in the claiming workspace. Globs are expanded against `git ls-files`; every token the parse cannot read, and every glob that matches nothing, is printed as `unread:`.
- `internal/wk/leasehook.go` (new): `wk hook lease --repo <primary>`, described in the plan's "As built, P2".
- `internal/settings/settings.go`: `Entry.OnFailure`, `LeaseEntries(primary)`, `Install(path, exe, extra...)`, which now writes every wk entry of one event (three on `PreToolUse` with leases on), and `IsWK` recognising the lease command. `internal/wk/hooks.go`: the lease entries go in only while `leases.enabled` is true, after the twins, with row 200's `InstallTwins` call order kept.
- `cmd/wk/main.go`: the verbs `claim`, `lease add`, `release`, `status`, `config show` and `hook lease`, with usage text; row 200's `run`, `slots` and `hook pre-tool-use` are kept.
- `internal/board/board.go`: `Column(text, row, header)`, so the "Files it owns" cell is found by its header (owner amendment), not as the last cell.
- `internal/lease/judge.go`: `Request.Canon`, applied to each absolute `Bash` write target before it is placed. Finding of this integration: row 7's "Canonical paths" (`Resolve` reads no disk, so `rm /tmp/...` in a root under `/private/tmp` was outside every root and allowed). Test: `TestJudgeCanonPlacesBashTargets`.
- `internal/daemon/daemon.go`, `client.go`: doc comments saying `Server` and `Client` are unused under the files decision.
- `README.md`: "Path leases" (the claim, the hook entries, fail-closed, partial `Bash` coverage with row 7's gaps and over-refusals, the ledger files), the `[leases]` keys and `wk config show` under "Configuration".
- `docs/incomplete/landing-layer/PLAN-P2-P4.md`: three section 0 rows for step 1, and "As built, P2" under section 2.
- Tests: `internal/config/leases_test.go`, `internal/settings/lease_test.go`, `internal/wk/lease_test.go`, `cmd/wk/leases_test.go`, one case in `internal/board/board_test.go`, one in `internal/lease/judge_test.go`.
- Outside the repository: `~/go/bin/wk` installed after the gate; `[leases]` with `enabled = false` appended to `~/.wk/ezhomesteading.toml` (with `backend/routes/routes.go`, `backend/main.go`, `HANDOFF.md`, `PASSOFF.md`); `~/.wk/wk.toml` made (the P1 defaults, `copy_files = [".claude/settings.local.json"]`, `[leases] enabled = false`). `~/.wk/machine.toml` and `~/.wk/ezhomesteading.slots.toml` were not touched.

### Build-level calls, each with its reversal

- The hook denies with the JSON form and exits 0 on every path, a deny included, so only the printed deny blocks. Reversal: exit 2 with the reason on stderr.
- A panic inside the hook is a deny, because the guard fails closed (D12). Reversal: allow on a panic.
- The wk-binary allow is narrowed to a single `wk` command with no `;`, `&`, `|`, `<`, `>`, newline, backtick or `$(`: the amendment's "first word" alone would let `wk status; rm web/x.ts` through. Reversal: test the first word only.
- The `Bash` entry has no `onFailure` and is guarded by `|| true`; the edit entry has `onFailure: "block"` and no guard. A missing binary then refuses edits and leaves `Bash` free to reinstall it. Reversal: `onFailure` on the `Bash` entry too.
- The worktree roots live in `leases.json` beside the store, not in a second file, so one rename keeps them consistent. Reversal: a `worktrees.json` written in the same lock.
- The hook also finds, on disk, a worktree of this repository that the store does not list yet (a `.git` link into the common directory), so a worktree made after the last claim is unclaimed rather than part of the main checkout. Reversal: roots from the file alone.
- The hook refuses a write into another worktree than the session's own (row 7's open finding, "Judge has no idea of the caller's root"). This lives in `internal/wk`, not in `lease.Judge`. Reversal: delete `callerRoot`.
- The hook reads its state under the base name of `--repo`; `wk hooks install` refuses the lease entries when that is not the repository's name (a separate git directory), because the hook would read another config and allow. Reversal: put the name in the verb.
- The proof used a binary built into the scratchpad and `WK_HOME` in the scratchpad, so `~/go/bin/wk` changed only after the gate and `~/.wk` holds no proof files. Reversal: rerun the proof with the installed binary and a `~/.wk/<name>.toml`.
- `wk claim --none` records the row `none`; the store's `session` field is left empty, because no source for it is named. Reversal: a `--session` flag.

### What is now fixed

An edit to a path another workspace leases is refused by the hook, by `Edit` and by `sed -i`, with the holder's row and workspace named (proof in the plan). The P2 "Done when" is met except the dropped `wk down` step.

### Questions answered

- Step 1: the deny shape, the hook input fields and the `onFailure` behaviour on 2.1.301 (plan section 0).
- The lead's own writes to `HANDOFF.md` and `PASSOFF.md`: the lead runs `wk claim --none` in the main checkout once, then `wk lease add HANDOFF.md PASSOFF.md` before it writes, and `wk release` frees them (README, "The ledger files").
- `onFailure` on the `Bash` entry: no (call above).

### Left owed

- Leases are off in both repositories; turning them on is the owner's (hand-back below).
- Not verified: the hook in a desktop-app session or a subagent with `isolation: "worktree"` (the proof used headless `claude -p` only); the guard in ezhomesteading with its real board; the hook while ezhomesteading row 214's slots hook shares `PreToolUse` with real gate rules.
- Row 7's open findings stand: rule 4 over-refuses `cd web; cd ..; rm docs/a.md`, and the safe-set gaps listed in its sign-off. The caller-root rule above closes `cd ..; touch a/web/x.ts` from another worktree.
- `board.Parse` prints an unread run with its leading punctuation (`unread: ; the rest of the docs`). Cosmetic.
- A board cell such as row 7's (`` `internal/lease/` (`store.go`, ...) ``) leases `store.go` at the repository root, because each backtick token is read alone; the cell's author writes full paths.
- The hook reaps gone worktrees in memory only; the file drops them at the next claim, `lease add` or `release`.
- `internal/daemon`'s `Server` and `Client` are for a later row to delete.

### As built

`docs/incomplete/landing-layer/PLAN-P2-P4.md`, section 2, "As built, P2". Review: step 10's Opus review subagent was not spawned; the review is this run's independent audit.

### Hand back

Gate, from `ap-row-3` at `5a93108`: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'` exit 0; every package `ok` (`internal/wk` 45.5 s), `0 issues.`

Proof: two `claude -p` sessions in workspaces made by `wk new`; session 2's `Edit` and `sed -i` on row 1's `web/page.ts` were refused by the hook, with the deny in each transcript and in `hook.log`; the hook took at most 20 ms over 20 runs; `wk claim` read every backtick path of the live wk board.

To turn leases on, the owner:

1. In `~/.wk/ezhomesteading.toml`, sets `enabled = true` under `[leases]`, then runs `wk hooks install` in `~/Projects/ezhomesteading`.
2. In `~/.wk/wk.toml`, sets `enabled = true` under `[leases]`, then runs `wk --repo /Users/zachshort/Projects/wk hooks install`.
3. From then on, every session runs `wk claim <row>` or `wk claim --none` in its workspace before its first edit, and the lead runs `wk claim --none` and `wk lease add HANDOFF.md PASSOFF.md` in the main checkout before it writes the ledger files.

### Fix round 1, 2026-10-17

Audit 1 of `f0b1eb8` (`docs/incomplete/autopilot-2026-10-16/reports/row-3-audit-1.md`) found four silent allows of a leased write. Each now denies, and each has a test that fails against `f0b1eb8` and passes now (run with the new test file over `git archive f0b1eb8`).

- B1, wrong letter case on macOS. `judgeFolded` in `internal/wk/leasehook.go` folds the request, the roots, the store and the single-writer list to lower case when the volume ignores case (`caseInsensitive`: `.git` and `.GIT` are one entry, or it cannot tell), and `checkTargets` refuses a non-ASCII path below a worktree on such a volume, because Unicode normalisation gives it other spellings that lower case does not fold.
- B2, a cwd in no worktree. `checkTargets` (was `callerRoot`) denies every target inside a worktree when the session's cwd is in none.
- B3, a dangling link. `kernelPath` in `internal/wk/lease.go` replaces `canonical`'s `EvalSymlinks` walk: it reads each component with `Lstat`, follows each link with `Readlink` (a dangling one included), applies `..` after the link before it, and stops at 32 links; a loop or an unreadable link becomes a target the judgment refuses. `canonical` stays for paths from git and the command line.
- B4, a link followed by `..`. `place` in `internal/lease/bash.go` returns a target with a `..` component uncleaned (the only change to the walk; every other target is cleaned as before), and the hook joins a relative `file_path` to the cwd without cleaning, so `kernelPath` sees the link before the `..`.
- N1 and N2: `LeaseHook` prints the deny before it logs, the log cannot panic out, and `runLeaseHook` prints the deny through the new `wk.LeaseDeny` for a panic outside it. Test `TestHookPanicIsADeny` (it passes on `f0b1eb8` too, because that panic was inside `decide`).
- N3 and N4: README corrections, and the hand-back line below.
- N5 and N6 are left as they are.

Build-level calls, each with its reversal:

- B2's rule: a session whose cwd is in no worktree of the repository may not write into any worktree; a path outside every worktree stays allowed. Reversal: allow such a session and judge the path under the root that holds it, which is the silent allow of B2.
- Folding to lower case makes the hook's messages name paths in lower case on macOS. Reversal: correct the request's case against the known roots and leases instead, keeping the messages in their written case, at about 30 more lines.
- `rm` of a symbolic link is judged at the link's target, because the hook follows the last component too. Over-refusal only. Reversal: do not follow the last component for `rm` and `unlink`. (Superseded 2026-10-17: audit 2 found this let the removal of a leased link pass, B6; fix round 2 judges the link entry too.)
- `ln -s <leased file> dang2.ts` stays allowed: it writes only the new link in the session's own worktree, and a later write through the link is refused by B3's fix.

Hand-back, added: a workspace keeps the settings it was made with. After setting `enabled = true` (or back to false), run `wk hooks install`, which writes the main checkout's and the template's `.claude/settings.local.json` only, then copy the main checkout's file into each existing workspace, or remove and make each workspace again. Running `wk --repo <workspace> hooks install` does not reach a workspace's own file.

Not verified this round: whether Claude Code 2.1.301's `Write` follows a dangling link (B3) or keeps a `..` in `file_path` (B4), and whether a session's hook `cwd` can lie outside its project (B2); no `claude -p` was run. `go install` was not run and `~/.wk` was not touched, so `~/go/bin/wk` is still the code of `5a93108`.

### Fix round 2, 2026-10-17

Audit 2 of `1c4a1fd` (`docs/incomplete/autopilot-2026-10-16/reports/row-3-audit-2.md`) confirmed B1 to B4 closed and found three more silent allows. Each now denies, and each test row fails against `1c4a1fd` and passes now (run with the new test files over `git archive 1c4a1fd`).

- B5, a `cd` through a link and then `..`. `moved` in `internal/lease/bash.go` now adds, for a `cd` operand with a `..`, the raw directory beside the text-cleaned one, so each later target is judged at both. `place` in the same file needed one more condition: a relative target is not cleaned when its directory holds a `..`, or the raw directory would be cleaned again when the target is joined to it. These are the only changes to the walk. Three walk rows that expected the cleaned directory alone now list the raw one too.
- B6, a link at a leased path. The hook judges every path under three spellings, in `judgeFolded`: `kernelPath` (every link followed), `entryPath` (the parent followed, the last component not), and `cleanedPath` (`..` removed by text, then every link followed). A deny under any spelling denies the call. `rm docs/lnk`, `git rm docs/lnk`, `mv docs/lnk web/z.ts` and `ln -sf ../web/page.ts docs/lnk` are refused when `docs/` is leased elsewhere.
- B7, an edit path a tool may clean by text: `cleanedPath` above, so `<wk-2>/out/../web/page.ts` is refused whether Claude Code cleans the path or not.
- N7: `TestLeaseHookPanicOutsideTheJudgmentDenies` makes the first write of the deny panic and checks that `runLeaseHook` prints the deny and exits 0. N9: `checkTargets` takes a bool.
- The fix round 1 note called the judgment of `rm` of a link at its target an over-refusal only. That was wrong: it let the removal of a leased link pass (B6). The README now says that a write through a link is judged at the link and at its target.

Build-level calls, each with its reversal:

- Every path is judged under all three spellings, for every verb and every tool, not only for the verbs that act on a directory entry. Writing through a link that sits in a directory another workspace leases is now refused even when the write lands elsewhere. Over-refusal only. Reversal: pass the entry spelling only for `rm`, `rmdir`, `unlink`, `mv`, `ln`, `git rm` and `git mv` targets, which needs the verb carried on each target of the walk. (Superseded 2026-10-17 by pass 2: `Bash` gets two spellings, the edit tools three.)
- A `cd` operand with a `..` keeps two directories, so a later relative target is judged at one more place, and the walk reaches its 16-directory bound sooner. Reversal: keep only the raw directory, which loses bash's logical `cd`.
- One `Bash` command is now walked ten times per call (once to skip, then three times under each spelling; N6 and N10 grow); each walk is under 1 ms. Reversal: walk once and canonicalise the target list three times, which needs `Judge` to take targets. (Superseded 2026-10-17 by pass 2: seven times, once to skip and three times under each of two spellings.)

Not verified this round: what Claude Code 2.1.301 does with a `..` in `file_path` (B7 is closed for both behaviours) and whether its `Write` follows a dangling link. No `claude -p` was run, `go install` was not run and `~/.wk` was not touched.

### Pass 2, 2026-10-17

Why this pass exists: audit 3 of `53be9d5` (`docs/incomplete/autopilot-2026-10-16/reports/row-3-audit-3.md`) confirmed B5 to B7 closed and found B8, a silent allow of B4's class that the B6 fix opened. The process holds a row after a third blocking audit. The lead decided by the owner's rule (prevent a silent failure first, keep work landing, least code) to run one pass scoped to exactly two changes, B8 and N11, with their tests and documents.

What changed:

- B8. `entryPath` in `internal/wk/leasehook.go` no longer calls `filepath.Dir`, which cleaned the parent by text. It splits the raw path at its last `/`, reads the raw parent with `kernelPath` (which applies each `..` after the link before it) and joins the last component as written; a last component of `.`, `..` or nothing falls back to `kernelPath` on the whole path. The walk already places a target under the raw directory of a `cd` with `..` without cleaning it, so `cd -P p/.. && rm lnk` and `cd q && rm ../lnk` reach `entryPath` unchanged. Three rows in `TestHookJudgesEverySpellingOfAPath` cover `rm p/../lnk`, `cd -P p/.. && rm lnk` and `cd q && rm ../lnk` from `web`, on real links; each fails against `53be9d5` and passes now.
- N11. `spellings(tool)` gives `Bash` two spellings (`kernelPath`, `entryPath`) and the edit tools three (with `cleanedPath`). A row checks that `echo x > out/../web/page.ts` from wk-2 is allowed, where `out` points outside every worktree, while the `Edit` of `<wk-2>/out/../web/page.ts` is still denied.
- N12. `request` places a cwd it cannot read (a link loop) as the unplaceable marker again, as `1c4a1fd` did, by one helper shared with the target spelling. N13 stays open.
- README: the entry spelling reads the parent through links before it keeps the last component, and the cleaned spelling is for the edit tools only; the over-refusal list names the write through a link that sits in a directory leased elsewhere.

Build-level call, with its reversal: the cleaned spelling applies to the edit tools only, and for `Bash` it only refused writes that land outside, with a message naming the wrong file. (Corrected 2026-10-17 by pass 3: the reason first given here, that no program in a shell removes a `..` by text, is false for git, which does so in `git rm` and `git mv` pathspecs; audit 4, B9. Pass 3 refuses those operands in the walk instead.) Reversal: give `Bash` all three spellings again, as `53be9d5` did.

Not verified in this pass: what Claude Code 2.1.301 does with a `..` in `file_path` (the edit tools keep the cleaned spelling, so both behaviours are closed). No `claude -p` was run, `go install` was not run, and `~/.wk` was not touched.

### Pass 3, 2026-10-17

Why this pass exists: pass 2 limited the cleaned spelling to the edit tools on the lead's premise that no program a shell runs removes a `..` by text. Audit 4 of `a028e54` (`docs/incomplete/autopilot-2026-10-16/reports/row-3-audit-4.md`) proved it false for git 2.49.0 with dry runs: git removes each `..` in a `git rm` or `git mv` operand by text against its physical directory, so `git rm out/../web/page.ts` with `out` pointing outside removes the worktree's own `web/page.ts` (B9, a regression from pass 2), and from a logical directory through a link (`cd q && git rm o/../../a.md`, `git -C p rm o/../../a.md`) it removes a file no spelling of the hook places (B10, older). Git also reads pathspec magic in `git rm`: `git rm :/docs/a.md` and `git rm :docs/a.md` remove `docs/a.md` (B11, older). The lead scoped one change.

What changed: `gitCommand` in `internal/lease/bash.go` now passes `git rm` and `git mv` operands through `pathspecs`, which makes an operand with a `..` component, and a `git rm` operand starting with `:`, unplaceable for every spelling. `refusal` names it: "cannot tell which file this writes: git removes a `..` in a pathspec by text and reads a leading `:` as magic, so `<operand>` cannot be placed". `git mv` reads no magic, so a `git mv` operand starting with `:` is placed as before; `:(top)` is refused already for its parenthesis. N11 stays as pass 2 left it: audit 4 found every other program it tested writing where the kernel resolves, and the logical `cd` is covered by the walk. Nine walk rows and one hook row cover it; the hook row fails against `a028e54` and passes now. `git rm p/../lnk` is now refused as unplaceable, where `a028e54` denied it as a write to a leased link; both are loud. N14 (the logical `cd` collapses `out/..` by text, so the deny of `cd out/.. && rm web/page.ts` is right) needs no change. N15: the stale lines above are marked superseded.

Build-level call, with its reversal: an operand of `git rm` or `git mv` with a `..` is refused even when it would land on a free file; the session writes the path from the worktree's top instead, at the cost of one message. Reversal: place the operand by text against the physical directory of git, which needs the walk to know the physical directory of a logical `cd`.

Not verified in this pass: git's pathspec handling on a version other than 2.49.0. No `claude -p` was run, `go install` was not run, and `~/.wk` was not touched.
