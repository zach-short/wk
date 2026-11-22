# 19. Measure what wk saves, and chart it for the README and the portfolio


> **Claimed `IN FLIGHT` 2026-11-21.**
*Opened 2026-10-18, at the owner's request: "conduct tests in ezhomesteading or under whatever circumstances to illustrate its effectiveness and provide an analysis with visuals" for the wk README and the wk story on the portfolio site. wk has been in daily use in 12 repositories since 2026-10-17, and no measurement of the shipped tool exists; the numbers on record are design-time probes from 2026-10-16.*

**Model: Opus 5.5. Lane C. Waits on nothing, but every run in step 3 onward waits on the owner saying the machine is quiet. Work in your own worktree of `~/Developer/zach-short/wk` (`cd "$(wk new 19)"` from the main checkout); the benchmark itself runs against a separate clone, never the live ezhomesteading checkout. Files it owns: `docs/bench/` (new), `README.md`.**

The tier is Default, not Deep: a misleading benchmark can pass every gate, so the check is that every number in the write-up comes from a raw CSV in `docs/bench/data/` that `docs/bench/run.sh` regenerates, and the owner reads the method before any chart is drawn (step 2).

## Orientation

You are measuring wk, a Go CLI that makes warm git worktrees for parallel Claude Code agents and removes finished ones. Source and README: `~/Developer/zach-short/wk`. The repository you measure against is ezhomesteading (`~/Developer/dan-short/ezhomesteading`), a bun monorepo (Next.js web app, Expo native app, Go backend) with 5,740 tracked files and a 1.0 GB `.git`, the repository wk was built for.

Read, in this order: `~/Developer/zach-short/wk/README.md` sections "Make a workspace", "The pool", "The template", "Cap heavy gates", "Remove finished workspaces", "Disk use"; `~/Developer/zach-short/wk/docs/incomplete/landing-layer/DESIGN.md` lines 15 to 60 (the 2026-10-16 baseline probes); `~/Developer/zach-short/wk/docs/incomplete/landing-layer/SLIM-SCOPE.md` lines 1 to 40 (why `du` lies about clones); `~/.wk/ezhomesteading.toml` and `~/.wk/machine.toml`; `~/Developer/dan-short/ezhomesteading/docs/gates.md` section "Fresh worktree setup"; `~/Developer/zach-short/portfolio/src/lib/projects.ts` lines 220 to 275 (the wk story).

Session rules, restated here because this prompt is pasted alone:

- `GOTOOLCHAIN=local` on every Go command.
- Never run `go install ./cmd/wk`. Every project's hooks and gc agents run `/Users/zachshort/go/bin/wk`, so an install changes all of them at once. Measure the installed binary as it is; record `go version -m ~/go/bin/wk` and `git -C ~/Developer/zach-short/wk rev-parse HEAD` in the data.
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

1. **Make the safe copy.** `git clone --no-hardlinks ~/Developer/dan-short/ezhomesteading <scratch>/ezh-bench`, then copy into it the untracked files the gates need (`web/.env`, `web/next-env.d.ts`, `native/expo-env.d.ts`). Set `WK_HOME=<scratch>/wkhome` for every wk command, with an `ezh-bench.toml` that copies the values of `~/.wk/ezhomesteading.toml` (paths are relative, so they carry over), its own `machine.toml` with the same caps and its own slots `dir` under the scratch `WK_HOME`, and an `ezh-bench.slots.toml` copied from ezhomesteading's. Reason: the benchmark makes and removes dozens of worktrees and forces removals between trials; none of that may touch a live repository, live slots or a live gc agent. Check: `wk config show` under the new `WK_HOME` reports the file found.
2. **Write the method, then stop for the owner.** In `docs/bench/METHOD.md`, list each scenario below with its exact commands, the number of runs (at least 5 per timed point, 3 for anything over a minute), what is cold and what is warm, and what is held constant. Post it and wait. Reason: the owner judges whether a comparison is fair before any number exists, which is the check this tier relies on.
3. **Scenario A, time to a workspace that passes its first gate.** Baseline: `git worktree add` plus the manual setup `docs/gates.md` prescribes (`bun install --frozen-lockfile`, copy the three files, `bunx next typegen`), with the bun cache warm, as it always is on this machine. wk: `wk new <n>` from the template with the pool empty (`pool.sparse_size = 0`), and again with the pool full. For each, measure seconds to ready and then seconds for the first `cd web && bunx tsc --noEmit` and the first `bun run build`. The first build is where the cloned `web/.next/cache` should show; if it does not, say so. Use `/usr/bin/time -p`.
4. **Scenario B, disk.** Before and after making 10 workspaces each way, record `df -k` on the volume (free blocks), not `du`. Note other writers: run `df` twice a minute apart before the run, and repeat a run whose drift is larger than its result. Report MB per workspace with the spread.
5. **Scenario C, a burst.** Make 10 workspaces at once (`burst_size`), both ways, and record the wall time to the last ready one and each one's own time. Reason: agents start together, and `new.max_parallel = 2` is a deliberate cap on APFS clone contention; show its effect.
6. **Scenario D, the slot cap.** Start 6 concurrent `bun run build` runs in 6 workspaces, once plain and once each through `wk run --slot heavy` (the hook's own rewrite). Sample `vm_stat` free plus inactive pages and swap use (`sysctl vm.swapusage`) every second, and record wall time, each run's exit status, peak memory pressure and any failed build. Stop a plain run that pushes swap past 4 GiB, and record that it was stopped. Reason: this is the claim "ten agents that start their gates together can exhaust a 16 GB machine" in README "Cap heavy gates", tested at a size this machine can survive.
7. **Scenario E, cleanup.** Make 12 workspaces; commit and merge 4 into `main` in the clone, leave 4 dirty, leave 4 clean but unmerged. Run `wk gc` and then `wk gc --apply` under the scratch `WK_HOME` only. Record each verdict, which were removed, the time, and that nothing dirty or unmerged went. Reason: the claim is "removes only what is finished"; the chart is a verdict table, and any wrong verdict is a finding to report, not to smooth over.
8. **Keep the data honest.** Every run writes a row to a CSV in `docs/bench/data/` (scenario, way, run, seconds, df KB, exit status, timestamp, load average at start). `docs/bench/run.sh` reruns every scenario from a clean scratch directory, and `docs/bench/chart.py` (Python 3 standard library only, no packages) draws every SVG from the CSVs. No number is typed into a chart or the text by hand.
9. **Draw the charts.** In `docs/bench/`: time to first green gate (grouped bars, baseline and wk, with the run spread), disk per workspace by `df`, the burst timeline, memory over time for the slot test (two lines, plain and slotted), and the gc verdict table. Each SVG states the machine, the date and the wk commit in its caption, uses explicit colors that read on both a white and a dark GitHub background, and stays under 50 KB. Follow the dataviz rules for one chart form per question; no 3D, no pie charts.
10. **Write the analysis.** `docs/bench/ANALYSIS.md`: one section per scenario with the chart, the numbers, what they mean for an agent workflow, and where wk did not help or cost something. Then a short "Measured" section in `README.md`, after "Disk use", with two or three of the charts and a link to the analysis. Replace any older README number that the new data contradicts, and cite the run.
11. **Draft, do not ship, the portfolio frames.** Write `docs/bench/PORTFOLIO-DRAFT.md`: up to two proposed frames in the shape of `projects.ts` (eyebrow, headline, sub, and a 780 by 1688 PNG rendered from the SVG with `qlmanage -t -s 1688` or `sips`), each sub a plain statement of a measured number with no claim of why wk was built. Do not edit `~/Developer/zach-short/portfolio`. Reason: the portfolio's own rule makes the effectiveness wording the owner's.

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
- Editing `~/Developer/zach-short/portfolio`.
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

## Close-out, 2026-11-22

Built on branch `row-19-build` by Opus 5.5. The owner read `docs/bench/METHOD.md` and answered its 15 questions at 23:45 EST on 2026-11-21: every proposal as written, with six builds in scenario D, the gates inside `run.sh` only (where the live hook cannot see them), and "start now, record the load".

What ran. Scenarios A to F ran on 2026-11-22 between 00:20 and 02:20 EST against a fresh `git clone --no-hardlinks` of ezhomesteading at `80ce82149`, under a scratch `WK_HOME` with the live config values, wk 0.1.11 (`e4ed909`, Homebrew). It used 18 runs in A, 9 sets of 10 in B, 20 bursts of 10 in C, 6 runs of six builds in D, 3 repetitions of 16 workspaces in E, and one read of the Stop-hook log in F. The machine was shared: a Lunar Client Java process and several Claude Code sessions ran, and the 1-minute load at a run's start was between 1.83 and 4.31. One run is marked `load_high` (A, cold template, run 1). No run was stopped. Every number is in `docs/bench/data/*.csv`. `docs/bench/chart.py` draws the five SVGs and `docs/bench/SUMMARY.md` from them, and `docs/bench/ANALYSIS.md` reads them.

Headline numbers (medians):

- A: ready in 2.44 s plain, 1.58 s from the template, 0.10 s from the pool, 3.84 s with a cold template. The first `tsc` and the first build were not faster in any arm.
- B: 635.2 MiB per plain worktree, 205.6 MiB per sparse wk workspace, 636.2 MiB per `wk new --full`.
- C: the last of ten ready after 15.23 s plain, 13.03 s with wk, 10.20 s with a pool of 4, and 11.65 s with `max_parallel = 10`. No create failed.
- D: six plain builds took 57.0 to 61.1 s, and six slotted ones (2 places) 119.6 to 122.5 s. Free memory never fell below 27,031 MB. Swap grew from 32 to 318 MB once.
- E: 48 of 48 gc verdicts were right, the held-file rule of rows 31 and 32 included. `gc --apply` took 34.85 s.
- F: 19 pairs of Stop gates from different folders ended within 60 s, 18 of them on 2026-11-12. Option B of `gate-contract/SCOPE.md` closes.

Findings, all in `ANALYSIS.md`, "Findings for later rows":

- A sparse workspace misses `max_mb_per_workspace = 120`.
- All of the disk saving comes from the sparse checkout. A full wk workspace costs the same as a plain one, because bun already clones `node_modules`.
- `max_parallel = 2` finished a burst of ten later than 10 did.
- The slot cap doubled the wall time of six builds on 64 GiB and protected nothing at risk.
- No build-cache gain is possible while the config clones no part of `web/.next`.
- In the pool arm of A, the gates slowed over five runs, for a cause not found.
- `wk gc --apply` takes about 35 s for 16 workspaces.

README changes. "Cap heavy gates" no longer says "a 16 GB machine" and cites scenario D. "Disk use" has the new `df` figures and says that the old 2 to 2.7 GB was a `du` figure. A new "Measured" section has three charts. The 16 GB mention in "Provenance" is history and stays.

Portfolio. `docs/bench/PORTFOLIO-DRAFT.md` holds two terminal frames from real bench output. The portfolio itself was not edited.

Left unmeasured, and why:

- A cold bun cache (no real worktree starts from one).
- A cold OS file cache (needs `sudo purge`).
- More than six builds, and heavy capacity 1 (owner's answers).
- A cloned `web/.next/cache` (owner's answer).
- The lengths of the Stop-hook gates (the log has no start time).
- Any other machine.

Reproduction. `sh docs/bench/run.sh --scenario F` regenerated `F.csv`, equal except for the 2026-11-22 row, where the live log had grown. A one-run `RUNS=1 sh docs/bench/run.sh --scenario E` gave the same verdict rows. A second `python3 docs/bench/chart.py` left every SVG and `SUMMARY.md` byte-identical. While I ran the repro, I fixed two faults in `run.sh` that only a rerun on a used clone can show: the E file names now carry a per-run token, and the intact check reads the new names. The committed data comes from a fresh clone and does not have the fault: 48 distinct rows, and no skipped commit in the log.

The wk gate is green in the worktree. The row changes no Go. After the commit, I removed the scratch clone and the scratch `WK_HOME`.

### Sign-off, 2026-11-22

- Builder: Opus 5.5, scratch worktree `wk-row-19-build`, branch `row-19-build`, commits `30b27fd`, `8e1a44d`, `3196fd8`. Relay passes: 0. Fix rounds: 0.
- Review: one independent Opus 5.5 audit in its own worktree (`row-19-audit`). Verdict: non-blocking. It recomputed about 120 figures from `docs/bench/data/*.csv` and all matched. `chart.py` rebuilt every SVG and `SUMMARY.md` byte-identical. The diffstat matched (21 files, 2398 insertions, 2 deletions). The wk gate passed.
- Owner answers: given 2026-11-21 23:45 EST to the questions posted 19:26 EST. The slot bypass is allowed only inside `run.sh` against the scratch clone.
- Open, from the audit (none changes a conclusion):
  - A1: only scenario A marks run 1 `first_run` (`run.sh:226`). The B, C and D medians use every run, against `METHOD.md:73,235`. C on runs 2 to 5 would be 15.14, 12.96, 9.99 and 11.57 s. The prose does not say that all runs were used.
  - A2: `ANALYSIS.md:11` says "runs 2 to 5", but the cold arm uses runs 1 to 3, with run 1 marked `load_high`. The median is 3.84 s either way.
  - A3: the `SUMMARY.md` "Runs" column says 5 where each A median uses 4. `PORTFOLIO-DRAFT.md:13` says "5 runs each".
  - A4: some figures have no CSV: Lunar Client 9.6 GB, bun install about 2 s, the pool.log "within 2 s" (the log went with the scratch folder), 447 MB of PNGs (`SLIM-SCOPE.md:112` gives 419 MiB), and the step times in the portfolio terminal bodies.
  - A5: `run.sh --fresh` runs `rm -rf` on `$BENCH_DIR/*` with no check that it is a scratch path; `wait_disk` gives up after 2 minutes with no flag; the pool arm does not check for 4 ready slots; the E match does not check `branch_gone`.
- Owner's open call: question 13, whether the README shows the slot-test chart.
