# 4. Build P3, the landing queue: integrate `wk submit`, the landing loop, `wk sync` and `wk land`

*Opened 2026-10-16, when the landing-layer work moved here from `~/Projects/ezhomesteading` (its board row 196, now SUPERSEDED). Prompt filled 2026-10-16 by row 2 from `docs/incomplete/landing-layer/PLAN-P2-P4.md` section 3.*

**Model: Fable 5.1. Lane A. Waits on 3; 9 and 10 (the P3 siblings).**

**Model: Fable 5.1** (Deep tier): this row moves `wk/landing` and, through `wk land --apply`, `main`, and a wrong tree on the landing ref compiles, passes every gate and is wrong. If you are not on Fable 5.1, stop and say so.

You are the integration row of P3 of the landing layer. Read, in this order: `docs/incomplete/landing-layer/PLAN-P2-P4.md` sections 0, 1 and 3 (binding), `DESIGN.md` D14, D15, D20, D26, options N to R and section 6, the close-outs of rows 9 and 10 in `passoff/9-*.md` and `10-*.md`, `internal/changeset` and `internal/landing` as `main` holds them, `internal/wk/daemon.go` and `internal/wk/lease.go` (P2's daemon and ops), `internal/config/config.go` and `cmd/wk/main.go`. D14's condition is met: `~/.claude/rules/commits.md` allows every git command (plan section 0). PD-6 is the order of this phase: the loop lands ungated onto `wk/landing`, and `main` moves only by `wk land --apply` until P4.

Do these, in order:
1. `internal/config/config.go`: the `[landing]` section (`enabled`, `ref`, `remote`, `push`) with the plan's defaults and validation (`ref` must start with `refs/heads/`). Additive.
2. `internal/wk/submit.go` and `cmd/wk/main.go`: `wk submit -m "<message>" [--row <n>]`: refuse the main checkout (D15) and a repository with `landing.enabled = false`; `changeset.Tree` and `changeset.Base` (PD-7); the row from the P2 claim of this workspace, `--row` over it, no prefix when there is none; message `row <n>: <message>` lowercased (option Q); the `submit` op returns the id, which is the only line on stdout.
3. `internal/wk/landing.go`: the loop as a goroutine of the daemon, started by `wk up` when `landing.enabled`: `landing.Ensure`, then for each `submitted` record in order `landing.Predict`, then `landing.Land` by the CAS, append `landed` or `refused` (PD-8), release the workspace's leases on `landed`, stop with a status reason when `main` and the ref diverged. One goroutine; a pass every 5 s and on every `submit` op.
4. `wk sync` (`landing.Sync`, with PD-7's refusal), `wk land` (the plan, the blockers in the main checkout) and `wk land --apply --expect <hash>` (`landing.FastForward` then `landing.Push` when `landing.push`), in `internal/wk/landing.go` and `cmd/wk/main.go`.
5. `internal/wk/status.go`: `wk status` lists every changeset with its state and reason, after the leases.
6. `README.md`: a section on submit, sync, land, the queue file, and that agents in a P3 repository run `wk submit` and never `git commit`, `git merge` or `git push` in a workspace (D14).
7. `GOTOOLCHAIN=local go install ./cmd/wk` before any new key; then `[landing]` with `enabled = false` into `~/.wk/ezhomesteading.toml`, and `enabled = true` into `~/.wk/wk.toml`.
8. The live proof of the plan, section 3 "Done when": the throwaway repository with a local bare remote, the shim log of row 10 run over the whole loop, and one real changeset of this repository landed on `wk/landing` then fast-forwarded and pushed by `wk land --apply`. Record it under "As built, P3" in `PLAN-P2-P4.md`.
9. Review: spawn one Fable 5.1 subagent in its own worktree with the diff applied there, briefed to read `internal/wk/landing.go` and `submit.go` for any path where the tree on the landing ref can differ from the predicted tree, where a ref is written without the CAS guard, where `main` is named outside `FastForward`, or where a refused changeset is retried without a new submit; it edits nothing and returns findings with file and line. Check its diffstat. Record each finding's outcome.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, plus the "Done when" of P3 in `PLAN-P2-P4.md` section 3.

Ask before building: nothing; every call is in the plan (PD-6 to PD-8). Only a fact that section 0 of the plan finds moved.
Not in scope, whoever asks: the batch gate, the trial workspace, the automatic fast-forward (P4); any change inside `internal/changeset` or `internal/landing` beyond a finding of step 9 (fixed here and named); folding `/land-worktrees`.

Hand back: the gate output, the proof, the As built note, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.
