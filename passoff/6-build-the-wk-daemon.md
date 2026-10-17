# 6. Build the wk daemon: a unix-socket server and client in `internal/daemon`

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
