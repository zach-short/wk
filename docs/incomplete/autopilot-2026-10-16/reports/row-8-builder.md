# Row 8, builder report (Opus 5.5, agent aae094256279607c9), arrived 2026-10-16 21:34 EDT

Commit `3c8c8ac` on `ap/row-8` from main 344f8c7 (ff main: Already up to date). Not merged, not pushed. No hook block, no patch. No trailer.
Gate exit 0 (internal/wk 22.5 s, 0 issues; board 1.4 s). Package tests ok. Full gate log: scratchpad/row-8-gate.log.
Diffstat main...HEAD: internal/board/board.go 184, internal/board/board_test.go 195, passoff/8 33 (close-out); 3 files, +412. Files added: internal/board/board.go, internal/board/board_test.go.
Surface (stdlib only, no file reads, no git): Cell(text, row) (cell, err); Parse(cell) (paths, unread); Expand(patterns, files) (matched, unmatched). Directory tokens end in one "/"; globs keep *, ?, [; exact paths path.Clean; unread: absolute, ~, .., whitespace, prose outside backticks.
Build-level calls (7, reversals in the close-out): whitespace token unread; token cleaning to "." unread; directory token with a glob base becomes "<dir>/**/*"; text after an unclosed backtick unread; symbols such as "→" unread; Expand accepts directory prefixes; Cell ignores "\|" (neither live board has one).
Numbers claimed: live-board probe (uncommitted test) on 2026-10-16: wk board 12 rows, 8 with unread text; ezhomesteading board 72 rows, 34 with unread. Log: scratchpad/row-8-live.log.
Findings for row 3 (carried to row 3's launch brief): (1) pass paths through Expand against git ls-files and print unmatched beside unread (file names such as daemon.go in rows 6 to 12 parse at the repo root); (2) a directory without a trailing "/" (item 200's internal/slot) parses as an exact path; decide whether row 7's overlap rule covers files below it; (3) row 2's range "passoff/3-*.md to passoff/5-*.md" leases its two ends only, "to" comes back unread; (4) lease directory prefixes as returned, use Expand's matches only for globs.
Owed: wiring, wk claim printing and the live proof (row 3). Questions: none.
