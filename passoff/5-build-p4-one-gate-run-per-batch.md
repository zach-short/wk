# 5. Build P4, one gate run per batch: integrate the trial, the batch loop, attribution and the fast-forward of `main`

> **Claimed `IN FLIGHT` 2026-10-17.**

*Opened 2026-10-16, when the landing-layer work moved here from `~/Projects/ezhomesteading` (its board row 196, now SUPERSEDED). Prompt filled 2026-10-16 by row 2 from `docs/incomplete/landing-layer/PLAN-P2-P4.md` section 4.*

**Model: Opus 5.5. Lane A. Waits on 4; 11 and 12 (the P4 siblings).**

You are the integration row of P4 of the landing layer. Read, in this order: `docs/incomplete/landing-layer/PLAN-P2-P4.md` sections 0, 1 and 4 (binding), `DESIGN.md` D14, D16, D20, D26, options S to V and section 6, the close-outs of rows 11 and 12 in `passoff/11-*.md` and `12-*.md`, `internal/gate` and `internal/batch` as `main` holds them, `internal/wk/landing.go` (P3's loop, which this row changes), `internal/wk/new.go` `create` and `internal/wk/gc.go` (the trial marker, PD-9), `internal/slot` (row 200; PD-10), `internal/config/config.go` and `cmd/wk/main.go`.

Do these, in order:
1. `internal/config/config.go`: `landing.ff_main` and the `[gates]` keys of the plan (`trial_dir`, `batch_max`, `batch_idle_seconds`, `min_free_mb`, `fresh_caches`, `[[gates.run]]` with `name`, `dir`, `run`, `needs_env`) with the defaults and validation (`trial_dir` relative; `batch_max` at least 1; a run with an empty `run` is an error). Additive.
2. `internal/wk/trial.go`: the trial workspace at `gates.trial_dir`, detached, made by `Env.create` and marked `land-worktrees-trial` (PD-9) so gc skips it; `read-tree -u --reset <tree>` into it under `~/.wk/<repo>/trial.lock`, held from the read to the last gate (section 6).
3. `internal/wk/gatekeeper.go` and `internal/wk/landing.go`: move the land step behind the gate (PD-6): the loop now forms a batch by `batch.Pick`, predicts each changeset on top of the previous prediction, runs `gate.Run` in the trial on the final tree with `gates.env`, fresh caches and `gate.WaitForMemory` before each gate, through `wk run --slot` where row 200's matcher recognises the command (PD-10); on green lands the batch in order by `landing.Land` (one CAS each), then `landing.FastForward` with `landing.ff_main` and `landing.Push` with `landing.push`; on red, `batch.Blame` over `diff-tree -r --name-only <base> <tree>` per changeset (`-r`, or git prints top-level entries only and nothing matches), with each gate's `dir` joined onto the paths `gate.Paths` returns (they are relative to the gate's root); on a unique match, re-gate the batch without that changeset first and mark it `failed` only when that re-gate is green (a type check can name an innocent file, row 12's audit probe B11); when that re-gate is red, `batch.Shrink` the original batch with the blamed changeset still in it and re-gate (T3, T1); land exactly the trees that were re-gated. (Amended 2026-10-16 by the autopilot run from row 12's audit; the earlier text read "`failed` for a unique match and re-gate the rest".) With no `[[gates.run]]`, land onto the ref and never fast-forward (PD-11).
4. `internal/wk/status.go`: `gated`, `failed` with gate and reason, `landed`, the batch in flight, and why `main` did not move.
5. `README.md`: a section on the batch gate, the trial, attribution, and that P4 is only cheaper if sessions stop running the gates themselves (option V).
6. `GOTOOLCHAIN=local go install ./cmd/wk` before any new key; then the `[gates]` section for `~/.wk/wk.toml` (the four wk gates, `enabled` landing) and for `~/.wk/ezhomesteading.toml` (the `CLAUDE.md` gates minus the DB suites, `needs_env = ["EZH_TEST_DB"]` on `sqlgate` and the DB suites, `landing.enabled` left as it is).
7. The live proof of the plan, section 4 "Done when": a throwaway repository in the session scratchpad with three gates in its `~/.wk/<name>.toml` and a `git init --bare` directory as `origin`, the memory samples, the `skip` gate, and two changesets of that repository batched, gated once in its trial, landed, fast-forwarded and pushed to the bare remote by the daemon. Never submit, land, fast-forward or push in this repository (`~/Projects/wk`): under this run only the orchestrating session moves its `main`, after an audit. Record the proof under "As built, P4" in `PLAN-P2-P4.md`.
8. Review: spawn one Opus 5.5 subagent in its own worktree with the diff applied there, briefed to read `internal/wk/gatekeeper.go` and `landing.go` for any path where a batch lands without a green gate, where a `skip` counts as green, where the trial is gated mid-read, or where the wrong changeset is blamed; it edits nothing and returns findings with file and line. Check its diffstat. Record each finding's outcome.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, plus the "Done when" of P4 in `PLAN-P2-P4.md` section 4.

Ask before building: nothing; every call is in the plan (PD-6, PD-9 to PD-11). Only a fact that section 0 of the plan finds moved.
Not in scope, whoever asks: `gates.parallel_tsc` and `gates.next_build_concurrency` (row 200's slots); a disk report in `wk status`; any change inside `internal/gate` or `internal/batch` beyond a finding of step 8 (fixed here and named); folding `/land-worktrees`.

Hand back: the gate output, the proof, the As built note, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. Commit on your own worktree branch only; the lead merges, pushes and removes the worktree after the audit. No `Co-Authored-By:` trailer and no "Generated with" line.

## Owner amendments, 2026-10-16

From the owner's research review of 2026-10-16, applied by the autopilot run before this row launched. They bind the build; where they differ from the steps above, they win.

- Use a `wk-trial` marker in gc's skip list instead of `land-worktrees-trial`, which `/land-worktrees` force-removes. Add a queue-depth warning to `wk status`.
- A separate Fable 5.1 session is fixing gc's stopped-subagent rule in `internal/wk/gc.go` (finding F1: a stopped subagent's worktree with a detached HEAD is removed with its commits on no ref). That fix landed on `main` at ddc541b (board row 13, DONE 2026-10-16), so this row may edit `gc.go` (noted 2026-10-16 22:51 EDT).
- Bisection replaces `batch.Shrink`'s drop-the-last fallback: split the failing batch in two and re-gate each half; keep path mapping (`batch.Blame`) as a first guess only; set `gates.batch_max` to 1 (a serial gate, then land) until the measured busy fraction, landings per hour times gate hours, passes 0.5; at the owner's estimate of 2026-10-16 it is about 0.1. `internal/batch` is no longer owned by an open row (row 12 landed 2026-10-16 at 22:03 EDT), so this row may change it. This reverses part of D16 (owner's choice: yes, 2026-10-16; recorded in `DESIGN.md`).

## Close-out, 2026-10-17

Built by Opus 5.5 in the worktree `ap-row-5` on `ap/row-5`, from `main` at `78dc9b9` (already the base; `git merge --ff-only main` said "Already up to date"). Commits: `8f2d091` (the code and its tests), `31006c5` (a status format fix found by the proof), and the commit that adds the README section, the plan's section 4 heading and "As built, P4", and this section. Not merged, not pushed; the lead lands it after the audit.

What changed and why:

- `internal/config/config.go`: `[gates]` gained `trial_dir`, `batch_max`, `batch_idle_seconds`, `min_free_mb`, `fresh_caches` and `[[gates.run]]` (`name`, `dir`, `run`, `needs_env`), and `[landing]` gained `ff_main`, with the plan's defaults (`batch_max` 1 by decision 20) and validation in `Gates.validate`. Additive. `internal/config/gates_test.go` (new) covers the defaults, a load in the live ezhomesteading order (`[gates.env]` before `[gates]`), twelve refusals and the show; `landing_test.go` loses the row that refused `ff_main` and checks its default.
- `internal/batch/batch.go`: `Halves` replaces `Shrink`, which is deleted (owner amendment: bisection replaces drop-the-last). Table test of five rows and an alias test in `batch_test.go`. `blame.go`: the doc names bisection and that a unique match is a first guess only.
- `internal/wk/new.go`, `internal/wk/pool.go`: `Env.create` takes an empty branch for a detached workspace (`worktree add --detach`, or `checkout --detach` on a claimed pool slot), which the trial needs.
- `internal/wk/gc.go`: the `wk-trial` marker (`GateTrialMarkerName`) is skipped beside `land-worktrees-trial`, which stays (owner amendment). `gc_test.go` adds a `wk-trial` worktree to the skip test.
- `internal/wk/trial.go` (new): the trial at `gates.trial_dir`, made once by `Env.create` and marked, a directory without the marker refused, and `withTrial`, which holds `~/.wk/<repo>/trial.lock` from `read-tree -u --reset <tree>` to the last gate.
- `internal/wk/gatekeeper.go` (new): the P4 pass. `requeue` (crash recovery), `batch.Pick` and the idle wait, `resolve` (gate, then land, or blame, or bisect, or fail a batch of one), `tryBlame`, `bisect`, `landPlan` and `landMember` (the `gated` lines, then `landing`, `landing.Land` and `landed` per member), `markFailed`, `diffs` (`diff-tree -r -z --name-only <base> <tree>`), and `advanceMain` (`landing.FastForward` with `ff_main`, then `landing.Push` with `push`).
- `internal/wk/gateplan.go` (new): the plan (each member predicted on the previous prediction; a conflict with the tip refused, one with an earlier member deferred), the verdict (green is no fail and at least one pass), `runGates` (`gate.WaitForMemory` before each gate, `gate.Run` per gate, `gate.Paths` over the gate's own directory with `dir` joined back on), the fresh caches, `wk run --slot` routing through `slot.Rewrite`, and `~/.wk/<repo>/gates.json` for `wk status`.
- `internal/wk/landing.go`: `landPass` runs `gatePass` when `[[gates.run]]` is not empty and P3's loop (now `landAll`) when it is (PD-11); `pending` includes `gated`.
- `internal/wk/status.go`: the last gate run or the batch in flight, `gate:` and `named:` lines on a failed changeset, why `main` is behind the landing ref (no gates, `ff_main` false, or the last fast-forward's refusal), and the queue-depth warning (owner amendment).
- `internal/wk/env.go`: `Env.WKExe` (the binary a gate is routed through) and a `sleep` the tests replace. `cmd/wk/main.go`: `wk land --loop` sets `WKExe` from the installed binary (none for a `go run` build); the usage text.
- `internal/wk/gatekeeper_test.go` (new): thirteen tests. A green batch of two gated once, landed in order with the gated tree, `main` fast-forwarded and pushed, the skip recorded, the trial detached and skipped by gc; the proof's three changesets with a unique blame confirmed by a green re-gate (two runs) and the trial without the red file afterwards; row 12's probe B11, where Blame names the innocent changeset, the re-gate is red, and bisection fails the right one (four runs); a conflict inside the batch deferred and then refused; an all-skip run that lands nothing; no gates and no fast-forward (PD-11); a `gated` line gated again after a stop and a swapped `landing` line landed without a run; the idle wait; the trial lock held while gating; an unmarked trial directory refused; a gate that cannot start blamed on nobody; and the slot routing table.
- `README.md`: the section "Gate each batch before it lands" (the batch, the trial, the skip rule, attribution and bisection, `batch_max` and the busy fraction, and that P4 is only cheaper if sessions stop running the gates themselves, option V) and the new keys in "Configuration".
- `docs/incomplete/landing-layer/PLAN-P2-P4.md`: the heading `## 4. P4, one gate run per batch` restored above the amendment paragraph (clarification 5), and "As built, P4" at the end of section 4 with the proof.

Fixed: P4's scope items 1 to 5 and the owner amendments (the `wk-trial` marker, the queue-depth warning, bisection and `batch_max = 1`), and the findings rows 11 and 12 addressed to this row (the gate's `dir` joined onto `gate.Paths`; `-r` on `diff-tree`; a start error never blamed; a unique Blame only a first guess; a red batch of one failed with whatever paths the gate named; the `gated` crash case).

Questions answered:

- Clarification 7, a `gated` line and a crash: `gated` is pending. A restarted worker treats a `gated` or `landing` record as `submitted` and gates it again from its submitted tree before anything lands; a `landing` record whose `predicted` tree is already the tip's tree is `landed` at the tip without a commit, because its swap went through (a `gated` record never is, fix round 1, B2). Reversal: write no `gated` line and land straight after the green run (`gated` would then never be read).
- Clarification 11, an all-skip run: not green. Nothing lands, the worker ends with "every gate skipped over ... a skip is never a pass, so nothing lands", the changesets stay `submitted`, and `wk status` prints `last gate run: every gate skipped; nothing lands`. The timer retries each minute; an all-skip run costs no gate time. Reversal: one line in `verdict.green`, a run with no fail counts as green.
- Clarification 6: `batch.Shrink` is deleted and `batch.Halves` replaces it.
- Step 8 is covered by the run's independent Opus 5.5 auditor (clarification 3); no review subagent was spawned.

Build-level calls, each with its reversal:

- The trial is made detached by `Env.create` with an empty branch. Reversal: create it on a branch such as `wk/trial` and detach it afterwards.
- Bisection lands as it goes: the first half is resolved, and its green parts landed, before the second half is predicted afresh on the new tip, so no landed tree was gated on a different tip. Reversal: gate both halves on the old tip and land only after both are known (then the second half's tree must be gated again on top of the first).
- `Halves` puts the extra record of an odd batch in the first half. Reversal: `mid := len(batch) / 2`.
- A changeset that conflicts only with an earlier member of its batch waits, still `submitted`, for the next batch; it is refused only when it conflicts with the tip itself. Reversal: refuse it at once, as P3 would.
- Fresh caches are one set per gate run, shared by its gates, made in `internal/wk`, because the worker calls `gate.Run` once per gate in order to wait for memory before each. Reversal: one `gate.Run` over the whole list, with one memory wait per batch.
- `gates.min_free_mb = 0` turns the pause off (no `vm_stat` call). Reversal: refuse 0 at load.
- A batch whose members change no tree lands without a gate run. Reversal: gate it anyway.
- `main` is fast-forwarded only after a pass landed something; a refused fast-forward or push is logged and kept in `gates.json` for `wk status`, never an error, because the landing stands. A worker stopped between the land and the fast-forward leaves `main` behind until the next green batch, and `wk status` says so. Reversal: fast-forward at the end of every pass while the ref is ahead of `main`.
- A gate is routed through `wk run --slot` by quoting its argument vector, running `slot.Rewrite` on the line, and running the result with `/bin/sh -c`. Reversal: parse the inserted prefix back into words and run without a shell.
- A slots file that does not load is said on the log and the gates run unslotted, as the hook fails open. Reversal: stop the pass.
- The queue-depth warning fires when more than two batches of `batch_max` are pending (3 with `batch_max = 1`). Reversal: another multiple.
- A `failed` changeset keeps its leases, because its workspace fixes the change and submits again. Reversal: release them as on `landed`.
- A landed record's `reason` carries the gate summary, so a skipped gate stays visible on what landed (DESIGN.md section 6, "wk status shows the gap"). Reversal: no reason on `landed`.
- A directory at `gates.trial_dir` without the `wk-trial` marker is refused with what to do, never taken over. Reversal: none advised.
- The madge gates of ezhomesteading keep CI's commands with `dir` at the root, not row 11's recommendation of the target's directory, because a changed madge command once passed vacuously (`ci.yml`, F-1); their paths then come relative to the target, Blame does not match them, and a red madge gate is bisected. Reversal: `dir = "web/app"` with `--ts-config ../tsconfig.json .`, after checking it walks every file.

The lines for the lead to write after `GOTOOLCHAIN=local go install ./cmd/wk` from `main` (both were loaded by `wk config show` with this branch's binary against copies in the scratchpad, `scratchpad/row-5/tomlcheck/`: 4 and 25 gates, exit 0). `batch_max` is 1 in both, the default, written out so the dial is seen. The other `[gates]` keys keep their defaults: `trial_dir = ".claude/worktrees/wk-trial"`, `batch_idle_seconds = 120`, `min_free_mb = 2048`, `fresh_caches = ["GOCACHE", "GOLANGCI_LINT_CACHE"]`.

`~/.wk/wk.toml`, appended at the end (the file ends in `[landing]`, so `ff_main` joins that table):

```toml
ff_main = true

[gates]
batch_max = 1

[gates.env]
GOTOOLCHAIN = "local"
PATH = "/usr/local/go/bin:/Users/zachshort/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"

[[gates.run]]
name = "gofmt"
run = ["sh", "-c", 'out=$(gofmt -l .); printf "%s\n" "$out"; test -z "$out"']

[[gates.run]]
name = "vet"
run = ["go", "vet", "./..."]

[[gates.run]]
name = "test"
run = ["go", "test", "-race", "-count=1", "./..."]

[[gates.run]]
name = "lint"
run = ["golangci-lint", "run", "./..."]
```

`~/.wk/ezhomesteading.toml`: one line into the existing `[gates.env]`, after `API_URL`:

```toml
PATH = "/usr/local/go/bin:/Users/zachshort/go/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
```

and appended at the end (the file ends in `[landing]` with `enabled = false`, which stays; `ff_main` joins that table):

```toml
ff_main = true

# The batch gate (wk board row 5, 2026-10-17): the CLAUDE.md gates in ci.yml's order. The DB
# suites and sqlgate need EZH_TEST_DB and skip without it; a skip is never a pass.
[gates]
batch_max = 1

[[gates.run]]
name = "go-build"
dir = "backend"
run = ["go", "build", "./..."]

[[gates.run]]
name = "go-vet"
dir = "backend"
run = ["go", "vet", "./..."]

[[gates.run]]
name = "lintgate"
dir = "backend"
run = ["go", "run", "./cmd/lintgate"]

[[gates.run]]
name = "synchelp"
dir = "backend"
run = ["go", "run", "./cmd/synchelp", "--check"]

[[gates.run]]
name = "sqlgate"
dir = "backend"
run = ["go", "run", "./cmd/sqlgate"]
needs_env = ["EZH_TEST_DB"]

[[gates.run]]
name = "go-test"
dir = "backend"
run = ["go", "test", "-race", "./..."]

[[gates.run]]
name = "db-suites"
dir = "backend"
run = ["sh", "-c", '''go test -race -count=1 -v ./integration/ ./repository/ > db-suites.log 2>&1 || { tail -80 db-suites.log; exit 1; }; if grep -- '--- SKIP' db-suites.log | grep -v -e 'TestLiveStripe' -e 'TestChaosCompose'; then exit 1; fi''']
needs_env = ["EZH_TEST_DB"]

[[gates.run]]
name = "test"
run = ["bun", "run", "test"]

[[gates.run]]
name = "web-typegen"
dir = "web"
run = ["bunx", "next", "typegen"]

[[gates.run]]
name = "web-tsc"
dir = "web"
run = ["bunx", "tsc", "--noEmit"]

[[gates.run]]
name = "web-build"
dir = "web"
run = ["bun", "run", "build"]

[[gates.run]]
name = "native-expo-env"
run = ["sh", "-c", '''printf '/// <reference types="expo/types" />\n' > native/expo-env.d.ts''']

[[gates.run]]
name = "native-tsc"
dir = "native"
run = ["bunx", "tsc", "--noEmit"]

[[gates.run]]
name = "native-lint"
dir = "native"
run = ["bun", "run", "lint"]

[[gates.run]]
name = "web-lint"
dir = "web"
run = ["bun", "run", "lint"]

[[gates.run]]
name = "lint-api"
dir = "packages/api"
run = ["bun", "run", "lint"]

[[gates.run]]
name = "lint-config"
dir = "packages/config"
run = ["bun", "run", "lint"]

[[gates.run]]
name = "lint-core"
dir = "packages/core"
run = ["bun", "run", "lint"]

[[gates.run]]
name = "lint-icons"
dir = "packages/icons"
run = ["bun", "run", "lint"]

[[gates.run]]
name = "lint-eslint-config"
dir = "packages/eslint-config"
run = ["bun", "run", "lint"]

[[gates.run]]
name = "08-verify"
run = ["bun", "scripts/codemods/08-verify.ts"]

[[gates.run]]
name = "madge-core"
run = ["bunx", "madge", "--circular", "--extensions", "ts,tsx", "packages/core/src/index.ts"]

[[gates.run]]
name = "madge-web"
run = ["bunx", "madge", "--circular", "--extensions", "ts,tsx", "--ts-config", "web/tsconfig.json", "web/app"]

[[gates.run]]
name = "madge-native"
run = ["sh", "-c", '''out=$(bunx madge --circular --warning --extensions ts,tsx --ts-config native/tsconfig.json native/app) || status=$?; echo "$out"; if echo "$out" | grep -q '^@/'; then echo "madge cannot resolve native's @/ aliases"; exit 1; fi; exit ${status:-0}''']

[[gates.run]]
name = "parity"
run = ["bun", "scripts/check-feature-parity.ts"]
```

The `PATH` line is there because a worker started by the launchd timer gets the plist's `PATH` (`/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin`), which lacks `/usr/local/go/bin` (`go`, `gofmt`) and `/Users/zachshort/go/bin` (`golangci-lint`); `[gates.env]` reaches each gate's children, but `exec` finds a gate's first word on the worker's own `PATH`, so the plist's `PATH` needs the same two directories too. Without them a Go gate run as it is fails to start, and a slotted one (`go test`, `go run ./cmd/lintgate`) is refused before it starts because its command is not on the gate's `PATH`; either way the worker stops with the error and blames nobody (since fix round 1; before it, a slotted gate's missing command came back as exit 127 and failed an innocent changeset). Plain command names are kept, not absolute paths, so the slots file's spellings (`go test`, `go run ./cmd/lintgate`, `bun run build`, `bunx tsc`) still match.

With these lines and `landing.enabled = true` in `wk.toml`, the worker gates wk's changesets, fast-forwards wk's `main` and pushes it to `origin` by itself (D14, D20).

Left owed:

- Findings for `internal/gate`, not changed here (out of scope): `go vet` prints a type error as `vet: ./b.go:3:23: ...`, a prefix `gate.Paths` does not read, so a type error found by `go vet` names no path and its batch is bisected (seen in the proof's first run, `proof-run1.out`); and `gate.Run` has no timeout, so a hung gate holds the trial and the worker.
- The trial's dependencies: `read-tree` changes tracked files only, and the post-clone commands run once, when the trial is made, so a batch that changes `bun.lock` is gated over the dependencies the trial was made with. Reversal: run `template.post_clone` in the trial when the batch's tree changes `template.refresh_on`.
- A re-gate without the blamed changeset, or the second half of a bisection, predicts its members again on the tip; one that was clean on top of the left-out changeset and conflicts with the tip alone is refused (loud, `wk sync`), which a later green would not have needed. Rare; not handled.
- The plist `PATH` above, and the timer itself, are the owner's to install.
- Rows 11 and 12's other notes stay as they were (a background child after `pipeGrace`; `Seconds` including `pipeGrace`).

Hand-back:

- Gate, `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, run in `ap-row-5` on 2026-10-17: 14:21 EDT at `8f2d091`'s tree, exit 0, every package `ok` (`internal/wk` 73.3 s), `0 issues.`, 1 min 23 s; 14:30 EDT at `31006c5`, exit 0, every package `ok` (`internal/wk` 64.0 s, `internal/batch` 1.8 s, `internal/config` 2.3 s), `0 issues.`, 1 min 13 s, load 4.37.
- Proof: `scratchpad/row-5/proof.sh`, `proof.out`, `samples.txt`; the summary and every number are in "As built, P4". The landing ref, the checkout's `main` and `origin.git`'s `main` all hold `09fe954428f988b75f247962e3c7e7d38554d5a2`, the last landing commit.

### Fix round 1, 2026-10-17

From the independent Opus 5.5 audit of `c4f9fac`.

- B1 (blocking, fixed): a slotted gate whose command could not start came back from `/bin/sh` and `wk run` as exit 127 (or 126), which `gate.Run` reads as a fail, so the changeset was marked `failed` with `exit status 127`. `gateList` now returns a `gateSpec` per gate (the gate, the configured first word, and whether `slotted` routed it, which `slotted` now returns), and the new `internal/wk/gatestart.go` holds two checks for slotted gates only. `canStart` resolves the first word before the gate runs, on the gate's `PATH` (`[gates.env]` over the worker's own) and in the gate's directory, as the shell and `wk run` will, and returns a start error when it is missing; a gate that will skip is not checked. `wrapperFailed` turns a slotted gate's exit 126 or 127 into a start error, because `wk run` exits so for a command it cannot run or find. Both errors take the path of an unslotted start error: the pass stops and nobody is blamed. A gate run as it is is not checked, because `exec` already reports its missing command as an error, and its first word is found on the worker's `PATH`, not `[gates.env]`'s. Exit 2, `wk run`'s usage error, stays a fail, because `tsc` and other gates exit 2 for a red tree. Reversal of the 126 and 127 rule: a slotted command that itself exits 126 or 127 for a red tree is then a start error; drop `wrapperFailed` if that is ever seen. Tests: `TestSlottedGateWithAMissingCommandBlamesNothing` (the start error, the changeset still `submitted`, and the stand-in `wk` never started) and `TestSlottedGateExit127IsAStartError` (exit 127 and 126 are start errors, exit 1 is still `failed`); both fail with the checks turned off. The close-out sentence on the timer's `PATH` is corrected above.
- B2 (blocking, fixed): `requeue` landed any `gated` or `landing` record whose `predicted` tree equalled the tip's tree. `landPlan` writes every member's `gated` line before any swap, so a gated prediction was made on a tip it may never have seen, and the audit's probe recorded a changeset `landed` whose file was not on the tip. The shortcut is now taken for `landing` records only; a gated member that changes nothing re-predicts to the tip and lands at it anyway. Reversal: none advised. Test: `TestGatedRecordEqualToTheTipIsNotTakenAsLanded` (both members gated again as one batch, one gate run, both files on the landing tip); it fails with the old condition. The "Questions answered" entry for clarification 7 now reads: a `landing` record whose `predicted` tree is the tip's tree is landed without a commit; a `gated` record is always gated again.
- N1 (non-blocking, fixed): after a killed worker, `wk status` printed "batch in flight" for ever, because `gates.json` stayed at `gating`. The gating record now carries the worker's PID, and `wk status` prints `stale gate run: ..., by worker <pid>, which has stopped; the next worker gates the batch again` when that process is gone (signal 0; EPERM counts as alive), and `batch in flight: ..., worker <pid>` while it runs. The status reads the process, not `landing.lock`, because a status that took the lock even for a moment could make a worker started at that moment leave its queue to nobody. Reversal: probe `landing.lock` with `flock.TryTake`, or clear `gates.json` when a worker starts. Test: `TestStatusSaysAStoppedWorkersBatchIsStale`.
- N3 (not changed): the marker cannot move before the create, because the trial's private git directory does not exist until `worktree add` runs inside `Env.create`; it is not a one-line reorder. N2 and N4 are left, as asked.
- The landing path changed in one place: `requeue`'s recovery of a `gated` record after a stopped worker, which the live proof never reaches (no worker was stopped in it). The batch, gate, land and fast-forward path the proof ran is unchanged, so the proof was not run again.
- Gate after the fix, 2026-10-17 14:49 EDT in `ap-row-5` (main still `78dc9b9`, `git merge --ff-only main` "Already up to date"): exit 0, every package `ok` (`internal/wk` 41.2 s), `0 issues.`, 47 s, load 3.40. A first run at 14:49 stopped at `gofmt -l` on the new test, which `gofmt -w` fixed.
