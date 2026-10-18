# Gate contract: scope

Dated 2026-10-18. This document proposes and decides nothing. The owner's answers go in the last section.

## Correction to the brainstorm

Earlier in this session I said the completion gate is silent in every wk worktree. That was wrong. I repeated a line from the personal-config `CLAUDE.md` ("The completion gate runs nothing in a worktree") without reading the hook. Row 122 (commit `b52201e`, 2026-10-16) already fixed it, and the gate log proves it. That `CLAUDE.md` line is stale: its last commit (`94260bc`) is about 11 hours older than the fix. The original step 2, a new tracked contract file, solved a problem that is mostly gone. The ground truth below shows what is left.

What I did not read: the archive index, and the two code standards in full. A reader who needs those should treat the options as unchecked against them.

## What exists, verified 2026-10-18

| Claim | Verified state | Citation |
|---|---|---|
| The Stop gate finds the main checkout's config from a worktree | True. It maps the git common dir to the main checkout. | `~/.claude/hooks/personal-config/completion-gate.sh`, function `config_dir`; commit `b52201e` |
| The gate runs in wk worktrees | True. Passing runs from `wk-19` are logged with the main checkout's config. | `~/.config/personal-config/completion-gate.log`, lines for `cwd=.../wk/.claude/worktrees/wk-19` |
| A stop in a subfolder skips the gate | True. `config_dir` appends the subfolder, so it looks for `<main>/docs/bench/.personal-config.json`, finds nothing and exits 0. 83 of 676 log lines show this for `wk/docs/bench` alone. | gate log, `config=none looked=.../wk/docs/bench/...`; `config_dir`, the `prefix` variable |
| The Stop gate does not use wk slots | True. The hook runs `eval "$gate"` and the file has no `wk run`. wk's hook only rewrites `Bash` tool calls, and a Stop hook is not one. | `grep "wk run" completion-gate.sh` gave nothing; wk `README.md`, "The hook" |
| wk keeps its own gate list | True. `~/.wk/wk.toml` has four `[[gates.run]]` entries. `.personal-config.json` has the same four in one `gateCommand` string. They agree today, for the wk repo only. I did not check other repos. | `wk config show`; `/Users/zachshort/Projects/wk/.personal-config.json` as logged |
| `doctor` compares the two | False. `wk-config` checks only that landing is on and that one `[[gates.run]]` exists. | `src/doctor/rules/wk-config.ts` |
| wk copies the config into worktrees | False, and not needed locally. `copy_files` holds only `.claude/settings.local.json`. | `wk config show` |
| The cloud VM has the gate | Probably not. claude-home ships the hook files but no `settings.json`, so nothing registers the hook. The config file is untracked, so a cloud clone has none. Inferred from the README and the file list. I did not run a cloud session. | claude-home `README.md`; its tree via `gh api` |
| Item 126 overlaps this work | No. It reconciles board rows with wk claims. It does not touch gates. | `docs/incomplete/autopilot-2026-10-17/STATE.md`, the 126 lines |

## What this is and what this is not

This is about four small defects in how the gate runs and how its definition is kept. It is a change to the personal-config hook template and doctor, plus a text fix.

This is not a new contract file. It is not the claude-home sync (step 1) or the wk event log (step 3). It does not change how wk lands changes or how wk judges gates. It does not add a cloud gate now.

## Options

**A. A stop in a subfolder skips the gate.**
- A1. Climb from the working folder toward the checkout root and use the nearest config. Defense: it removes 83 silent skips in one folder and keeps a per-package config where one exists. Against: a nested config could shadow the root gate without the owner noticing.
- A2. Leave it. Defense: a folder target may want no gate. Against: the log shows an ordinary code subfolder skipped for a whole row, and the skip is silent.
- Recommended: A1.

**B. The Stop gate bypasses the memory cap.**
- B1. When wk is on `PATH` and the repo has a slots file, run the gate as `wk run --slot check -- <gate>`. Fail open if wk is missing. Defense: it puts the one heavy command that every agent runs at the end of every turn under the cap. Against: the hook has a 570 second clock, and time spent waiting for a place counts against it, so a queued gate can time out and block the turn.
- B2. Measure first. Item 19 can count how often stop gates overlap. Defense: it avoids a hook change that may fix nothing. Against: it leaves the bypass open while you wait.
- B3. Leave it. Against: the cap exists for exactly this load.
- Recommended: B2, then B1 if the overlap is real. Only ezhomesteading has a slots file today, so wk and personal-config gain nothing from B1 yet.

**C. Two definitions of one gate.**
- C1. Keep both and add a doctor rule that warns when they disagree. Defense: small, and it works for any repo. Against: the two are different shapes (a list with timeouts and env, and one string), so a comparison is approximate.
- C2. Make wk read `gateCommand`. Against: wk loses per-gate names, timeouts and cache resets that the batch gate needs.
- C3. A shared, tracked list that renders both. Against: it is the large change this scope set out to avoid, and a generic file cannot hold wk's env and timeouts.
- Recommended: C1.

**D. The cloud VM has no gate.**
- D1. claude-home ships a settings fragment, and `cloud-setup.sh` merges it. The repo then needs a tracked gate definition. Against: it needs the tracked file this scope dropped.
- D2. Rely on CI for cloud work. Defense: cloud work lands as a pull request, and personal-config already has CI. Against: wk has no CI.
- Recommended: D2 now, D1 only if you run cloud sessions on these repos.

**E. The stale line.** Fix the `CLAUDE.md` sentence. There is no real option against it. It is free and it stops the next agent from repeating my mistake.

## Dials

| Dial | Recommended default | Where it lives |
|---|---|---|
| Stop gate clock | 570 seconds, unchanged | `PERSONAL_CONFIG_GATE_TIMEOUT` |
| Slot class for the Stop gate | `check` | the repo's slots file |
| How far A1 climbs | to the git top level, never beyond | the hook |
| Gate log size | 1000 lines, trimmed to 500, unchanged | the hook |

## Hazards

- The hook at `~/.claude/hooks/personal-config/` serves every project. A bad edit changes all of them at once. The claude-home copy must change with it, or the cloud keeps the old one.
- A Stop hook fails open. A bug in A1 or B1 would pass every stop with no sign. That is a silent failure, so the model routing rule puts this at the Deep tier, with a Deep review. C1 and E are Default tier.
- The wk repo has a rule against running `go install` anywhere but `main`. None of this needs it, because the changes are in personal-config.
- B1 adds waiting to a path where a block costs a whole turn.
- The log still counts test temp folders as `config=none`. Any count of silent skips needs those filtered out. The 83 figure is for `wk/docs/bench` only.

## Open questions for the gate

1. A: climb to the nearest config (recommended), or leave it?
2. B: measure overlap first through item 19 (recommended), build B1 now, or leave it?
3. C: add the doctor comparison (recommended), or nothing?
4. D: rely on CI for cloud work (recommended), or ship a settings fragment?
5. E: fix the stale `CLAUDE.md` line now (recommended)?

## Answers

Recorded 2026-10-18, in chat. The owner answered "all", which accepts every recommendation.

1. A: A1, climb to the nearest config. Tier changed the same day, at the owner's request, from Deep to Default (Opus 5.5 build, separate Opus 5.5 review); the reasoning is in `PASSOFF-PROMPTS.md`.
2. B: B2, measure overlap through item 19 first. No hook change until the data says so.
3. C: C1, a doctor rule that warns when the two gate definitions differ. Default tier.
4. D: D2, rely on CI for cloud work. No change now.
5. E: fix the stale `CLAUDE.md` line now. Default tier.

The build prompts are in `PASSOFF-PROMPTS.md` in this folder. They are for other models than the one that wrote this scope.

## What landed

Recorded 2026-10-18.

- A and E: personal-config `main` at `c5c1365`. The hook now climbs to the nearest config and runs the gate in the folder the config came from, mapped into the checkout being stopped in. A first review blocked an earlier version, because the gate still ran in the subfolder and `go vet ./...` failed on a green tree from `wk/docs`. A second review approved the fix. The `CLAUDE.md` line is corrected.
- C1: `src/doctor/rules/gate-agree.ts`, an advisory finding that runs only when `landStep` is `wk`. It is on `main` at `927bf10`.
- Installed: `~/.claude/hooks/personal-config/completion-gate.sh`, with a backup in `~/.claude/backups/`. The claude-home copy was updated at `80a7396`. That copy also took the earlier timeout and unreadable-gate changes it lacked.
- B: added as step 12 of `passoff/19-*.md`. The existing gate log has no start time or length, so overlap can be bounded but not measured exactly. The item records that choice for the owner.
- D: no change, as answered.
- Not verified: a real Stop event with the installed hook, and the Linux CI run.
