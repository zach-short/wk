# Row 7, pass 2 audit 5 (fix round 1), Opus 5.5, agent a31ac30ea584ee340

Launched 2026-10-16 23:04 EDT, returned 2026-10-16 23:14 EDT. Detached worktree `.claude/worktrees/ap-row-7-audit-5` at `713b8b4`.

## Verdict: one blocking finding (B2)

B2. A space or tab between `<<` and the delimiter opens no heredoc, so B1 comes back in the most common heredoc spelling.

- Cause: in `flush()`, `internal/lease/bash.go:877`, `p.pending = nil` runs on every flush, including when no word is open.
- How it fails: `operator()` sets `pending` at bash.go:863-865. The next byte is a space, so `step()` calls `flush()` with `p.in == false`, and that clears `pending` before the delimiter word arrives.
- Result: `cat << EOF` registers no heredoc. The body is read as shell text, and the apostrophe pairing from B1 hides every command between the two apostrophes.
- The comment at bash.go:868-869 does not cover the whitespace case.

Each of these is ALLOWED by `Judge` at cwd /r/b, targets `["/r/b/docs/a.md"]` or `[]`:

- `cat << EOF > docs/a.md` NL `don't` NL `EOF` NL `rm web/x.ts # it's gone`
- `cat <<- EOF > docs/a.md` NL TAB `don't` NL TAB `EOF` NL `rm web/x.ts # it's gone`
- `cat << 'EOF' > docs/a.md` NL `don't` NL `EOF` NL `rm web/x.ts # it's gone`
- `cat <<` TAB `EOF > docs/a.md` NL `don't` NL `EOF` NL `rm web/x.ts # it's gone`
- `cat << EOF` NL `say "hi` NL `EOF` NL `touch web/x.ts` NL `cat << EOF` NL `there"` NL `EOF` (targets `[]`)

All five run in /bin/bash 3.2.57 and zsh 5.9 with `touch` in place of `rm`: both shells wrote web/x.ts in all five.

Suggested fix (not applied): clear `pending` in `flush()` only inside `if p.in`; add `p.pending = nil` to the newline case of `step()` and at the top of `operator()` before a new `<<` sets it; add the five inputs as test rows; correct the comment at 868-869.

## Reproduction

- HEAD 713b8b40185bb14fc40acb0b624b6ddb6f159863. `git merge-tree --write-tree main HEAD` exit 0 (tree ded6ded). `go mod download` exit 0.
- `git diff --stat main...HEAD`: 7 files, +1955/-1, same as the builder (bash.go 924, bash_test.go 269, judge.go 154, judge_test.go 152, lease_test.go 149, store.go 173, passoff item 135).
- `git diff f9ee256..HEAD --stat`: bash.go +88/-10, bash_test.go +15/-3, judge_test.go +2, passoff +4. judge.go, store.go and lease_test.go unchanged. Exported surface the same.
- bash.go changes: the `WriteTargets` doc comment, the `openHeredoc` const, `refusal()`, `nested()`, the splitter (new struct fields, `split` returns a string, new `run`, `bodies`, `cutBody`, `bodyLine`; `step`, `quote`, `operator`, `flush` changed). Verb tables unchanged. No attribution line.

## Gates (audit worktree)

- Full gate exit 0; tail all packages `ok` (`internal/lease 2.960s`, `internal/wk 45.133s`), `0 issues.`
- `go test -race -count=1 -v ./internal/lease/...`: 308 `--- PASS`, 0 `--- FAIL`.
- `go list -deps ./internal/lease | grep zach-short` prints only the lease package.

## Decision table (roots /r/a row 1 web/, backend/routes/routes.go; /r/b row 2 docs/; cwd /r/b)

| Row | Result | Targets | Expected |
|---|---|---|---|
| a | Row 1 deny | docs/a.md, web/x.ts | yes |
| b | Row 1 deny | docs/a.md, web/x.ts | yes |
| c | Row 1 deny | web/x.ts | yes |
| d | ALLOW | /r/b/docs/a.md | yes |
| e | Row 1 deny | web/x.ts | yes |
| f | Row 1 deny | web/x.ts, `<standard input>` | yes |
| g | DENY, holds `$` (safe set refuses first) | `<character $>` | loud, by design; without `$` it is ALLOW |
| h | ALLOW | /r/b/docs/a.md | yes |
| i | DENY, open-heredoc message | `<open heredoc>` | yes |
| j | ALLOW | /r/b/docs/a.md | yes |
| k | Row 1 deny | docs/a.md, web/x.ts | yes |
| l | Row 1 deny | docs/a.md, web/x.ts | yes |
| m | Row 1 deny | docs/a.md, web/x.ts | yes, both bodies cut in order |
| n | Row 1 deny | docs/a.md, web/x.ts | yes |
| o | ALLOW | /r/b/docs/a.md, /r/b/docs/b.md | yes |

Shell probe for a, b and c with `touch`: web/x.ts written in bash 3.2.57(1) and zsh 5.9.

## Hunt

- Space or tab after `<<` or `<<-`: BLOCKING (B2).
- Terminator with trailing space or `;`: bash does not take it as the terminator; the walk does not either and walks the body: DENY, over-refusal only.
- `<<E'O'F`: delimiter EOF after quote removal in both; DENY row 1. A body line `E'O'F` is not the terminator: `<open heredoc>`.
- Here-string `<<<'x'` then `rm`: not a heredoc; DENY row 1. `<<<"don't"` gives an open-quote DENY.
- `$(cat <<EOF ...)`: `<character $>`, DENY.
- `3<<EOF` and `cat <<EOF>docs/a.md`: heredoc opens; DENY row 1.
- `cat <<123>docs/a.md`: `<open heredoc>`, loud.
- `cat <<EOF > docs/a.md && rm web/x.ts` and `cat <<EOF && echo "don't" > docs/b.md`: DENY row 1; the rest of the line is walked.
- `let x=1<<2` then `rm`: `<open heredoc>`; bash also reads that as a heredoc.
- `<<''`: terminator is an empty line, as in bash; DENY row 1.
- Bodies a shell runs: `sh <<EOF`, `bash -s <<EOF`, `source /dev/stdin <<EOF` give `<standard input>`, DENY. This closes the per-line read hiding a body like ``echo '`` NL ``#' ; rm web/x.ts``.
- `sh -c`, `eval` with a heredoc inside: DENY row 1.
- `<<\EOF` and backslash-newline refused by the safe set; `(( ))` refused on `(`; tab before `EOF` with `<<` is not the terminator in either.

## Lean

bash.go 924 (+88/-10). The fix has the small shape: on a newline a pre-pass cuts each opened body (`bodies`/`cutBody`), then the splitter reads each line through `run()`, plus the open-heredoc refusal; no duplicated quoting logic. Longest new function `run()` 15 lines (792-806); `cutBody` 14 (894-907). `body` is needed. The quote-stripping second pass in `bodyLine` (911-919) keeps rule 3 over-refusals for data bodies. Optional: inline `bodies()` (883-892) into its one caller. Nothing else cuts without reopening a case.

## Item file

`### Fix round 1, 2026-10-16` at line 157 under `## Pass 2` (line 98); `## Held` (line 94) precedes. No em dashes; apostrophes only in backticked inputs and one possessive. One claim is now wrong because of B2: "on the newline that ends a line holding `<<` ... and a delimiter word".

## Not verified / notes

Suggested fix not applied or tested. Scratch test and probe kept in the session scratchpad `audit-5/`. `git status --porcelain` empty in the audit worktree. Nothing committed.
