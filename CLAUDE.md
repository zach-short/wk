<!-- personal-config v0.8.0 · 2026-10-16 · config d645852c · standard v2.1.0 · adapted -->
# wk

> **Code standard:** `docs/conventions-go.md` for go. It answers how code is written here; open it before
> writing or editing code, and look up the rule you need.

> **Process standard:** `docs/AGENT-PRACTICES.md`. It answers how a feature is scoped, planned and
> built; open the heading for the step you are on.

wk is a Go CLI that makes warm git worktrees for parallel agents and removes finished ones
(`README.md`). It is phase P1 of the landing layer; P2 to P4 are designed, not built
(`docs/incomplete/landing-layer/PLAN.md` section 4). Its one live user on 2026-10-16 is
ezhomesteading, whose Claude Code hooks and launchd gc agent run the installed binary.

(Superseded 2026-10-17: "P2 to P4 are designed, not built" is out of date. They are planned in
`docs/incomplete/landing-layer/PLAN-P2-P4.md` and `docs/incomplete/integration/PLAN.md`, and seven
of their libraries are on `main` with their tests, imported by no command: `internal/lease`,
`internal/board` and `internal/daemon` (P2), `internal/changeset` and `internal/landing` (P3),
`internal/gate` and `internal/batch` (P4). Board rows 3, 4 and 5 wire them.)

## The rules that get broken

Each rule comes from the code or the README, not from an incident log: the ledger here
started on 2026-10-16. Four have a test that fails when they are broken: the stdout rule
(`cmd/wk/main_test.go:19-28`, `cmd/wk/main_test.go:176-177`), the `subagent-stop` exit
(`cmd/wk/main_test.go:237`), the `WK_HOOK_FAIL` fallback (`internal/wk/hook_test.go:72-87`) and
the `go run` refusal (`cmd/wk/main_test.go:196-209`, `internal/wk/gc_test.go:489`). For the
other three, no test was checked on 2026-10-16, so read the cited code before you change it.

- **Print nothing on stdout but the path** in `wk new` and `wk hook worktree-create`. Claude Code
  reads any stdout line from the hook as the worktree path (`cmd/wk/main.go:145-146`), and
  `cd "$(wk new 12)"` captures all of stdout (`README.md:30`). Progress goes to stderr.
- **Keep `wk hook subagent-stop` at exit 0.** A non-zero exit keeps the subagent running
  (`cmd/wk/main.go:133-137`).
- **Keep `wk hook worktree-create` working when the warm path fails.** Claude Code has no
  fallback, so wk falls back to a plain `git worktree add` (`README.md:55`). Prove it with
  `WK_HOOK_FAIL=1` (`internal/wk/hook.go:18-20`).
- **Rename a field of `wk.Report` only together with the page that reads it**
  (`internal/wk/report.go:9-10`, read by `internal/viz/index.html`).
- **Never let `gc` force.** It runs `git worktree remove` without `--force` and deletes a branch
  only when the base already holds it (`internal/wk/gc.go:354`, `internal/wk/gc.go:362`).
- **Every test package that builds a git repository calls `testrepo.Isolate()` from
  `TestMain`** (`internal/testrepo/testrepo.go:13-14`), and sets `WK_HOME` to a temp directory
  (`cmd/wk/main_test.go:21`). No test may touch a real repository, a real settings file or
  launchd (`README.md:146`).
- **Install with `go install ./cmd/wk`, never `go run`.** Hooks and the gc agent run the binary
  at its absolute path, and wk refuses to install a `go run` build (`README.md:19`,
  `internal/wk/hooks.go:25`, `internal/wk/schedule.go:35`).

## Stack

- Go 1.24: `go.mod:3` says `go 1.24.0`; the installed toolchain is `go1.24.1 darwin/arm64` at
  `/usr/local/go/bin/go` (`go version`, 2026-10-16).
- `GOTOOLCHAIN=local` on every Go command. The default is `auto` (`go env GOTOOLCHAIN`), which
  downloads a newer toolchain when `go.mod` asks for one.
- Direct dependencies: `BurntSushi/toml` v1.6.0, `charmbracelet/bubbletea` v1.3.10,
  `charmbracelet/lipgloss` v1.1.0, `golang.org/x/sys` v0.41.0 (`go.mod:5-10`).
- Lint: `golangci-lint` v2.12.2 at `/Users/zachshort/go/bin/golangci-lint`, with no config file in
  the repo, so it runs the v2 default set: errcheck, govet, ineffassign, staticcheck, unused
  (`golangci-lint linters`, 2026-10-16).
- macOS on APFS only. `wk new` needs `clonefile(2)`; any other system gets a refusal
  (`internal/clone/clone_other.go`). `gc schedule` writes a launchd agent, and `wk gc` reads
  `lsof` (`README.md:9`, `internal/procs/procs.go:31`).
- No CI: there is no `.github/` directory (`ls .github`, 2026-10-16).
  (Superseded 2026-10-21: `.github/workflows/release.yml` runs on a `v*` tag only. It builds the
  darwin binaries, publishes the release and writes `Formula/wk.rb` in `zach-short/homebrew-tap`
  with the `TAP_DEPLOY_KEY` secret. It runs no gate, so still no CI checks a push.)

## Architecture

- One binary. `cmd/wk/main.go` parses the arguments and dispatches in `run()`
  (`cmd/wk/main.go:79-140`); every command opens a `wk.Env` (`internal/wk/env.go`) and works
  through it.
- There is no daemon, no socket and no lease. Those are P2 and later, and are not built
  (`docs/incomplete/landing-layer/PLAN.md` section 4).
  (Superseded 2026-10-17: the kill test of 2026-10-16, `docs/incomplete/integration/KILL-TEST.md`,
  decided files, so no phase builds a resident daemon or a socket. Leases go in
  `~/.wk/<repo>/leases.json` under an exclusive `flock` on `leases.lock`, and the hook reads that
  file itself. The landing loop is a detached `wk land --loop` worker. `internal/lease` is on
  `main`, imported by no command; `internal/daemon` keeps `SaveJSON`, `LoadJSON` and `Spawn` for
  that work, and its unused `Server` and `Client` are deleted after P3 lands.)
- State lives in three places: the config at `$WK_HOME/<repo>.toml` (default `~/.wk`,
  `internal/config/config.go:1`), logs under `$WK_HOME/<repo>/`, and marker files in each
  worktree's private git directory (`internal/wk/template.go:15`, `internal/wk/pool.go:21`,
  `internal/wk/agent.go:16`).
- Every path in the config is relative to the main checkout and must stay inside it; an
  unknown key is an error (`internal/config/config.go:221-222`, `internal/config/config.go:137`).
- Git is run as a subprocess through `internal/gitx`; there is no git library.

## Directory map

| Path | Belongs here | Does not |
|---|---|---|
| `cmd/wk/` | Argument parsing, dispatch, the usage text | Logic another command needs; that goes in `internal/wk/` |
| `internal/wk/` | Workspaces, template, pool, gc and its report, hooks, launchd schedule | Terminal or web drawing |
| `internal/config/` | Reading and checking `$WK_HOME/<repo>.toml` | Defaults scattered elsewhere |
| `internal/gitx/` | Running git and finding the repository | Decisions about worktrees |
| `internal/clone/` | `clonefile(2)` copies; a refusal on non-darwin | A byte-copy fallback |
| `internal/flock/` | `flock(2)` shared and exclusive locks | |
| `internal/procs/` | Which processes have a working directory inside a path (`lsof`) | |
| `internal/settings/` | Adding and removing wk's entries in a Claude Code settings file | Writes to `~/.claude/settings.json` |
| `internal/setup/` | The guided setup a bare `wk` opens | |
| `internal/hive/`, `internal/viz/`, `internal/theme/` | The terminal view, the web page, and the colors they share | Anything that removes a worktree |
| `internal/testrepo/` | Throwaway test repositories and git isolation | Production code |
| `internal/lease/` | P2: the lease store and `Judge`, which decides each edit against the leases | Disk or git; the caller saves the store |
| `internal/board/` | P2: one row's "Files it owns" cell, parsed into lease paths plus what it could not read | Reading the board file or running git |
| `internal/daemon/` | `SaveJSON`, `LoadJSON` and `Spawn`; a unix-socket `Server` and `Client` that nothing calls | A resident process; the kill test decided files |
| `internal/changeset/` | P3: a workspace's tree through a private index, its base, and the queue file | A commit in the workspace |
| `internal/landing/` | P3: predicting a merge, the compare-and-swap of the landing ref, `wk sync`, the fast-forward and push of `main` | A ref write without the old value |
| `internal/gate/` | P4: running the configured gates, the paths a failing gate prints, the wait for free memory | Batch or blame decisions |
| `internal/batch/` | P4: which changesets form a batch, how a red batch shrinks, which changeset a red gate blames | Git or any write |
| `docs/conventions-go.md` | The Go code standard | |
| `docs/incomplete/landing-layer/` | The design and plans for P1 to P4 | Closed efforts |
| `docs/incomplete/integration/` | The lifecycle plan, the kill test and the owner's decision record | |
| `passoff/` | One prompt file per board item, `<n>-<slug>.md` | |

(Rows `internal/lease/` to `internal/batch/` and `docs/incomplete/integration/` added 2026-10-17.
No command imports the seven packages yet; board rows 3, 4 and 5 wire them.)

## Commands

Fresh checkout or worktree, once on entry. Nothing gitignored is needed for the gates; this
fetches modules into the shared module cache, which `go build` would otherwise do on first use:

```bash
GOTOOLCHAIN=local go mod download
```

Build:

```bash
GOTOOLCHAIN=local go build ./...
```

The gates, one at a time:

```bash
test -z "$(gofmt -l .)"
```

```bash
GOTOOLCHAIN=local go vet ./...
```

```bash
GOTOOLCHAIN=local go test -race -count=1 ./...
```

```bash
GOTOOLCHAIN=local golangci-lint run ./...
```

Together, before handing work back:

```bash
GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'
```

Each block above was run green on 2026-10-16 in a worktree at `a4d4ed2`. The whole line took
45 to 50 s, of which `internal/wk` took 40 to 46 s under `-race`. It also ran green in a
`git archive HEAD` copy with an empty module cache, so it needs no `.git` directory.

Install, which replaces the binary that ezhomesteading's hooks and gc agent run (see "Never do
this"). Verified on 2026-10-16 only into a temporary `GOBIN`:

```bash
GOTOOLCHAIN=local go install ./cmd/wk
```

**Gates that lie.** Each was seen on 2026-10-16.

- **`gofmt -l .` in the main checkout also checks every nested worktree.** It walks hidden
  directories, so it reads `.claude/worktrees/*`: 80 of the 120 `.go` files there on
  2026-10-16. A hit may be another session's file. `go vet`, `go test` and `golangci-lint` take
  `./...`, which skips dot directories (`go list ./...` gave 13 packages, none under
  `.claude/`). Run the gates in your own worktree. From the main checkout, `gofmt -l cmd internal`
  checks only this repo's files: every tracked `.go` file is under those two directories
  (`git ls-files '*.go'`, 2026-10-16).
- **`test -z "$(gofmt -l .)"` passes on a file that does not parse.** gofmt prints the parse
  error on stderr and lists nothing, so the test sees an empty string (probe in a temp
  directory). `go vet` then fails, so run the whole line, never the gofmt step alone.
- **`go test` without `GOTOOLCHAIN=local` may run on a toolchain nobody installed.** The
  README's Development block (`README.md:142-143`) has neither `GOTOOLCHAIN=local` nor
  `-race`; it is not the gate.
- **The golangci-lint cache is shared across checkouts** (`golangci-lint cache status`:
  `~/Library/Caches/golangci-lint`). A run with `GOLANGCI_LINT_CACHE=$(mktemp -d)` also gave
  `0 issues`, so no stale result was seen here. Re-run with a fresh cache before you believe a
  red one.
- **The `plutil -lint` check in `internal/wk/gc_test.go:467` skips silently** when `plutil` is
  not on `PATH`. It is on `PATH` on macOS.
- **No CI runs these gates.** A push to `main` checks nothing.

**What an agent cannot verify here.** The interactive screens need a TTY: the guided setup and
home screen of a bare `wk` (`cmd/wk/main.go:89-90`) and `wk hive`. The `wk viz` page needs a
browser. A real launchd load, a real Claude Code hook call and a `df` disk measurement need the
owner's machine and a live repository; the tests stub launchd and use throwaway repositories.
For these, end with the exact command and what should appear, and let the owner run it.

**Merged is not shipped.** There are no tags (`git tag`, 2026-10-16) and no CI.
(Superseded 2026-10-21: to release, run the gates, then `git tag -a vX.Y.Z -m "wk vX.Y.Z"` and
`git push origin vX.Y.Z`; the release workflow updates the Homebrew tap.) A merge to
`main` changes nothing that runs until someone runs `go install ./cmd/wk`. On 2026-10-16 the
installed `~/go/bin/wk` was built from `30989b5` with a dirty tree (`go version -m
~/go/bin/wk`).
(Superseded 2026-10-21: `~/go/bin/wk` is deleted. The installed wk is the Homebrew release at
`/opt/homebrew/bin/wk`, and every project's hooks and gc agent run `/opt/homebrew/opt/wk/bin/wk`.
A tag ships; `brew upgrade wk` installs it. Do not `go install` it again: `~/go/bin` comes first
on `PATH` and would hide the release.)

## Where work is written down

- `HANDOFF.md` says what is true: environment, settled decisions, a one-line log. Search it for the topic at hand.
- `PASSOFF.md` says what is next, one row per item; each item's prompt is its own file in `passoff/`.

## Git

Claude may run `git add`, `git commit` and `git push`. Add no `Co-Authored-By:` trailer and no
"Generated with" line.

## Never do this

- Run `go install ./cmd/wk` from a branch that is not on `main`. ezhomesteading's hooks
  (`/Users/zachshort/Projects/ezhomesteading/.claude/settings.local.json`) and its launchd
  agent `com.github.zach-short.wk.gc.ezhomesteading` run `/Users/zachshort/go/bin/wk`, so an
  install changes them at once.
  (Superseded 2026-10-21: the hooks and agents of every wk project run
  `/opt/homebrew/opt/wk/bin/wk`. Do not run `go install ./cmd/wk` at all; tag a release.)
- Point a test or a manual `wk gc --apply` at a real repository to check a change. Use
  `internal/testrepo` fixtures.
- Add a byte-copy fallback for non-APFS systems. The refusal is the design
  (`internal/clone/clone_other.go`).
