# 8. Build the board cell parser in `internal/board`

> **Claimed `IN FLIGHT` 2026-10-16.**

*Opened 2026-10-16 by row 2, as a P2 sibling of row 3 (`docs/incomplete/landing-layer/PLAN-P2-P4.md` section 2).*

**Model: Opus 5.5. Lane E. Waits on 2.**

You are building one package of P2 of the landing layer, `wk`. Read first: `PLAN-P2-P4.md` section 2, the row 8 line of its sub-row table (binding), `DESIGN.md` D10 and option G (a lossy parse that stays quiet is a lease that does not exist), `docs/conventions-go.md`, and the two live boards for the cell shapes: `/Users/zachshort/Projects/wk/PASSOFF.md` (read only; it is untracked and the orchestrating session owns it) and `/Users/zachshort/Projects/ezhomesteading/PASSOFF.md` rows 76, 77 and 200 (their item files are `passoff/76-*.md`, `77-*.md` and `200-*.md` there). You own only `internal/board/`; row 3 wires it. Nothing in your package imports another `internal` package or runs git.

Do these, in order:
1. `internal/board/board.go`: `Cell(text, row string) (cell string, err error)`: the last cell of the first markdown table line whose first cell, trimmed, equals `row`; an error naming the row when there is none, and an error when the line has fewer than two cells. `Parse(cell string) (paths, unread []string)`: every backtick-quoted token is read; a token ending in `/` or in `/**` or `/**/*` is a directory prefix returned with one trailing `/`; a token with `*`, `?` or `[` elsewhere is returned as is for `Expand`; a token that is absolute, starts with `~`, or holds `..` is unread; every run of text outside backticks that is more than punctuation and whitespace is unread, verbatim, so the caller can print it. `Expand(patterns, files []string) (matched, unmatched []string)`: a pattern matches a file when `path.Match` holds segment by segment, with `**` matching any number of segments; a pattern that matches nothing is unmatched.
2. `internal/board/board_test.go`: a fixture board with five rows in the wk shape (`| # | Task | Status | Model | Lane | Waits on | Files it owns |`); cells copied from the three ezhomesteading rows above and from wk rows 1 to 5, each with its expected `paths` and `unread` written out; `Expand` over a tracked-file list with `passoff/3-*.md`, `docs/incomplete/landing-layer/`, `src/**/*.ts`, and a pattern that matches nothing.

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`; the package tests above green.

Ask before building: nothing.
Not in scope, whoever asks: reading the board from disk, `git ls-files`, the lease store, any subcommand. A need outside your package is a finding for row 3 in your close-out, never an edit.

Hand back: the gate output, the exported surface with each function's one-line contract, the unread tokens the two live boards produce (run `Parse` over every cell in a test log), the findings for row 3, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.

## Close-out, 2026-10-16

Built by an Opus 5.5 session in the worktree `ap-row-8` on branch `ap/row-8`, from `main` at `344f8c7`. The change adds two files, `internal/board/board.go` and `internal/board/board_test.go`, and touches nothing outside `internal/board/`. The package imports only the standard library (`fmt`, `path`, `strings`, `unicode`), reads no file and runs no git.

What is now true. The exported surface is three functions:

- `Cell(text, row string) (cell string, err error)` returns the last cell, trimmed, of the first markdown table line (a line that starts with `|`) whose first cell, trimmed, equals `row`. It returns an error naming the row when no line matches, and when the matching line has fewer than two cells.
- `Parse(cell string) (paths, unread []string)` reads every backtick token. A token ending in `/`, `/**` or `/**/*`, or in any run of them such as `dir/**/`, is a directory prefix with one trailing `/`; a token with `*`, `?` or `[` is a glob for `Expand`; any other token is an exact path. Globs and exact paths are both returned in `path.Clean` form. A token that is absolute, starts with `~`, holds `..` or holds whitespace is unread, as written. Every run of text outside backticks that holds a rune other than punctuation and whitespace is unread, trimmed of surrounding whitespace. `paths` keeps cell order and drops repeats.
- `Expand(patterns, files []string) (matched, unmatched []string)` returns the files, in files order, that any pattern matches, and the patterns that match none. Matching is `path.Match` per segment, with a `**` segment matching any number of segments.

The tests copy the "Files it owns" cells of wk rows 1 to 5 (as a five-row fixture board in the wk shape) and of ezhomesteading rows 76, 77 and 200 (row 200 twice: the board's short cell and the fuller cell its item file states), each with its expected `paths` and `unread` written out. `Expand` is tested over a tracked-file list with `passoff/3-*.md`, `docs/incomplete/landing-layer/`, `src/**/*.ts`, `missing/**/*.go`, an exact path, a non-path token and a malformed pattern.

Build-level calls, each with its reversal:

- A token that holds whitespace is unread (for example a command in backticks). The prompt did not name this case, and reading it as a path would lease a file that cannot exist. Reversal: drop the whitespace check in `classify`.
- A token that cleans to the repository root (`.` or `./`) is unread, because a lease on it would hold everything or nothing. Reversal: drop the check in `cleaned`.
- A directory token whose base is itself a glob (`web/*/`, `web/*/**`, `web/*/**/*`) is returned as the glob `web/*/**/*` for `Expand`, because a literal prefix `web/*/` would lease nothing. Reversal: return it unread.
- An unclosed backtick: the text after it is unread, with the backtick shown. Reversal: none needed.
- A symbol outside backticks, such as an arrow, counts as a word and is unread, because it may mean a range. Reversal: treat `unicode.IsSymbol` as punctuation in `hasWords`.
- `Expand` also takes a directory prefix ending in `/` and matches every file below it, so row 3 can run every form `Parse` returns through one check. Reversal: drop the suffix case in `mark`.
- `Cell` splits on every pipe. Neither live board escapes a pipe inside a cell (`grep -c '\\|'` gave 0 on both, 2026-10-16). Reversal: honour `\|`.

The live boards, run through `Cell` and `Parse` on 2026-10-16 by a test that was not committed (it read the untracked boards): wk's board gives 12 rows, 8 with unread text, every one of them a parenthesis, a `~/.wk/` token or the word "to" in row 2's range. ezhomesteading's board gives 72 rows, 34 with unread text: parenthetical notes such as "(new)," and "(line 326 only, ...)", prose such as "per item in the audit" (row 77) and "filled by the session that claims it" (row 198), `~` tokens (rows 196, 200), and `web/.../close-market-day.tsx` in row 70, which is unread because it holds `..`.

Findings for row 3, none of which is an edit here:

- An exact path that matches no tracked file is either a new file or not a path at all. Row 76 yields `GetChat` (a function name) as a path, and wk rows 6 to 12 yield `daemon.go` and the like, which the author meant inside the directory named before the parenthesis but which `Parse` reads at the repository root. Pass `paths` through `Expand` against `git ls-files` and print its `unmatched` beside `unread`, so the session sees both.
- A directory written without a trailing `/` (ezhomesteading row 200's `internal/slot`) is an exact path to `Parse`. Whether the lease store's overlap rule covers files below it depends on row 7's prefix rule; row 3 should check that, or treat an exact path that is a prefix of tracked files plus `/` as a directory.
- A range written as two globs and "to" (wk row 2, `passoff/3-*.md` to `passoff/5-*.md`) leases only its two ends; the "to" comes back unread, so `wk claim` must print it.
- Lease directory prefixes as `Parse` returns them, not as `Expand`'s matched files, so a new file under the directory is covered. Lease the matched files only for globs.

Fix round 1, 2026-10-16, after an Opus 5.5 audit of `3c8c8ac` that found no blocker. A token `dir/**/` became the glob `dir/**/**/*`, so under the rule above (lease only a glob's matched files) a new file under `dir/` got no lease, silently; `trimDir` now strips every trailing `/`, `/**` and `/**/*`, so `dir/`, `dir/**`, `dir/**/` and `dir/**/*` all return `dir/`. Globs were returned uncleaned, so `./src/*.ts` and `src//*.ts` matched nothing; `classify` now cleans a glob as it cleans a path (`path.Clean` keeps `*`, `?` and `[`), `Expand` cleans each pattern before it matches, and a glob that matches nothing comes back in `unmatched` in its cleaned form. Both have tests. Under rule L1, `Parse` was split at its seam (the loop, and a `parsed` collector with `text` and `token`), and `matchSegments` gave its `**` loop to `matchAnyDepth`.

Questions answered: none were open. Left owed: the wiring, `wk claim`'s printing and the live proof on wk's own board are row 3's.
