// Package daemon is wk's unix-socket server and its client. Each side writes one JSON object
// per line: a request is {"op": "...", ...fields}, a reply is {"ok": true, "data": ...} or
// {"ok": false, "error": "..."}.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"os"
	"sync"

	"github.com/zach-short/wk/internal/flock"
)

// maxSocketPath is the 104-byte sun_path of macOS less its terminator (PD-13). A longer path
// fails at bind with an error that does not name it.
const maxSocketPath = 103

// Handler answers one op. req is the whole request object, op included; the value it returns
// becomes the reply's data, and its error the reply's message.
type Handler func(req json.RawMessage) (any, error)

// Server listens on Socket while it holds LockPath exclusively, so one daemon runs per lock.
type Server struct {
	Socket, LockPath string

	handlers map[string]Handler
	// mu runs handlers one at a time: the state behind them is one in-memory store.
	mu     sync.Mutex
	stop   chan struct{}
	once   sync.Once
	connMu sync.Mutex
	conns  map[net.Conn]struct{}
	wg     sync.WaitGroup
}

// Handle registers fn for op. Call it before ListenAndServe; ping and shutdown are built in.
func (s *Server) Handle(op string, fn Handler) {
	if s.handlers == nil {
		s.handlers = map[string]Handler{}
	}
	s.handlers[op] = fn
}

// ListenAndServe takes the lock, replaces any stale socket file, and serves until ctx ends or
// a client sends shutdown, when it returns nil. It refuses when another daemon holds the lock.
func (s *Server) ListenAndServe(ctx context.Context) error {
	if len(s.Socket) > maxSocketPath {
		return fmt.Errorf("socket path %s is %d bytes; macOS allows %d", s.Socket, len(s.Socket), maxSocketPath)
	}
	lock, ok, err := flock.TryTake(s.LockPath, true)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("a wk daemon is already running: %s is locked, its socket is %s", s.LockPath, s.Socket)
	}
	defer func() { _ = lock.Release() }()
	// Only the lock holder may remove the file: before the lock, it may be a live daemon's.
	if err := os.Remove(s.Socket); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	ln, err := net.Listen("unix", s.Socket)
	if err != nil {
		return err
	}
	s.stop, s.conns = make(chan struct{}), map[net.Conn]struct{}{}
	go func() {
		select {
		case <-ctx.Done():
		case <-s.stop:
		}
		_ = ln.Close() // also removes the socket file
	}()
	return s.accept(ln)
}

// accept serves connections until the listener closes, then closes every open connection and
// waits for its handler, so the caller can release the lock with nothing left running.
func (s *Server) accept(ln net.Listener) error {
	var err error
	for {
		var conn net.Conn
		conn, err = ln.Accept()
		if err != nil {
			break
		}
		s.track(conn, true)
		s.wg.Add(1)
		go s.serve(conn)
	}
	s.shutdown()
	_ = ln.Close()
	s.connMu.Lock()
	for conn := range s.conns {
		_ = conn.Close()
	}
	s.connMu.Unlock()
	s.wg.Wait()
	if errors.Is(err, net.ErrClosed) {
		return nil
	}
	return err
}

func (s *Server) track(conn net.Conn, add bool) {
	s.connMu.Lock()
	defer s.connMu.Unlock()
	if add {
		s.conns[conn] = struct{}{}
	} else {
		delete(s.conns, conn)
	}
}

func (s *Server) shutdown() { s.once.Do(func() { close(s.stop) }) }

// serve answers requests on one connection until the client closes it or sends bad JSON.
func (s *Server) serve(conn net.Conn) {
	defer s.wg.Done()
	defer s.track(conn, false)
	defer func() { _ = conn.Close() }()
	dec, enc := json.NewDecoder(conn), json.NewEncoder(conn)
	for {
		var req json.RawMessage
		if err := dec.Decode(&req); err != nil {
			if !errors.Is(err, io.EOF) {
				_ = enc.Encode(reply{Error: "bad request: " + err.Error()})
			}
			return
		}
		op, out := s.dispatch(req)
		if enc.Encode(out) != nil {
			return
		}
		if op == "shutdown" {
			s.shutdown()
		}
	}
}

type reply struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func (s *Server) dispatch(req json.RawMessage) (string, reply) {
	var head struct{ Op string }
	if err := json.Unmarshal(req, &head); err != nil {
		return "", reply{Error: "bad request: " + err.Error()}
	}
	switch head.Op {
	case "ping", "shutdown":
		return head.Op, reply{OK: true}
	}
	fn, ok := s.handlers[head.Op]
	if !ok {
		return head.Op, reply{Error: fmt.Sprintf("unknown op %q", head.Op)}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	data, err := fn(req)
	if err != nil {
		return head.Op, reply{Error: err.Error()}
	}
	return head.Op, reply{OK: true, Data: data}
}
