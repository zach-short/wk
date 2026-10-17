# Row 3 audit 1, Opus 5.5, agent ac09ec4c05b3f6060

Launched 2026-10-17 03:00 EDT, returned 2026-10-17 03:22 EDT. Detached worktree `.claude/worktrees/ap-row-3-audit` at `f0b1eb8`.

## Verdict: blocking findings

Gates green; every case the brief listed behaves as specified. Four inputs pass the hook silently while the file system writes a leased path, three through the edit-tool entry. README.md:201 says that entry has no gap. No hang path, no red gate.

### Blocking

- B1. A path with the wrong letter case passes on macOS. `Edit` of `<wk-2>/WEB/Page.ts` and Bash `rm Web/page.ts` from wk-2, with wk-1 leasing `web/page.ts`: both ALLOWED, no log line. `stat` shows one inode (APFS ignores case). Cause: `canonical` (internal/wk/lease.go:284-302) keeps the case as written; `touches`/`covers` (internal/lease/store.go:144-159) compare bytes. Fix: fold case on a case-insensitive volume, or read each component's real name in `canonical`.
- B2. A cwd outside every root lets a session write any worktree's files. `{"cwd":"/private/tmp", Edit <wk-1>/web/page.ts}` and Bash `rm <wk-1>/web/page.ts`: both ALLOWED while wk-1 holds the lease. Cause: `callerRoot` returns allow when cwd resolves to no root (internal/wk/leasehook.go:224-227); `Judge` then sees wk-1's own lease on wk-1's path. Control: the same Edit on wk-2's copy was denied. Fix: when cwd is in no root, deny every target that is in a root. Not named in the close-out as a build-level call.
- B3. A dangling symlink to a leased new file passes. wk-1 ran `wk lease add web/new.ts` (leased, not made); `<wk-2>/dang.ts` points at `<wk-1>/web/new.ts`. `Write` of `dang.ts`, Bash `echo x > dang.ts`, and `ln -s <wk-1>/web/new.ts dang2.ts`: all ALLOWED. Cause: `EvalSymlinks` fails with `ErrNotExist` on a dangling link; `canonical` walks up and keeps the link's own path (lease.go:291-299). The shell follows the link; whether Claude Code's `Write` follows it is not verified. Fix: when `Lstat` shows a symlink, `Readlink` it and canonicalize the target.
- B4. A symlink followed by `..` hides the real target. `<wk-2>/sub/e` points at `<wk-1>/docs`; Bash `rm sub/e/../web/page.ts` ALLOWED (`stat` shows wk-1's `web/page.ts`); the same path as an `Edit` `file_path` also allowed. Cause: `place` uses `filepath.Join` (internal/lease/bash.go:219-229) and `canonical` calls `filepath.Clean` (lease.go:288); both remove `..` by text before any symlink is read. README.md:191 claims the hook places each path through symbolic links. Fix: resolve symlinks before cleaning, or deny a target that has `..` after a symlink; part of it is in row 7's walk.

### Reproduction

HEAD f0b1eb8eda7a9e3459400a29d1c4ef091670ed13; merge-tree with main exit 0 (tree e632017); go mod download exit 0; diffstat matches (19 files, +1484/-41, six added); commits 98d1844, 5a93108, f0b1eb8 without attribution lines. Files outside the row's list each justified: board.go (header amendment), judge.go (row 7 finding, test `TestJudgeCanonPlacesBashTargets`), daemon/*.go (doc comments, files decision), tests.

### Gates

Full gate exit 0, every package `ok` (internal/wk 41.4 s), `0 issues.` Verbose run over wk, config, settings, cmd/wk: 224 PASS, 0 FAIL. Scratch binary build exit 0.

### Live checks (scratch `WK_HOME`, repo `lt`, wk-1 row 1 `web/page.ts`, wk-2 row 2 `docs/`; every call exit 0; each deny printed the JSON shape and one hook.log line; no allow logged)

| # | Input, from wk-2 unless stated | Result |
|---|---|---|
| 1 | Edit of wk-2's own `web/page.ts` | DENY, leased by row 1 in wk-1 |
| 1b | Edit of wk-1's `web/page.ts` | DENY, in the workspace wk-1 while this session works in wk-2 |
| 2 | Edit of `web/free.ts` | ALLOW |
| 3 | Write of relative `web/page.ts` | DENY; relative `web/free.ts` ALLOW; relative cwd DENY "cannot tell which file" |
| 4 | Edit through symlink `alias.ts` and the `/tmp` spelling | DENY both |
| 5 | Bash `rm web/page.ts` | DENY |
| 6 | `cat > docs/b.md <<EOF` with `it's mine` | ALLOW (also `<<'EOF'`); into `web/page.ts` DENY |
| 7 | `wk status` | ALLOW |
| 8 | `wk status; rm web/page.ts` and the `&&` form | DENY |
| 9 | `echo $HOME > docs/a.md` | DENY, holds `$`; `echo $HOME` alone also DENY (known over-refusal) |
| 10 | `cd <wk-1> && touch x` | DENY |
| extra | MultiEdit; NotebookEdit with `notebook_path` | DENY |
| extra | Edit with no path; non-JSON input | DENY |
| extra | Edit of a main-checkout file; edit by an unclaimed worktree | DENY |
| extra | Edit of a path outside every root | ALLOW |
| extra | A claim whose worktree is gone | Reaped, ALLOW |
| extra | `--repo` with the same base name | DENY; a second repo named `lt`: `wk claim` exit 1 |
| extra | `/tmp/x/wk rm 1` | ALLOW: any binary named `wk` passes |
| B1 to B4 | above | ALLOW, silently |

Timing: 20 denied Edits mean 3 ms, max 20 ms. 1.1 MB heredoc 0.33 s; 3000 `cd`s 0.05 s; a 2000-deep path 0.13 s. With `leases.lock` held elsewhere the hook took 0.00 s (it takes no lock).

`wk hooks install`: `enabled = false` leaves only row 200's `wk hook pre-tool-use || true`; `enabled = true` adds the edit entry (`onFailure: block`, timeout 5) and the Bash entry (`|| true`, timeout 5); a second install reports `unchanged`; `uninstall` leaves `{}`; with a foreign `commit-guard` entry, a `Stop` hook and `permissions`, install keeps all three and uninstall restores the file exactly.

### Non-blocking

- N1. A panic outside `decide` becomes an allow: `runLeaseHook` turns it into exit 0 with no output (cmd/wk/main.go:240-244). Only `wkHome`, the JSON encode and `logLine` run there. The recover inside `decide` (leasehook.go:67-71) is correct.
- N2. No test covers panic-to-deny.
- N3. Existing workspaces are unguarded after the owner turns leases on: `settings.local.json` is copied once at `wk new`; workspaces made before the turn-on stay unguarded silently, and after a turn-off old workspaces keep the entries and log an allow line per call. The hand-back must tell the owner to recreate or refresh them.
- N4. README stale or wrong: the heredoc-apostrophe over-refusal (203) is gone; "no such gap" (201) and "places each path through symbolic links" (191) are false by B1 to B4.
- N5. The wk-binary allow accepts any file named `wk` and allows `wk rm <n>` of another workspace (as the amendment reads).
- N6. One Bash command is walked three times per call: leasehook.go:103, :162, judge.go:88.

### Lean

lease.go 302, leasehook.go 253; no new function over 40 lines; over-40 functions (`config.validate` 72, `main.run` 74, `settings.withoutWK` 47) predate the row. `worktreeOf` + `linksInto` (about 30 lines) and `callerRoot` are not needed by the proof; both justified in the close-out. No duplicated logic apart from N6. Small shape.

### Documents

README, As built and close-out: sentence case, no dashes, no contractions, no AI-signal words. Close-out names 10 build-level calls with reversals, omits B2's allow. As built cites scratchpad files that do not persist. Step 10's "record each finding's outcome in the As built note" is still owed.

### Could not verify

Whether Claude Code 2.1.301 `Write` follows a dangling symlink (B3); whether it normalizes `..` in `file_path` (B4); whether Edit succeeds on a wrongly cased path (B1, file-system half proven); whether a session's hook `cwd` can sit outside the project (B2). No `claude -p` run.

### Housekeeping

Audit worktree status empty, HEAD f0b1eb8; scratch under `row-3-audit/` (gate output in `gate.txt`); no commit; `~/.wk`, the builder's worktree, HANDOFF.md and PASSOFF.md untouched.
