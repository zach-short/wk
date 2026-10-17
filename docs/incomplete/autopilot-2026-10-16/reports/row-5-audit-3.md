# Row 5, audit 3 (Opus 5.5, fresh), 2026-10-17

Launched 2026-10-17 15:01 EDT in the detached worktree `/Users/zachshort/Projects/wk/.claude/worktrees/ap-row-5-audit-3` at `98160da`, scoped to fix round 2's 42 lines; returned 15:07 EDT. Diffstat 22 files, 1957 insertions, 61 deletions; the round 5 files, 42 insertions, 13 deletions. Gate line exit 0 (15:01:01 to 15:01:57 EDT; `internal/wk` 49.2 s; `0 issues.`; `scratchpad/row-5-audit-3/gate.out`).

Verdict: non-blocking (1). B-new and N-b closed; no regression in the changed lines.

- Exit 75 with the real `wk run` (`max_wait_seconds = 1`, a `check` slot of capacity 1 held by another run): the loop ended with the start error, both records stayed `submitted`, `wk status` printed `last gate run: error ...`, and the next loop landed both once the holder was killed. Exit 75 mid-bisection (stand-in, batch of three, red first then 75), with and without a named path: the loop stopped, all three stayed `submitted`, `main` did not move, a later loop landed all three. Exit 1 still `fail` and blamed; 126 and 127 still start errors; a death by signal still `fail`.
- Status: a live pid prints "batch in flight ... worker <pid>", a dead pid "stale gate run ... which has stopped", pid 0 or no `pid` key "worker unknown"; the pid branch is read only inside `printGateRun`'s `gating` case.
- N1 (non-blocking, prose): `README.md:275` says a worker stopped for exit 75, 126 or 127 needs a person and meets the same error until the command or the slot is fixed; true for 126 and 127, not for 75, which the next worker clears once a place is free. Fixed at the land step.
- Observation (not in the round): the landing log echoes `gate <name>: fail, exit status 75` before `wrapperFailed` reads it as a start error; the record and status say "could not run". Cosmetic, the same for 126 and 127 since `a2bca12`.

Prose: absolute dates, no dashes, no contractions. `98160da` carries no trailer. `git status --porcelain` empty afterwards; probe sources kept in `row-5-audit-3/audit3_probe_test.go` and `audit3_probe2_test.go`.
