# Row 1, builder report (Opus 5.5, agent a62bd52ebf20cb308), arrived 2026-10-16 20:58 EDT

Commit `7be3406` on `ap/row-1` ("fill CLAUDE.md from the part 0 inventory and close out item 1"). No trailer. Not merged, not pushed. No hook block, no patch. No new files.

Diffstat (`git diff --stat main...HEAD`): CLAUDE.md 175 (+170/-17 total with the item file); passoff/1-run-part-0-for-wk.md 12; 2 files changed.

## Gates seen

Worktree at `a4d4ed2` after `git merge --ff-only main` (Already up to date) and `GOTOOLCHAIN=local go mod download` (exit 0). 2026-10-16 20:45 EDT, full gate line: every package ok, `0 issues.`, exit 0, 43.98 s wall; `internal/wk` 39.983 s.
Also green: `git archive HEAD` copy with empty `GOMODCACHE` (go mod download, go build ./..., full gate, exit 0); `GOLANGCI_LINT_CACHE=$(mktemp -d) golangci-lint run ./...` 0 issues; gofmt, go vet, go build each exit 0; `GOBIN=$(mktemp -d) go install ./cmd/wk` exit 0 (real `~/go/bin/wk` untouched). Proposed gateCommand run through bash `eval`: exit 0 in 44.8 s. Gate not re-run after the doc-only edits.

## Numbers claimed

Gate about 44 s; `internal/wk` about 40 s under -race (39.98, 40.88, 41.95 s). 80 of 120 .go files in the primary checkout are under `.claude/worktrees/`; `go list ./...` gives 13 packages, none under `.claude/`; 0 tracked .go files outside cmd/ and internal/. No ledger or migration number claimed.

## Build-level calls

1. Fresh-checkout recipe = `GOTOOLCHAIN=local go mod download` only (no gitignored file needed). Reversal: edit the one block.
2. "The rules that get broken" section lists rules from code comments and the README, not incidents, and says so. Reversal: cut it to incidents.
3. Gate line kept exactly as the item wrote it; the scoped form `gofmt -l cmd internal` is documented as the way to run it from the main checkout.

## Questions for the owner (routed by the wake procedure's step 4)

1. Is a 45 s `gateCommand` on every stop acceptable, or a lighter one without -race? Proposed value (no double quotes, because the hook reads it with sed and runs it with eval): `export GOTOOLCHAIN=local && ! gofmt -l cmd internal | grep -q . && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...`
2. `README.md:5` still says the design and plan live in ezhomesteading; they are in `docs/incomplete/landing-layer/` here. Fix in a separate row?
3. `git worktree list` shows `/Users/zachshort/Projects/landing-layer-phase3` (`p1-phase-3`, `30989b5`, in main) as prunable; run `git worktree prune` and delete the branch?

## Left owed to this session

- Paste the six blocks below into HANDOFF.md (Environment, Code map, Invariants, Known facts) and docs/AGENT-PRACTICES.md (the two placeholders).
- Log line `- 2026-10-16 · closed item 1` after landing.
- `gateCommand` in `/Users/zachshort/Projects/wk/.personal-config.json` (untracked): see question 1.
- Part 0 steps 0.4, 0.6, 0.7, 0.8 in `docs/AGENT-PRACTICES.md` (cut CI and migrations; fix survivors of `grep -n 'Part 12'`; ask the batched GATE 0 question; stamp the file and delete Part 0; add `· adapted`). Untracked file, so the worktree build could not do them.

## Not verified

A real launchd load, a real Claude Code hook call, a `df` measurement, the TTY screens (bare `wk`, `wk hive`), the `wk viz` page.

## Warnings

In the main checkout `gofmt -l .` reads every worktree under `.claude/worktrees/`. The golangci-lint cache is shared across checkouts. `~/go/bin/wk` was built from `30989b5` with a dirty tree and is what ezhomesteading's hooks and launchd agent run.

## Hand-back blocks, paste-ready

### HANDOFF.md: Environment

| Fact | Value | Source (verified 2026-10-16) |
|---|---|---|
| Toolchain | Go `go1.24.1 darwin/arm64` at `/usr/local/go/bin/go`; `go.mod:3` asks for `go 1.24.0` | `go version`; `go.mod` |
| Toolchain pin | `GOTOOLCHAIN=local` on every Go command; the default is `auto`, which downloads a newer toolchain when `go.mod` asks for one | `go env GOTOOLCHAIN` printed `auto` |
| Fresh checkout | `GOTOOLCHAIN=local go mod download`; nothing gitignored is needed for the gates | Ran in a `git archive HEAD` copy with an empty `GOMODCACHE`; full gate green there |
| Build | `GOTOOLCHAIN=local go build ./...` (under 1 s warm) | Ran in worktree `ap/row-1` at `a4d4ed2` |
| Format | `test -z "$(gofmt -l .)"` | Ran, exit 0 |
| Typecheck / vet | `GOTOOLCHAIN=local go vet ./...` | Ran, exit 0 |
| Test | `GOTOOLCHAIN=local go test -race -count=1 ./...`; `internal/wk` takes about 40 s under `-race` | Ran, all `ok` |
| Lint | `GOTOOLCHAIN=local golangci-lint run ./...`; golangci-lint v2.12.2 at `/Users/zachshort/go/bin/golangci-lint`; no config file, so the v2 default set (errcheck, govet, ineffassign, staticcheck, unused) | Ran, `0 issues.`; also `0 issues.` with a fresh `GOLANGCI_LINT_CACHE` |
| Whole gate | `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`, about 44 s | Ran, exit 0, in the worktree and in an archive copy |
| Install | `GOTOOLCHAIN=local go install ./cmd/wk` writes `/Users/zachshort/go/bin/wk`, the binary that ezhomesteading's hooks and launchd gc agent run | Ran only into a temporary `GOBIN` |
| CI | None: no `.github/` directory | `ls .github` |
| Ship | No tags and no CI. A merge to `main` changes nothing that runs until `go install ./cmd/wk` is run. The installed `~/go/bin/wk` was built from `30989b5` with a dirty tree | `git tag` printed nothing; `go version -m ~/go/bin/wk` |
| Platform | macOS on APFS only (`clonefile(2)`, launchd, `lsof`) | `internal/clone/clone_other.go`; `README.md:9` |

**What an agent cannot verify here.** The interactive screens need a TTY: the guided setup and home screen of a bare `wk` (`cmd/wk/main.go:89-90`) and `wk hive`. The `wk viz` page needs a browser. A real launchd load, a real Claude Code hook call and a `df` disk measurement need the owner's machine and a live repository; the tests stub launchd and use throwaway repositories (`README.md:146`). For these, the session ends with the exact command and what should appear, and the owner runs it.

### HANDOFF.md: Code map

- `cmd/wk/main.go`: argument parsing and dispatch in `run()` (`:79-140`), the usage text, `WK_HOME` resolution (`:467`), the launchd and pool-fill wiring.
- `internal/wk/env.go`: `wk.Env`, opened by every command; the package's entry point.
- `internal/wk/new.go`: `wk new`, the checkout shape (sparse or full) and the clone of a workspace.
- `internal/wk/template.go`: the template worktree, its marker and its refresh.
- `internal/wk/pool.go`: the warm pool, its slot marker, claim and fill.
- `internal/wk/gc.go`: gc verdicts and `--apply` removal.
- `internal/wk/agent.go`: the subagent stop marker and its gc windows.
- `internal/wk/report.go`: `wk gc --json`, the `Report` type that the viz page reads.
- `internal/wk/hook.go`: the `WorktreeCreate`, `WorktreeRemove` and `SubagentStop` hook handlers, and `WK_HOOK_FAIL`.
- `internal/wk/hooks.go`: `wk hooks install|uninstall`, and the refusal of a `go run` binary.
- `internal/wk/schedule.go`: the launchd plist for `wk gc schedule`.
- `internal/config/config.go`: reads and checks `$WK_HOME/<repo>.toml`.
- `internal/gitx/gitx.go`: runs git and finds the repository and its main checkout.
- `internal/clone/clone_darwin.go`, `clone_other.go`: `clonefile(2)` copies, and the refusal on other systems.
- `internal/flock/flock.go`: shared and exclusive `flock(2)` locks.
- `internal/procs/procs.go`: an `lsof` snapshot of process working directories.
- `internal/settings/settings.go`: adds and removes wk's entries in a Claude Code settings file.
- `internal/setup/`: the guided setup a bare `wk` opens (`detect.go`, `model.go`, `render.go`, `view.go`).
- `internal/hive/`: the terminal view. `internal/viz/`: the web page and its server. `internal/theme/`: the colors they share.
- `internal/testrepo/testrepo.go`: throwaway test repositories and `Isolate()`.
- `docs/conventions-go.md`: the Go code standard. `docs/incomplete/landing-layer/`: the P1 to P4 design (`DESIGN.md`), plan (`PLAN.md`) and the slots and slim scopes.

### HANDOFF.md: Invariants

- `wk new` and `wk hook worktree-create` print nothing on stdout but the path. Claude Code takes any stdout line from the hook as the path (`cmd/wk/main.go:145-146`), and `cd "$(wk new 12)"` captures all of stdout (`README.md:30`).
- `wk hook subagent-stop` always exits 0 and prints nothing on stdout. A non-zero exit keeps the subagent running (`cmd/wk/main.go:133-137`).
- `wk hook worktree-create` falls back to a plain `git worktree add` when the warm path fails. It exits non-zero only when git cannot make a worktree either (`README.md:55`). `WK_HOOK_FAIL=1` forces the fallback (`internal/wk/hook.go:20`).
- The JSON field names of `wk.Report` are a contract with `internal/viz/index.html` (`internal/wk/report.go:9-10`).
- gc never forces. It runs `git worktree remove` without `--force` (`internal/wk/gc.go:354`). It deletes a branch only when the branch is an ancestor of the base (`internal/wk/gc.go:362`), and it judges each candidate again at removal (`README.md`, "Remove finished workspaces").
- Config paths are relative to the main checkout and stay inside it (`internal/config/config.go:221-222`). An unknown key is an error (`internal/config/config.go:137`). The gc windows and `new.max_parallel` are at least 1 (`internal/config/config.go:214`).
- wk takes over no directory that lacks its marker, and gc never removes a template (`internal/wk/template.go:15`; `README.md`, "The template").
- Hooks and the gc agent run an installed binary at an absolute path. A `go run` or `go test` build is refused (`internal/wk/hooks.go:25`; `internal/wk/schedule.go:35`).
- No byte-copy fallback: on a system without `clonefile(2)`, `clone.Dir` refuses (`internal/clone/clone_other.go`).
- Every test package that builds a git repository calls `testrepo.Isolate()` from `TestMain`: `cmd/wk/main_test.go`, `internal/setup/setup_test.go` and `internal/wk/wk_test.go` (`internal/testrepo/testrepo.go:13-14`). Each such test sets `WK_HOME` to a temp directory.

### HANDOFF.md: Known facts and quirks

- 2026-10-16: In the main checkout, `gofmt -l .` walks hidden directories, so it reads every worktree under `.claude/worktrees/`: 80 of the 120 `.go` files there. A hit may be another session's file. `./...` patterns skip dot directories (`go list ./...`: 13 packages, none under `.claude/`). `gofmt -l cmd internal` checks only this repo's files, because every tracked `.go` file is under those two (`git ls-files '*.go'`).
- 2026-10-16: `test -z "$(gofmt -l .)"` passes on a file that does not parse. gofmt prints the error on stderr and lists nothing (tested in a temp directory). `go vet` then fails, so the whole gate line catches it; the gofmt step alone does not.
- 2026-10-16: The golangci-lint cache is shared across checkouts (`~/Library/Caches/golangci-lint`, from `golangci-lint cache status`). A fresh-cache run agreed with the shared one (`0 issues.`). The ezhomesteading plan recorded a case where this cache lied (`docs/incomplete/landing-layer/PLAN.md` section 5).
- 2026-10-16: `internal/wk/gc_test.go:467` runs `plutil -lint` only when `plutil` is on `PATH`, and skips the check silently otherwise.
- 2026-10-16: `internal/wk` tests took 39.98 s, 40.88 s and 41.95 s under `-race` in three runs. An earlier run the same day at 20:39 EDT took about 23 s, so the time depends on load.
- 2026-10-16: The README's Development block (`README.md:142-143`) omits `GOTOOLCHAIN=local` and `-race`. It is not the gate.
- 2026-10-16: `README.md:5` is stale. It says the design and plan live in the ezhomesteading repository, but they are in `docs/incomplete/landing-layer/` here since commit `1c3fee1`.
- 2026-10-16: `/Users/zachshort/go/bin/wk` was built from `30989b5` with a dirty tree (`go version -m`). ezhomesteading's `.claude/settings.local.json` (lines 126, 137, 148) and the launchd agent `com.github.zach-short.wk.gc.ezhomesteading` both run it, so `go install ./cmd/wk` changes them at once.
- 2026-10-16: wk does not manage its own worktrees. There is no `~/.wk/wk.toml` and no `.claude/settings.local.json` in this repo; `~/.wk` holds only `ezhomesteading.toml` and the per-repo log directory `ezhomesteading/`.
- 2026-10-16: `git worktree list` shows `/Users/zachshort/Projects/landing-layer-phase3` (`p1-phase-3`, `30989b5`, already in `main`) as prunable, because its directory is gone.
- 2026-10-16: The prompt (`PART0-PROMPT.md`) says `gateCommand` takes no quotes. The hook reads it with `sed 's/..."\([^"]*\)".*/\1/'` and runs it with `eval` (`~/.claude/hooks/personal-config/completion-gate.sh:142,156`), so only double quotes break it.

### AGENT-PRACTICES.md: the WORKTREE_SETUP placeholder

```bash
GOTOOLCHAIN=local go mod download
```

Verified in a `git archive HEAD` copy with an empty `GOMODCACHE`: exit 0, and the full gate passed afterwards.

### AGENT-PRACTICES.md: the BUILD_CMD placeholder

```bash
GOTOOLCHAIN=local go build ./...
```

HEAD-isolation line, verified exit 0: `d=$(mktemp -d) && git archive HEAD | tar -x -C "$d" && cd "$d" && GOTOOLCHAIN=local go mod download && GOTOOLCHAIN=local go build ./...`
