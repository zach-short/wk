# Row 7, pass 2 audit 6 (fix round 2), Opus 5.5, agent ad82ddd0857051ed1

Launched 2026-10-17 01:33 EDT, returned 2026-10-17 01:45 EDT. Detached worktree `.claude/worktrees/ap-row-7-audit-6` at `cd9f720`.

## Verdict: clean

No blocking and no non-blocking findings. B2 is closed. All five B2 inputs and every B1 row give the expected result. The hunt found no spelling that a real shell runs as a leased write and the walk allows.

## Reproduction

- HEAD cd9f720b69f90e3ef5b947ba6243b1f5ccdd28e9. `git merge-tree --write-tree main HEAD` tree 2bd631a6, exit 0. `go mod download` exit 0.
- `git diff --stat main...HEAD`: 7 files, +1978/-1 (bash.go 933, bash_test.go 278, judge.go 154, judge_test.go 153, lease_test.go 149, store.go 173, passoff item +139/-1). Matches the builder.
- `git diff 713b8b4..HEAD`: bash.go +13/-4 (net +9), bash_test.go +10/-1, judge_test.go +1, item file +5/-1. judge.go, store.go, lease_test.go and the exported surface unchanged. Only the splitter changed, in four functions: `run()` end check `len(p.open) > 0 || p.pending != nil`; `step()` newline refuses when `pending != nil`, operator case returns `p.failed == ""`; `operator()` refuses a waiting `pending` at an operator as `<open heredoc>` (after the fd-digit reset); `flush()` clears `pending` only inside `if p.in`, comment corrected.
- Test diff: 9 new TestWriteTargets rows and 1 TestJudge row. One pass-2 row (`cat <<` NL `rm y` NL `rm z`) changed from targets y,z to `<open heredoc>`; bash 3.2.57 gives a syntax error on that input and runs nothing.
- Builder claim checked: with 713b8b4's bash.go swapped in, the 8 new walk rows other than no-space fail, the changed pass-2 row fails, the new judge row fails; the no-space row passes. File restored.
- Commit message carries no attribution line.

## Gates

- Full gate exit 0; tail `ok .../internal/lease 3.322s` ... `ok .../internal/wk 37.536s`, `0 issues.`
- `go test -race -count=1 -v ./internal/lease/...` exit 0: 318 `--- PASS`, 0 `--- FAIL`.
- `go list -deps ./internal/lease | grep zach-short` prints only the lease package.

## Decision table (roots /r/a row 1 web/, backend/routes/routes.go; /r/b row 2 docs/; cwd /r/b)

R1 = DENY "web/x.ts is leased by row 1 in /r/a ... (the command writes /r/b/web/x.ts)". OH = DENY `<open heredoc>`.

- B2-1 `cat << EOF > docs/a.md` ... `rm web/x.ts # it's gone`: docs/a.md, web/x.ts. R1.
- B2-2 `cat <<- EOF` with tabs: R1. B2-3 `cat << 'EOF'`: R1. B2-4 `cat <<` TAB `EOF`: R1. B2-5 double quote across two bodies: web/x.ts, R1.
- B1 rows: a, b, c, e, f, k, l, m, n R1 (f with `<standard input>`); d, h, j, o ALLOW with docs targets only; i OH. Same as audit 5.
- Whitespace variants: one and two spaces before EOF with an apostrophe body ALLOW docs/a.md only; `cat << EOF`/`EOF`/`rm web/x.ts` R1; `cat <<`/`rm web/x.ts` OH; `cat << > docs/a.md` OH; `cat << && rm web/x.ts` OH; `cat << EOF | sh` DENY `<standard input>`; `echo a <<EOF >docs/a.md` ... R1; two heredocs on one line R1; terminator with trailing space then `rm` R1 (a safe over-refusal; real shells do not run the rm).
- Real shells: bash 3.2.57(1) and zsh 5.9. B2-1 to B2-5 and B1 a, b, c each wrote web/x.ts with exit 0 in both shells. DENY is right for all eight.

## Hunt

No leased write allowed through. Comment after `<<` gives OH (shells error). `<<\EOF` refused by the safe set. `<<-` with tab or spaces, `<<` with two tabs: R1. Digit-first and all-digit delimiters: R1 or OH (all-digit before `>` is read as an fd: OH; zsh would write, bash errors; the walk refuses, loud). Dash delimiters R1. Operator right after the delimiter (`>`, `|sh`, `;`, `&&`, `true&&cat<<EOF>`): R1 or `<standard input>`. `<< ''`, `<< A << B`, `3<< EOF`, trailing space, `E"O"F`, `E#F`, `EOF!`, `EOF # it's`, `<<< 'don''t'`: R1. `cat << ;` and `cat << ` at end of input: OH. The per-line quote fallback in `bodyLine` can turn a quoted `#` into a comment, in data bodies only; bodies fed to a shell are denied as `<standard input>`.

## Lean

The 9 net lines are the minimum shape; each guard is needed (newline, operator, end of input). One optional early-exit line after `p.operator()` in `step`. Comment rewrite 3 lines for 2. bash.go 933.

## Item file

`## Held` (line 94) before `## Pass 2` (line 98); `### Fix round 1` (157) before `### Fix round 2` (161); the Close-out section's own earlier Fix round headings at 86 and 90 untouched. Round-1 delimiter sentence corrected. No contractions or dashes outside backticks.

## Could not verify

The Claude Code hook path end to end (tested through `Judge` and `WriteTargets` only). Shells other than bash 3.2.57 and zsh 5.9.

## Worktree state

`git status --porcelain` empty; scratch test deleted; bash.go restored after the 713b8b4 check; no commit; no command denied.
