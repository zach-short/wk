# 19. Measure what wk saves, and chart it for the README and the portfolio

*Opened 2026-10-18, at the owner's request: "conduct tests in ezhomesteading or under whatever circumstances to illustrate its effectiveness and provide an analysis with visuals" for the wk README and the wk story on the portfolio site. wk has been in daily use in 12 repositories since 2026-10-17, and no measurement of the shipped tool exists; the numbers on record are design-time probes from 2026-10-16.*

**Model: Opus 5.5. Lane C. Waits on nothing, but every run in step 3 onward waits on the owner saying the machine is quiet. Work in your own worktree of `~/Projects/wk` (`cd "$(wk new 19)"` from the main checkout); the benchmark itself runs against a separate clone, never the live ezhomesteading checkout. Files it owns: `docs/bench/` (new), `README.md`.**

The tier is Default, not Deep: a misleading benchmark can pass every gate, so the check is that every number in the write-up comes from a raw CSV in `docs/bench/data/` that `docs/bench/run.sh` regenerates, and the owner reads the method before any chart is drawn (step 2).

## Orientation

You are measuring wk, a Go CLI that makes warm git worktrees for parallel Claude Code agents and removes finished ones. Source and README: `~/Projects/wk`. The repository you measure against is ezhomesteading (`~/Projects/ezhomesteading`), a bun monorepo (Next.js web app, Expo native app, Go backend) with 5,740 tracked files and a 1.0 GB `.git`, the repository wk was built for.

Read, in this order: `~/Projects/wk/README.md` sections "Make a workspace", "The pool", "The template", "Cap heavy gates", "Remove finished workspaces", "Disk use"; `~/Projects/wk/docs/incomplete/landing-layer/DESIGN.md` lines 15 to 60 (the 2026-10-16 baseline probes); `~/Projects/wk/docs/incomplete/landing-layer/SLIM-SCOPE.md` lines 1 to 40 (why `du` lies about clones); `~/.wk/ezhomesteading.toml` and `~/.wk/machine.toml`; `~/Projects/ezhomesteading/docs/gates.md` section "Fresh worktree setup"; `~/Projects/portfolio/src/lib/projects.ts` lines 220 to 275 (the wk story).

Session rules, restated here because this prompt is pasted alone:

- `GOTOOLCHAIN=local` on every Go command.
- Never run `go install ./cmd/wk`. Every project's hooks and gc agents run `/Users/zachshort/go/bin/wk`, so an install changes all of them at once. Measure the installed binary as it is; record `go version -m ~/go/bin/wk` and `git -C ~/Projects/wk rev-parse HEAD` in the data.
- Never run `wk gc --apply` against a real repository, and never remove a worktree with `--force` outside the benchmark clone.
- No test or run touches the live ezhomesteading checkout, its `~/.wk/ezhomesteading*` files or its launchd agent. Everything runs under a throwaway `WK_HOME` against a clone (step 1).
- Real disk cost is `df`, never `du`: `du` counts a cloned directory at full size because it cannot see shared APFS blocks.
- Written work follows `~/.claude/rules/writing-style.md`: no em dashes, no contractions, none of its listed AI-signal words, sentence-case headings, no closing paragraph that restates the text.
- Commit with no `Co-Authored-By:` trailer and no "Generated with" line.

## Why this exists

The owner wants to show, with evidence, what wk buys an agent workflow: a worktree that is ready for the gates in seconds instead of minutes, almost no extra disk per worktree, heavy gates that take turns instead of exhausting a 16 GB machine, and finished worktrees that leave without a person. The audience is a reader of the README deciding whether to use wk, and a visitor to the portfolio site. Both should be able to trust every number, so an honest small win beats an inflated large one, and a place where wk does not help (or costs something) is shown, not hidden.

## What is fixed

Read these before changing anything; do not relitigate them.

- **The machine:** Apple M4, 10 cores, 16 GiB (`sysctl hw.memsize hw.ncpu`), APFS, 460 GiB volume with about 52 GiB available on 2026-10-18 (`df -h /`). Results hold for this machine only; say so on every chart.
- **The slot caps** (`~/.wk/machine.toml`): `heavy` capacity 1, `check` capacity 2, `order = "fifo"`, `max_wait_seconds = 0` (wait without a limit), `min_free_mb = 0`. ezhomesteading's slots file routes `next build`, `bun run build`, `bunx next build`, `bun run e2e` and Playwright to `heavy`.
- **What a wk workspace gets in ezhomesteading** (`~/.wk/ezhomesteading.toml`): clones of `node_modules`, `native/node_modules`, `web/.next/cache` and `web/.next/types` from the template at `.claude/worktrees/wk-template`; copies of `web/next-env.d.ts`, `native/expo-env.d.ts`, `web/.env` and `.claude/settings.local.json`; a pattern-sparse checkout that drops images and JSON under `docs/incomplete/`; post-clone commands (`bun install --frozen-lockfile`, `bunx next typegen` in `web`) skipped when the workspace sits at the template's commit. A pool of 4 sparse slots (`pool.sparse_size = 4`) is kept ready.
- **Observed on 2026-10-18:** `wk new 9999` in ezhomesteading took 2.8 s wall and produced every copied file plus `node_modules` and `web/.next/cache`. `wk template refresh` took about 41 s (2.35 s worktree add, 2.76 s `bun install`, 1.76 s `next typegen`, 33.9 s `bun run build`).
- **Design-time probes, 2026-10-16** (`DESIGN.md` line 58, README "Disk use"): about 7 s full or 5 s sparse from a warm template; by `df`, about 100 MB for a sparse workspace, 520 MB for a full one, 2 to 2.7 GB for a worktree with its own install.
- **The fact that shrinks the disk claim:** on macOS bun already installs packages with `clonefile` from `~/.bun/install/cache`, so a plain `bun install` worktree shares most `node_modules` blocks too (`SLIM-SCOPE.md` line 9). The honest disk comparison is against that, not against a byte copy. The win is in `web/.next/cache` (up to 765 MiB per template generation, `SLIM-SCOPE.md` line 165), the sparse checkout and time, and that must be what the write-up says if the data agrees.
- **The thresholds wk's config records and never enforces** (`[new]` in `~/.wk/ezhomesteading.toml`): `max_seconds_cold = 10`, `max_mb_per_workspace = 120`, `burst_size = 10`, `burst_max_seconds = 60`. Report each as met or not met.
- **The portfolio's rule** (`projects.ts` lines 221 to 226): "Nothing here says why they were built or how well they work; that is Zach's to write", and every capture is a real run against throwaway repos under a stand-in home folder, so no real repository content, file content or private path appears. The existing frames are 780 by 1688 PNGs in `src/assets/projects/wk/01.png` to `05.png`.

## Do these, in order

1. **Make the safe copy.** `git clone --no-hardlinks ~/Projects/ezhomesteading <scratch>/ezh-bench`, then copy into it the untracked files the gates need (`web/.env`, `web/next-env.d.ts`, `native/expo-env.d.ts`). Set `WK_HOME=<scratch>/wkhome` for every wk command, with an `ezh-bench.toml` that copies the values of `~/.wk/ezhomesteading.toml` (paths are relative, so they carry over), its own `machine.toml` with the same caps and its own slots `dir` under the scratch `WK_HOME`, and an `ezh-bench.slots.toml` copied from ezhomesteading's. Reason: the benchmark makes and removes dozens of worktrees and forces removals between trials; none of that may touch a live repository, live slots or a live gc agent. Check: `wk config show` under the new `WK_HOME` reports the file found.
2. **Write the method, then stop for the owner.** In `docs/bench/METHOD.md`, list each scenario below with its exact commands, the number of runs (at least 5 per timed point, 3 for anything over a minute), what is cold and what is warm, and what is held constant. Post it and wait. Reason: the owner judges whether a comparison is fair before any number exists, which is the check this tier relies on.
3. **Scenario A, time to a workspace that passes its first gate.** Baseline: `git worktree add` plus the manual setup `docs/gates.md` prescribes (`bun install --frozen-lockfile`, copy the three files, `bunx next typegen`), with the bun cache warm, as it always is on this machine. wk: `wk new <n>` from the template with the pool empty (`pool.sparse_size = 0`), and again with the pool full. For each, measure seconds to ready and then seconds for the first `cd web && bunx tsc --noEmit` and the first `bun run build`. The first build is where the cloned `web/.next/cache` should show; if it does not, say so. Use `/usr/bin/time -p`.
4. **Scenario B, disk.** Before and after making 10 workspaces each way, record `df -k` on the volume (free blocks), not `du`. Note other writers: run `df` twice a minute apart before the run, and repeat a run whose drift is larger than its result. Report MB per workspace with the spread.
5. **Scenario C, a burst.** Make 10 workspaces at once (`burst_size`), both ways, and record the wall time to the last ready one and each one's own time. Reason: agents start together, and `new.max_parallel = 2` is a deliberate cap on APFS clone contention; show its effect.
6. **Scenario D, the slot cap.** Start 6 concurrent `bun run build` runs in 6 workspaces, once plain and once each through `wk run --slot heavy` (the hook's own rewrite). Sample `vm_stat` free plus inactive pages and swap use (`sysctl vm.swapusage`) every second, and record wall time, each run's exit status, peak memory pressure and any failed build. Stop a plain run that pushes swap past 4 GiB, and record that it was stopped. Reason: this is the claim "ten agents that start their gates together can exhaust a 16 GB machine" in README "Cap heavy gates", tested at a size this machine can survive.
7. **Scenario E, cleanup.** Make 12 workspaces; commit and merge 4 into `main` in the clone, leave 4 dirty, leave 4 clean but unmerged. Run `wk gc` and then `wk gc --apply` under the scratch `WK_HOME` only. Record each verdict, which were removed, the time, and that nothing dirty or unmerged went. Reason: the claim is "removes only what is finished"; the chart is a verdict table, and any wrong verdict is a finding to report, not to smooth over.
8. **Keep the data honest.** Every run writes a row to a CSV in `docs/bench/data/` (scenario, way, run, seconds, df KB, exit status, timestamp, load average at start). `docs/bench/run.sh` reruns every scenario from a clean scratch directory, and `docs/bench/chart.py` (Python 3 standard library only, no packages) draws every SVG from the CSVs. No number is typed into a chart or the text by hand.
9. **Draw the charts.** In `docs/bench/`: time to first green gate (grouped bars, baseline and wk, with the run spread), disk per workspace by `df`, the burst timeline, memory over time for the slot test (two lines, plain and slotted), and the gc verdict table. Each SVG states the machine, the date and the wk commit in its caption, uses explicit colors that read on both a white and a dark GitHub background, and stays under 50 KB. Follow the dataviz rules for one chart form per question; no 3D, no pie charts.
10. **Write the analysis.** `docs/bench/ANALYSIS.md`: one section per scenario with the chart, the numbers, what they mean for an agent workflow, and where wk did not help or cost something. Then a short "Measured" section in `README.md`, after "Disk use", with two or three of the charts and a link to the analysis. Replace any older README number that the new data contradicts, and cite the run.
11. **Draft, do not ship, the portfolio frames.** Write `docs/bench/PORTFOLIO-DRAFT.md`: up to two proposed frames in the shape of `projects.ts` (eyebrow, headline, sub, and a 780 by 1688 PNG rendered from the SVG with `qlmanage -t -s 1688` or `sips`), each sub a plain statement of a measured number with no claim of why wk was built. Do not edit `~/Projects/portfolio`. Reason: the portfolio's own rule makes the effectiveness wording the owner's.

12. **Scenario F, how often Stop gates overlap (added 2026-10-18 from `docs/incomplete/gate-contract/SCOPE.md`, option B).** The personal-config Stop hook runs a repository's whole gate with `eval`, outside `wk run`, so it is not under the slot cap, and each gate peaks near 1 GiB (README, "Cap heavy gates"). The question is whether two or more of them really run at the same time on this machine. This step only reads. Copy `~/.config/personal-config/completion-gate.log` and `~/.wk/slots/runs.log` into the scratch directory first and read the copies. The gate log has one line per stop, stamped when the gate ended and, since 2026-10-18, ending in `ran=<folder>`. It has no start time or length, so the intervals cannot be rebuilt exactly. Two ways to answer, and the owner picks in "Ask before building": count stops from different folders that end within the same 60 seconds as an upper bound on overlap, or leave the log as it is, and propose as a finding that the hook should log its own length (a change to `templates/hooks/completion-gate.sh` in personal-config, a separate row, never made here). Report the count with its limit stated. If overlap is rare, say so: that closes option B and the hook stays as it is.

## Ask before building

- **owner's call:** the method in step 2. Either a scenario is fair or it is not; the owner may cut one, change a baseline, or say the slot test must not run at all on the machine they work on.
- **owner's call:** when the machine is quiet enough for steps 3 to 7. The timings are worthless with other agents building, so ask, and record the load average at each run's start either way.
- **owner's call:** whether the portfolio gets the frames, and their wording. The draft in step 11 is all this row does there; either answer may be no.
- **owner's call:** for step 12, whether the 60 second end-time count is enough, or whether the hook should first log its own length so a later measurement is exact.
- Whether the README section shows the slot test at all if the plain run never got close to exhausting memory at 6 builds. Say what the data showed and let the owner pick; do not raise the count past what the machine survives to make the chart dramatic.

## Not in scope, whoever asks

- Any change to wk's Go code, config defaults or behavior, even a fix a scenario points to. File a finding in `ANALYSIS.md` instead.
- Any run against the live ezhomesteading checkout, its `~/.wk` files, its pool, its template or its launchd agent.
- `go install ./cmd/wk`, or a benchmark of an unreleased build.
- Editing `~/Projects/portfolio`.
- Comparisons against other tools (git-worktree managers, devcontainers, Nix). Only plain `git worktree add` with the documented manual setup is the baseline.
- Turning on the landing queue or path leases, or measuring them.

## Hand back

The gate, in your wk worktree, must be green, though this row should change no Go:

```bash
GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'
```

Also show that the data reproduces: `sh docs/bench/run.sh --scenario A` (or the cheapest scenario) and `python3 docs/bench/chart.py` regenerate a chart byte for byte except for timing values, and every SVG opens in a browser.

The owner should see: `docs/bench/ANALYSIS.md` with every chart rendered, the new README section, and `PORTFOLIO-DRAFT.md` with its PNGs. Then list what was not measured and why.

Commit `docs/bench/` and `README.md` in your worktree with a message starting `row 19:`, with no `Co-Authored-By:` trailer and no "Generated with" line. Remove the scratch clone and `WK_HOME` once the CSVs are committed, and say that you did. Write the close-out section at the end of this file: what ran, the headline numbers, the findings, and what was left unmeasured.
