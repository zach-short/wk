# Gate contract: pass-off prompts

Written 2026-10-18 from `SCOPE.md` in this folder. The scope session ran on Sonnet 5.5. Both builds below are assigned to other models by the routing rule, so neither was started. Each prompt stands alone. Both work in `~/Projects/personal-config`, in a worktree, and neither touches wk.

## Prompt 1. Model: Opus 5.5, with an independent Opus 5.5 review

Changed 2026-10-18 at the owner's request, from Fable 5.1 with a Fable 5.1 review. The first assignment sized the work by the fact that the hook fails open. The question that decides the tier is whether a new failure can be silent. Here the worst silent result equals today's behavior (a skipped gate), and a wrong config makes a gate fail loudly. The one real risk is breaking an existing guard in `config_dir`, and a fixture for each guard catches that. The owner may add one Fable 5.1 review of the diff alone.

Work in `~/Projects/personal-config`. Read `CLAUDE.md`, `docs/conventions-ts.md` and `docs/incomplete/gate-contract/SCOPE.md` from `~/Projects/wk`. Change `templates/hooks/completion-gate.sh`, function `config_dir`. Today it appends the subfolder (`prefix`) to the main checkout path, so a stop in `docs/bench` looks for `<main>/docs/bench/.personal-config.json`, finds nothing and exits 0. The gate log at `~/.config/personal-config/completion-gate.log` shows 83 such skips for `wk/docs/bench`.

Required behavior: start at the working folder and climb toward the main checkout root. Use the nearest `.personal-config.json`. Stop at the git top level and never go above it. Keep every existing guard in `config_dir`: the common-dir round trip, the `GIT_*` unsets, the deleted-files check, and the fail-open direction for anything it cannot decide. Keep the stop at `config=none` only when no config exists anywhere on the path.

Tests: add a violating fixture (a subfolder stop that today skips) and a passing one, in the style of the existing gate tests. Also add or confirm one fixture for each existing guard in `config_dir` (separate git dir, `GIT_COMMON_DIR`, deleted-files check, bare main, no config anywhere), so a regression in any guard fails a test. Use a temp `HOME`. Never touch the real log. Do not install the hook into `~/.claude` and do not edit claude-home. Report that both copies need the same change, and give the owner the exact install step.

Done means: the full gate line passes (`bun run typecheck && bun run lint && bun test && bun run doctor . examples`, run `doctor .` in the main checkout), and the review finds no path where a gate that should run now exits 0.

## Prompt 2. Model: Opus 5.5

Work in `~/Projects/personal-config`. Read `CLAUDE.md`, `docs/conventions-ts.md`, `src/doctor/rules/wk-config.ts` and `docs/incomplete/gate-contract/SCOPE.md` from `~/Projects/wk`.

Part C1. Add a doctor rule, one file under `src/doctor/rules/` with a violating and a passing fixture test, that warns when `gateCommand` in `.personal-config.json` and the `[[gates.run]]` entries in `~/.wk/<repo>.toml` disagree. The two are different shapes, a string and a list, so compare the command words of each gate in order and warn on a gate present in one and absent from the other. Do not try to be exact. Only run when `landStep` is `wk`, as `wk-config` does. Read the toml the way `wk-config.ts` does, without a new dependency. Register it in `src/doctor/index.ts`.

Part E. In `CLAUDE.md`, replace the line "The completion gate runs nothing in a worktree" with the true behavior: since row 122 the hook finds the main checkout's config through the git common directory, and the gate runs in the worktree. Leave a note that a stop in a subfolder still skips the gate until the Prompt 1 change lands.

Do not edit `~/.wk/*.toml`. Do not run `setup` against this checkout without `--dry-run`. Done means the gate line passes, `doctor` in the main checkout reports no new finding, and a changelog line exists if the repo's practice asks for one.

## Item 19, no build

Row B of the scope needs one number from wk's item 19: how often two or more Stop gates run at the same time. Add that count to item 19's list of measures. The Stop hook logs a timestamp and a result at the end, not at the start, so the overlap must be read from `runs.log` and the gate log together. Decide the method inside item 19.
