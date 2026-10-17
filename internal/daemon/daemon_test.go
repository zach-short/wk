package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/flock"
)

// paths returns a socket and a lock in a fresh temporary directory. Test names stay short:
// t.TempDir carries the name, and a socket path over 103 bytes is refused.
func paths(t *testing.T) (socket, lock string) {
	t.Helper()
	dir := t.TempDir()
	return filepath.Join(dir, "s"), filepath.Join(dir, "l")
}

// start runs s until the test ends and returns once it answers ping. The returned channel
// closes when ListenAndServe returns, and *result then holds what it returned.
func start(t *testing.T, s *Server) (done <-chan struct{}, result *error) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	closed, res := make(chan struct{}), new(error)
	go func() {
		*res = s.ListenAndServe(ctx)
		close(closed)
	}()
	t.Cleanup(func() {
		cancel()
		<-closed
	})
	waitUp(t, s.Socket, closed, res)
	return closed, res
}

func waitUp(t *testing.T, socket string, done <-chan struct{}, result *error) {
	t.Helper()
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(5 * time.Millisecond) {
		select {
		case <-done:
			t.Fatalf("server stopped before it answered: %v", *result)
		default:
		}
		if (Client{Socket: socket}).Call("ping", nil, nil) == nil {
			return
		}
	}
	t.Fatalf("no answer to ping on %s", socket)
}

type echo struct {
	N int `json:"n"`
}

func echoServer(t *testing.T) *Server {
	t.Helper()
	socket, lock := paths(t)
	s := &Server{Socket: socket, LockPath: lock}
	s.Handle("echo", func(req json.RawMessage) (any, error) {
		var in echo
		err := json.Unmarshal(req, &in)
		return in, err
	})
	return s
}

func TestRoundTrip(t *testing.T) {
	s := echoServer(t)
	start(t, s)
	var out echo
	if err := (Client{Socket: s.Socket}).Call("echo", echo{N: 7}, &out); err != nil {
		t.Fatal(err)
	}
	if out.N != 7 {
		t.Fatalf("got %d, want 7", out.N)
	}
}

func TestWireFormat(t *testing.T) {
	s := echoServer(t)
	start(t, s)
	conn, err := net.Dial("unix", s.Socket)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close() }()
	r := bufio.NewReader(conn)
	for _, c := range []struct{ in, want string }{
		{`{"op":"echo","n":3}`, `{"ok":true,"data":{"n":3}}`},
		{`{"op":"ping"}`, `{"ok":true}`},
		{`{"op":"nope"}`, `{"ok":false,"error":"unknown op \"nope\""}`},
	} {
		if _, err := conn.Write([]byte(c.in + "\n")); err != nil {
			t.Fatal(err)
		}
		line, err := r.ReadString('\n')
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.TrimSpace(line); got != c.want {
			t.Errorf("%s: got %s, want %s", c.in, got, c.want)
		}
	}
}

func TestSecondRefused(t *testing.T) {
	s := echoServer(t)
	start(t, s)
	other := &Server{Socket: s.Socket + "2", LockPath: s.LockPath}
	err := other.ListenAndServe(context.Background())
	if err == nil || !strings.Contains(err.Error(), "already running") || !strings.Contains(err.Error(), s.LockPath) {
		t.Fatalf("got %v, want a refusal naming %s", err, s.LockPath)
	}
	if _, statErr := os.Stat(other.Socket); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("the refused server made %s", other.Socket)
	}
	if err := (Client{Socket: s.Socket}).Call("ping", nil, nil); err != nil {
		t.Fatalf("the first server stopped answering: %v", err)
	}
}

func TestStaleSocket(t *testing.T) {
	socket, lock := paths(t)
	ln, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = ln.Close()
	err = (Client{Socket: socket}).Call("ping", nil, nil)
	if !errors.Is(err, ErrDown) {
		t.Fatalf("a stale socket file: got %v, want ErrDown", err)
	}
	start(t, &Server{Socket: socket, LockPath: lock})
}

func TestShutdown(t *testing.T) {
	s := echoServer(t)
	done, result := start(t, s)
	if err := (Client{Socket: s.Socket}).Call("shutdown", nil, nil); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
		if *result != nil {
			t.Fatalf("ListenAndServe: %v", *result)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not end ListenAndServe")
	}
	if _, err := os.Stat(s.Socket); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("socket file left behind: %v", err)
	}
	lock, ok, err := flock.TryTake(s.LockPath, true)
	if err != nil || !ok {
		t.Fatalf("lock still held after shutdown: ok=%v err=%v", ok, err)
	}
	_ = lock.Release()
}

func TestTenClients(t *testing.T) {
	s := echoServer(t)
	calls := 0 // unguarded on purpose: -race fails if two handlers ever run at once
	s.Handle("count", func(req json.RawMessage) (any, error) {
		calls++
		var in echo
		err := json.Unmarshal(req, &in)
		return in, err
	})
	start(t, s)
	var wg sync.WaitGroup
	errs := make([]error, 10)
	for i := range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var out echo
			errs[i] = (Client{Socket: s.Socket}).Call("count", echo{N: i}, &out)
			if errs[i] == nil && out.N != i {
				errs[i] = errors.New("got another client's answer")
			}
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("client %d: %v", i, err)
		}
	}
}

func TestUnknownOp(t *testing.T) {
	s := echoServer(t)
	start(t, s)
	err := (Client{Socket: s.Socket}).Call("nope", nil, nil)
	if err == nil || errors.Is(err, ErrDown) || !strings.Contains(err.Error(), `unknown op "nope"`) {
		t.Fatalf("got %v, want the daemon's unknown-op refusal", err)
	}
}

func TestHandlerError(t *testing.T) {
	s := echoServer(t)
	s.Handle("fail", func(json.RawMessage) (any, error) {
		return nil, errors.New("src/a.go is leased by row 4 in ws-4")
	})
	start(t, s)
	err := (Client{Socket: s.Socket}).Call("fail", nil, nil)
	if err == nil || err.Error() != "src/a.go is leased by row 4 in ws-4" {
		t.Fatalf("got %v, want the handler's message as is", err)
	}
}

func TestErrDownMissing(t *testing.T) {
	socket, _ := paths(t)
	err := (Client{Socket: socket}).Call("ping", nil, nil)
	if !errors.Is(err, ErrDown) {
		t.Fatalf("got %v, want ErrDown", err)
	}
}

func TestLongSocket(t *testing.T) {
	_, lock := paths(t)
	long := "/" + strings.Repeat("d", maxSocketPath)
	err := (&Server{Socket: long, LockPath: lock}).ListenAndServe(context.Background())
	if err == nil || !strings.Contains(err.Error(), long) {
		t.Fatalf("got %v, want a refusal naming the path", err)
	}
	if _, err := os.Stat(lock); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the lock was taken before the path was checked")
	}
}

type state struct {
	Items []int `json:"items"`
}

func TestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "leases.json")
	var got state
	if found, err := LoadJSON(path, &got); found || err != nil {
		t.Fatalf("missing file: found=%v err=%v", found, err)
	}
	if err := SaveJSON(path, state{Items: []int{1, 2}}); err != nil {
		t.Fatal(err)
	}
	if found, err := LoadJSON(path, &got); !found || err != nil || len(got.Items) != 2 {
		t.Fatalf("found=%v err=%v got=%v", found, err, got)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("want only leases.json in the directory, got %v (%v)", entries, err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o644 {
		t.Fatalf("mode: %v %v", info, err)
	}
}

func TestSaveAtomic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "leases.json")
	if err := SaveJSON(path, state{}); err != nil {
		t.Fatal(err)
	}
	stop := make(chan struct{})
	writeErr := make(chan error, 1)
	go func() {
		defer close(stop)
		for i := range 200 {
			if err := SaveJSON(path, state{Items: make([]int, (i%2)*5000)}); err != nil {
				writeErr <- err
				return
			}
		}
	}()
	for reads := 0; ; reads++ {
		select {
		case <-stop:
			if reads == 0 {
				t.Fatal("no read overlapped a write")
			}
			select {
			case err := <-writeErr:
				t.Fatal(err)
			default:
			}
			return
		default:
		}
		var got state
		if found, err := LoadJSON(path, &got); !found || err != nil {
			t.Fatalf("read %d: found=%v err=%v", reads, found, err)
		}
	}
}

func TestSpawnTransient(t *testing.T) {
	log := filepath.Join(t.TempDir(), "log")
	exe := filepath.Join(os.TempDir(), "go-build123", "b001", "exe", "wk")
	if err := Spawn(exe, nil, log); err == nil || !strings.Contains(err.Error(), "go run") {
		t.Fatalf("got %v, want a go run refusal", err)
	}
	if _, err := os.Stat(log); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("the refused spawn opened its log")
	}
}

func TestSpawn(t *testing.T) {
	log := filepath.Join(t.TempDir(), "log")
	if err := os.WriteFile(log, []byte("before\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := Spawn("/bin/sh", []string{"-c", "pwd; echo done"}, log); err != nil {
		t.Fatal(err)
	}
	var data []byte
	for end := time.Now().Add(2 * time.Second); time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		data, _ = os.ReadFile(log)
		if strings.HasSuffix(string(data), "done\n") {
			break
		}
	}
	if string(data) != "before\n/\ndone\n" {
		t.Fatalf("log: %q, want the old line kept and the child run from /", data)
	}
}
