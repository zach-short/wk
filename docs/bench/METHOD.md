# Benchmark method for row 19

Written 2026-11-21 for board row 19 (`passoff/19-measure-what-wk-saves-and-chart-it-for-the.md`). This file is step 2 of the row. No timed run has happened yet. The owner reads this file, answers the questions at the end, and says when the machine is quiet; only then do steps 3 onward run.

Every number in the later write-up will come from a CSV in `docs/bench/data/` that `docs/bench/run.sh` writes. The figures in this file are facts about the setup and one probe, not results.

## What changed since the row was written

The row was written on 2026-10-18. Several of its fixed facts are out of date. Each value below was checked on 2026-11-21 with the command in the last column.

| Fact | Row says | True on 2026-11-21 | How it was checked |
|---|---|---|---|
| Machine | Apple M4, 10 cores, 16 GiB | Apple M5 Pro, 15 cores (15 physical), 64 GiB (68,719,476,736 bytes) | `sysctl hw.memsize hw.ncpu hw.physicalcpu machdep.cpu.brand_string` |
| Volume | APFS, 460 GiB, about 52 GiB free | APFS, 926 GiB, 664 GiB free, swap 0 | `df -h /`; `diskutil info /`; `sysctl vm.swapusage` |
| Installed wk | `~/go/bin/wk` | `/opt/homebrew/opt/wk/bin/wk`, `wk 0.1.11` (Homebrew, linked 2026-11-21 19:17), built with go1.24.0 from `vcs.revision=e4ed9098b736e88163f77f894b99dde17822ad9f`, `vcs.modified=false`. `~/go/bin/wk` does not exist | `wk --version`; `go version -m /opt/homebrew/opt/wk/bin/wk`; `ls -l ~/go/bin/wk` |
| wk source | not stated | tag `v0.1.11` is `e4ed909`; `main` is `7c999d0`, which adds only the claim line of row 19 | `git rev-parse v0.1.11^{commit}`; `git diff --stat e4ed909 7c999d0` |
| Toolchain | not stated | git 2.56.0, bun 1.4.4, go1.26.11 (local), Python 3.15.2 | `--version` of each |
| Slot caps | heavy 1, check 2 | heavy 2, check 4, `order = "fifo"`, `max_wait_seconds = 0`, `min_free_mb = 0` | `~/.wk/machine.toml` |
| Clones into a workspace | `node_modules`, `native/node_modules`, `web/.next/cache`, `web/.next/types` | eight `clone_dirs` entries; only `node_modules` and `native/node_modules` exist after `bun install`, so six are skipped. No `web/.next` entry at all | `~/.wk/ezhomesteading.toml`; `ls -d` in the live template, `wk-245` and a pool slot (read only); the probe below |
| Copied files | four | six: `web/next-env.d.ts`, `native/expo-env.d.ts`, `.claude/settings.local.json`, `web/.env`, `backend/.env`, `native/.env` | `~/.wk/ezhomesteading.toml` |
| Post-clone | `bun install --frozen-lockfile` and `bunx next typegen` | `bun install --frozen-lockfile` only, skipped at the template's commit | `~/.wk/ezhomesteading.toml` |
| Sparse rule | images and JSON under `docs/incomplete/` | `sparse = true`, `sparse_exclude = ["docs/incomplete/**/*.png"]` | `~/.wk/ezhomesteading.toml` |
| Pool | 4 sparse | `pool.sparse_size = 4`, `pool.size = 0` | `wk config show` |
| `[new]` thresholds | written in `~/.wk/ezhomesteading.toml` | not in the file; the defaults apply: `max_parallel = 2`, `max_seconds_cold = 10.0`, `max_mb_per_workspace = 120`, `burst_size = 10`, `burst_max_seconds = 60.0` | `WK_HOME=<scratch> wk --repo <clone> config show` |
| Slots file | five heavy spellings | seven heavy (`next build`, `bun run build`, `bunx next build`, `bun run e2e`, `playwright test`, `bunx playwright test`, `go run ./cmd/lintgate`), nine check (`tsc`, `bunx tsc`, `eslint`, `bunx eslint`, `bun run lint`, `vitest`, `bunx vitest`, `bun run test`, `go test`); `trim_after = ["web/.next/cache"]` on the three build spellings | `~/.wk/ezhomesteading.slots.toml` |
| Tracked tree | 5,740 files | 6,931 files, 612 MB, of which 447 MB are PNGs under `docs/incomplete/` | `git ls-files` with `stat -f %z` summed |
| `.git` | 1.0 GB | 967 MB by `du`; 4 packs, 125,269 objects, 912.72 MiB packed; 3,278 loose objects, 17 MiB | `du -sh .git`; `git count-objects -vH` |
| Live worktrees | not stated | 39 entries, the template and 4 pool slots among them | `git worktree list` |
| Manual setup (baseline) | `bun install`, three copies, `bunx next typegen` | `docs/gates.md`, "Fresh worktree setup", names three files (`web/next-env.d.ts`, `native/expo-env.d.ts`, `web/.env`) and no `next typegen`. `CLAUDE.local.md`, "Fresh checkout", runs `bun install --frozen-lockfile` and copies five files (the three `.env` files and the two `.d.ts` files) from `/Users/zachshort/Projects/ezhomesteading`, a path that no longer exists | `grep` in both files |
| Portfolio frames | 780 by 1688 PNGs, `01.png` to `05.png` | the wk story uses `display: 'terminal'` frames with a text `body`; only `src/assets/projects/wk/01.png` (the cover) is on disk | `projects.ts` lines 215 to 275; `ls src/assets/projects/wk/` |

Context from the live logs, not benchmark data (`wk stats`, 2026-11-06 to 2026-11-21, read by the lead): 231 warm creates in ezhomesteading, median 2.94 s, 90th percentile 5.76 s, max 12.7 s. The checkout is the longest step, median 2.49 s. A sparse pool fill on 2026-11-21 took 1.5 s per checkout.

At 19:19 on 2026-11-21 the load averages were 5.59, 4.22 and 3.46, with 24 `claude` processes and 91 percent memory free. The machine was not quiet.

## The safe copy (step 1, done)

| Path | What it is |
|---|---|
| `<scratch>/bench/ezh-bench` | `git clone --no-hardlinks` of ezhomesteading at `80ce82149`, on `main`, with the `origin` remote removed so no push can reach the live checkout. The clone took 3.66 s and about 1.5 GiB of `df`. |
| `<scratch>/bench/ezh-bench/{web,backend,native}/.env`, `web/next-env.d.ts`, `native/expo-env.d.ts`, `.claude/settings.local.json` | copied with `cp` from the live checkout, never read or printed |
| `<scratch>/bench/wkhome/ezh-bench.toml` | `~/.wk/ezhomesteading.toml` byte for byte except the first comment line |
| `<scratch>/bench/wkhome/machine.toml` | the live caps, with `slots.dir` set to `<scratch>/bench/wkhome/slots` |
| `<scratch>/bench/wkhome/ezh-bench.slots.toml` | `~/.wk/ezhomesteading.slots.toml`, identical |

`<scratch>` is `/private/tmp/claude-503/-Users-zachshort-Developer-dan-short-ezhomesteading/1c53f89c-6cfb-4a6d-9e3b-fd3208354035/scratchpad`. `WK_HOME=<scratch>/bench/wkhome wk --repo <scratch>/bench/ezh-bench config show` printed the path with `(found)`, and `wk slots` under the same `WK_HOME` listed 4 free check places and 2 free heavy places in the scratch directory.

The copied `.claude/settings.local.json` holds the live hook entries. They run only when a Claude Code session opens inside the clone, and no benchmark step does that.

## The probe

One probe ran to settle how scenario A defines "the first gate". In the clone, at load averages near 7.9, a plain `git worktree add` (1.81 s), `bun install --frozen-lockfile` with the bun cache warm (2.26 s; it fetched 1 package) and the five copies made a worktree. `bunx tsc --noEmit` in `web` then exited 0 in 8.75 s with no `web/.next/types` directory, although `web/next-env.d.ts` imports `./.next/types/routes.d.ts`. `bunx next typegen` took 1.29 s, and a second `tsc` exited 0 in 10.74 s. Only `node_modules` and `native/node_modules` existed after the install. The probe worktree and its branch were removed from the clone with `git worktree remove --force` and `git branch -D`.

So `next typegen` is not needed for the first web gate today, and the baseline below leaves it out. These timings are one run on a loaded machine and are not data.

## Rules for every run

These hold in every scenario.

- Every timed command runs from `docs/bench/run.sh`, with `WK_HOME=<scratch>/bench/wkhome` exported and every wk call as `/opt/homebrew/opt/wk/bin/wk --repo <scratch>/bench/ezh-bench`. The script is started as `sh docs/bench/run.sh --scenario <X>`. Its gate commands are inside the script, so the live `PreToolUse` hook sees no gate spelling and wraps nothing; see the first owner question.
- Nothing reads or writes the live ezhomesteading checkout, `~/.wk`, the live pool, template, slots or launchd agent. No `wk gc schedule install` and no `wk hooks install` under the scratch `WK_HOME`.
- Times come from `/usr/bin/time -p` (real seconds) around each step, and a wall clock from `python3 -c 'import time; print(time.time())'` for bursts and timelines.
- Disk is `df -k /` free blocks, never `du`.
- At the start of each run the script records `sysctl -n vm.loadavg`, `memory_pressure` free percent, `wk --version`, the installed commit and the clone's `HEAD`. A run that starts with a 1-minute load average above 4 waits up to 5 minutes for it to fall, then runs anyway and is flagged in the CSV.
- Arms run interleaved (baseline, wk, baseline, wk), so a slow drift in the machine spreads over both arms.
- After a run, its workspaces are removed inside the clone with `git worktree remove --force` and `git branch -D`, the bench config is restored, and the script waits until no `wk pool fill` process of the scratch `WK_HOME` is running (`pgrep -f 'ezh-bench pool fill'`) and two `df -k` readings 10 s apart differ by less than 50 MB.
- Bench rows are numbered from 9001, so a bench branch cannot be mistaken for a board row.
- `brew upgrade wk` must not run during the benchmark. Homebrew relinked wk at 19:17 on 2026-11-21, so the script refuses to start a scenario when `wk --version` differs from the first run's.

What "cold" and "warm" mean here:

- The bun install cache is always warm, as it is on this machine every day. A cold bun cache is not measured, because no real worktree starts from one.
- The OS file cache cannot be dropped without `sudo purge`, which this row does not run. The first run of each arm is recorded and marked; the claim is made on runs 2 onward and run 1 is shown on the chart.
- For wk there are three temperatures. Cold template: the template is missing, so `wk new` builds it first. Warm template: the template is at the base commit and the pool is empty. Pool: a ready sparse slot is claimed.

## Scenario A, time to a workspace that passes its first gate

Question: how long from "I want a workspace" to a green first gate.

Arms:

1. Baseline, the documented manual setup (`docs/gates.md` plus `CLAUDE.local.md`), run in the clone:
   ```sh
   W="$B/.claude/worktrees/bench-$n"
   git -C "$B" worktree add -q -b bench/$n "$W" main
   (cd "$W" && bun install --frozen-lockfile)
   for f in web/.env backend/.env native/.env web/next-env.d.ts native/expo-env.d.ts; do cp "$B/$f" "$W/$f"; done
   ```
2. wk, warm template, empty pool (a bench config with `pool.sparse_size = 0`):
   ```sh
   W=$(wk --repo "$B" new $n)
   ```
3. wk, pool: the same command with `pool.sparse_size = 4` and the pool full (`wk pool status` shows 4 ready) before the run starts. The script then waits for the background fill to finish before the next run.
4. wk, cold template: the template is removed in the clone (`git worktree remove --force`), then `wk new $n` runs with `pool.sparse_size = 0`. This is a separate point, 3 runs.

Then, in each workspace, timed one after the other:

```sh
(cd "$W/web" && bunx tsc --noEmit)
(cd "$W/web" && bun run build)
```

Both gates run plain, not through `wk run`, in every arm, because the question is the setup time and not the slot.

Held constant: the clone's commit, the bun cache, the five copied files, the gate commands, plain gates. The baseline gets a full checkout because `git worktree add` gives one; wk gets the sparse checkout its config sets. Scenario B separates the two.

Runs: 5 per arm for arms 1 to 3, 3 for arm 4. Recorded per run: ready seconds, `tsc` seconds and exit, build seconds and exit.

The `.next` claim. The row expected the first build in a wk workspace to be faster because wk cloned `web/.next/cache`. The config no longer clones any part of `web/.next`, and the template has no build step, so the first build is cold in both arms. The method measures it anyway and the write-up will say that wk gives no build gain as configured on 2026-11-21. Whether to add a bench-only arm that shows what a cloned cache would buy is an owner question.

Estimated wall time: about 50 s per run (5 s setup, 10 s `tsc`, 35 s build), 18 runs, plus cleanup: about 20 minutes.

Thresholds reported: `max_seconds_cold = 10` against arm 4 and arm 2.

## Scenario B, disk per workspace

Question: how much real disk one more workspace costs.

Arms: the baseline of A; `wk new` (sparse, the config's default); `wk new --full`. The `--full` arm is new: it separates the saving of the sparse checkout from the saving of the clones, because the baseline is a full checkout and the sparse rule alone leaves out 447 MB.

Per set: two `df -k /` readings 60 s apart (the drift), then 10 workspaces made one after another (not at once), then one `df -k /` reading after the last is ready and 10 s of quiet. MB per workspace is the free-block change divided by 10. A set whose drift over 60 s is larger than its own change is run again, up to two more times, and each attempt is kept in the CSV with a flag.

Commands: as in A, with the pool empty for both wk arms, so every wk workspace comes from the template. No gate runs in B, so `web/.next` does not exist.

Held constant: the commit, the bun cache, the pool empty, no gates.

Runs: 3 sets per arm. Reported: MB per workspace with its spread, against `max_mb_per_workspace = 120`.

What the data should show if the earlier probes hold: bun installs with `clonefile` from `~/.bun/install/cache`, so the baseline's `node_modules` shares most blocks too (`SLIM-SCOPE.md`). The difference should come mostly from the checkout: about 612 MB written by a full checkout against about 165 MB by the sparse one. The row's other disk claim, the cloned `web/.next/cache`, does not apply to today's config.

Estimated wall time: about 3 minutes per set (1 minute of drift, 1 minute to make, 1 minute to clean up), 9 sets: about 30 minutes.

## Scenario C, a burst of 10

Question: how long until the last of 10 workspaces started together is ready.

Arms:

1. Baseline: 10 background jobs, each the baseline of A, started in the same second.
2. wk, empty pool, `new.max_parallel = 2` (today's default): 10 background `wk new` calls started in the same second.
3. wk, pool full (4 ready sparse slots), `max_parallel = 2`: 10 background `wk new` calls. Four should claim slots and six come from the template, while the background fill competes with them. That competition is part of what a real burst does, so it is measured, not removed.
4. wk, empty pool, `new.max_parallel = 10` (a bench config value only). This shows the effect of the cap that the row asks about. Whether to run it is an owner question.

Each job writes its own start and end epoch to the CSV. Reported: wall time to the last ready, each job's own time, failures (exit status and the error line). Concurrent `git worktree add` calls on one repository can fail on a lock; any such failure is data.

Held constant: as in A; no gates.

Runs: 5 per arm. Threshold: `burst_max_seconds = 60` for `burst_size = 10`.

Estimated wall time: under 1 minute per burst plus about 1 minute of cleanup and pool refill, 20 bursts: about 40 minutes, or about 30 without arm 4.

## Scenario D, the slot cap

Question: does the slot cap protect this machine when several web builds start together, and what does it cost in wall time.

The row sized this test for 16 GiB, where 6 builds near 2 GiB each would press on memory. This machine has 64 GiB, so 6 builds will probably use under a quarter of it and show no memory pressure. The honest test runs at today's caps and reports what happens, even if the plain run is fine. It must not grow the count until the machine suffers, only to make a chart.

Proposal: 6 workspaces made by `wk new` (pool empty), each with `rm -rf web/.next` before each run so every build is cold. Then:

1. Plain: 6 background `(cd "$W/web" && bun run build)` started in the same second.
2. Slotted, the hook's own rewrite at today's capacity (`heavy = 2`):
   ```sh
   (cd "$W/web" && wk run --slot heavy --trim web/.next/cache -- bun run build)
   ```
3. Optional, an owner question: 10 builds each way, the most I propose on this machine. At about 2 GiB per build that is about 20 GiB, which leaves more than 30 GiB free with the usual load.

A sampler runs every second for the whole run: `vm_stat` (free, inactive and speculative pages times the page size), `sysctl vm.swapusage`, `memory_pressure` free percent, and the 1-minute load average. Recorded per build: start, end, exit status. Recorded per run: wall time, peak memory used, lowest free, peak swap, failed builds.

Stop rule: the script kills every build of a run (by process group) when swap passes 4 GiB or the free percent from `memory_pressure` falls under 15, and writes `stopped` in the CSV. With 64 GiB I do not expect either to happen at 6 or 10.

Held constant: the same 6 workspaces for both arms, a cold `web/.next` for every build, `NEXT_TELEMETRY_DISABLED=1`, plain `bun run build` with no other flags.

Runs: 3 per arm (each is over a minute). Expected: the plain run uses all 15 cores and the slotted one runs in three waves of 2, so the slotted wall time may well be longer. If so, the write-up says that on this machine the cap costs time and buys nothing measurable at 6 builds.

Estimated wall time: setup 1 minute; about 2 minutes per run, 6 runs: about 15 minutes. With the 10-build arm: about 30 minutes.

## Scenario E, cleanup

Question: does `wk gc --apply` remove only what is finished.

Setup, in the clone, under the scratch `WK_HOME` only, all workspaces from `wk new` with the pool empty:

| Group | Count | What is done to it | Expected verdict |
|---|---|---|---|
| Merged | 4 | one commit of a new file, then `git -C "$B" merge --no-edit wk/<n>` on `main` | removable; branch deleted |
| Dirty, tracked | 2 | an edit to a tracked file, not committed | kept |
| Dirty, untracked | 2 | a new untracked file at a path `main` does not have | kept |
| Clean, unmerged | 4 | one commit that `main` does not hold | kept |
| Held | 2 | after the workspace is made, a file is committed on `main`; the same bytes are written to the same path in the workspace as an untracked file | removable; the held file is deleted with it (rows 31 and 32) |
| Held, differing | 2 | as "Held", with different bytes in the workspace | kept |

The two held groups are new. They test the rule that wk 0.1.11 added on 2026-11-21: an untracked file byte-equal to the base's blob no longer keeps a worktree. The row's 12 workspaces become 16.

The quiet window. gc keeps any worktree changed in the last `gc.quiet_minutes`, 60 in the live config. I propose `gc.quiet_minutes = 1` in the bench config for this scenario and a wait of 90 s after the last write. The other choice is to keep 60 and wait 61 minutes per run. This is an owner question.

Then:

```sh
/usr/bin/time -p wk --repo "$B" gc
/usr/bin/time -p wk --repo "$B" gc --json
/usr/bin/time -p wk --repo "$B" gc --apply
wk --repo "$B" gc
```

Recorded: each worktree's verdict and reasons from `--json`, the expected verdict, a match or a mismatch, which directories and branches are gone after `--apply`, and the time of each call. After `--apply` the script checks that every dirty file and every unmerged commit still exists. Any wrong verdict is a finding for `ANALYSIS.md`, and no code changes in this row.

Runs: 3 repetitions of the whole setup.

Estimated wall time: about 1 minute of setup, 90 s of quiet, under 30 s of gc, 1 minute of cleanup: about 4 minutes each, 12 minutes in all.

## Scenario F, how often Stop gates overlap

Question: do two or more Stop-hook gates, which run outside `wk run`, run at the same time on this machine.

This only reads. The script copies `~/.config/personal-config/completion-gate.log` (657 lines on 2026-11-21) and `~/.wk/slots/runs.log` (1,364 lines) into `<scratch>/bench/logs/` and reads the copies. The gate log has one line per stop, stamped at the end, with `ran=<folder>` since 2026-10-18, and no start time or length.

Proposed count: pairs of stops from different folders whose end stamps are within 60 s of each other, per day, as an upper bound on overlap. The limit is stated with the number: a gate that ran 5 minutes and one that ran 10 s can end 60 s apart without overlapping, and two long gates can overlap with ends more than 60 s apart. If the count is low, the write-up says so and option B of `gate-contract/SCOPE.md` closes.

Runs: 1. Estimated wall time: under a minute. It does not need a quiet machine.

## Data and reproduction

Every run appends rows to `docs/bench/data/<scenario>.csv` with these columns: `scenario`, `way`, `run`, `step`, `seconds`, `df_free_kb_before`, `df_free_kb_after`, `exit`, `started_at` (ISO 8601 with the offset), `load1`, `load5`, `mem_free_pct`, `wk_version`, `wk_commit`, `ezh_commit`, `flag`. Scenario D's sampler writes `docs/bench/data/D-samples.csv` (`way`, `run`, `t`, `free_mb`, `swap_mb`, `mem_free_pct`, `load1`). Scenario E writes `docs/bench/data/E-verdicts.csv`.

`sh docs/bench/run.sh --scenario <X>` makes a fresh clone and `WK_HOME` under a scratch directory given by `BENCH_DIR`, runs that scenario, and appends to the CSVs. `python3 docs/bench/chart.py` (standard library only) draws every SVG from the CSVs. No number is typed by hand.

Total estimated wall time on a quiet machine: A 20, B 30, C 30 to 40, D 15 to 30, E 12, F 1: about 1 h 50 min to 2 h 15 min, not counting waits for the load to fall.

## What can make the test unfair

- The machine is shared. At 19:19 on 2026-11-21 the load averages were 5.59, 4.22 and 3.46 with 24 `claude` processes. Other agents' builds would add noise to A, C and D far larger than wk's effect, so steps 3 to 7 need the owner to say the machine is quiet.
- The live `PreToolUse` hook rewrites any gate spelling in a `Bash` command of a session in ezhomesteading, against the live slots. A gate typed on the command line would take a live place, write to the live `runs.log`, and make the "plain" arms slotted. Starting `run.sh` as a script keeps the gates out of the hook's sight. That is the only way to get a plain arm, and it is what the owner question below asks about.
- A claim from the pool starts a background `wk pool fill`, which inherits `WK_HOME`. The script waits for it to end before the next run, except inside a burst in C, where it is part of the measured behavior.
- The baseline is a full checkout and wk is sparse. A compares the two as each is used; B adds `wk new --full` so the sparse effect can be read on its own.
- The OS file cache is warm after run 1. Run 1 is marked and shown.
- `df` sees every writer on the volume. The drift readings in B bound that, and a set with large drift runs again.

## Questions for the owner

1. Is the method fair? Approve as written / change a named scenario / cut a scenario.
2. May `run.sh` run the gate commands inside a script, where the live hook cannot see them? This is needed for a plain baseline and to keep runs out of the live slots log. Yes / no (then no plain arm can be measured from this session).
3. When is the machine quiet for steps 3 to 7? Name a window / "now". Either way each run records its load average, and a run that starts above load 4 waits up to 5 minutes.
4. Scenario A baseline: which manual setup? `CLAUDE.local.md`'s five copies and `bun install`, no `next typegen` (proposed, the probe shows `tsc` green without it) / the same plus `bunx next typegen` (about 1.3 s more) / `docs/gates.md`'s three copies only.
5. Scenario A, the build cache: measure wk only as configured, or also a bench-only arm? As configured only (proposed; the write-up says there is no build gain today) / add an arm with `web/.next/cache` in `clone_dirs` and a `template.build` step, to show what the clone would buy.
6. Scenario B: add the `wk new --full` arm? Yes (proposed) / no.
7. Scenario C: run the `max_parallel = 10` arm? Yes (proposed, the row asks for the cap's effect) / no.
8. Scenario D: how many builds? 6 only (proposed) / 6 and 10 / do not run the slot test on this machine. The stop rule is swap over 4 GiB or memory free under 15 percent, and I will not go past 10.
9. Scenario D: which heavy capacity? Today's 2 (proposed) / the row's 1 / both (adds about 6 minutes).
10. Scenario E: the quiet window. `gc.quiet_minutes = 1` in the bench config and a 90 s wait (proposed) / the live 60 and a 61-minute wait per repetition.
11. Scenario E: add the held-file groups from rows 31 and 32? Yes (proposed) / no, the row's 12 only.
12. Scenario F: is the 60 s end-time count enough? Yes / no, first make the Stop hook log its own length (a separate personal-config row, never made here).
13. The README section: show the slot test if the plain run never pressed on memory? Show it with the plain finding / leave it out / decide after the data.
14. The portfolio draft. The wk story now uses terminal frames with text bodies, not PNGs. Draft up to two terminal frames from real bench output (proposed) / draft PNG frames from the SVGs as the row says / no portfolio draft. The wording stays the owner's either way.
15. The time budget. All scenarios, about 2 hours (proposed) / A, B, E and F only, about 1 hour.
