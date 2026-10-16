# Scope: `wk slim`, disk in the worktrees made before `wk`

Status: RATIFIED 2026-10-16. `wk slim` is not built (S0). Two one-off commands for Zach replace it: a trim of `web/.next/cache` and a pattern-sparse checkout of the old worktrees ("Answers" at the end). Written 2026-10-16 by an Opus 5.5 session started from the session that asked for it. It answers the `wk slim` seam of `PLAN.md` section 4 and the "Owed" line of "As built, phase 3". Sections 1 to 6 are the scope as Zach read it; "Answers" records what he chose. Model check: a scope is Default-tier work (`AGENT-PRACTICES.local.md` Part 4), and this session runs Opus 5.5.

Side effects of this session on disk: two calibration files and a small `git clone --shared` were written to and removed from this session's scratchpad, and a read-only C program was compiled there. One read, `git status` in `nifty-lehmann-452d6e` at about 15:33, let git refresh that worktree's index stat cache (its index mtime is 15:33:02, every other old worktree's is 14:02 or earlier); the index content is unchanged. Later reads used `GIT_OPTIONAL_LOCKS=0`. Nothing else in the repository or in any worktree was changed.

## The short version

The request assumed that each old worktree holds a real `node_modules` of about 1.5 GB, about 37 GB over 17 worktrees, and that cloning the template's tree over it frees that space. The measurement says otherwise. On macOS bun installs every package with `clonefile` from its global cache, so every `node_modules` in this repository, old or new, already shares its blocks with `~/.bun/install/cache`. One 5.3 MB file of `next` has the same APFS clone ID in bun's cache, the template, three old worktrees, two `wk` workspaces and the main checkout, with 68 clones of one physical copy. Replacing an old `node_modules` with a template clone frees 0 bytes.

The 17 old worktrees hold 14,409 MiB (14.1 GiB) that no other file shares. All of it is in two places. The checkout is 478 MiB in each worktree, and 426 MiB of that is `docs/incomplete`. The web build output, `web/.next`, is 758 to 812 MiB in 8 of the 17, and almost all of it is `web/.next/cache`. So the `.next/cache` half of `wk slim` is real: 5,990 MiB in all, or 5,216 MiB once the one worktree that `gc` removes tomorrow is left out. The `node_modules` half, and with it the lockfile question, has nothing to act on.

## 1. What exists, verified 2026-10-16

Read between 15:27 and 15:36 EDT in the main checkout at `65b51149b`. Sizes are MiB (1,048,576 bytes) unless a line says otherwise.

### 1.1 How real bytes were measured

`du` reports a clone at its full size, and a `df` delta needs a deletion, which this scope may not make. So the measure is APFS's own count of unshared bytes: `getattrlist(2)` with `FSOPT_ATTR_CMN_EXTENDED` returns `ATTR_CMNEXT_PRIVATESIZE`, the bytes of a file that no other file shares, together with `ATTR_CMNEXT_CLONEID`, `ATTR_CMNEXT_CLONE_REFCNT` and `ATTR_CMNEXT_EXT_FLAGS` (`EF_MAY_SHARE_BLOCKS`, `EF_SHARES_ALL_BLOCKS`). A read-only C walker (`fts` plus `getattrlist`, about 50 lines, compiled with `/usr/bin/clang` in the session scratchpad) sums them per tree. The flags and attributes are in the SDK headers (`sys/attr.h:560-569`, `sys/stat.h:368`, `:374`).

The method was checked against `df -k /` in the scratchpad, on the same volume, with a file of 512 MiB of random bytes:

| Step | `df` change | Private size the walker reported |
|---|---|---|
| Write the 512 MiB file | 568 MB used (other sessions were writing too) | 512 |
| `cp -c` it to a clone | 0 MB used | 0 for each file, `allocated` 1,027 for the two |
| Delete the clone | 0 MB freed | |
| Delete the original, now the last owner | 514 MB freed | |

A second check: a 5 MiB rewrite inside a 50 MiB clone left 5 MiB private in each of the two files.

One limit. Private size is exact for the question "what does deleting this tree free while every other tree stays". When two trees share blocks only with each other, each reports 0, yet deleting both frees the blocks. That case does not arise for `node_modules` here, because bun's cache holds a reference to every block (section 1.2), so nothing frees while the cache entry exists.

A history check agrees: phase 1's full `wk new` used 521 MB of `df` (`PLAN.md`, "As built, phase 1") for a 478 MiB checkout plus a cloned `node_modules` of 96,147 files, so the clone cost about 40 MB of metadata and no data.

### 1.2 The one-file proof

`node_modules/next/dist/compiled/next-server/app-page-experimental.runtime.dev.js.map`, 5,323,402 bytes, read with `getattrlist` in each tree at 15:31:

| Tree | Inode | Clone ID | Clones of this data | Private |
|---|---|---|---|---|
| `~/.bun/install/cache/next@16.3.8@@@1/` | 103951625 | 103951625 | 68 | 0 |
| `wk-template` | 146089327 | 103951625 | 68 | 0 |
| `agent-a18e6d6ebabbfe17c` (old) | 143016807 | 103951625 | 68 | 0 |
| `agent-a323447ff297d7206` (old) | 142088058 | 103951625 | 68 | 0 |
| `row-195` (old) | 147644001 | 103951625 | 68 | 0 |
| `wk-9902` (`wk new`) | 146382164 | 103951625 | 68 | 0 |
| `burst-0` (hook) | 152809572 | 103951625 | 68 | 0 |
| main checkout | 115565428 | 103951625 | 68 | 0 |

The cache file's clone ID equals its own inode, so the cache holds the original and every tree holds a clone of it. bun 1.2.9 runs here with no `bunfig.toml` in the repository or in `~`, so its backend is the default, which bun's documentation names as `clonefile` on macOS (Context7 `/websites/bun`, "Platform-specific backends").

### 1.3 The 17 worktrees made before `wk`, by real bytes

"Made" is the directory's birth time (`stat -f %SB`). "`du` view" is allocated blocks, which is what `du` and the Fable review counted. "`gc` at 15:29" is `~/go/bin/wk --repo /Users/zachshort/Projects/ezhomesteading gc`, dry run, at 15:29:16.

| Worktree | Made | `du` view | Private | `node_modules` private | `web/.next` private (of it `cache`) | `gc` at 15:29 |
|---|---|---|---|---|---|---|
| `agent-a02d20bbe0ab981c8` | 01:21 | 2,810 | 1,289 | 0 | 810 (774) | kept: absorbed and clean, no stop marker, so it waits 24 hours; removable about 10:43 on 2026-10-17 |
| `agent-a18e6d6ebabbfe17c` | 02:32 | 2,779 | 1,257 | 0 | 778 (742) | kept, run /land-worktrees: 7 changed paths |
| `agent-a2335e7c25d78a156` | 01:21 | 2,000 | 478 | 0 | none | kept: commits `main` does not hold |
| `agent-a323447ff297d7206` | 01:48 | 2,000 | 478 | 0 | none | kept, run /land-worktrees: 18 |
| `agent-a427932828c3648cc` | 02:16 | 2,001 | 479 | 0 | none | kept, run /land-worktrees: 4 |
| `agent-a93d2107c291f21d1` | 02:48 | 2,779 | 1,257 | 0 | 778 (742) | kept, run /land-worktrees: 8 |
| `agent-aaabdb197e0066348` | 01:21 | 2,000 | 478 | 0 | none | kept: commits `main` does not hold |
| `agent-aaf40772c4156377b` | 02:35 | 2,001 | 479 | 0 | none | kept, run /land-worktrees: 2 |
| `agent-ab496235aa1690306` | 02:32 | 2,779 | 1,258 | 0 | 779 (743) | kept, run /land-worktrees: 6 |
| `agent-ac597cbb52da49fe1` | 02:17 | 2,000 | 478 | 0 | none | kept, run /land-worktrees: 7 |
| `agent-ad34f8b06e21b510d` | 02:28 | 2,001 | 479 | 0 | none | kept, run /land-worktrees: 2 |
| `agent-adbbf050d4404acf8` | 01:48 | 2,812 | 1,290 | 0 | 812 (781) | kept: commits `main` does not hold |
| `agent-add71e46821d75ce7` | 01:21 | 2,779 | 1,258 | 0 | 779 (743) | kept: commits `main` does not hold |
| `agent-ae636546177a35039` | 02:17 | 2,778 | 1,256 | 0 | 778 (742) | kept, run /land-worktrees: 15 |
| `agent-afb01f9d2bb9fa56e` | 02:16 | 2,001 | 479 | 0 | none | kept: commits `main` does not hold |
| `nifty-lehmann-452d6e` | 12:09 | 4,074 | 479 | 0 (root tree 143,199 files, `web/node_modules` 121,376 files, both 0) | none | kept: detached HEAD with commits `main` does not hold |
| `row-195` | 13:40 | 2,759 | 1,237 | 0 | 758 (723) | kept: branch `row-195-core-helpers` has commits `main` does not hold |
| **17 worktrees** | | **42,353** | **14,409** | **0** | **6,272 (5,990)** | 1 by `gc` tomorrow, 9 by /land-worktrees, 7 need Zach |

Every `native/node_modules` (70 files, 7 MiB) is 0 private too. In one old worktree, `docs/incomplete` is 426 of the 478 MiB checkout (1,564 files, `agent-a323447ff297d7206`) and `native/assets` is 13.

Grouped by what frees them:

| Group | Worktrees | Private | Of it `web/.next/cache` |
|---|---|---|---|
| `gc` removes it after 24 hours of quiet | 1 | 1,289 | 774 |
| Absorbed but dirty, "run /land-worktrees" | 9 | 7,421 | 2,969 |
| Commits `main` does not hold | 7 | 5,699 | 2,247 |

### 1.4 Controls: trees that `wk` made, and the main checkout

| Tree | `du` view | Private | Note |
|---|---|---|---|
| `wk-template` | 2,795 | 31 | its `web/.next/cache` (765) is 0 private: every workspace clone shares it |
| `wk-9902` (phase 1 sparse `wk new`) | 2,339 | 53 | |
| `burst-0` (hook, pattern-sparse) | 2,344 | 57 | |
| `agent-a4be01d96a23ad438` (hook, 15:12, 25 changed paths) | 2,344 | 57 | its `web/.next` is 0 private |
| `agent-a99035391e1eee7e2` (hook, full, before BD-16) | 2,765 | 479 | the full checkout |
| Main checkout `node_modules` | 2,330 | 0 | 143,199 files |
| Main checkout `web/node_modules` | 1,259 | 0 | 121,376 files |
| Main checkout `web/.next/cache` | 1,559 | 1,559 | real, and the main checkout is Zach's (D15) |
| `~/.bun/install/cache` | 4,246 | 1,992 | 249,746 files in 4,650 entries; the shared 2,254 is the one physical copy behind every `node_modules` |

### 1.5 Other facts

| Claim | Verified state | Citation |
|---|---|---|
| Free disk | 42,734,068 KB available at 15:27:55; 40,636,528 KB at 15:36 (other sessions writing) | `df -k /` |
| Worktrees | 63 entries: the main checkout, 61 directories under `.claude/worktrees/`, one land-worktrees trial in a scratchpad | `git worktree list --porcelain`; `ls .claude/worktrees` |
| Lockfiles | Every one of the 61 worktrees' `bun.lock` is blob `027889f88`, the template's recorded `refresh_blob`. The "lockfile differs" case has 0 members | `git hash-object <wt>/bun.lock`; `~/.wk/ezhomesteading/template.json` |
| `gc` overall | 1 of 63 removable at 15:29 (`agent-a0e4b4b91ae6b9fae`); no process named in any of the 17; `wk-9902` kept for a process inside | `wk gc` dry run, 15:29:16 |
| `gc` schedule | Loaded, `RunAtLoad` false, `StartInterval` 900. The first run started 15:32:30 and ended by 15:43 with "1 of 65 removed" (`agent-a0e4b4b91ae6b9fae`), about 11 minutes. It kept all 17 old worktrees. Outside this scope, for the owner of `gc`: it printed `wk-pool-a1e0cfa8` as "kept, run /land-worktrees" because "its last activity cannot be read" (a pool slot that moved during the run), and `agent-a4be01d96a23ad438` as "changed -1 minutes ago" | `~/.wk/ezhomesteading/gc.log` read at 15:43:41; `launchctl print gui/501/com.github.zach-short.wk.gc.ezhomesteading` |
| The `wk` code a build would start from | Phase 3 is merged into `wk` `main` as `30989b5`, committed in this repository at 15:36 (`77b033a5d`). At 15:27 the merge was still owed | `git -C ~/Projects/wk log --oneline -1 main`; `git log -1 -- docs/incomplete/landing-layer/PLAN.md` |
| Tracked images under `docs/incomplete` | 1,327 files, 419 MiB, in each of three old worktrees checked (`agent-a323447ff297d7206`, `row-195`, `nifty-lehmann-452d6e`) | `git ls-tree -r -l HEAD docs/incomplete`, summed for the eight `workspace.sparse_exclude` extensions |
| `git sparse-checkout set` on a full, dirty checkout | Removes the tracked images, keeps markdown, keeps a changed markdown file, keeps an untracked image, keeps a changed tracked image with the warning "not up to date and were left despite sparse patterns", and exits 0. `git status` then lists only the real changes | a `git clone --shared --no-checkout` of this repository in the session scratchpad, at `65b51149b`, restricted to two `docs/incomplete` folders; removed afterwards |
| Go access to these attributes | `golang.org/x/sys` v0.41.0 (the pin of BD-13) has the `Attrlist` type, `FSOPT_ATTR_CMN_EXTENDED` and `SYS_GETATTRLIST`, and no exported `Getattrlist` function | `grep` of `~/go/pkg/mod/golang.org/x/sys@v0.41.0/unix/*darwin*.go`; `~/Projects/wk/go.mod:9` |
| The seam text this replaces | `PLAN.md` section 4 says "about 2.2 GB each, about 37 GB over the 17 counted by the Fable review". The 2.2 GB and the 37 GB are `du` figures; the real figure is 0 for `node_modules` and 5,990 MiB for `.next/cache` | `PLAN.md:334`; sections 1.2 and 1.3 here |
| No earlier work on this | No archived project mentions `node_modules` sharing, `clonefile` or worktree disk | `grep -i` over `~/Projects/archive/ezhomesteading/INDEX.md` |

## 2. What this is, and what this is not

### What this is

A decision on whether `wk slim` is built, and in what shape, against bytes measured with a method that sees clones. If it is built, it acts on build output only, because that is the only part of an idle worktree that is private, regenerable and not work.

### What this is not

- Not a `node_modules` deduplication. bun already does it (section 1.2). Any option that replaces `node_modules` is listed in section 3 only so it is not proposed again.
- Not landing, committing or discarding the dirty or unlanded worktrees. That is `/land-worktrees` and Zach's call per worktree.
- Not a change to `gc`'s removal rules (E2, BD-19), its 24-hour wait for unmarked subagent worktrees, or the speed of the scheduled run.
- Not the main checkout. Its `web/.next/cache` is 1,559 MiB of real bytes, and the main checkout is Zach's (D15).
- Not the git pack or the screenshots in it (the Fable proposal in "As built, phase 3", Zach's call).
- Not `wk status` and its disk report (P4). The private-size reader would be its seam.
- Not the build semaphore (`wk run --slot build`), which a sibling session scopes.
- Not bun's cache. Its 1,992 MiB of private entries are packages no tree uses; `bun pm cache rm` is a separate, one-line choice.

## 3. Options, with the strongest argument against each

**S0. Do not build `wk slim`. Correct the seam in `PLAN.md` section 4 and let landing free the old worktrees.** Defense: the request's half that was meant to carry the weight frees 0 bytes. The old worktrees are a closed set, because the hook has made every new worktree since phase 3. `/land-worktrees` already exists and frees the whole worktree, checkout and cache together (1,257 to 1,290 MiB with a build, 478 without); the nine absorbed-but-dirty ones hold 7,421 MiB. `gc` removes `agent-a02d20bbe0ab981c8` (1,289) tomorrow with no step from anyone. Against: the seven worktrees with unlanded commits (5,699, of which 2,247 is cache) free nothing until Zach decides each one, which can take days, and storage is the problem today.

**S1. A one-off trim with no code: Zach runs one printed command that deletes `web/.next/cache` in the seven idle old worktrees that `gc` will not take soon.** Defense: 5,216 MiB today for one command. The cache is regenerated by the next `next build` in that worktree (36 s cold against 17 s warm, `PLAN.md` "As built, phase 1"), and it is ignored, so no work is lost. Against: it reaches into worktrees with unlanded work, a resumed session there pays one cold build, and a command run by hand has no process test unless it carries one. A session's `Bash` call is also likely to be refused for it, as worktree removal is, so it must be Zach's command.

**S2. `wk slim` as its own command, narrowed to build output.** Dry run by default, `--apply` to act. For each worktree that is not the main checkout, the template or a pool slot, has no process inside (one `lsof -d cwd` snapshot, as `gc`), and has been quiet for `slim.quiet_minutes` by BD-10's newest-of rule plus the newest mtime under each target, it deletes each `slim.dirs` entry whose private bytes exceed `slim.min_private_mib`, and prints the private bytes before and after. Defense: it covers the future as well as the 17. One `bun run build` in a fresh workspace wrote 0.9 GB (`PLAN.md` phase 3, measurement 3), each old worktree that built kept 758 to 812 MiB of it private, and `gc` keeps every dirty or unlanded worktree with no end date. It reuses `gc`'s tests. Against: today there are seven trees and 5.2 GiB; it adds a second destructive path to the tool beside `gc`; reading private size in Go needs a raw `SYS_GETATTRLIST` call or cgo (section 1.5), and raw syscalls are discouraged on darwin.

**S3. Fold the trim into `gc --apply` as a third verdict, "trimmed", with a dial `gc.trim_dirs`.** Defense: no new command, and the 15-minute schedule is already the thing that acts with no step from Zach (D9). The worktrees it would trim are exactly `gc`'s "kept" set. Against: D9 and E2 kept `gc` to the trivial case on purpose. Trimming inside kept worktrees, some with live but idle sessions, widens what runs unattended into worktrees that hold unlanded work, and a bug there is quiet until a build is slow.

**S4. Make the old worktrees pattern-sparse: `git sparse-checkout set --no-cone` with `workspace.sparse_exclude` in each idle one.** Defense: the largest amount after landing, 426 MiB in each of 17, about 7,242 MiB, and it gives the old worktrees the shape new ones already have (BD-16). Against: it changes the visible tree under a session that may resume; worktrees on the art rows may hold changed or new images under `docs/incomplete`, which git keeps but which then need care one at a time; it writes per-worktree config; and undoing it with `git sparse-checkout disable` writes the 426 MiB back.

**S5. The request as written: replace each old `node_modules` and `web/.next/cache` with template clones when the lockfile matches.** Defense: none survives section 1.2 for `node_modules`. For `.next/cache`, cloning the template's cache gives the same bytes freed as deleting and keeps the next build warm. Against: the `node_modules` swap moves 96,147 to 143,199 files for 0 bytes; and a cloned template cache pins that template generation, so after the template's next build the clones keep the old 765 MiB alive (section 5).

## 4. Dials

Only S2 and S3 read dials. Each goes in `~/.wk/<repo>.toml`, never in code (D19).

| Dial | Recommended default | Why |
|---|---|---|
| `slim.dirs` (S2) or `gc.trim_dirs` (S3) | `["web/.next/cache"]` | The one private, regenerable, non-work target measured. Never `node_modules` (0 private), never the rest of `web/.next` (about 36 MiB, and a `next start` after a build needs `server` and `static`) |
| `slim.mode` | `delete` | Frees the same bytes as a template clone, needs no template lock and pins no template generation; the cost is one cold build |
| `slim.quiet_minutes` | 60, the same as `gc.quiet_minutes` | The loss from a wrong call is one cold build, not work |
| `slim.min_private_mib` | 100 | Skips trees whose cache is a clone (0 private), such as every hook-made worktree that has not built |
| `slim.scope` | every kept worktree but the main checkout, the template and pool slots | The old 17 are a closed set; the kept hook worktrees that build are the ongoing source |

## 5. Hazards this work walks into

- `du` reports a clone at full size. The 37 GB that started this request was a `du` figure. Any report the tool prints must use private bytes, and any proof must use them or `df`.
- Private size is a lower bound when blocks are shared only among trees that are all being removed (section 1.1).
- A template build pins its predecessor. After `template.build` runs on a `bun.lock` change, every workspace that cloned the old `web/.next/cache` keeps those blocks alive, up to 765 MiB per template generation, until the last such workspace goes. `wk new` has this property today, whichever option is chosen. Not measured here.
- A session between tool calls leaves no process in its worktree (BD-19). The process test therefore misses a gate paused between `tsc` and `bun run build`; the result of deleting the cache then is one cold build, which is why the target list stops at `cache`.
- Two `next build` runs over one `.next` print a bare `Failed to compile.` (DESIGN.md section 6). Deleting `.next/cache` during a build is the same class of failure; the process test catches a running build, whose cwd is `web/`.
- The auto-mode classifier refuses worktree removal from a session's `Bash` call. An `rm -rf` inside a worktree may be refused too, so S1 is Zach's command and S2 or S3 run from the launchd job or Zach's shell.
- `p1-phase-3` was not merged into `wk` `main` when this scope began; it was merged as `30989b5` by 15:36 (section 1.5), so S2 or S3 would start from `main`.
- `golang.org/x/sys` v0.41.0 exports no `Getattrlist`, and BD-13 pins it there for Go 1.24 (section 1.5).
- Other sessions write in this folder: `PLAN.md` holds another session's uncommitted edits (`git status`), so this scope edits nothing but this file.

## 6. Open questions, the GATE 1 batch

1. Build `wk slim`? S0, do not build it and land the old worktrees instead (recommended); S2, its own command for build output; S3, a trim verdict in `gc --apply`; S1 only, one printed command now and no code.
2. Trim the 5,216 MiB of `web/.next/cache` in the seven idle old worktrees now, with one command printed for Zach to run? Yes, print it (recommended); no, wait for landing.
3. The 7,242 MiB of `docs/incomplete` in the old checkouts: leave it, landing frees it (recommended); or apply S4.
4. Replace the `wk slim` seam in `PLAN.md` section 4 with these numbers, once the other session's edits to `PLAN.md` are committed? Yes (recommended); no.

## 7. Board row proposal, not yet appended

The next free row on 2026-10-16 at 15:40 is 199: `PASSOFF.md` ends at row 198, and row 197 was used by the workflow refinement (`git log --all --grep='row 197'` gives `0f534e494`). The sibling session scoping the build semaphore may take 199 first, so re-read before appending. Zach chose S0, so the row is about landing, not about `wk`. After the two commands of "Answers" run, each of the 16 holds about 59 MiB of checkout (478 less 419 of images) and, if it built, about 36 MiB of `web/.next`, so about 1.2 GiB in all. The row's value is then the unlanded work more than the disk.

| # | Task | Status | Model | Lane | Waits on | Files it owns |
|---|---|---|---|---|---|---|
| 199 | Land or discard the 16 worktrees made before `wk` (9 absorbed but dirty, 7 with unlanded commits); `wk slim` settled as not built (`docs/incomplete/landing-layer/SLIM-SCOPE.md`) | OPEN | Opus | tooling | Zach: a keep or discard call for each of the 7 with unlanded commits | none in the tree; the 16 worktrees under `.claude/worktrees/` named in `SLIM-SCOPE.md` section 1.3 |

Prompt body for `passoff/199-...md`: run `/land-worktrees` over the nine absorbed-but-dirty worktrees of section 1.3 and hand Zach the commit or discard blocks it writes; list the seven with unlanded commits with what each branch holds (`git log main..<branch> --stat`), so Zach can keep or discard each; leave `agent-a02d20bbe0ab981c8` to `gc`. Read every worktree with `GIT_OPTIONAL_LOCKS=0`, so a read does not move its quiet clock. Done when each of the 16 is landed, discarded by Zach, or kept with his reason, and `df -k /` before and after is recorded here. Model: Opus, because the skill's output needs judgment and a wrong discard of unlanded work is quiet; not Fable, because Zach makes every discard call himself.

If S2 or S3 is ever wanted after all, its row is: "`wk`: trim build output in kept, idle worktrees (S2 command or S3 `gc` verdict, per `SLIM-SCOPE.md`)", Opus, lane tooling, owning `~/Projects/wk/internal/wk/`, `internal/config`, a new private-size reader and `~/.wk/ezhomesteading.toml`. The trigger to revisit is a runtime week of `gc.log` that shows kept hook worktrees holding real `web/.next` output.

## Answers

Asked 2026-10-16 in chat, in one batch, answered the same day by Zach. Each answer is quoted by its option label.

1. Build `wk slim`: "Do not build (Recommended)". S0: `wk slim` is not built. The `node_modules` half frees 0 bytes (section 1.2), and the `.next/cache` half is a one-off (answer 2). Supersedes the `wk slim` seam of `PLAN.md` section 4, which now points here (answer 4).
2. Trim `web/.next/cache` in the seven idle old worktrees now: "Yes, print it (Recommended)". Command A below, for Zach to run.
3. The `docs/incomplete` media in the old checkouts: "Make them sparse". S4, command B below, for Zach to run. It covers 16 of the 17: `agent-a02d20bbe0ab981c8` is left out, because `gc` removes it about 10:43 on 2026-10-17 and a sparse checkout rewrites its index, which would restart its 24-hour wait.
4. Replace the seam in `PLAN.md` section 4: "Yes (Recommended)". The other session's edits to `PLAN.md` were already committed at 15:36 (`77b033a5d`, no diff at 15:39), so this session replaced the one seam line the same day.

### The two commands, as printed for Zach

Both run from Zach's shell, never from a session (section 5, the classifier). Both take one `lsof -d cwd` snapshot first and skip a worktree that has a process inside. Command A also skips a worktree whose `web/.next` changed in the last hour; command B skips one whose index changed in the last hour, which on 2026-10-16 includes `nifty-lehmann-452d6e` until about 16:33 (section 1, side effects). Each prints the `df` change at the end.

Command A, the cache trim. Expected: about 5,216 MiB freed. Undo: none needed; the next `next build` in that worktree recreates the cache, cold.

```bash
cd /Users/zachshort/Projects/ezhomesteading/.claude/worktrees && before=$(df -k / | awk 'NR==2{print $4}') && procs=$(lsof -d cwd -Fn 2>/dev/null); for w in agent-a18e6d6ebabbfe17c agent-a93d2107c291f21d1 agent-ab496235aa1690306 agent-adbbf050d4404acf8 agent-add71e46821d75ce7 agent-ae636546177a35039 row-195; do if grep -q "^n$PWD/$w" <<<"$procs"; then echo "skip $w: a process is inside"; elif [ -n "$(find "$w/web/.next" -mmin -60 -print -quit)" ]; then echo "skip $w: web/.next changed in the last hour"; else rm -rf "$w/web/.next/cache" && echo "trimmed $w"; fi; done; echo "freed $(( ($(df -k / | awk 'NR==2{print $4}') - before) / 1024 )) MB by df"
```

Command B, the pattern-sparse checkout, with the eight patterns `wk new` uses (`~/.wk/ezhomesteading.toml`, `workspace.sparse_exclude`; `internal/wk/new.go`, `sparseExcludes`). Expected: about 16 times 419 MiB, 6,704 MiB, freed. A changed or untracked image stays (section 1.5). Undo for one worktree: `git -C <worktree> sparse-checkout disable`, which writes its 419 MiB back.

```bash
cd /Users/zachshort/Projects/ezhomesteading/.claude/worktrees && before=$(df -k / | awk 'NR==2{print $4}') && procs=$(lsof -d cwd -Fn 2>/dev/null); for w in agent-a18e6d6ebabbfe17c agent-a2335e7c25d78a156 agent-a323447ff297d7206 agent-a427932828c3648cc agent-a93d2107c291f21d1 agent-aaabdb197e0066348 agent-aaf40772c4156377b agent-ab496235aa1690306 agent-ac597cbb52da49fe1 agent-ad34f8b06e21b510d agent-adbbf050d4404acf8 agent-add71e46821d75ce7 agent-ae636546177a35039 agent-afb01f9d2bb9fa56e nifty-lehmann-452d6e row-195; do if grep -q "^n$PWD/$w" <<<"$procs"; then echo "skip $w: a process is inside"; elif [ -n "$(find "$(git -C "$w" rev-parse --absolute-git-dir)/index" -mmin -60)" ]; then echo "skip $w: its index changed in the last hour"; else git -C "$w" sparse-checkout set --no-cone '/*' '!/docs/incomplete/**/*.png' '!/docs/incomplete/**/*.svg' '!/docs/incomplete/**/*.json' '!/docs/incomplete/**/*.jpg' '!/docs/incomplete/**/*.jpeg' '!/docs/incomplete/**/*.gif' '!/docs/incomplete/**/*.webp' '!/docs/incomplete/**/*.riv' && echo "sparse $w"; fi; done; echo "freed $(( ($(df -k / | awk 'NR==2{print $4}') - before) / 1024 )) MB by df"
```

Neither command has been run. When Zach runs them, the printed `df` figures belong here, under this heading.

### Row 199: landing the 16, 2026-10-16

Run by an Opus 5.5 session as board row 199 (`passoff/199-land-or-discard-the-16-worktrees-made-before-wk.md`, which holds what each worktree held and Zach's four answers). Every read of a worktree used `GIT_OPTIONAL_LOCKS=0`, the survey included.

Before: 38,614,720 KB available at 15:48:09 EDT (`df -k /`). Commands A and B had not run: no old worktree was sparse, and all seven caches of command A were in place.

Zach chose to land rows 184, 186, 188, 189 and 191 and the workflow-refinement plan, and to discard the other five. So all 16 go. `land.sh` merges six branches and removes eleven worktrees (`agent-a2335e7c25d78a156`, `agent-a323447ff297d7206`, `agent-a93d2107c291f21d1`, `agent-aaabdb197e0066348`, `agent-aaf40772c4156377b`, `agent-ab496235aa1690306`, `agent-ac597cbb52da49fe1`, `agent-adbbf050d4404acf8`, `agent-add71e46821d75ce7`, `agent-ae636546177a35039`, `agent-afb01f9d2bb9fa56e`). One discard command removes the other five (`agent-a427932828c3648cc`, `agent-a18e6d6ebabbfe17c`, `agent-ad34f8b06e21b510d`, `nifty-lehmann-452d6e`, `row-195`). Section 1.3 gives the 16 about 13,120 MiB of private bytes, so `df` should rise by about that much.

Commands A and B act only on these 16 worktrees. Once `land.sh` and the discard command have run, neither has anything left to act on; a worktree that a discard skips is still in their lists.

Tips that only the discarded worktrees held, for recovery while git keeps unreachable commits: `row-195-core-helpers` at `84df21ecf`, and the detached HEAD of `nifty-lehmann-452d6e` at `a9475f366` (parent `dbcc06ddc`).

What ran. The 17 gates of the trial gave 15 pass and 2 skip (`sqlgate` and e2e, no local test database). Zach ran `land.sh --apply` at about 16:08: six merges, main at the gated tree `c725bc6f1`, 11 worktrees and 12 branches removed, 0 skipped. It also removed a stale trial from another session's scratchpad (made 13:19, idle since 13:34). At 16:08:27 a reword of the earlier merge `cc9dc8207` rewrote every commit above it, so the six merges are `5d0690090` to `de786cee9` on `main`, not the `81a7b22b5` to `51d42843b` that `land.sh` printed; the tree and the six code commits are unchanged. Zach then ran the discard command, which removed the other five. At 16:18 `git worktree list` held none of the 16, and `agent-a02d20bbe0ab981c8` was still in place for `gc`.

After, by `df -k /`:

| When | Available | Change | The 16 by section 1.3 |
|---|---|---|---|
| 15:48:09, before | 38,614,720 KB | | |
| 16:11:14, after `land.sh` | 51,212,516 KB | +12,303 MiB | 11 worktrees, 9,189 MiB |
| 16:18:31, after the discards | 55,904,064 KB | +4,582 MiB | 5 worktrees, 3,931 MiB |
| In all | | +16,884 MiB | 13,120 MiB |

`df` counts the whole volume, so the change holds more than the 16. In the same 30 minutes `land.sh` removed the stale trial, and `wk gc --apply` removed 15 sparse workspaces of about 55 MiB each (`wk-9881` to `wk-9888` and `wk-9902` at 15:54, `burst-0` to `burst-5` at 16:16, `~/.wk/ezhomesteading/gc.log`); other sessions wrote and deleted files throughout. This run did not separate those parts. Commands A and B now have nothing to act on, because every worktree in their lists is gone.
