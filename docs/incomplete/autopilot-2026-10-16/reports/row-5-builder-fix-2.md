# Row 5, builder fix round 2 (Opus 5.5), 2026-10-17

Sent 2026-10-17 14:57 EDT with audit 2's B-new and N-b (N-a as a README sentence, N-c left); returned 15:00 EDT. Commit `98160da` on `ap/row-5` (5 files, 42 insertions, 13 deletions: `internal/wk/gatestart.go`, `internal/wk/status.go`, `internal/wk/gatekeeper_test.go`, `README.md`, the item file). Whole change 22 files, 1957 insertions, 61 deletions; patch rewritten (126054 bytes). Gate line exit 0 at 14:59 EDT in 48 s, load 5.04, `internal/wk` 42.3 s, `0 issues.` `git merge --ff-only main` said already up to date.

B-new: `wrapperFailed` holds a `notRun` set of exit 75 (`exitTempFail`, `wk run` waited past `slots.max_wait_seconds`), 126 (`exitNotExecutable`) and 127 (`exitNotFound`), each named with its source in `internal/slot/run.go`; a slotted gate with one of them is a start error. Reversal: drop 75 from `notRun`. Test: a new exit 75 row in `TestSlottedGateExit127IsAStartError`.

N-b: a missing or zero pid in `gates.json` prints "batch in flight: ..., worker unknown", never "stale ... by worker 0". Reversal: read a missing pid as stale. Test: a pid 0 row in `TestStatusSaysAStoppedWorkersBatchIsStale`.

N-a: the README's batch-gate section says a slotted gate that exits 75, 126 or 127 stops the queue and blames nobody, including a slotted command that exits 126 or 127 for a red tree, and that a worker stopped for that reason needs a person because each worker the timer starts meets the same error. N-c left. The close-out records audit 2's proof re-run at `a2bca12` and a "Fix round 2, 2026-10-17" paragraph. Each new test row was run with its fix turned off and failed. The live proof was not re-run: the landing path did not change this round.
