# 4. Build P3, the landing queue: integrate `wk submit`, the landing loop, `wk sync` and `wk land`

> **Claimed `IN FLIGHT` 2026-10-17.**

*Opened 2026-10-16, when the landing-layer work moved here from `~/Projects/ezhomesteading` (its board row 196, now SUPERSEDED). Prompt filled 2026-10-16 by row 2 from `docs/incomplete/landing-layer/PLAN-P2-P4.md` section 3.*

**Model: Fable 5.1. Lane A. Waits on 3; 9 and 10 (the P3 siblings).**

**Model: Fable 5.1** (Deep tier): this row moves `wk/landing` and, through `wk land --apply`, `main`, and a wrong tree on the landing ref compiles, passes every gate and is wrong. If you are not on Fable 5.1, stop and say so.

You are the integration row of P3 of the landing layer. Read, in this order: `docs/incomplete/landing-layer/PLAN-P2-P4.md` sections 0, 1 and 3 (binding), `DESIGN.md` D14, D15, D20, D26, options N to R and section 6, the close-outs of rows 9 and 10 in `passoff/9-*.md` and `10-*.md`, `internal/changeset` and `internal/landing` as `main` holds them, `internal/wk/daemon.go` and `internal/wk/lease.go` (P2's daemon and ops), `internal/config/config.go` and `cmd/wk/main.go`. D14's condition is met: `~/.claude/rules/commits.md` allows every git command (plan section 0). PD-6 is the order of this phase: the loop lands ungated onto `wk/landing`, and `main` moves only by `wk land --apply` until P4.

Do these, in order:
1. `internal/config/config.go`: the `[landing]` section (`enabled`, `ref`, `remote`, `push`) with the plan's defaults and validation (`ref` must start with `refs/heads/`). Additive.
2. `internal/wk/submit.go` and `cmd/wk/main.go`: `wk submit -m "<message>" [--row <n>]`: refuse the main checkout (D15) and a repository with `landing.enabled = false`; `changeset.Tree` and `changeset.Base` (PD-7); the row from the P2 claim of this workspace, `--row` over it, no prefix when there is none; message `row <n>: <message>` lowercased (option Q); the `submit` op returns the id, which is the only line on stdout.
3. `internal/wk/landing.go`: the loop as a goroutine of the daemon, started by `wk up` when `landing.enabled`: `landing.Ensure`, then for each `submitted` record in order `landing.Predict`, then `landing.Land` by the CAS, append `landed` or `refused` (PD-8), release the workspace's leases on `landed`, stop with a status reason when `main` and the ref diverged. One goroutine; a pass every 5 s and on every `submit` op (PD-13).
4. `wk sync` (`landing.Sync`, with PD-7's refusal), `wk land` (the plan, the blockers in the main checkout) and `wk land --apply --expect <hash>` (`landing.FastForward` then `landing.Push` when `landing.push`), in `internal/wk/landing.go` and `cmd/wk/main.go`.
5. `internal/wk/status.go`: `wk status` lists every changeset with its state and reason, after the leases.
6. `README.md`: a section on submit, sync, land, the queue file, and that agents in a P3 repository run `wk submit` and never `git commit`, `git merge` or `git push` in a workspace (D14).
7. `GOTOOLCHAIN=local go install ./cmd/wk` before any new key; then `[landing]` with `enabled = false` into `~/.wk/ezhomesteading.toml`, and `enabled = true` into `~/.wk/wk.toml`.
8. The live proof of the plan, section 3 "Done when": a throwaway repository in the session scratchpad whose `origin` is a `git init --bare` directory beside it, with its own `~/.wk/<name>.toml` (`landing.enabled = true`), the shim log of row 10 run over the whole loop, `wk land --apply` moving that repository's `main` and pushing it to the bare remote. Never submit, land, fast-forward or push in this repository (`~/Projects/wk`): under this run only the orchestrating session moves its `main`, after an audit. Record the proof under "As built, P3" in `PLAN-P2-P4.md`.
9. Review: spawn one Fable 5.1 subagent in its own worktree with the diff applied there, briefed to read `internal/wk/landing.go` and `submit.go` for any path where the tree on the landing ref can differ from the predicted tree, where a ref is written without the CAS guard, where `main` is named outside `FastForward`, or where a refused changeset is retried without a new submit; it edits nothing and returns findings with file and line. Check its diffstat. Record each finding's outcome.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, plus the "Done when" of P3 in `PLAN-P2-P4.md` section 3.

Ask before building: nothing; every call is in the plan (PD-6 to PD-8). Only a fact that section 0 of the plan finds moved.
Not in scope, whoever asks: the batch gate, the trial workspace, the automatic fast-forward (P4); any change inside `internal/changeset` or `internal/landing` beyond a finding of step 9 (fixed here and named); folding `/land-worktrees`.

Hand back: the gate output, the proof, the As built note, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. Commit on your own worktree branch only; the lead merges, pushes and removes the worktree after the audit. No `Co-Authored-By:` trailer and no "Generated with" line.

## Owner amendments, 2026-10-16

From the owner's research review of 2026-10-16, applied by the autopilot run before this row launched. They bind the build; where they differ from the steps above, they win.

- Held until the daemon-or-files question is decided by the owner's rule (see row 3's amendments: `docs/incomplete/integration/KILL-TEST.md`, files unless the file store shows more lost or stale leases, or more manual recovery steps, than the daemon). If the rule picks files, the landing loop is a detached run-to-completion worker started by `wk submit` and by the launchd timer, not a goroutine of the daemon; the plan's P3 section is changed before this row launches.
- Journal the landing commit hash before the compare-and-swap, and treat a merge result equal to the tip's tree as already landed, so a crash cannot write a duplicate commit.
- After a changeset lands, gc treats the workspace as absorbed when its working tree, written through a private index, equals the landed tree.
- Give `wk sync` a path for a branch that carries commits (PD-7 refuses it today), and list at submit time what `add -A` will include.
- A changeset carries a reference to its review, and `landing.require_review` refuses to land without one. Default (owner's choice): off, so D20 stands.

### Decided by the rule, 2026-10-16 22:51 EDT: files

The kill test picked files (see row 3's amendments). Step 3's landing loop is therefore a detached run-to-completion worker, `wk land --loop` (the `spawnFill` pattern of P1), started by `wk submit` after it appends the record and by the launchd timer; one worker at a time under `~/.wk/<repo>/landing.lock`; each pass runs `landing.Ensure`, then for each `submitted` record `Predict` then `Land`, appends `landed` or `refused`, and exits when the queue holds no `submitted` record. Before each `Land` the worker appends a `landing` record carrying the predicted tree, and a pass that finds a `submitted` or `landing` record whose tree equals the tip's tree appends `landed` without a new commit (the journal of the amendment above; the kill test's F18). PD-13's "a pass every 5 s" becomes the launchd timer's interval plus the start on every `submit`. `internal/landing` keeps no state between passes (verified in the kill test's last section), so nothing of the daemon is needed. The plan's section 3 carries a dated note pointing here.

## Close-out, 2026-10-17

Built by a Fable 5.1 builder in the worktree `ap-row-4` on `ap/row-4`, from `main` at `e13aa27` (fast-forwarded from `5bf4e7d` before the first gate). Commits: `4774e55` (the code and its tests) and the commit that adds the README section, the As built note and this section. Not merged, not pushed; the lead lands it after the audit.

What changed and why:

- `internal/config/config.go`: the `[landing]` section (`enabled`, `ref`, `remote`, `push`, `require_review`) with the plan's defaults and validation (`ref` must start with `refs/heads/` and name a branch; `remote` not empty). Additive; `config.Show` prints it. `internal/config/landing_test.go` covers the defaults, the additive load, five refusals and the show.
- `internal/changeset/queue.go`: the `Landing` state and the `Review` field on `Record`, which the owner amendments need (the journal line before the swap, and `require_review`). The queue's doc line now names its two writers.
- `internal/wk/submit.go`: `Env.Submit` as the plan's scope item 1 and the amendment on listing what `add -A` includes; `workspaceOnly` is D15's refusal, shared with `wk sync`.
- `internal/wk/landing.go`: `Env.LandLoop` (the worker), `landPass`, `landOne`, `Env.Sync` with `foldCommits`, `Env.Land`, `Env.LandApply` and the read-only `landingState`. Every ref write goes through `internal/landing`, except the fold's `update-ref refs/heads/<branch> <base> <head>`, which carries the old value.
- `internal/wk/status.go`: `Env.Status` (the leases, then the queue) and `Env.QueueStatus`.
- `internal/wk/landing_test.go`: eight tests, among them the section 3 proof in a fixture (two overlapping changesets, the refusal, the sync with markers, the resubmit, the landed tree against `landing.Predict`), the journal line landed without a commit and the empty submit landed without a commit, `require_review`, the refusals, the held lock, the fold, the dry run with a blocker and the apply with a stale hash and then the push to a bare remote, and the diverged ref in the worker and in `wk status`.
- `cmd/wk/main.go`: `wk submit`, `wk sync`, `wk land [--loop | --apply --expect <hash>]`, `wk status` through `Env.Status`, `spawnLand` through `daemon.Spawn`, the usage text. `cmd/wk/landing_test.go`: the id alone on stdout, the included paths on stderr, the worker and status through the command line, and nine argument lists that must be usage errors.
- `README.md`: the section "Land work through the queue" with the launchd plist, and the `[landing]` keys in "Configuration".
- `docs/incomplete/landing-layer/PLAN-P2-P4.md`: "As built, P3" under section 3.

Fixed: the three P3 scope items the siblings left to this row (the subcommands, the loop, the status) and the four owner amendments that are built (the journal line and the landed-without-a-commit rule; `wk sync` for a branch with commits; the list at submit time; `require_review`).

Questions answered: the base of a changeset while the landing ref does not exist is the base branch's hash, where the first pass creates the ref; a `landing` record is re-predicted from its submitted tree (`Tree`), never from the journalled prediction, which is right only against the tip it was made on; the prediction is journalled in a second field, `Predicted` (corrected in fix round 1, F1; the first build re-predicted from the prediction, which was wrong); a refused swap ends the worker instead of looping; `wk sync` exits 1 on a conflict so a script sees it.

Build-level calls, each with its reversal:

- The gc amendment is not built (see the As built note for why: gc never forces, so an absorbed verdict could not remove a dirty worktree). Reversal: a queue read in `takeSurvey` and a `changeset.Tree` compare in `judge`. The owner may want this; it is a question below.
- The worker reads the base branch from `workspace.base` (`main` by default), the one name the config already has. Reversal: a `landing.base` key.
- `wk sync` folds commits with no flag, saying how many and that they stay in the reflog. Reversal: refuse as PD-7 did unless `--fold` is given.
- A `submitted` or `landing` record whose tree, or whose prediction, is the tip's tree is `landed` at the tip. Reversal: an empty landing commit.
- The timer's interval is 60 s in the plist text. Reversal: any other integer.
- The landing line journals the prediction in a second field, `Predicted`, and `Tree` stays the submitted tree for the record's whole life, so a rerun after a crash predicts from what was submitted (fix round 1, F1). Reversal: the single field of the first build, which carried the old tip's changes over a moved tip and is the defect.

Left owed: step 7 (the lead, at the land step); step 9 (this run's auditor); the gc amendment; row 10's duplicate tree builder in `landing.Sync` (the plan's "Not built" names row 4 as the one to fold it; not done, because `landing.Sync` takes a base and a tip, not a tree, and the fold would change `internal/landing`); row 12's `type State string` note; `internal/daemon`'s unused `Server` and `Client`, which `CLAUDE.md` says go after P3 lands.

As built: `docs/incomplete/landing-layer/PLAN-P2-P4.md`, section 3, "As built, P3".

Hand-back:

- Gate, `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, run twice in `ap-row-4` on 2026-10-17: 11:01 EDT at load 6.76, every package `ok` (`internal/wk` 72.2 s, `internal/landing` 20.2 s), `0 issues.`, 1 min 28 s; 11:05 EDT at load 6.33, exit 0, `internal/wk` 70.0 s, `0 issues.`
- Proof: `scratchpad/row-4/proof.sh` and `proof.out`; summary in the As built note. 127 git calls, 4 `update-ref` (three on the landing ref with the old value, one on `refs/heads/b` in the sync), 2 `commit-tree` with the predicted trees, no ref write naming `main`; `wk land --apply --expect <tip>` moved `main` to `dcb271e` and pushed it to the bare `origin.git`.
- The config lines the lead writes after `GOTOOLCHAIN=local go install ./cmd/wk` from `main`:

  `~/.wk/ezhomesteading.toml`, appended:

  ```toml
  [landing]
  enabled = false
  ```

  `~/.wk/wk.toml`, appended:

  ```toml
  [landing]
  enabled = true
  ```

  The other keys keep their defaults: `ref = "refs/heads/wk/landing"`, `remote = "origin"`, `push = true`, `require_review = false`.

- The launchd timer for this repository, which the owner saves as `~/Library/LaunchAgents/com.github.zach-short.wk.landing.wk.plist` and loads with `launchctl bootstrap gui/$(id -u) ~/Library/LaunchAgents/com.github.zach-short.wk.landing.wk.plist`:

  ```xml
  <?xml version="1.0" encoding="UTF-8"?>
  <!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
  <plist version="1.0">
  <dict>
  	<key>Label</key>
  	<string>com.github.zach-short.wk.landing.wk</string>
  	<key>ProgramArguments</key>
  	<array>
  		<string>/Users/zachshort/go/bin/wk</string>
  		<string>--repo</string>
  		<string>/Users/zachshort/Projects/wk</string>
  		<string>land</string>
  		<string>--loop</string>
  	</array>
  	<key>EnvironmentVariables</key>
  	<dict>
  		<key>PATH</key>
  		<string>/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
  	</dict>
  	<key>WorkingDirectory</key>
  	<string>/</string>
  	<key>StartInterval</key>
  	<integer>60</integer>
  	<key>RunAtLoad</key>
  	<false/>
  	<key>ProcessType</key>
  	<string>Background</string>
  	<key>StandardOutPath</key>
  	<string>/Users/zachshort/.wk/wk/landing.log</string>
  	<key>StandardErrorPath</key>
  	<string>/Users/zachshort/.wk/wk/landing.log</string>
  </dict>
  </plist>
  ```

Questions for the owner, decided here by the rule (prevent a silent failure, keep work landing, the least code) and reversible:

1. The gc amendment: is a reason line naming the landed changeset enough, or should gc clean the workspace (a `wk sync` by gc, which writes a workspace's index and tree)? Built: neither; `wk sync` by the workspace's session makes it clean.
2. Should `wk sync` fold a branch's commits without a flag? Built: yes, with the count and the reflog said on stderr.

### Fix round 1, 2026-10-17

From audit 1 (`docs/incomplete/autopilot-2026-10-16/reports/row-4-audit-1.md`), after `git merge --no-edit main` brought `941667c` and the row state files (`6a840d4`).

- F1 (blocking, fixed): `changeset.Record` has `Predicted` (`omitempty`), the tree the landing line journals; `Tree` is the submitted tree for the record's whole life. `landOne` predicts from `r.Tree` and `r.Base` on every pass, sets `r.Predicted` on the landing line, and lands without a commit when `Tree` or `Predicted` equals the tip's tree. `TestLoopLandsWhatIsAlreadyOnTheTipWithoutACommit` sets the new field on its hand-made journal. New test `TestLoopAfterACrashLandsOnlyTheSubmittedChange`: `a` lands and is applied; `b`, two landings behind, edits one file; a git on PATH fails the first swap, as a killed worker leaves it; main moves ahead with a hand revert of `a`'s file; the rerun lands a commit whose diff against its parent is the one submitted path, and the reverted file keeps the revert. Reproduced live too: `proof.sh` section 8 (`proof-fix1.out`): `c` at M0 edits `docs/NOTES.md`, killed before the swap (the shim's `WK_KILL_ON_SWAP`), the landing line holds `tree` and `predicted`, main moves ahead with the revert of `web/page.ts`, the rerun lands `7a7a627` on top of main with `1 file changed` (`docs/NOTES.md`) and `page.ts = 1`.
- N1 (fixed): `Config.validate` refuses `landing.ref == "refs/heads/" + workspace.base`; two rows in `TestLandingValidation` (`main`, and `trunk` with `workspace.base = "trunk"`).
- N2 (fixed): `landingState` reads `refs/remotes/<remote>/<base>`; when it is an ancestor of the base branch and not at it, `wk land` and `wk status` print one line saying it is behind and that `wk land --apply --expect <base hash>` pushes it (the fast-forward is then a no-op). `TestLandAndStatusSayWhenThePushWasRefused`: a `pre-receive` hook in the bare remote exits 1; the apply fails with the hook's message, both commands print the line, the hook is removed, the apply again pushes, and the line is gone.
- N5 (fixed): the `require_review` check comes before the tip-tree shortcut; `TestRequireReviewRefusesAChangesetWithoutOne` adds an empty submit without a review, refused.
- N3 (open): `wk submit` appends to the queue with no lock, relying on `O_APPEND` and one write per line; a line torn by a crash mid-write fails `Load` for every reader until a hand repair.
- N4 (open): `foldCommits` moves the branch before `landing.Sync` runs; a failure after the fold leaves the branch at the merge base with the work in the tree and the reflog hash on stderr.
- N6 (open): `spawnLand` returns silently when `installedExe` fails or the binary is transient; the record stays `submitted` for the timer.

Line counts after the fix: `landing.go` 340, `submit.go` 113 (`Submit` 40 lines, unchanged), `status.go` 52; `landOne` 31.
