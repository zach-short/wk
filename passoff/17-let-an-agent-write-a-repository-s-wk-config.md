# 17. Let an agent write a repository's wk config without a terminal

> **Claimed `IN FLIGHT` 2026-10-17.**

*Opened 2026-10-17, for the rollout of wk to the owner's other repositories. On that day 27 repositories under `~/Projects` carry a `.personal-config.json`, and only `ezhomesteading` and `wk` have a `~/.wk/<repo>.toml`. The only way to write one is the guided setup a bare `wk` opens, which needs a terminal at both ends (`cmd/wk/main.go`, `interactive`, about line 631), so an agent cannot do it.*

**Model: Sonnet 5.5. Lane B. Waits on nothing. Work in your own worktree. Files it owns: `cmd/wk/main.go`, `cmd/wk/main_test.go`, `internal/setup/auto.go` (new), `internal/setup/auto_test.go` (new), `README.md`.**

## Orientation

You add one command to wk, a Go CLI over git for parallel agent workspaces: `wk setup --yes`, which writes `~/.wk/<repo>.toml` from what the guided setup would offer by default, with no prompts. It reuses the setup package's reader and writer; it adds no new judgment.

Read, in this order: `docs/conventions-go.md` (the Go code standard); `internal/setup/detect.go` (`Detect`, `Found`, `Option`, `Install`); `internal/setup/render.go` (`Choices`, `Render`, `Write`); `cmd/wk/main.go` from `run()` (about line 79) and `setupPlan` (about line 682) to see how the wizard is fed; the README section that describes setup (search for "opens setup").

Session rules, restated here because this prompt is read alone:

- Work in your own worktree branch, made from `main` at its tip. Commit on that branch only; the lead lands it. Claude may run any git command. Add no `Co-Authored-By:` trailer and no "Generated with" line.
- Run git as `git -C <path>` and use absolute paths. Do not `cd` at the top level of a shell call; use a subshell, `(cd <dir> && ...)`.
- Never write under `~/.wk` or `~/Library/LaunchAgents`. Tests set `WK_HOME` to a temp directory and call `testrepo.Isolate()` from `TestMain`, as `cmd/wk/main_test.go` already does.
- Written text (README, comments, close-out) uses absolute dates, no contractions and no em dashes.

## Why this exists

The owner wants wk's warm workspaces and cleanup in every repository they work in, and the agents that do the rollout have no terminal. With `wk setup --yes`, an agent can write a repository's config, read it back with `wk config show`, then run `wk hooks install`, `wk template refresh` and `wk gc schedule install` as it does today in ezhomesteading. Without it, 25 configs are written by hand, and each one drifts from what setup would write.

## What is fixed

Read these before changing anything; do not relitigate them.

- `Detect(primary)` reads names only and changes nothing. Each `Option` has `On`, the wizard's default: every clone dir and every untracked `.env` file on, a tracked `.env` off, `.claude/settings.local.json` on when present, and the first install found on (`detect.go` lines 77 to 131). `Found.installs` (unexported) holds the parts of each install.
- `Render(repo, Choices)` writes every dial, with `config.Defaults()` for all but the choices (`render.go` line 26). `Write(path, text)` writes by temp file and rename (line 83).
- `Found.Branches` lists local branches with the current one first. The current branch of a main checkout can be a feature branch.
- The guided setup never overwrites an existing config (the usage text: "Once the config exists it opens a home screen instead, and never overwrites the file"). `--yes` keeps that rule.
- Landing, gates and leases stay at their defaults (off, none, off). A rollout turns them on per repository later, by hand.
- `wk new` prints only its path on stdout (an invariant of `HANDOFF.md`). Follow the same split here: the written path on stdout, everything else on stderr.

## Do these, in order

1. **`wk setup --yes`.** In a new `internal/setup/auto.go`, add `Defaults(f Found) Choices`: each option's `On` value, every install whose option is on (in `Found` order), `Schedule` and `Warm` false, and `Base` chosen as `main` if that branch exists, else `master`, else the current branch, because the base of a workspace must be the integration branch, not whatever is checked out. In `cmd/wk/main.go`, wire `wk [--repo <dir>] setup --yes [--print]`: it refuses, exit 1, when `$WK_HOME/<repo>.toml` exists, naming the file; otherwise it writes `Render(repo, Defaults(Detect(primary)))` with `Write`, prints the path on stdout, and prints one stderr line per choice it made (the base, each clone dir, each copy file, each install). `--print` writes nothing and prints the TOML on stdout. `setup` without `--yes` or `--print` prints the usage error, because the guided setup is a bare `wk`. Add the line to the usage text and a sentence to the README.
2. Tests: `TestDefaultsTakeEachOptionsOn` and `TestDefaultsPickMainOverTheCurrentBranch` in `internal/setup/auto_test.go` (a fixture repo with `main` and a checked-out `feature` branch: the base is `main`); `TestSetupYesWritesAConfigThatLoads` in `cmd/wk/main_test.go` (a repo with `package.json`, `bun.lock` and an untracked `.env`: the file is written, `config.Load` accepts it, and it holds `refresh_on = "bun.lock"`, `node_modules` and `.env`); `TestSetupYesRefusesAnExistingConfig` (the file is unchanged byte for byte); `TestSetupPrintWritesNothing`. Each fails with its fix turned off; say how in the close-out.
3. Run the gate below. Write a `## Close-out, <date>` section at the end of this file: what changed, each test with its fix-off result, the gate output with its time and load.

## Ask before building

Nothing is the owner's call. The base rule in step 1 is a default; its reversal is the current branch. Take it unless a fact you find contradicts it, and say so in the close-out.

## Not in scope, whoever asks

- Running `wk hooks install`, `wk gc schedule install` or a template warm from `setup --yes`. Each is its own standing change, and the rollout runs them as separate commands.
- Writing `[landing]`, `[gates]` or `[leases]` values other than the defaults.
- Any change to the guided setup's screens (`model.go`, `view.go`), to `Detect`'s rules, or to `Render`'s output for the wizard.
- Running the command against any real repository.

## Hand back

Gate, run in your worktree:

```bash
GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'
```

If it fails only on `TestHookPreToolUseTiming` in `internal/slot` under a load average above about 35, run it again when the load is lower, and report every run with its load.

Done when the five tests pass, each fails with its fix off, and the gate exits 0 in your worktree. Hand back the gate output, the diffstat and the close-out. Commit on your worktree branch only, with no `Co-Authored-By:` trailer and no "Generated with" line.
