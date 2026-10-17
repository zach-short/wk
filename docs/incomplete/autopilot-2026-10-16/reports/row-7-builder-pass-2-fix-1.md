# Row 7, pass 2 fix round 1, Opus 5.5, agent ac78c9607da6dd6d1

Sent 2026-10-16 23:00 EDT, returned 2026-10-16 23:03 EDT. Worktree `.claude/worktrees/ap-row-7`, branch `ap/row-7`, commit `713b8b4`. No merge, push or worktree removal. No trailer.

## The fix (`internal/lease/bash.go`, splitter only)

- Bodies are cut out first. A line can hold `<<` or `<<-` with a delimiter word. When that line ends, the splitter takes the following lines up to the terminator as opaque lines. The terminator is the delimiter alone on a line, after leading tabs for `<<-`, with a quoted or an unquoted delimiter.
- Each body line is read on its own. No quote in a body can pair with anything outside it. If a quote does not close on its line, the quote is taken out and the line is read again. A `<<` inside a body opens nothing.
- A heredoc with no end is refused, including one at the end of the input. New target `<open heredoc>`; denial: "wk: cannot tell which file this writes: a heredoc in the command has no end line; end it with its delimiter alone on a line".
- Rule 3 still holds: `cat <<EOF` / `rm web/x.ts` / `EOF` is denied; `cat <<EOF |` with `sh` on a later line keeps its `<standard input>` target.
- Only the splitter, `refusal()` and `nested()` changed (`split` now returns its refusal target). Verb tables, exported surface and `judge.go` unchanged.

## Tests

- 11 new walk rows in `bash_test.go`: the audit's three cases, a delimiter word inside prose, a write in a body with an open quote, a `'EOF'` delimiter, an `"EOF"` delimiter, the `<<-` terminator after tabs, `<<` inside a body, a heredoc at the end of the input, a body that never ends.
- 2 new rows in `judge_test.go`: the apostrophe case denied with the holder named; the no-end case gets the new message.
- 3 pass-2 rows now expect `<open heredoc>`: the two unterminated-body rows, and `let x=1<<2` followed by `rm a`.

## Gates

Full gate exit 0 (tail: `ok .../internal/lease 3.122s` ... `ok .../internal/wk 41.763s` / `0 issues.`). Package `-v` run: 308 PASS, 0 FAIL.

## Diffstat `main...HEAD`

7 files, +1955/-1: bash.go 924 (was 846, +78), bash_test.go 269, judge.go 154, judge_test.go 152, lease_test.go 149, store.go 173, passoff/7 135. `### Fix round 1, 2026-10-16` appended under the Pass 2 section of passoff/7.

## Not verified

- The three B1 cases were not run in bash or zsh by the builder; it relies on the audit's shell report.
- The zsh claims from pass 2 are still unverified in a real shell (earlier scratch probe denied, not retried).

No denied calls this round. The non-blocking items are untouched, including the twice-coded expansion rule.
