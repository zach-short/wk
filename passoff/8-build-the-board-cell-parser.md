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
