# 6. Build the wk daemon: a unix-socket server and client in `internal/daemon`

> **Claimed `IN FLIGHT` 2026-10-16.**

*Opened 2026-10-16 by row 2, as a P2 sibling of row 3 (`docs/incomplete/landing-layer/PLAN-P2-P4.md` section 2).*

**Model: Opus 5.5. Lane C. Waits on 2.**

You are building one package of P2 of the landing layer, `wk`, a Go CLI that makes warm git worktrees for parallel agents. Read first: `PLAN-P2-P4.md` sections 0, 1 (PD-1 to PD-3) and 2, the row 6 line of its sub-row table (binding), `DESIGN.md` options M and Z and D12, `docs/conventions-go.md`, `internal/flock/flock.go` (the lock you reuse), `cmd/wk/main.go` `spawnFill` (the detached-process pattern you copy into a function) and `internal/settings/settings.go` `writeAtomic` (the atomic-rename pattern). You own only `internal/daemon/`; row 3 wires it into `cmd/wk` and `internal/wk`. Nothing in your package imports `internal/wk` or `internal/config`.

Do these, in order:
1. `internal/daemon/daemon.go`: `Server{Socket, LockPath string}` with `Handle(op string, fn func(req json.RawMessage) (any, error))` and `ListenAndServe(ctx) error`. One JSON object per line each way: in `{"op": "...", ...fields}`, out `{"ok": true, "data": ...}` or `{"ok": false, "error": "..."}`. Built-in ops `ping` and `shutdown` (which ends `ListenAndServe`). Single instance: take `LockPath` exclusive with `flock.TryTake`; when it is held, return an error naming the running instance; remove a stale socket file only after the lock is held. Refuse a socket path over 103 bytes (the 104-byte macOS `sun_path` less its terminator; PD-13) with a message naming the path. Handlers run one at a time (one mutex), because the state behind them is one in-memory store.
2. `internal/daemon/client.go`: `Client{Socket string}` with `Call(op string, in any, out any) error`; `ErrDown` (wrapped) when the socket file is missing or the connect is refused, so a caller can tell "not running" from "refused the request". A 2 s dial and read deadline (PD-13).
3. `internal/daemon/spawn.go`: `Spawn(exe string, args []string, logPath string) error`: starts `exe args...` from `/` in its own session (`Setsid`), stdout and stderr appended to `logPath`, released at once; refuses a transient binary by the same test as `wk.TransientBinary` (copy the one-line check; do not import `internal/wk`).
4. `internal/daemon/state.go`: `SaveJSON(path string, v any) error` by write-to-temp-and-rename in the same directory, mode 0644; `LoadJSON(path string, v any) (found bool, err error)` with a missing file as `found == false` and no error.
5. `internal/daemon/daemon_test.go`: a round trip over a socket in `t.TempDir()`; a second server on the same lock refused; a stale socket file replaced; `shutdown` ends the server; ten concurrent clients each get their own answer; an unknown op returns `ok: false`; a handler error reaches the client as its message; `ErrDown` on a missing socket; `SaveJSON` leaves no temp file and a reader never sees a partial file (write while reading in a loop).

Gate: `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`; the package tests above green.

Ask before building: nothing.
Not in scope, whoever asks: the lease ops, any subcommand, any config key, the hook; anything outside `internal/daemon/`. A need you find outside your package is a finding for row 3, written in your close-out, never an edit.

Hand back: the gate output, the exported surface (each function with its one-line contract), the findings for row 3, and a `## Close-out, <date>` section in this file. Git: Claude may run any git command, including `git add`, `git commit`, `git merge` and `git push`. Work in your own worktree branch. When the gates pass, commit naming the exact files, merge the branch into `main`, push, and remove the worktree. No `Co-Authored-By:` trailer and no "Generated with" line.

## Close-out, 2026-10-16

Built by an Opus 5.5 builder in the worktree `ap-row-6` on branch `ap/row-6`, from `main` at `344f8c7`. The gate line ran green (exit 0), and `go test -race -count=1 ./internal/daemon/...` passed in 2.3 s; 20 runs with `-count=20` passed in 27 s.

What changed. The new package `internal/daemon/` has five files and imports only `internal/flock` from this repository. `daemon.go` is the server, `client.go` the client, `spawn.go` the detached start, `state.go` the JSON state file, and `daemon_test.go` the tests that steps 1 to 5 list, plus a raw wire-format test and two `Spawn` tests. Nothing outside `internal/daemon/` was touched.

The exported surface, each with its contract:

- `type Handler func(req json.RawMessage) (any, error)`: answers one op. `req` is the whole request object, `op` included; the value becomes the reply's `data` and the error the reply's `error`.
- `type Server struct{ Socket, LockPath string }`: listens on `Socket` while it holds `LockPath` exclusively.
- `(*Server).Handle(op string, fn Handler)`: registers `fn` for `op`. Call it before `ListenAndServe`; the map is not guarded.
- `(*Server).ListenAndServe(ctx) error`: refuses a socket path over 103 bytes with the path named, before it takes the lock. Refuses with "a wk daemon is already running: <lock> is locked, its socket is <socket>" when the lock is held. Removes a stale socket file only after the lock is held. Serves until `ctx` ends or a client sends `shutdown`, then returns nil. Handlers run one at a time under one mutex; `ping` and `shutdown` are built in and answer `{"ok":true}` with no data.
- `var ErrDown`: wrapped in `Call`'s error when the socket file is missing (`ENOENT`) or the connect is refused (`ECONNREFUSED`, which a stale socket file gives).
- `type Client struct{ Socket string }` and `(Client).Call(op string, in, out any) error`: one connection per call, a 2 s dial and a 2 s deadline on the exchange (PD-13). `in` must encode as a JSON object or be nil; its fields go beside `op`. A reply of `ok: false` comes back as `errors.New(message)`, the daemon's text with no prefix.
- `Spawn(exe string, args []string, logPath string) error`: starts `exe args...` from `/` with `Setsid`, stdout and stderr appended to `logPath`, and releases the process. It refuses a path containing `/go-build` (the `wk.TransientBinary` test, copied) before it opens the log.
- `SaveJSON(path string, v any) error`: indented JSON plus a newline, written to a temporary file in the same directory, made mode 0644, synced, then renamed over `path`. The temporary file is removed on failure.
- `LoadJSON(path string, v any) (found bool, err error)`: a missing file is `false, nil`; a file that does not decode is `true` and an error naming the path.

What is now fixed, and how it was shown. Two probes made the tests fail on purpose and were then reverted. With `SaveJSON` replaced by a plain `os.WriteFile`, `TestSaveAtomic` failed at read 3 of its loop (counted from 0) with "unexpected end of JSON input". With the handler mutex removed, `TestTenClients` failed with two `DATA RACE` reports, because its handler bumps an unguarded counter.

Build-level calls, each with its reversal:

- `ListenAndServe` returns nil when `ctx` ends, as it does on `shutdown`. Reversal: return `ctx.Err()`.
- On stop, the server closes every open connection and waits for any running handler before it releases the lock, so the lock never frees while a handler still writes state. Reversal: none needed.
- `ping` returns no data. Reversal: return the pid, if `wk up` or `wk down` wants to print it.
- `SaveJSON` uses `os.CreateTemp` (a random temporary name) and an `fsync` before the rename, where `settings.writeAtomic` uses a fixed `.wk.tmp` name and no sync. A crash then leaves the old file or the new one, never an empty new one. Reversal: the fixed name and no sync.
- `Call` builds the request through a map, so an `op` field inside `in` is overwritten, never sent twice. Reversal: splice the bytes.
- A line that is not JSON gets one `ok: false` reply ("bad request: ...") and the connection closes.
- `Handle` takes the named type `Handler`; a plain `func(json.RawMessage) (any, error)` literal is assignable to it, so the prompt's signature still works. The `any` in the API breaks conventions rule T1 because the prompt names it; the prompt wins.

Findings for row 3 (none is an edit; each is outside this package):

- Nothing in `internal/daemon` creates `~/.wk/<repo>/`. `flock.TryTake` opens the lock with `O_CREATE` but makes no directory, so `wk up` must make the state directory before `ListenAndServe`.
- Run the daemon under `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`. A cancelled context closes the listener, which removes the socket file; a process killed without it leaves a stale socket, which the next start removes after it takes the lock.
- `wk down` can send `shutdown` and then poll `ping` until it gets `ErrDown`.
- Tests in `internal/wk` that start the daemon under a `WK_HOME` in `t.TempDir()` must keep the path short. On this machine `TMPDIR` is 50 bytes, and `t.TempDir()` adds the test name, so `<tempdir>/<repo>/wk.sock` can pass 103 bytes with a long test name. This package's tests use short test names and a one-letter socket name for the same reason.
- `cmd/wk`'s `spawnFill` repeats the body of `daemon.Spawn`. Row 3 may call `daemon.Spawn` there, which is optional.
- A handler that runs longer than 2 s makes its client time out (PD-13), while the handler keeps the mutex until it ends.

Questions answered. None was open; "Ask before building: nothing".

Left owed. Row 4 (P3) runs the landing loop as a goroutine of the daemon over the same in-memory state. The server's handler mutex is unexported, so that loop cannot take it. No reader needs it before row 4, so no method was added. Reversal when row 4 needs it: an exported `(*Server).Do(func())` that runs a function under the same mutex, or a lock in the store itself. Git, under this run's rules: committed on `ap/row-6` only; the orchestrating session merges, pushes and removes the worktree after its audit.

Sign-off, 2026-10-16, under /autopilot (agent-pushes). Builder: Opus 5.5. Auditor: Opus 5.5, round 1 (non-blocking: exported type Handler not asked for; any in four signatures as the prompt names them, T1; five functions over L1's length; no read deadline or size limit on a request; op matched without case; no test for the ctx-cancel stop path or Setsid). Worktree `.claude/worktrees/ap-row-6`, branch `ap/row-6`, commit a543f46 and the land commit. Relay passes: 0. Verdict: non-blocking, findings left open for row 3 (the read deadline) and row 4 (a way to take the handler mutex). Commit state: landed on main from ap/row-6, 2026-10-16. Not a Deep row.
