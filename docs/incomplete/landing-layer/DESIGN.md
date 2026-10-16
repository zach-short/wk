# Design: the landing layer, a Go CLI and daemon over git for parallel agent workspaces

Status: DESIGNED (Stage 3), ratified 2026-10-16. Opened 2026-10-16 by a side session of the session in which Zach asked how hard a small personal version of Anthropic's Scion would be. Scoped the same day by a Fable 5.1 session in the main checkout at `d17444591` (`SCOPE.md`, committed at `db9984002`). Ratified the same day: Zach answered every section 7 question in chat, in a second Fable 5.1 session in the main checkout, and the file was renamed from `SCOPE.md` to `DESIGN.md`. Section 7 holds the decisions D1 to D20; the "Answers" section at the end holds the chat record they cite. Sections 1 to 6 and 8 to 10 are the scope text as ratified; where a decision supersedes a line, the line carries a dated tag. From here the design is frozen: it changes by a new dated `D<n>` or an `As built:` note, never by editing a decision in place.

Working title: landing layer. Proposed command name: `wk` (workspace). The name is decision Q2.

The brief, from the parent session: do not rebuild git storage. The repository is 5,519 tracked files and a 1 GB `.git`, so git is not the bottleneck. Build a thin layer over git in four phases, each useful on its own: cheap workspaces (P1), path leases (P2), server-side landing (P3), one gate run per batch (P4). The parent session estimated two to three weeks of agent-driven work. That estimate is re-done in section 8 against the numbers below.

## 1. What exists, verified 2026-10-16

Every number was read on 2026-10-16 between 10:43 and 10:54 EDT, in the main checkout at `d17444591`, unless the citation says otherwise. The parent brief carried some numbers from an earlier reading; where they differ, this table wins and says so.

### 1.1 The machine and the toolchain

| Claim | Verified state | Citation |
|---|---|---|
| Memory and cores | 16 GB; 10 cores, 10 physical | `sysctl -n hw.memsize`, `hw.ncpu`, `hw.physicalcpu` |
| Disk | APFS; the system volume reports 460 GiB with 53 GiB available. The brief said 41 GB free; the measured value is 53 GiB | `diskutil info /`; `df -h /` |
| git | 2.49.0, which has `merge-tree --write-tree --merge-base`, `--name-only`, and `update-ref <ref> <new> <old>` | `git --version`; `git merge-tree -h`; `git update-ref -h` |
| Go on PATH | go1.24.1 darwin/arm64. The backend is on its own toolchain (row 184 moved it to 1.26.9); a separate tool pins its own | `go version`; `HANDOFF.md` step 137 |
| bun | 1.2.9, the version CI pins | `bun --version`; `.github/workflows/ci.yml` |
| Load at 10:45 EDT | 31 `claude` processes; 27 node, bun or next processes holding 2,222 MB RSS; 76% memory free; swap 0; 7,378 pageouts since boot | `pgrep -fl claude`; `ps -axo rss,comm`; `memory_pressure`; `sysctl vm.swapusage` |
| Shared caches | go-build 4.5 GB; bun install cache 4.1 GB; golangci-lint 22 MB | `du -sh` on `~/Library/Caches/go-build`, `~/.bun/install/cache`, `~/Library/Caches/golangci-lint` |

### 1.2 The repository and its worktrees

| Claim | Verified state | Citation |
|---|---|---|
| Tracked files | 5,519 | `git ls-files \| wc -l` |
| `.git` | 1.0 GB; objects 995 MB; 16 packs holding 121,269 objects (969.5 MiB); 1,833 loose objects (20.7 MiB); one garbage `tmp_obj` file | `du -sh .git`; `git count-objects -vH` |
| Worktrees | 22 entries at 10:43: the main checkout, 20 under `.claude/worktrees/`, and one prunable trial of a sibling session whose directory was already gone. 3 of the 20 are locked (live sessions) | `git worktree list` |
| Worktree disk | 42 GB under `.claude/worktrees/`. Sizes: one at 442 MB, two at 478 MB, eight at 2.0 GB, eight at 2.7 GB, one at 3.1 GB | `du -sh .claude/worktrees` and `.claude/worktrees/*` |
| One 2.7 GB worktree, by part | `node_modules` 1.5 GB; `web/.next` 778 MB; `docs` 427 MB | `du -sh` in `agent-a18e6d6ebabbfe17c` |
| The main checkout, by part | `node_modules` 2.3 GB in 143,199 files with 90 top-level symlinks; `web/.next` 2.6 GB, of which `cache` is 1.5 GB; `docs` 441 MB, of which `docs/incomplete` is 440 MB | `du -sh`; `find node_modules -type f \| wc -l` |
| Workspace package links are relative | `node_modules/@ezh/core -> ../../packages/core`, and the same for `api`, `config`, `eslint-config`, `icons`. A `node_modules` copied into a worktree therefore resolves to that worktree's own `packages/` | `ls -la node_modules/@ezh/` |
| Branches | 21 local; 17 are ancestors of `main`; 8 are not (`claude/focused-bassi-248015`, `row128-turnstile`, and six `worktree-agent-*`) | `git branch --merged main`, `--no-merged main` |
| Worktree naming by Claude Code | Directory `.claude/worktrees/agent-<17 hex>`, branch `worktree-agent-<same>`; the desktop app locks a live session's worktree | `git worktree list --porcelain` |
| `.claude/worktrees` is ignored | `.gitignore:54` and `.git/info/exclude` | `git check-ignore -v .claude/worktrees` |
| `extensions.worktreeConfig` | `true` in `.git/config` (line 25), last modified 01:23 EDT today, before this session's tests | `git config --get extensions.worktreeConfig`; `stat -f '%Sm' .git/config` |
| The main checkout is itself a dirty workspace | 69 status entries at 10:54: 46 tracked changes, 23 untracked. Sessions edit there directly | `git status --short` |
| Composition roots, by size | `backend/routes/routes.go` 629 lines; `backend/main.go` 260; `HANDOFF.md` 468; `PASSOFF.md` 1,101 | `wc -l` |
| Next free numbers | Board row 196 (`### 195.` at `PASSOFF.md:1098` is the last); ledger step 143 (step 142 is the last). Re-read both before use | `grep -nE '^### [0-9]+' PASSOFF.md`; `HANDOFF.md` step log |

### 1.3 Measurements for P1, taken in the session scratchpad on the same APFS volume

| Operation | Time | Disk written | Note |
|---|---|---|---|
| `git worktree add --detach` of HEAD, full checkout | 2.24 s | 478 MB real | `/usr/bin/time -p`; removed again at once |
| `git worktree add --no-checkout`, then sparse-checkout excluding `docs/incomplete`, then checkout | 0.35 s | 52 MB real | 5,519 paths still listed by `ls-files`; 440 MB of docs not written |
| `cp -c -R node_modules` | 23.27 s, of which 19.63 s system | about zero | one `clonefile` call per file; 143,199 files |
| `clonefile(2)` on the `node_modules` directory | 4.48 s | about zero | `golang.org/x/sys/unix.Clonefile`; the whole tree in one call |
| `cp -c -R web/.next` | 0.33 s | about zero | few, large files |
| `clonefile(2)` on the `web/.next` directory | 0.074 s | about zero | |
| `df` after 4.9 GB of clones | unchanged at 12 GiB used | | `du` reports the full size of a clone; only `df` tells the truth about free space |

So one workspace from a warm template costs about 7 s with a full checkout (2.2 s git, 4.5 s `node_modules`, 0.1 s `.next`), or about 5 s with the sparse checkout. The brief's target of "under 5 s" is met only by the sparse variant or by a pre-made pool (section 4, P1 option D). The brief's second target, "near-zero disk until written", holds for the clones and not for the checkout: git writes real bytes (478 MB full, 52 MB sparse).

The test of `cp -c` by itself does not meet the target; a directory-level `clonefile` does. That is one reason the tool is Go and not a shell script: `cp` has no flag for a directory clone, and the syscall is one line in Go.

### 1.4 Tooling already in place

| Claim | Verified state | Citation |
|---|---|---|
| No lease, lock or workspace tool exists | No `wk` on PATH; `grep -rliE 'lease\|flock'` over `~/.claude/hooks`, the land-worktrees scripts and `scripts/` matches only the word "releases" in `plan.sh`; `scripts/ezh` has no worktree command (the word appears once, in a comment at line 181) | run 2026-10-16 |
| `personal-config worktree <lane>` | Prints a lane's worktree plan and checkout recipe; it creates nothing | `personal-config --help` |
| `/land-worktrees` already does predicted merges | `lib.sh` `merge_step` runs `git merge-tree --write-tree --name-only --merge-base=<base> <tree> <tip>`; `apply_resolutions` lays resolution files over a tree in a private `GIT_INDEX_FILE`; `is_absorbed` treats a branch as landed when merging it changes nothing; `classify_path` sorts a dirty file as `base`, `gone`, `past` or `new`; `trial.sh` builds a detached worktree and `read-tree -u --reset`s the predicted tree into it; `land.sh` (owner-run) makes each merge with `commit-tree` from the planned tree, then `merge --ff-only --no-autostash` in the base's checkout, or `update-ref <ref> <new> <old>` when no checkout holds the base, and refuses when the result tree differs from the plan. `selftest.sh` builds a throwaway repository for every case | `~/.claude/skills/land-worktrees/scripts/{lib,plan,trial,land,selftest}.sh` |
| The 2026-10-03 landing | 26 branches, 4 conflicts, 3 semantic fixes, every merge matched its prediction; a 9-row landing on 2026-10-06 was gated with no `.env` using the CI placeholder env | memory `landing-many-branches-via-predicted-merges` |
| The commit guard | `~/.claude/settings.json:143` runs `~/.claude/hooks/personal-config/commit-guard.sh` as a `PreToolUse` hook on `Bash`; policy file reads `print-blocks`. It walks the command's words and refuses `commit`, `push`, `pull`, `cherry-pick`, `revert`, `rebase`, `am`, `merge` (except `--ff-only` without autostash), and the discarding forms of `stash`, `checkout`, `switch`, `restore`, `reset` (`commit-guard.sh:541-545`). The words `commit-tree` and `update-ref` do not appear in the script, so plumbing is not refused by the guard; the rule against it is in `/land-worktrees` ("never find another way to make its commits") | `commit-guard.sh`; `commit-policy` |
| Claude Code hooks available | `PreToolUse` input carries `session_id`, `cwd`, `tool_name`, `tool_input` (raw: `file_path` may be relative, resolve against `cwd`); a hook denies with exit 2 and a reason on stderr, or with JSON `hookSpecificOutput.permissionDecision: "deny"`. A `WorktreeCreate` hook event exists and returns `hookSpecificOutput.worktreePath`; the command-type input fields were not found in the documentation reachable on 2026-10-16 and must be verified at build time | Context7 `/anthropics/claude-code`, `plugins/plugin-dev/skills/hook-development/SKILL.md`, `mods/types/claude-code.d.ts`, `CHANGELOG.md` |
| Hooks in place that touch edits | `PostToolUse` on `Write\|Edit` runs `no-em-dash.sh`; no `PreToolUse` hook on `Edit`, `Write` or `MultiEdit` exists. `.claude/settings.json` is tracked and holds one `permissions.allow` line; `.claude/settings.local.json` is untracked and empty | `~/.claude/settings.json`; `git ls-files .claude/` |
| The board's collision check | The `Files it owns` column, filled by the session that takes a row, before it starts, against every `IN FLIGHT` row. Cells are free text: paths, globs, and prose such as "per item in the audit" or "web navbar badge hook" | `PASSOFF.md:11-13`, `:22`, rows 76 and 77 |
| CI builds web with no `.env` | The `web` job sets `NEXT_PUBLIC_SUPABASE_URL: https://ci.invalid`, `NEXT_PUBLIC_SUPABASE_PUBLISHABLE_KEY: ci-placeholder`, `NEXT_PUBLIC_STRIPE_PUBLISHABLE_KEY: pk_test_ci_placeholder`, `NEXT_PUBLIC_MAPS_KEY: ci-placeholder`, `NEXT_PUBLIC_API_URL: https://ci.invalid`, `API_URL: https://ci.invalid`; runs `bunx next typegen` for `next-env.d.ts`; writes `native/expo-env.d.ts` with one `printf` | `.github/workflows/ci.yml`, jobs `web`, `native`, `standards` |
| Gates that cannot run on this machine | `sqlgate` and the DB suites need `EZH_TEST_DB`; CI uses a `supabase/postgres:15.14.1.167` service; Docker was not running on 2026-10-15 | `ci.yml` job `backend`; `CLAUDE.local.md` "Gates that lie" |

### 1.5 The collision history this layer answers

Each line is a dated event from agent memory, kept here so the options below can cite it.

| Date | What happened | Which phase answers it |
|---|---|---|
| 2026-07-28 | A parallel session committed all 33 modified files of the shared checkout as `b7fcc344` under a reviewing session | P3 |
| 2026-08-07 | `git commit` after `git add <paths>` shipped another agent's staged rename (`5af4759a`): one shared index | P1, P3 |
| 2026-08-10 | A whole report-affordance build was swept into two parallel commits while its author ran gates | P3 |
| 2026-08-12 | `82ec00f3d` committed a file whose dependents stayed uncommitted; every gate was green because gates read the working tree, and `main` was unbuildable | P3, P4 |
| 2026-08-18 | `routes.go` carried hunks of two sessions; `commit -o` took both and left `main` unbuildable again | P2, P3 |
| 2026-08-18 | A worktree that merely exists turns `lintgate` red in the main checkout (path-anchored exclusions), and a run inside a worktree lints the main checkout | P4 |
| 2026-08-11 | A fresh worktree fails three gates until `.env`, `next-env.d.ts`, `expo-env.d.ts` are copied and `bun install` runs; until then `@ezh/core` resolves to the main checkout | P1 |
| 2026-09-22 | A `git mv` was swept half-done into `bb82e94e5`; `main` unbuildable from there on, no gate saw it | P3, P4 |
| 2026-10-10 | Row 149 pasted into two sessions; both started it | P2 |
| 2026-10-14 | A record commit swept an autopilot's ledger edits (`782ebbe6e`); `HANDOFF.md` and `PASSOFF.md` are the docs-side composition roots | P2 |
| 2026-10-14 | The commit guard refused the integration merges an overnight prompt asked for; rows landed uncommitted in an integration worktree instead | P3 |
| recurring | Two `next build` runs over one `web/.next` print a bare `Failed to compile.` | P4 |
| recurring | `rm` of a worktree and worktree removal are refused by auto mode; `land.sh` is run by Zach | P1 `gc` |

### 1.6 What the Anthropic article says, read 2026-10-16

Source: "Redesigning source control at Anthropic", Anthropic Engineering, 2026-10-13, `https://www.anthropic.com/engineering/redesigning-source-control` (the URL in the brief returned 404; this one was found through secondary coverage and read with WebFetch). Quotes are the article's own words.

- Server-centric: "every commit, rebase, or merge happens on the server", so nothing is pushed. The client keeps a bounded local object cache.
- The unit of work is a changeset, "an entry in a write-ahead log", created server-side, recording the files changed against its first parent.
- Moving a ref is "a compare-and-swap on a single Spanner row", separate from changeset creation. Refs are discouraged except for main and release branches.
- Accepting changesets involves "no locking or cross-server coordination". The team could not "effectively replicate the industry standard of a global repository lock on the merge queue".
- No conventional merge queue: they "invested in post-merge and pre-deploy automated testing" and let conflicts happen, because Claude fixes them.
- "Scion's protocol is centered around sparse checkouts"; the working copy is optional, and CI can ask the server for diffs.
- Scale: about 2,500 merges a day in May 2026, over 30,000 by September; main history near 100 GB; five months, seven people.
- "by no means a suitable replacement for git for general use"; only for "truly enormous monorepos".

The parent session's four-word summary (server-side merges, CAS refs, no global lock, sparse workspaces) matches the article. One point matters for P3 and P4: the article does not build a merge queue; it gates after the merge. This layer does the opposite, on purpose, because a 16 GB machine cannot afford to find a red `main` after the fact, and because Zach's standing rule keeps him as the only one who moves `main`. Section 9 records that provenance for the tool's README.

### 1.7 Side effects of this scoping session

Two temporary worktrees were added in the session scratchpad for the timings in 1.3 and removed at once. `git worktree prune` then ran and dropped the one prunable entry (a sibling session's trial whose directory was already gone). `.git/config` was not changed: `extensions.worktreeConfig` was already `true` at 01:23. About 4.9 GB of APFS clones were made and deleted in the scratchpad; `df` did not move. No file outside `docs/incomplete/landing-layer/` was written.

## 2. What this is, and what this is not

### What this is

A Go module with one binary, `wk`, that runs as a CLI and, for P2 to P4, as a daemon listening on a unix socket on this machine. It sits on top of git in `/Users/zachshort/Projects/ezhomesteading` and on top of Claude Code's hooks. Four phases, in order, each useful without the next:

- P1, cheap workspaces: `wk new <row>` makes a worktree in seconds and clones `node_modules`, `web/.next` and copies the three untracked files from a template; `wk gc` removes worktrees whose branch is landed and whose tree is clean.
- P2, path leases: the daemon holds exclusive write leases per workspace, seeded from the board row's `Files it owns`; a `PreToolUse` hook refuses an edit to a path another workspace leases and names the holder. The four composition roots get a stricter rule.
- P3, landing: agents never run `git commit` or `git merge`; `wk submit` sends a tree hash, a base commit and a message to a queue; the daemon predicts each merge against the current tip with `merge-tree --write-tree`, applies with `commit-tree` and a compare-and-swap `update-ref`, and refuses on a mismatch. Zach stays the only one who moves `main`. (Last sentence superseded 2026-10-16 by D14 and D20: the daemon fast-forwards `main` after a green batch gate and pushes it.)
- P4, one gate run per batch: the daemon gates the predicted merged tree once per batch in one trial workspace, with the CI placeholder env, and reports per changeset which gate failed.

Also in scope: a README that says which ideas come from the article and which are local; a self-test in the style of `selftest.sh`; a launchd plist or an on-demand start for the daemon; a config file outside the repository for the repo-specific rules.

### What this is not

- Not a replacement for git storage. Objects, packs, refs and worktrees stay git's. The 1 GB `.git` and 5,519 files are not the problem.
- Not a virtual filesystem. Files are real files; clones are APFS clones.
- Not a server beyond localhost. One unix socket, one machine, one user. No TCP, no auth, no remote.
- Not an edit to any tracked rule file of the repository: `CLAUDE.md`, `docs/agent-practices.md`, `docs/conventions*.md`, `.claude/rules/`, `.claude/settings.json` (tracked), `.github/workflows/ci.yml`, `.golangci.yml`, any ratchet file. The hook registration goes in a file the repository does not track (Q6).
- Not a change to Zach's commit rule. No process but one Zach runs by hand writes a commit or moves `main`. Where the brief says "the daemon applies with commit-tree", section 4 P3 option M turns that into the owner's command, as `land.sh` already does. (Superseded 2026-10-16 by D14, D20 and workflow refinement D7: the commit rule is reworded, the daemon writes landing commits, fast-forwards `main` and pushes it. What survives: no commit carries a trailer or a "Generated with" line, and no agent commits on `main` directly.)
- Not a merge queue in the article's sense of replacing review. Review stays `/delegate`'s auditor and Zach's walk.
- Not a replacement for `/land-worktrees`. The skill keeps the hard cases (stray worktrees, conflicts resolved by hand, real work in a dead worktree). Q13 asks whether to fold it later.
- Not CI. CI on GitHub keeps every job, including the DB suites this machine cannot run.
- Not a scheduler for agents, a board tool, or a replacement for `PASSOFF.md`.
- Not Windows or Linux. `clonefile(2)` is APFS only; the tool may refuse to run elsewhere.

## 3. The shape, in one paragraph per phase

P1. `wk new <row>` reads the row number, makes `git worktree add -b wk/<row> .claude/worktrees/wk-<row> main` (or the sparse variant), clones the template's `node_modules` and `web/.next` with one `clonefile` each, copies `web/.env`, `backend/.env`, `native/.env`, `web/next-env.d.ts`, `native/expo-env.d.ts` from the template, runs `bun install --frozen-lockfile` (a no-op when the lockfile matches), and prints the path. The template is a worktree the tool owns and refreshes (option C). `wk gc` lists worktrees whose branch is absorbed by `main` (the `is_absorbed` test from `lib.sh`) and whose tree is clean, skips locked ones, ones with a process inside, and ones changed in the quiet window, and removes them only under `--apply`. (D9, 2026-10-16: the daemon runs `--apply` on a schedule, dial `gc.apply_interval_minutes`; a session may still run the dry run.)

P2. `wk up` starts the daemon. `wk claim <row>` in a workspace records row, workspace path and session, parses the row's `Files it owns` cell into path and directory leases, prints what it could not parse, and refuses when a lease is held elsewhere, naming the holder. A `PreToolUse` hook on `Edit`, `Write`, `MultiEdit` and `NotebookEdit` resolves the path against `cwd`, asks the daemon over the socket, and denies with the holder's row and workspace when another workspace leases it. The composition roots have a rule of their own (option L).

P3. `wk submit -m "<message>"` in a workspace builds the changeset's tree from the workspace (tracked changes and untracked non-ignored files, through the workspace's own index, which no other session shares), records base commit, tree, message, row and leases in an append-only queue file, and returns a changeset id. The daemon predicts the merge of each queued changeset onto the current landing tip with `merge-tree --write-tree --merge-base=<base>`, in queue order, and either records the predicted tree or marks the changeset refused with the conflicting paths. `wk sync` brings a workspace up to the landing tip without a commit: merge-tree of the workspace tree against the tip, `read-tree -u -m` of the result, branch ref moved by `update-ref` with the old value as the guard. `wk land` prints the plan; `wk land --apply --expect <hash>` is Zach's command and is the only thing that runs `commit-tree` and `update-ref` on the landing ref. Moving `main` is the second block, a `--ff-only` merge in the main checkout, as `land.sh` prints today. (Superseded 2026-10-16 by D14 and D20: the daemon runs `commit-tree` and `update-ref` on `wk/landing` itself, fast-forwards `main` after a green P4 batch gate, and pushes `main`. The dry run `wk land` stays; `--apply` becomes the daemon's path and a manual override.)

P4. The daemon keeps one trial workspace (a P1 workspace) and, when the queue has predicted changesets and the trial is idle, takes a batch, `read-tree -u --reset`s the batch's final tree into the trial, runs the gates in a fixed order with the CI placeholder env and isolated caches, and records pass, fail or skip per gate. On a failure it attributes the gate's file paths to changesets through their diffs; when attribution is not unique, it re-gates the batch minus the last changeset and repeats. `wk status` shows the queue with each changeset's state: queued, predicted, refused, gated, failed (with gate and reason), landed.

## 4. Options, with the strongest argument against each

Each option carries a recommendation. The recommendation is a proposal; the decision is Zach's, in section 7.

### P1 options

A. How workspaces are created.

- A1. `wk new` is a command Zach or a session runs; sessions then start with the workspace as their folder (`use_worktree: false`, cwd set). For: no dependency on an unverified hook. Against: every worktree Claude Code makes by itself (`EnterWorktree`, `isolation: "worktree"`, the desktop app's worktree sessions) stays cold, which is the 20 worktrees on disk today.
- A2. A `WorktreeCreate` hook calls `wk new`, so every worktree Claude Code makes is warm. For: it covers the path that produces the 42 GB. Against: the hook's input contract was not readable on 2026-10-16; the branch and directory names come from Claude Code, not from a row; a hook that fails leaves the session with no worktree.
- A3. Both: the CLI is the implementation, the hook is one caller, and the hook falls back to a plain worktree when `wk` fails. Recommended, with the hook contract verified before P1 is planned.

B. What is cloned from the template.

- B1. `node_modules`, the whole `web/.next`, and the five untracked files (`web/.env`, `backend/.env`, `native/.env`, `web/next-env.d.ts`, `native/expo-env.d.ts`). For: `.next/types` makes `tsc` pass without a build, and this is what the existing worktrees already carry. Against: a stale `.next/server` from another tree sits in the workspace until the next build overwrites it, and a `.next` cloned mid-build carries half-written output.
- B2. `node_modules`, `web/.next/cache` only, the five files, then `bunx next typegen`. For: nothing from another tree's build output is ever reused. Against: one more step of a few seconds per workspace.
- Recommended: B2. The cost is small and it removes one way a workspace can lie.

C. Where the template comes from.

- C1. The main checkout's live directories. For: nothing to maintain. Against: the main checkout is a dirty workspace (69 entries) whose `node_modules` and `.next` may be mid-install or mid-build at the moment of the clone; the snapshot is then inconsistent and nothing says so.
- C2. A template worktree the tool owns (`.claude/worktrees/wk-template`), kept at `main`, refreshed by `wk template refresh` (checkout `main`, `bun install --frozen-lockfile`, `next build` with the placeholder env), never used for work, and refreshed only when no clone is in progress. For: clones are always of a quiet, complete tree. Against: one more worktree (about 0.5 GB real plus its own `node_modules`, which can itself be a clone), and a refresh that must run when `bun.lock` changes on `main`.
- Recommended: C2, refreshed on a `bun.lock` hash change and on demand.

D. Sparse checkout of `docs/incomplete`.

- D1. Full checkout by default (2.2 s, 478 MB real). For: no gate or grep misses a file; a row whose `Files it owns` names a doc folder needs nothing extra. Against: 440 MB of real disk per workspace that is almost never read.
- D2. Sparse by default, excluding `docs/incomplete` and adding back any path the row's cell names (0.35 s, 52 MB). For: meets the under-5-s target and cuts real disk by nine tenths. Against: `08-verify`, `madge` and the parity check do not read `docs/`, but a session that greps the repository for a design decision finds nothing and does not know why; `git status` and `git diff` behave normally, which hides the gap.
- D3. Full checkout, and a reserved seam: clone the template's working tree with `clonefile`, then let `git checkout <commit>` rewrite only the delta. Untested; expected under 1 s and near zero real disk for tracked files as well. Against: the index of the new worktree must be made to believe the files are fresh, which is stat-cache work that can be wrong silently.
- Recommended: D1 now, D2 as a flag, D3 measured in P1 and built only if the measurement holds.

E. What `wk gc` decides by itself.

- E1. It implements the four-way sort of `plan.sh` (`base`, `gone`, `past`, `new`) in Go. For: one tool. Against: it duplicates 300 lines of tested bash and its self-test, and any divergence between the two is the kind of error nobody sees.
- E2. It handles only the trivial case: branch absorbed by `main` and every non-ignored path clean; everything else is listed as "kept, run /land-worktrees". For: small, and the skill keeps the judgment. Against: the common leftover case (a worktree with a stray `.md` that equals what `main` holds) stays for the skill.
- Recommended: E2 in P1; Q13 revisits folding after P4.

F. Who applies `gc`. Auto mode refuses worktree removal and `land.sh` is Zach's. `wk gc` is a dry run by default and `--apply` is a command Zach runs; a session may run the dry run. Recommended as stated; the alternative (agents remove) is what auto mode already refuses. (Decided otherwise 2026-10-16, D9: the daemon runs `--apply` on a schedule. The auto-mode classifier sees `Bash` calls from a session, not the daemon, so the refusal it applies to agents does not apply here.)

### P2 options

G. Where leases come from.

- G1. Parsed from the board: the daemon reads `PASSOFF.md`, takes every `IN FLIGHT` row's `Files it owns` cell, and turns paths and globs into leases. For: no new ritual. Against: cells are prose as often as paths (rows 76, 77, 83), so the parse is lossy, and a lossy parse that stays quiet is a lease that does not exist.
- G2. Explicit: `wk claim <row>` parses the cell, prints the parts it could not read, and takes leases only for what it read; `wk lease add <path>` adds the rest. For: every lease is one a session saw. Against: a session that skips `claim` has no leases, which is today.
- G3. G2, plus the hook refuses any edit from a workspace that has not claimed a row. For: no workspace edits without leases. Against: a quick read-only session that writes one note is refused until it claims.
- Recommended: G3, with a `wk claim --none` for sessions that only read.

H. The identity of a holder. Keyed by workspace path, which the hook learns from `cwd` and which a subagent inside the same worktree shares; the `session_id` is recorded for display. The alternative, keying by session, makes a builder and its auditor in one worktree fight each other. Recommended as stated.

I. What the hook covers.

- I1. `Edit`, `Write`, `MultiEdit`, `NotebookEdit`. For: exact paths in `tool_input`. Against: `sed -i`, `tee`, `>` and heredocs in `Bash` are not seen, and auto mode steers sessions toward `Bash` for edits.
- I2. I1 plus a `Bash` matcher that walks the command's words the way the commit guard does and refuses the obvious write spellings (`sed -i`, `tee`, `> path`, `>> path`, `cp`, `mv`, `rm`, `python -c` with `open(..., "w")`) aimed at a leased path. For: closes most of the gap. Against: the word walk is partial by nature, and the commit guard's own history shows each spelling it misses becomes a bug report; a false refusal on a harmless command costs one message, a false pass costs a collision.
- Recommended: I2, failing closed on the spellings it recognises and saying plainly in the README that `Bash` coverage is partial.

J. When the daemon is down. Fail closed (every edit refused with "start the daemon: wk up") or fail open (edits pass, no leases). The commit guard fails closed. Fail open is the silent failure. Recommended: fail closed, with `wk` starting the daemon on demand so the case is rare.

K. Lease granularity and conflict rule. A lease is an exact path or a directory prefix; a glob from the cell is expanded against `git ls-files` at claim time. Leases are exclusive for writes; reads are free. Two workspaces asking for overlapping leases: the first holder keeps it, the second `claim` fails and names the holder, so the collision is found at claim time, not at landing. A lease lives until `wk release`, `wk submit` lands it, or `wk gc` removes the workspace. No TTL by default (Q10 dial). Recommended as stated.

L. The composition roots (`backend/routes/routes.go`, `backend/main.go`, `HANDOFF.md`, `PASSOFF.md`).

- L1. Single writer: one workspace at a time may hold a lease on each; the second waits or asks. For: the 2026-08-18 `routes.go` case cannot recur. Against: every backend row touches `routes.go`, so rows serialize on a 629-line file for a one-line route addition.
- L2. Insert-only for the two ledger files: an `Edit` is allowed only when `new_string` contains `old_string` (an insertion), a `Write` only when every existing line survives in order; `routes.go` and `main.go` stay L1. For: two sessions can append steps and rows at once, and nobody can delete another session's line. Against: a status cell change on the board (`OPEN` to `IN FLIGHT`) is an in-place edit and would be refused; the rule needs an exception for the row the workspace claimed.
- L3. L2 with that exception: in-place edits are allowed inside the claimed row's own table line and its own `### <row>.` section, and inside a ledger step the workspace wrote. For: matches what sessions do today. Against: the check needs to find row and step boundaries in markdown, which is more parsing than the rest of P2.
- Recommended: L1 for the Go roots, L3 for the ledger files, and the P3 landing (tree-level, no shared index) is what finally removes the sweep-at-commit case.

M. State and transport. Unix socket at `~/.wk/<repo-hash>.sock`, newline-delimited JSON; state in one JSON file rewritten by atomic rename, held in memory, the daemon the only writer. SQLite would need cgo or a pure-Go driver for tens of rows. Recommended as stated.

### P3 options

N. Who writes the commits and who moves `main`. The brief says the daemon applies with `commit-tree` and CAS `update-ref` and Zach stays the only one who moves `main`.

- N1. The daemon writes landing commits onto a branch `wk/landing` continuously with `commit-tree` and `update-ref <ref> <new> <old>`; Zach moves `main` with one `--ff-only` merge. For: closest to the brief and the article. Against: a daemon that writes commits is a process that commits, which the commit rule reserves for Zach; the guard would not see it (the daemon is not a `Bash` call), so the only thing keeping the rule is the daemon's own code.
- N2. The daemon predicts, gates and prepares; nothing it does writes a ref. `wk land --apply --expect <hash>` is Zach's command and is the only code path that runs `commit-tree` and `update-ref`, followed by the `--ff-only` merge of `main`. The dry run and the apply are the two commands the brief asks for. For: identical to `land.sh`'s contract, which Zach approved and ran on 2026-10-03 and 2026-10-06; the guard's spirit holds by construction. Against: nothing lands while Zach is away, so a long overnight run queues and does not land; the queue's predictions go stale as nothing moves, which is harmless because the prediction is re-done at apply time.
- N3. N2 for `main`, N1 for a landing branch that is never `main`: the daemon may write to `wk/landing` and Zach fast-forwards `main` to it. For: an overnight run lands onto the landing branch and is gated as one tree by morning. Against: the branch is a commit stream Zach did not make, and the two blocks become "inspect the landing branch, then fast-forward", which is the integration-worktree pattern of 2026-10-14 with the agent writing the merges.
- Recommended: N2. If overnight landing matters more than the rule, N3 is the fallback and needs Zach's explicit yes, because it changes who writes commits. (Decided 2026-10-16, D14: N1, with the daemon also fast-forwarding `main`. Zach's explicit yes is in the Answers section.)

O. What a changeset is. A tree (from the workspace's own index), its base commit (the tip the workspace was synced to), a message, the row, the leases held, and the session. Not a commit: the workspace never commits. The queue file is append-only JSONL, which is the article's write-ahead log in the smallest form. Recommended as stated.

P. Prediction mismatch. A changeset whose merge-tree result has conflicts, or whose base is behind the tip and whose re-prediction conflicts, is marked refused with the conflicting paths; the session runs `wk sync`, fixes, and resubmits. With P2 leases the conflict set is empty by construction unless two rows leased overlapping paths through prose the parser missed, so a refusal is also a lease-parser bug report. Recommended as stated.

Q. Message and attribution. `row <n>: <message>` in lowercase, no trailer, author as `git config` sets it, which is Zach, as every commit here is. Recommended as stated, per the commits rule.

R. Keeping the workspace's HEAD. Workspaces keep a branch (`wk/<row>`), moved by `update-ref` with the old value as guard, so the desktop app's branch-aware features keep working; detached HEADs are not used. `wk sync` writes conflict markers into the working tree on conflict and never commits. Recommended as stated.

### P4 options

S. Gate set and order. The `CLAUDE.md` list minus what needs a database (`sqlgate`, the DB suites), in CI's order, with the placeholder env, `bunx next typegen`, the `expo-env.d.ts` line, and `GOCACHE` plus `GOLANGCI_LINT_CACHE` set to fresh directories per trial. A skipped gate is recorded as `skip` with the reason, as `trial.sh --record` does. Recommended as stated.

T. Attribution of a red gate to a changeset.

- T1. Gate the batch; on red, drop the last changeset and re-gate, repeat. For: always correct. Against: up to k extra gate runs of the slowest gate (`next build`, 39 s clean, longer on this machine under load).
- T2. Gate each changeset alone on the tip first. Against: n runs; defeats the phase.
- T3. Gate the batch; on red, map the file paths in the gate's output (`tsc`, eslint, `go build`, `go vet`, madge all print paths) to the changesets whose diffs touch them; when exactly one matches, blame it; otherwise fall back to T1. For: usually one run plus one. Against: a path-less failure (a vitest assertion, `08-verify`'s count ratchet, the bare `Failed to compile.`) always falls back.
- Recommended: T3.

U. Batch policy. Idle-driven: when the trial is idle and the queue holds predicted changesets, take up to N of them (dial, default 6) in queue order; a changeset refused by prediction is skipped, not blocking. A failed changeset is marked failed and the batch's others re-gate without it, so one red row does not hold the rest. Recommended as stated.

V. Machine limits. One `next build` at a time machine-wide, enforced by the daemon, because two over one `.next` print a bare failure; `tsc` runs may run in parallel up to a dial; gating pauses while free memory is under a dial. The daemon cannot stop sessions from running gates themselves in their own workspaces; the README says that P4 is only cheaper if sessions stop doing that, and the pass-off prompts say so. Recommended as stated.

### Cross-cutting options

W. Where the code lives.

- W1. A new repository, `~/Projects/landing-layer` (or the chosen name), its own `go.mod`, its own README and self-test. Repo-specific config in `~/.wk/<repo>.toml`. For: the portfolio angle, nothing tracked in ezhomesteading changes, and the tool's gates do not run inside the app's gates. Against: two repositories to keep in step when the lease rules change.
- W2. Under `~/.claude/skills/wk/` beside `land-worktrees`. For: where the shell version lives. Against: a Go module in the skills folder has no CI and no portfolio page.
- W3. Inside ezhomesteading at `tools/wk/`. Against: `CLAUDE.md`'s directory map has no row for it, the repo's lint and conventions would apply to a tool that is not the product, and the task forbids editing the rule files that would need a line.
- Recommended: W1.

X. Model per phase, by the discriminator in the routing rule (can the failure be silent?). P1: Opus; a bad workspace fails loudly at the first gate. P2: Opus for the CLI and the hook, with a Fable review of the lease matcher and the `Bash` word walk, because a pass that should be a refusal is quiet until landing. P3: Fable; a wrong tree on the landing ref compiles, passes every gate and is wrong. P4: Opus, with the attribution logic reviewed; a wrong attribution blames a row loudly. Recommended as stated; Zach assigns.

Y. The main checkout's role. Today sessions edit it directly (69 entries). Under the layer, either it is a workspace like any other (it claims rows and submits), or it is reserved for Zach and for landing, and `wk submit` refuses to run there. The second removes the whole class of "a dirty file in the main checkout blocks the fast-forward" (`land.sh` prints such blockers today). Recommended: reserved, with sessions moved to `wk new` workspaces; this is the one change in Zach's own habit the layer asks for.

Z. Daemon lifecycle. Started on demand by any `wk` command when the socket is missing, kept alive by an optional launchd agent, stopped by `wk down`; state survives restarts through the JSON file. Recommended as stated.

## 5. Dials

Every number the design leaves open, destined for `~/.wk/<repo>.toml`, not for a constant written twice.

| Dial | Default | Why this value |
|---|---|---|
| `template.path` | `.claude/worktrees/wk-template` | inside the ignored worktrees folder, visible to `git worktree list` |
| `template.refresh_on` | `bun.lock` blob hash change on `main`, or `wk template refresh` | the only change that invalidates `node_modules` |
| `workspace.dir` | `.claude/worktrees/wk-<row>` | beside Claude Code's own |
| `workspace.branch` | `wk/<row>` | one namespace, easy to list and to `gc` |
| `workspace.sparse` | `false` | option D1 |
| `workspace.sparse_exclude` | `docs/incomplete` | the 440 MB |
| `gc.quiet_minutes` | 60 | `plan.sh`'s default |
| `lease.require_claim` | `true` | option G3 |
| `lease.ttl_minutes` | 0 (none) | leases end at release, land or gc |
| `lease.bash_guard` | `true` | option I2 |
| `lease.fail_closed` | `true` | option J |
| `roots.single_writer` | `backend/routes/routes.go`, `backend/main.go` | option L1 |
| `roots.insert_only` | `HANDOFF.md`, `PASSOFF.md` | option L3 |
| `landing.ref` | `refs/heads/wk/landing` | a visible branch |
| `queue.path` | `~/.wk/<repo>/queue.jsonl` | outside the repo, append-only |
| `batch.max` | 6 | about the size of one overnight run's signed-off rows |
| `batch.idle_seconds` | 120 | lets a burst of submits form one batch |
| `gates.parallel_tsc` | 2 | two `tsc` runs fit beside one `next build` in 16 GB |
| `gates.next_build_concurrency` | 1 | two over one `.next` print a bare failure |
| `gates.min_free_mb` | 2048 | gating pauses under this |
| `gates.env` | the six CI placeholder values | `ci.yml` web job |
| `hook.timeout_ms` | 200 | a slow hook slows every edit of every agent |
| `daemon.socket` | `~/.wk/<repo-hash>.sock` | one per repository |
| `gc.apply_interval_minutes` | 60 | added at Stage 3 by D9; the daemon removes trivial-case worktrees on this schedule, only after the quiet window |
| `landing.ff_main` | `true` | added at Stage 3 by D14; the daemon fast-forwards `main` to `wk/landing` after a green batch gate |
| `landing.push` | `true` | added at Stage 3 by D20; the daemon pushes `main` after the fast-forward |

## 6. Hazards this work walks into

Filtered from agent memory and `docs/agent-practices.md`'s appendix to what this tool touches.

- The main checkout is dirty (69 entries) and shared. A `--ff-only` merge of `main` is refused when a dirty file would be overwritten; `land.sh` lists these as blockers. Option Y is the answer; until then `wk land` must print the blockers the way `land.sh` does.
- `git worktree prune` drops entries whose directory is missing at that moment. A workspace on a volume that is briefly unmounted, or a trial being rebuilt, would be pruned. `wk gc` must never prune while a `wk new` or a trial rebuild is in flight.
- A session that stands in a worktree (`cd`) blocks its removal; the shell's cwd also resets between calls, so a relative path can land in the wrong tree. `wk` takes absolute paths only and refuses relative ones.
- A directory `clonefile` of a tree that another process is writing yields a consistent directory but inconsistent contents. The template must be quiet during a clone (option C2), and `wk new` must refuse to clone from a template whose refresh lock is held.
- `du` on a clone reports the full size; only `df` and `wk status` tell the truth. The README must say so, or the first `du` after P1 reads as a failure.
- `bun install --frozen-lockfile` on a cloned `node_modules` rewrites only changed packages, which breaks the clone for those files only. Fine, but a lockfile that drifts far from the template means a slow install; the template refresh dial exists for that.
- The golangci-lint cache is shared across checkouts and serves findings keyed to other trees; `lintgate` in the trial needs `GOLANGCI_LINT_CACHE=$(mktemp -d)` and a fresh `GOCACHE`, or it reports 151 forbidigo findings from another worktree. A worktree that exists at all turns `lintgate` red in the main checkout; the trial must not be where the main checkout's `lintgate` runs.
- Two `next build` runs over one `.next` print `Failed to compile.` with no file; the daemon serializes builds, but a session running `bun run build` in the main checkout at the same time still collides with a build there. The daemon only ever builds in the trial.
- `next build` type-checks the working tree, so a trial mid-`read-tree` must not be gated; the daemon holds the trial lock from `read-tree` to the last gate.
- Every gate reads the working tree, not HEAD. The trial holds the predicted tree, which is exactly what lands (the `land.sh` property); nothing else may be gated as proof.
- `go test` prints `ok` with every DB suite skipped, and `sqlgate` errors without `EZH_TEST_DB`. Both are recorded as `skip`, never as `pass`, and `wk status` shows the gap.
- The `PreToolUse` hook input's `file_path` is raw: it may be relative, may hold `..`, may be a symlink into another worktree. The hook resolves against `cwd`, cleans the path, and refuses what it cannot resolve to a workspace.
- `~/.claude/settings.json` is written by personal-config's wizard. The hook registration goes in `.claude/settings.local.json` (untracked, empty today) or in a file `wk` manages under `~/.claude/`; Q6 decides. Nothing tracked changes.
- Claude Code's own worktree flow (`EnterWorktree`, `isolation: "worktree"`, the desktop app) bypasses `wk new` unless the `WorktreeCreate` hook is wired (option A2); until it is, those worktrees stay cold and `wk gc` still sees them.
- The guard does not see `commit-tree` or `update-ref`. The rule that only Zach's own command writes a commit lives in `wk`'s code and in its self-test, not in the guard. A test that proves the daemon process never calls either is part of P3's done-when.
- The `WorktreeCreate` hook contract is unverified on 2026-10-16. P1's plan must start by reading the live Claude Code version's documentation for it, and A1 must work without it.
- A refusal message that reaches the model must say what to do next (the holder's row and workspace, `wk sync`, `wk up`); the commit guard's wording is the precedent.
- Reading or printing a `.env` file is refused by the auto-mode classifier; `wk new` copies the three env files inside one process and never prints them. The daemon never needs them: gates use the placeholder env.

## 7. Decisions, D1 to D20, ratified 2026-10-16

Every decision below was given by Zach in chat on 2026-10-16 and is quoted or paraphrased in the "Answers" section at the end of this file, which is the citation for each. D1 to D18 answer the eighteen GATE 1 questions, kept below the decisions as asked. D19 and D20 record the two intent answers that changed defaults. Each decision carries the decision, the defense, what it supersedes, and the date.

- **D1. Build it, in order P1, P2, P3, P4.** Yes, P1 first. Defense: P1 alone pays back the 42 GB of worktrees and the fresh-worktree gate gaps (section 1.2); P2 pays back before P3 exists; each phase is useful without the next (section 2). Supersedes: nothing. 2026-10-16.
- **D2. Name.** Binary `wk`, repository `landing-layer`. Defense: short to type, one namespace for branches (`wk/<row>`) and workspaces (`wk-<row>`); `scion-lite` would trade on Anthropic's name, which the README must not do (section 9). Supersedes: nothing. 2026-10-16.
- **D3. Where the code lives.** W1: a new repository at `~/Projects/landing-layer`, its own `go.mod`, README and self-test; per-repository config in `~/.wk/<repo>.toml`. Defense: nothing tracked in ezhomesteading changes, the tool's gates do not run inside the app's gates, and D19 asks for use across several projects, which W2 (a skills folder) and W3 (inside ezhomesteading) cannot give. Supersedes: nothing. 2026-10-16.
- **D4. Workspace creation.** A3: the CLI is the implementation, a `WorktreeCreate` hook is one caller, and the hook falls back to a plain worktree when `wk` fails; the hook contract is verified before P1 is planned and A1 must work alone. Defense: the desktop app's worktree sessions are what produce the 20 cold worktrees today (section 1.2); a CLI alone leaves that path cold. Supersedes: nothing. 2026-10-16.
- **D5. What is cloned.** B2: `node_modules`, `web/.next/cache`, the five untracked files, then `bunx next typegen`. Defense: nothing from another tree's build output is reused, so a workspace cannot carry a stale `.next/server`; the cost is a few seconds per workspace. Supersedes: nothing. 2026-10-16.
- **D6. Where hooks register.** `.claude/settings.local.json` in the repository (untracked), written by `wk hooks install`; never `~/.claude/settings.json` (personal-config's) and never the tracked `.claude/settings.json`. Defense: no tracked rule file changes (section 2), and personal-config's wizard rewrites the home file. Supersedes: nothing. 2026-10-16.
- **D7. Template source.** C2: a tool-owned template worktree at `.claude/worktrees/wk-template`, kept at `main`, refreshed on a `bun.lock` blob-hash change and by `wk template refresh`, never used for work, never cloned while its refresh lock is held. Defense: the main checkout is dirty and may be mid-install or mid-build at the moment of a clone (option C1); a directory `clonefile` of a tree being written is inconsistent and silent (section 6). Supersedes: nothing. 2026-10-16.
- **D8. Sparse checkout.** D1: full checkout by default (2.2 s, 478 MB real, section 1.3); `--sparse` as a flag excluding `docs/incomplete`; D3 (clone the working tree, then `git checkout` the delta) measured in P1 and built only if the measurement holds. Defense: a sparse workspace that greps for a design decision finds nothing and does not know why; the gate target of under 8 s full is met without it. Supersedes: nothing. 2026-10-16.
- **D9. `gc` scope and who applies it.** E2: `wk gc` handles only the trivial case (branch absorbed by `main` by the `is_absorbed` test, every non-ignored path clean, not locked, no process inside, unchanged for the quiet window) and lists everything else as "kept, run /land-worktrees". The dry run is for everyone. `--apply` is run by the daemon on a schedule (dial `gc.apply_interval_minutes`, default 60), not by Zach. Defense: Zach's intent answer, "Everything lands", to "how much should run with no command from you" (Answers); the auto-mode classifier refuses worktree removal by a session's `Bash` call, which the daemon is not. Supersedes: section 4 option F's "`--apply` is a command Zach runs" and the scope's default for Q9; the E2 scope half survives. 2026-10-16.
- **D10. Lease source and strictness.** G3: `wk claim <row>` parses the row's `Files it owns` cell, prints what it could not read, takes leases only for what it read; the hook refuses any edit from a workspace that has not claimed; `wk claim --none` for read-only sessions; no TTL (`lease.ttl_minutes` 0). Defense: a lossy parse that stays quiet is a lease that does not exist (option G1); Zach chose "Refuse, fail closed" over "Warn first" and "Skip P2" (Answers). Supersedes: nothing. 2026-10-16.
- **D11. `Bash` coverage of the lease hook.** I2: `Edit`, `Write`, `MultiEdit`, `NotebookEdit` by exact path, plus a `Bash` matcher that walks the command's words as the commit guard does and refuses the recognised write spellings aimed at a leased path, failing closed on what it recognises. The README says plainly that `Bash` coverage is partial. Defense: auto mode steers sessions to `Bash` for edits, so I1 alone misses most writes; a false refusal costs one message, a false pass costs a collision. Supersedes: nothing. 2026-10-16.
- **D12. Daemon down.** Fail closed: every edit is refused with "start the daemon: wk up", and any `wk` command starts the daemon on demand so the case is rare. Defense: fail open is the silent failure; the commit guard fails closed and is the precedent. Supersedes: nothing. 2026-10-16.
- **D13. Composition-root rule.** L1 (single writer) for `backend/routes/routes.go` and `backend/main.go`; L3 (insert-only, with in-place edits allowed inside the claimed row's own table line and `### <row>.` section and inside a ledger step the workspace wrote) for `HANDOFF.md` and `PASSOFF.md`. Whether `/land-worktrees` folds into `wk` after P4 is a later row. Defense: the 2026-08-18 `routes.go` case cannot recur under L1; L3 matches what sessions do today on the two ledger files; P3's tree-level landing removes the sweep-at-commit case. Note: workflow refinement D2 and D5 split the board into a table and per-item files under `passoff/`, so by the time P2 is built the L3 target is the table plus `passoff/<n>-<slug>.md`; P2's plan re-reads the file layout. Supersedes: nothing. 2026-10-16.
- **D14. Who writes landing commits and moves `main`.** N1, extended: the daemon writes landing commits with `commit-tree` and `update-ref <ref> <new> <old>` onto `refs/heads/wk/landing`, and after a green P4 batch gate fast-forwards `main` to it (dial `landing.ff_main`). Agents never run `git commit` or `git merge` in a workspace. Defense: Zach's two answers of 2026-10-16 (Answers): agents may commit and merge once a project is in production, his objection was attribution only; and "Everything lands". The daemon is the one place every commit carries no trailer and no Claude attribution by construction: the message is `row <n>: <message>`, the author is `git config`'s, which is Zach (option Q). The guard does not see `commit-tree` or `update-ref`, so the rule that only the daemon writes a landing commit lives in `wk`'s code and self-test (section 6). Supersedes: the brief's and section 2's "Zach stays the only one who moves `main`"; section 2's "Not a change to Zach's commit rule"; section 3 P3's "`--apply` is Zach's command"; option N's recommendation of N2; section 9's second divergence from the article. Partial: `wk land` (dry run) and `wk land --apply --expect <hash>` survive as a manual override. Condition: `~/.claude/rules/commits.md` still says never commit on 2026-10-16; P3 is not built before Zach pastes the wording in workflow refinement D7 into that file, so the P3 build row waits on it. 2026-10-16.
- **D15. The main checkout's role.** Reserved for Zach and for landing. Sessions work in `wk new` workspaces; `wk submit` refuses in the main checkout. Defense: Zach's intent answer "Yes, reserve it" (Answers); it removes the class of "a dirty file in the main checkout blocks the fast-forward" that `land.sh` prints as blockers today (section 6). Supersedes: nothing. 2026-10-16.
- **D16. Gate attribution and batch dials.** T3: gate the batch; on red, map the paths in the gate's output to the changesets whose diffs touch them; blame when exactly one matches, else fall back to T1 (drop the last changeset and re-gate). `batch.max` 6, `batch.idle_seconds` 120, one `next build` at a time machine-wide. Defense: usually one run plus one; two `next build` runs over one `.next` print a bare failure (section 6). Supersedes: nothing. 2026-10-16.
- **D17. Models per phase.** P1 Opus; P2 Opus with a Fable review of the lease matcher and the `Bash` word walk; P3 Fable; P4 Opus with the attribution logic reviewed. Defense: the routing rule's discriminator, can the failure be silent: a bad workspace fails at the first gate, a wrong tree on the landing ref compiles and passes every gate, a wrong attribution blames a row loudly (option X). Supersedes: nothing. 2026-10-16.
- **D18. The README's provenance section.** As drafted in section 9, with the article cited by its real URL and the merge-queue divergence stated; the second divergence ("the owner is the only one who writes a commit") is rewritten to say the daemon gates before it lands and writes commits with no attribution. Defense: D19 keeps the portfolio purpose; the README must say which ideas are the article's and which are local. Supersedes: section 9's second divergence bullet, by D14. 2026-10-16.
- **D19. Purpose.** Workflow and portfolio; the tool is for use in several projects, and ezhomesteading is the first and the most important. Zach's words: "my workflow/portfolio because id like to use this in multiple projects but the most important is ezhomesteading". Consequence: per-repository config in `~/.wk/<repo>.toml` is required, nothing ezhomesteading-specific is compiled in, and the composition-root and ledger rules of D13 are config for this repository, not code. Supersedes: nothing; it confirms D2, D3 and D18. 2026-10-16.
- **D20. Who pushes `main`.** The daemon pushes `main` after the fast-forward (dial `landing.push`), so CI and `deploy-backend` run with no step from Zach. Agents never push by hand. Defense: Zach chose "Daemon pushes main" over "You push" and "Agents may push branches" (Answers); `deploy-backend` waits for a green `ci` on `main` (`CLAUDE.local.md`, "Red on HEAD"), so a landed but unpushed `main` ships nothing. Supersedes: the "Never push" sentence of the commit rule, which workflow refinement D7 drops for the landing tool only. 2026-10-16.
- **D21. Name, again.** The project, the repository and the Go module are all `wk`: repository `~/Projects/wk`, module `github.com/zach-short/wk`, binary `wk`. Branches `wk/<row>`, workspaces `wk-<row>`, `~/.wk`, `WK_HOME` and the launchd label `com.github.zach-short.wk.gc.<repo>` keep their names. This folder keeps the name `docs/incomplete/landing-layer/`, and "landing layer" stays the name of the P3 idea in the README. Defense: one name everywhere is simpler than a name and an abbreviation. On 2026-10-16 `wk` had no collision in the macOS system commands, the Homebrew formula names and aliases, the Debian stable package contents, command-not-found.com (Debian, Ubuntu, Arch, Fedora, Alpine, Homebrew), the oh-my-zsh aliases or the zsh builtins; `gp`, `bt`, `jw`, `ap` and `fl` each failed at least one of those checks. "Wicket" was ruled out because wicket.sh sells an authorization layer for AI agents that works with Claude Code. No domain is needed: every two-letter `wk` domain is premium at $219 to $709 a year, and `usewk.dev` is $12.87 if one is wanted later. Supersedes: D2 (repository `landing-layer`) and the path in D3 (`~/Projects/landing-layer`). 2026-10-16.

### Rules that survive unchanged

- No tracked rule file of ezhomesteading changes: `CLAUDE.md`, `docs/agent-practices.md`, `docs/conventions*.md`, `.claude/rules/`, the tracked `.claude/settings.json`, `.github/workflows/ci.yml`, `.golangci.yml`, every ratchet file (section 2).
- The gates are the `CLAUDE.md` list and CI keeps every job, including the DB suites this machine cannot run; a skipped gate is recorded as `skip`, never `pass` (section 6).
- No commit, by any process, carries a `Co-Authored-By:` trailer or a "Generated with" line (`~/.claude/rules/commits.md`, "Attribution"; workflow refinement D7).
- No agent commits on `main` directly; `main` moves only by the landing tool's fast-forward of a gated tree (D14, workflow refinement D7).
- Git storage, refs and worktrees stay git's; files are real files; one unix socket, one machine, one user (section 2).
- `/land-worktrees` keeps the hard cases (stray worktrees, conflicts resolved by hand, real work in a dead worktree) and its `is_absorbed` test is the one `wk gc` uses (option E, D9).
- Review stays `/delegate`'s auditor and Zach's runtime walk (section 2).
- `~/.claude/rules/commits.md` and the standards are Zach's files to edit; an agent proposes wording and never writes them (workflow refinement section 2).

### The questions as asked at GATE 1

Kept for the record; the decisions above answer them by number.

1. Build it at all, and in this order (P1, P2, P3, P4)? Default: yes, P1 first; P1 alone pays back the 42 GB and the fresh-worktree gate gaps, and P2 pays back before P3 exists.
2. Name. Default: binary `wk`, repository `landing-layer`. Alternatives: `land`, `scion-lite` (the second trades on Anthropic's name, which the README should not do).
3. Where the code lives (option W). Default: W1, a new repository `~/Projects/landing-layer`, config in `~/.wk/`.
4. Workspace creation (option A). Default: A3, CLI plus the `WorktreeCreate` hook, with the hook verified first and A1 working alone.
5. What is cloned (option B). Default: B2, `node_modules`, `web/.next/cache`, the five files, then `bunx next typegen`.
6. Where the hooks are registered. Default: `.claude/settings.local.json` in the repository (untracked), written by `wk hooks install`, never `~/.claude/settings.json` (personal-config's) and never the tracked `.claude/settings.json`.
7. Template source (option C). Default: C2, a tool-owned template worktree refreshed on `bun.lock` change.
8. Sparse checkout (option D). Default: D1 full, with `--sparse` as a flag and D3 measured in P1.
9. `gc` scope and who applies it (options E, F). Default: E2 trivial cases only, dry run for everyone, `--apply` for Zach.
10. Lease source and strictness (options G, K, dial `lease.ttl_minutes`). Default: G3 (claim required, `--none` for read-only sessions), no TTL.
11. `Bash` coverage of the lease hook (option I). Default: I2, partial word walk, fail closed on recognised spellings.
12. Daemon down behaviour (option J). Default: fail closed.
13. Composition-root rule (option L). Default: L1 for `routes.go` and `main.go`, L3 for `HANDOFF.md` and `PASSOFF.md`. Whether `/land-worktrees` folds into `wk` after P4 is a later row, not this decision.
14. Who writes landing commits (option N). Default: N2, only `wk land --apply`, run by Zach; N3 only on an explicit yes because it changes who commits.
15. The main checkout's role (option Y). Default: reserved for Zach and landing; sessions work in `wk new` workspaces; `wk submit` refuses in the main checkout.
16. Gate attribution (option T) and batch dials (section 5). Default: T3, `batch.max` 6, `batch.idle_seconds` 120, one `next build` at a time.
17. Models per phase (option X). Default: P1 Opus, P2 Opus with a Fable review of the matcher, P3 Fable, P4 Opus with a review.
18. The portfolio README's provenance section (section 9). Default: as drafted there, with the article cited by its real URL and the merge-queue divergence stated.

## 8. Size, by phase

Rough, from the shape above and from `land-worktrees` (about 1,100 lines of bash with self-test, built over 2026-10-03 to 2026-10-15). Not measured. Each session is one Part 5 context budget.

| Phase | Driver | Go, about | Sessions, about | Done when (the proof a green gate cannot supply) |
|---|---|---|---|---|
| P1 | Opus | 700 lines plus a self-test | 2 to 3 | `wk new` on a cold machine returns in under 8 s full, under 5 s sparse, measured with `/usr/bin/time`; `df` moves by under 600 MB; every CLAUDE.md gate is green in the new workspace with no manual step; `wk gc --apply` run by Zach removes only the 17 absorbed worktrees and keeps the 3 locked ones |
| P2 | Opus, Fable review | 900 lines, one hook script, a self-test | 3 to 4 | two live sessions claim rows 1 and 2 of a fixture board; an edit by session 2 to a path session 1 leases is refused with session 1's row named; a `sed -i` to the same path is refused; `wk down` makes every edit refuse with the start instruction |
| P3 | Fable | 1,000 lines plus a self-test | 4 to 5 | in a throwaway repository, two overlapping changesets land in order with the second refused and resubmitted after `wk sync`; the landing tree equals the predicted tree byte for byte; a test proves the daemon process never calls `commit-tree` or `update-ref`; the two blocks are printed and only Zach's `--apply` moves a ref |
| P4 | Opus, review | 700 lines | 3 to 4 | a batch of three changesets where one breaks `tsc` is gated once plus one re-gate, the broken one is named, the other two land; memory stays under the dial, measured; the skipped DB gates show as `skip` |

Total: about 12 to 16 sessions, which agrees with the parent's two to three weeks at the current pace of four to six sessions a day.

## 9. Provenance, for the tool's README

Ideas taken from Anthropic's "Redesigning source control at Anthropic" (2026-10-13):

- Merges happen away from the client, on the server. Here: the daemon predicts and prepares every merge with `merge-tree --write-tree`; no workspace runs `git merge`.
- A reference moves by compare-and-swap, separately from creating the change. Here: `update-ref <ref> <new> <old>`, git's own CAS, and the tree is written long before the ref moves.
- No global repository lock. Here: predictions run concurrently and only the ref update is serialized by git's CAS; leases are per path, not per repository.
- The unit of work is a changeset in a write-ahead log, not a commit on a branch. Here: a tree hash plus a base and a message, appended to a JSONL queue; the commit is made only at landing.
- Sparse and optional checkouts. Here: the sparse flag, and the single trial workspace that gates for every changeset instead of each workspace gating itself.

Where this tool departs from the article, on purpose:

- The article gates after the merge ("post-merge and pre-deploy automated testing"). This tool gates before, once per batch, because a 16 GB machine cannot afford a red `main` discovered later and because the repository's own rule is that gates are green before a change is done.
- The article discourages refs and lets agents commit on the server. This tool keeps the owner as the only one who writes a commit or moves `main`, which is Zach's standing rule. (Superseded 2026-10-16 by D14, D18 and D20: the daemon writes the landing commits, fast-forwards `main` after the batch gate and pushes it; what stays local is that every commit carries no attribution and a message the tool writes. The README states the divergence as: gates run before landing, not after.)

Local ideas, not from the article:

- APFS directory clones for `node_modules` and build caches (`clonefile(2)`), measured at 4.5 s for 143,199 files.
- Path leases seeded from a board row's `Files it owns` cell, enforced by a Claude Code `PreToolUse` hook; the insert-only rule for the two ledger files.
- The owner's two commands, dry run and apply with a plan hash, inherited from `/land-worktrees`, which also contributed the predicted-merge and trial-worktree mechanics and the four-way sort of dirty files.
- The CI placeholder env for building web with no secrets, from `.github/workflows/ci.yml`.
- Per-changeset attribution of a red gate through the paths in the gate's output.

## 10. Draft board row, not yet appended

The row below is in `PASSOFF.md` shape. It is not on the board. Whoever appends it re-reads the next free number first (`grep -nE '^### [0-9]+' PASSOFF.md | tail -1`; 196 on 2026-10-16) and fills `Files it owns` against every `IN FLIGHT` row at that moment.

Table line:

```
| 196 | Landing layer: a Go CLI and daemon over git for parallel agent workspaces (P1 cheap workspaces, P2 path leases, P3 queued landing, P4 one gate run per batch); design in `docs/incomplete/landing-layer/DESIGN.md`, ratified 2026-10-16 | OPEN | Opus (P1), Fable (P3) | tooling | nothing; next is `PLAN.md` for P1 and GATE 2 | `docs/incomplete/landing-layer/`; `~/Projects/landing-layer` (D3) |
```

Prompt section:

```
### 196. Landing layer: design and plan P1, cheap workspaces

**Model: Opus. Lane tooling. Waits on** nothing; the design was ratified 2026-10-16 (`DESIGN.md` section 7, D1 to D20).
Work in a fresh worktree of this repository for the docs, and in the new repository Q3 names for the code; nothing in this repository's tracked tree changes except `docs/incomplete/landing-layer/`.

You are picking up the landing layer at `docs/incomplete/landing-layer/DESIGN.md`. Read it in full first: section 1 holds every measured number and every citation, section 7 holds Zach's decisions D1 to D20 with their defenses, and nothing in it is to be re-derived or relitigated. Then read `~/.claude/skills/land-worktrees/scripts/lib.sh` and `trial.sh`, which hold the predicted-merge and trial mechanics this tool inherits, and `~/.claude/hooks/personal-config/commit-guard.sh` for the word-walk precedent. Session rules: absolute dates, a citation on every claim, ask Zach in chat in one batch before building anything that is his call, never run `git commit`, `git push`, `git add -A` or `git add .`, print the two commit blocks at hand-back, no attribution lines.

**Why this exists.** Zach runs tens of parallel agents on a 16 GB machine against one repository. Each gets a 2 to 3 GB worktree (42 GB today, 22 worktrees), collides with the others on shared files and on the shared index of the main checkout, and runs the same gates. The layer makes a workspace cost seconds and near zero disk, keeps two agents off one file, lands changes as predicted trees that Zach commits with one command, and gates once per batch.

**What is fixed.** Read these before changing anything; do not relitigate them.
- The measurements in DESIGN.md 1.3: a directory `clonefile` of `node_modules` is 4.5 s, `cp -c` is 23 s; a full checkout is 2.2 s and 478 MB real; sparse without `docs/incomplete` is 0.35 s and 52 MB.
- The template is a tool-owned worktree, never the main checkout (D7); what is cloned is D5; the main checkout is reserved for Zach and landing (D15).
- `wk gc` is a dry run unless `--apply`; the daemon runs `--apply` on a schedule (D9); it removes only worktrees whose branch is absorbed by `main` (the `is_absorbed` test in `lib.sh`: ancestor, or merge-tree is a no-op) and whose non-ignored paths are clean, and never a locked one, one with a process inside, or one changed in the quiet window.
- No tracked rule file of ezhomesteading changes. Hook registration goes in `.claude/settings.local.json` (D6).
- The commit guard does not see `commit-tree` or `update-ref`; the rule that only the daemon writes a landing commit, with no trailer, lives in this tool's code and self-test (D14).
- The tool is for several projects, ezhomesteading first (D19): per-repository config in `~/.wk/<repo>.toml`, nothing repository-specific compiled in.

Do these, in order:
1. **Stage 3 is done** (2026-10-16): `DESIGN.md` holds D1 to D20 and "Rules that survive unchanged". Do not redo it; add an `As built:` note or a new dated `D<n>` if a fact has moved.
2. **Verify the `WorktreeCreate` hook contract** against the installed Claude Code version before planning A3; record the input fields with a citation, or record that A1 stands alone.
3. **Write `PLAN.md`** for P1 only (Stage 4): facts re-verified, phases, done-when with the timing proofs in DESIGN.md section 8, watch-for from section 6, dials from section 5 as config.
4. **Stop at GATE 2.** Phases, order and dials go to Zach before any code.
5. **Record it.** Set this row to `IN FLIGHT` with the folder and the new repository path in `Files it owns`; add a ledger step at the next free number.

Gates: none for this row; it writes docs. The P1 build row that follows names its own.
Subagents: none.

Ask before building: nothing from section 7, every question is answered; only a fact that section 0 of the plan finds moved.
Not in scope, whoever asks: P2 to P4 code; any edit to `CLAUDE.md`, `docs/agent-practices.md`, `.claude/rules/`, `.claude/settings.json`, `ci.yml`; a virtual filesystem; any server beyond localhost; folding `/land-worktrees`.

Hand back: `DESIGN.md` and `PLAN.md` in the folder; the GATE 2 question in chat; the two commit blocks for the folder's files.
```

## Answers

Each answer is recorded here with its date, by question number.

- Q14, answered 2026-10-16 in chat: Zach does not mind an agent committing and merging to `main` or to a testing branch once a project is in production. His objection was attribution only: GitHub listing Claude as a contributor and the `Co-Authored-By` trailer, which he believes is now fixed. He said the commit, merge and push rules can be reworded. Consequence: option N1 (the daemon writes landing commits and may move `main`) is no longer ruled out by the commits rule, and the brief's line "Zach stays the only one who moves main" is superseded by this answer. The written rule in `~/.claude/rules/commits.md` still says never commit; until Zach rewords it, agents follow the file, and the daemon is the one place that can make every commit carry no trailer and no Claude attribution by construction (`commit-tree` with a message the tool writes). The choice between N1, N2 and N3 is therefore open again, with N1 now the simplest; it is folded into the workflow refinement scope (`docs/incomplete/workflow-refinement/SCOPE.md`), where the commit rule itself is being reworded.
- Q14, answered again 2026-10-16 in chat, in the session that collected the section 7 answers: N1. The daemon writes landing commits with `commit-tree` and `update-ref <ref> <new> <old>` onto `wk/landing`, and may fast-forward `main` after a green batch gate (P4). Whether any agent-run process may move `main` is also workflow refinement Q3 (the commit-rule wording, `docs/incomplete/workflow-refinement/DESIGN.md` section 5, then still `SCOPE.md`), open at the time of this answer; while that wording says "never commit on `main`", the daemon stops at `wk/landing` (N3 behaviour) and Zach fast-forwards `main`. The line in section 2, "Zach stays the only one who moves `main`", is superseded.
- Q1 to Q13 and Q15 to Q18, answered 2026-10-16 in chat in the session that collected the answers, after the four intent questions below: "All, as amended", where the one amendment was Q9 (the daemon runs `gc --apply` on a schedule). Recorded as D1 to D18.
- Intent questions, asked and answered 2026-10-16 in the same session before the defaults were confirmed. Zach first asked whether a Fable audit of the document and of how well it fits ezhomesteading and personal-config, with questions about what he is after, should come first, or whether it would reach the same results; the session answered that a re-audit of the options by the same model would mostly repeat the document, and that the questions about intent were the part that would differ, then asked them. Purpose: "i think my workflow/portfolio because id like to use this in multiple projects but the most important is ezhomesteading" (D19). How much runs with no command from Zach after a green batch gate: "Everything lands" (D9, D14). P2 refusal strength: "Refuse, fail closed" (D10, D11, D12). Reserve the main checkout: "Yes, reserve it" (D15).
- Push, asked and answered 2026-10-16 in the same session: "Daemon pushes main" (D20).
- The commit-rule wording that D14 and D20 depend on was accepted the same day; it is recorded in `docs/incomplete/workflow-refinement/DESIGN.md` section 5 and D7.
- Name, asked and answered 2026-10-16 in chat, in a side session on naming: Zach's first condition was that the two-letter command "would not be commonly taken". He chose Wicket with `wk` as the command, dropped it when wicket.sh turned up, and then chose `wk` as the name (D21).
