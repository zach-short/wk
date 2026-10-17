# Row 8, audit round 2 (Opus 5.5, agent a5eb43c4fed2f7eb3), arrived 2026-10-16 21:45 EDT

Verdict: non-blocking (1). Reproduced: ff main, ff ap/row-8 344f8c7..a564db6, diffstat 3 files +457 matches, name-only matches, fix commit touches only the three files (+84/-39), no trailer. Gates: full gate exit 0 (board 1.9 s, wk 38.0 s, 0 issues); 7 tests PASS; deps only the package.
Scratch: dir/, dir/**, dir/**/, dir/**/*, dir/**/** all give ["dir/"]; ./web/*/**/ gives web/*/**/*; Expand(dir/) matches dir/a.go and dir/sub/b.go only; ./src/*.ts and src//*.ts fold to one glob src/*.ts; raw Expand(./src/*.ts) matches src/a.ts only; prose between tokens unread; **/*.go matches all depths; nine live cells pass. Diff read: signatures unchanged, no new exported name, all functions within 15 lines, close-out clean.
Finding: Expand returns a raw miss as written (board.go:154-166 appends p; only mark cleans); through Parse the miss is cleaned; the close-out line 53 overstates. Fixed at the land step.
