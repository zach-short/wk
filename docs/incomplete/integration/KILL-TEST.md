# Kill test: a file store against the daemon for path leases

Run 2026-10-16 between 22:25 and 22:30 EDT by the integration session (Fable 5.1), in a scratch clone of wk at `029aaa6` with `ap/row-7` (`82e9dcf`) merged in for `internal/lease`. Nothing ran against a live repository or under `~/.wk`. This is experiment 1 of the review's prompt; decision 3 of the owner's record reads its table.

## Reading under decision 3

Files. The rule says files unless the file store shows more lost or stale leases, or more manual recovery steps, than the daemon. Lost leases: 0 for both stores in 900 kills. Stale leases: 0 for both. Manual recovery steps: 0 for the file store; one restart per crash for the daemon (100 of 100), and every hook call during the gap came back as a deny (100 of 100), which under D12 is a refused edit nobody holds a lease on. One failure class exists only in the daemon: a save that fails after the in-memory store has taken the claim (probe D below).

So, for the lead: retire the socket half of row 6 behind row 7's `lease.Store`, and build row 4's landing loop as a detached run-to-completion worker started by `wk submit` and by the launchd timer. The amendments are in the last section.

## What was built

- The file store (`internal/killtest/filestore.go`, 55 lines): `leases.json` beside `leases.lock`. A writer takes `flock(2)` exclusive on the lock file, loads the JSON, calls `lease.Store.Claim` or `Release`, writes a temp file and renames it (`daemon.SaveJSON`, which syncs first). A reader loads the JSON with no lock.
- The daemon (`cmd/kt daemon`): the landed `internal/daemon.Server` with `claim`, `release`, `check` and `dump` handlers over one in-memory `lease.Store`, saving `leases.json` by the same `SaveJSON` after each change and before it replies, as PD-3 and amendment 2 say. The server runs handlers one at a time under its mutex.
- The writer in both cases is a separate process (`cmd/kt claim`, `cmd/kt fclaim`), killed with `SIGKILL` at a delay drawn from the measured claim duration (natural timing), or aimed at the middle of a 30 ms stall placed either before the save or after the save and before the reply.
- After each kill the harness (`cmd/kt-harness`) reads `leases.json`, compares the daemon's memory with the file through a `dump` call that runs under the same mutex as the handlers, retries the same claim as a person would, and reads a free path the way the hook does.

## The table

100 kills per row. "Kill landed in time" counts kills that arrived before the writer exited on its own.

| Scenario | Kill landed in time | Ok and present | Committed, no ack | Lost with ack | Clean loss | Corrupt file | Temp files left | Stale lock | Refused for no reason | Recovery steps | Stale socket | Memory != disk | Follow-up claim p50 / max | Restart p50 / max |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| files, kill the writer, natural timing | 75 | 25 | 5 | 0 | 70 | 0 | 41 | 0 | 0 | 0 | 0 | 0 | 9.9 ms / 12.4 ms | n/a |
| daemon, kill the client, natural timing | 70 | 30 | 41 | 0 | 29 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 4.7 ms / 31 ms | n/a |
| daemon, kill the daemon, natural timing | 100 | 16 | 8 | 0 | 76 | 0 | 45 | 0 | 100 | 100 | 100 | 0 | 9.4 ms / 11.1 ms | 4.6 ms / 5.7 ms |
| files, kill the writer, 30 ms stall before the save | 100 | 0 | 0 | 0 | 100 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 10.4 ms / 14.4 ms | n/a |
| daemon, kill the client, 30 ms stall before the save | 100 | 0 | 100 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 5.1 ms / 15.3 ms | n/a |
| daemon, kill the daemon, 30 ms stall before the save | 100 | 0 | 0 | 0 | 100 | 0 | 0 | 0 | 100 | 100 | 100 | 0 | 41.9 ms / 44.9 ms | 5.9 ms / 8 ms |
| files, kill the writer, 30 ms stall after the save, before the reply | 100 | 0 | 100 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 5.3 ms / 8.3 ms | n/a |
| daemon, kill the client, 30 ms stall after the save, before the reply | 100 | 0 | 100 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 0 | 5.4 ms / 8 ms | n/a |
| daemon, kill the daemon, 30 ms stall after the save, before the reply | 100 | 0 | 100 | 0 | 0 | 0 | 0 | 0 | 100 | 100 | 100 | 0 | 4.8 ms / 9.6 ms | 4.8 ms / 8.3 ms |

Columns: "Ok and present" is the normal case (the writer printed `ok` and the claim is in the store). "Committed, no ack" means the claim is durable but the writer died before it could say so; the retry then prints `/r/N has already claimed row N`. "Lost with ack" would be a claim the writer called done that is not in the store; none occurred. "Clean loss" is a claim that never committed and whose writer never said it did. "Temp files left" are `.leases.json.*.tmp` files from a kill between `CreateTemp` and `rename`. "Refused for no reason" counts reads of a path nobody holds that came back deny or down. "Recovery steps" counts daemon restarts (the harness restarted it; a person would run `wk up`). The daemon's startup already removes a stale socket once it holds the lock, so each restart succeeded with no manual step beyond the restart itself. The harness printed one setup start for each daemon scenario that the table does not count as recovery.

Probe D, a failed save. With the state directory read-only for one claim, the daemon returned `permission denied`, then `check b/f.go` said `held by /r/b (row 2)`, and the same claim again said `/r/b has already claimed row 2; run wk release there first`, while `leases.json` did not hold `/r/b`. Memory was ahead of disk until a restart, which then dropped the claim the holder had been told it held. The file store under the same fault returned `permission denied`, `check` said `free`, and the claim again said `ok`.

Unkilled claim times: file store p50 10.2 ms, daemon p50 10.4 ms, both dominated by process start of the Go binary; the daemon's follow-up claims ran at 4.7 ms p50 against 9.9 ms for the file store, the one place the daemon is faster.

## Does the landing loop keep state between passes

No. `internal/landing` (row 10, on `main`) has no package-level state: `Ensure`, `Predict`, `Land`, `Sync`, `FastForward` and `Push` take and return hashes. `internal/changeset.Queue.Load` folds `queue.jsonl` to the last line per id on every call. Row 4's prompt step 3 names one piece of cross-pass memory, "stop with a status reason when main and the ref diverged", and `Ensure` recomputes that on every pass (`ErrDiverged`). A detached worker therefore has everything it needs in `queue.jsonl` and the refs. The one gap is F18: a crash between `Land`'s compare-and-swap and `Append(landed)` leaves a `submitted` record whose tree is already on the ref, and the next pass would land it again as a commit with no diff. The amendment below closes it.

## Findings for row 3, from the numbers

1. The retry after a kill must be idempotent. In 5 to 100 of 100 kills (by timing) the claim was durable and the writer never said so, and today's `Claim` refuses the retry with "has already claimed". `wk claim <row>` should treat a claim by the same root for the same row, with the same or fewer leases, as success and print the claim it found.
2. Reap temp files. A kill between `CreateTemp` and `rename` leaves `.leases.json.<n>.tmp` (41 and 45 of 100 at natural timing). The lock holder removes any such file older than one minute before it writes.
3. With the file store the hook reads `leases.json` directly and makes no socket call, so there is no down state and no deny for an absent process. D12's "deny when the socket does not answer" has no equivalent and is dropped.
4. If a daemon ever returns, it must mutate a copy and swap it in after the save, or roll back on a failed save (probe D).

## Amendments the lead applies under decision 3

- Row 3 (P2 integration): `internal/wk/lease.go` keeps `lease.Store` in `~/.wk/<repo>/leases.json` under `~/.wk/<repo>/leases.lock` (exclusive `flock` from read to rename, readers lock-free, temp reaping, idempotent re-claim). `wk up` and `wk down` are not built; no `daemon.go`, no socket, no `leases.json` load in a resident process. The hook (`wk hook lease --repo <primary>`) reads `leases.json` itself and judges with `lease.Judge`; its worktree list comes from a file `wk claim` and `wk release` maintain beside the store, never from git (PD-2 holds). D12's down-state deny is removed. Everything else in the row stands.
- Row 4 (P3 integration): step 3 becomes a detached run-to-completion worker, `wk land --loop` (the `spawnFill` pattern of P1), started by `wk submit` after it appends the record and by the launchd timer; one worker at a time under `~/.wk/<repo>/landing.lock`; each pass runs `Ensure`, then for each `submitted` record `Predict` then `Land`, appends `landed` or `refused`, and exits when the queue holds no `submitted` record. Before each `Land`, the worker appends a `landing` record carrying the predicted tree, and a pass that finds a `submitted` or `landing` record whose tree equals the tip's tree appends `landed` without a new commit (F18). PD-13's "a pass every 5 s" becomes the launchd timer's interval plus the start on every `submit`.
- Row 6 (`internal/daemon`, DONE): `SaveJSON`, `LoadJSON` and `Spawn` stay in use; `Server` and `Client` are kept on `main` with a doc comment that nothing calls them, and deleted by a later row once P3 lands with the worker. Reversal: wire them back behind the same `lease.Store`.
- Row 7 (HELD, `internal/lease`): under the standing rule this session takes the lead's option (a): land `82e9dcf` with `leases.bash_guard` default `false` and the gap list in the README; the Bash word walk becomes its own later row (a real parser once row 200's `mvdan.cc/sh` is on `main`, PD-4's reversal). Reason: with the walk off nothing silent is introduced, the edit-tool judgment held across three audits, and row 3 unblocks. Reversal: hold row 7 until the walk is rebuilt.

## Commands

In the scratch clone, `GOTOOLCHAIN=local`:

```bash
go build -o ../bin/kt ./cmd/kt && go build -o ../bin/kt-harness ./cmd/kt-harness
```

```bash
../bin/kt-harness ../bin/kt ./work-100 100
```

The harness and the two probe programs are in the scratch clone under `cmd/kt`, `cmd/kt-harness` and `internal/killtest`; they are not for `main`. The full log is `killtest-100.log` beside them.

## Row 13, for the board

The F1 and F2 fix landed from this session's worktree `fx/gc` as board row 13, with its item file `passoff/13-fix-gc-stopped-detached-and-reused-marker.md` tracked in the same change. This session does not write `PASSOFF.md` or `HANDOFF.md` while the autopilot lead is live, so the lead adds the row and the log line:

```
| 13 | Fix gc for a stopped subagent on a detached HEAD, and clear the stop marker on reuse | `DONE - 2026-10-16` | Fable 5.1 | J | nothing | `internal/wk/gc.go`; `internal/wk/hook.go`; `internal/wk/agent_test.go`; `internal/wk/hook_test.go` |
```

```
- 2026-10-16 · closed item 13 (gc F1 and F2: a stopped detached worktree with unlanded commits is kept; a reused worktree loses its stop marker), landed from fx/gc
```
