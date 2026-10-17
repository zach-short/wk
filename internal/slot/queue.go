package slot

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/flock"
)

// record is written into a place file by its holder and into a ticket by its waiter. A reader
// trusts a record only while its file is locked: a released file keeps its last record.
type record struct {
	PID       int       `json:"pid"`
	Workspace string    `json:"workspace"`
	Command   string    `json:"command"`
	Start     time.Time `json:"start"`
}

// queue is one class under <slots.dir>/<class>/ (SB-5): place-<i>.lock for each place,
// admit.lock, a seq counter, and queue/<seq>.ticket for each waiter.
type queue struct {
	dir       string
	class     string
	capacity  int
	fifo      bool
	minFreeMB int
	memFree   func() (int, error)
}

type ticket struct {
	lock *flock.Lock
	path string
	seq  int64
}

type place struct {
	lock *flock.Lock
	path string
}

// waitState is what one pass saw, for the line a waiter prints and for its next poll.
type waitState struct {
	ahead   int
	holders []record
	memLow  bool
	memMB   int
	memErr  error
}

type queuedTicket struct {
	path string
	seq  int64
}

const ticketSuffix = ".ticket"

func newQueue(m Machine, class string, memFree func() (int, error)) queue {
	return queue{
		dir:       filepath.Join(m.Dir, class),
		class:     class,
		capacity:  m.Classes[class].Capacity,
		fifo:      m.Order == orderFIFO,
		minFreeMB: m.MinFreeMB,
		memFree:   memFree,
	}
}

func (q queue) queueDir() string            { return filepath.Join(q.dir, "queue") }
func (q queue) tmpDir() string              { return filepath.Join(q.dir, "tmp") }
func (q queue) placePath(i int) string      { return filepath.Join(q.dir, fmt.Sprintf("place-%d.lock", i)) }
func (q queue) admitPath() string           { return filepath.Join(q.dir, "admit.lock") }
func (q queue) seqPath() string             { return filepath.Join(q.dir, "seq") }
func ticketName(seq int64) string           { return fmt.Sprintf("%016d%s", seq, ticketSuffix) }
func (p *place) release() error             { return p.lock.Release() }
func (st waitState) withinReach(c int) bool { return !st.memLow && st.ahead < c }

func (q queue) prepare() error {
	for _, dir := range []string{q.queueDir(), q.tmpDir()} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

// withAdmit runs fn under the exclusive admit.lock. Every admission and every status probe
// happens under it, so no later waiter can win a race for a place and no probe takes a place
// from a waiter.
func (q queue) withAdmit(fn func() error) error {
	lock, err := flock.Take(q.admitPath(), true, nil)
	if err != nil {
		return err
	}
	err = fn()
	if relErr := lock.Release(); err == nil {
		err = relErr
	}
	return err
}

// enqueue gives the run its ticket. order = "any" takes none, so its waiters only poll.
func (q queue) enqueue(rec record) (*ticket, error) {
	if !q.fifo {
		return nil, nil
	}
	var t *ticket
	err := q.withAdmit(func() error {
		seq, err := q.nextSeq()
		if err == nil {
			t, err = q.writeTicket(seq, rec)
		}
		return err
	})
	return t, err
}

func (q queue) nextSeq() (int64, error) {
	data, err := os.ReadFile(q.seqPath())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return 0, err
	}
	var seq int64
	if s := strings.TrimSpace(string(data)); s != "" {
		if seq, err = strconv.ParseInt(s, 10, 64); err != nil {
			return 0, fmt.Errorf("%s: %w", q.seqPath(), err)
		}
	}
	seq++
	return seq, writeByRename(q.seqPath(), []byte(strconv.FormatInt(seq, 10)+"\n"))
}

// writeByRename replaces path whole, so a run killed mid-write never leaves a torn counter.
func writeByRename(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// writeTicket makes the ticket in tmp/ and locks it before the rename into queue/, so a ticket
// in queue/ is either locked by a live waiter or left behind by a dead one.
func (q queue) writeTicket(seq int64, rec record) (*ticket, error) {
	tmp := filepath.Join(q.tmpDir(), ticketName(seq))
	data, err := json.Marshal(rec)
	if err == nil {
		err = os.WriteFile(tmp, data, 0o644)
	}
	if err != nil {
		return nil, err
	}
	lock, ok, err := flock.TryTake(tmp, true)
	if err == nil && !ok {
		err = fmt.Errorf("%s: locked by another process", tmp)
	}
	t := &ticket{lock: lock, path: filepath.Join(q.queueDir(), ticketName(seq)), seq: seq}
	if err == nil {
		err = os.Rename(tmp, t.path)
	}
	if err != nil {
		_ = os.Remove(tmp)
		t.cancel()
		return nil, err
	}
	return t, nil
}

// cancel removes the ticket before it drops the lock, so nobody sees it unlocked while it is
// still in queue/. It is safe to call twice and on a nil ticket.
func (t *ticket) cancel() {
	if t == nil || t.lock == nil {
		return
	}
	_ = os.Remove(t.path)
	_ = t.lock.Release()
	t.lock = nil
}

// tryAdmit is one pass. The run takes a place only when fewer live tickets are ahead of it
// than there are free places, and the memory dial allows it (SB-5, SB-19).
func (q queue) tryAdmit(t *ticket, rec record) (*place, waitState, error) {
	var p *place
	var st waitState
	err := q.withAdmit(func() (err error) {
		p, err = q.pass(t, rec, &st)
		return err
	})
	if err != nil {
		return nil, st, err
	}
	return p, st, nil
}

func (q queue) pass(t *ticket, rec record, st *waitState) (*place, error) {
	var err error
	if st.ahead, err = q.liveAhead(t); err != nil {
		return nil, err
	}
	free, holders, err := q.probePlaces()
	if err != nil {
		return nil, err
	}
	st.holders = holders
	if !q.admits(len(free), st) {
		releaseAll(free)
		return nil, nil
	}
	releaseAll(free[1:])
	return free[0], q.hold(free[0], t, rec)
}

func (q queue) admits(free int, st *waitState) bool {
	if st.ahead >= free {
		return false
	}
	if q.minFreeMB <= 0 {
		return true
	}
	mb, err := q.memFree()
	if err != nil {
		st.memErr = err
		return true
	}
	if mb < q.minFreeMB {
		st.memLow, st.memMB = true, mb
		return false
	}
	return true
}

// hold writes the holder's record while admit.lock is still held, so a status probe never reads
// a half-written one, and only then gives up the ticket.
func (q queue) hold(p *place, t *ticket, rec record) error {
	rec.Start = time.Now()
	data, err := json.Marshal(rec)
	if err == nil {
		err = os.WriteFile(p.path, data, 0o644)
	}
	if err != nil {
		_ = p.release()
		return err
	}
	t.cancel()
	return nil
}

// liveAhead counts the live tickets ahead of t and removes the dead ones among them.
func (q queue) liveAhead(t *ticket) (int, error) {
	if t == nil {
		return 0, nil
	}
	tickets, err := q.tickets()
	if err != nil {
		return 0, err
	}
	ahead := 0
	for _, other := range tickets {
		if other.seq >= t.seq {
			break
		}
		live, err := ticketLive(other.path)
		if err != nil {
			return 0, err
		}
		if live {
			ahead++
		}
	}
	return ahead, nil
}

// tickets lists queue/ in seq order: the names are zero-padded, and ReadDir sorts by name.
func (q queue) tickets() ([]queuedTicket, error) {
	entries, err := os.ReadDir(q.queueDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []queuedTicket
	for _, e := range entries {
		digits, ok := strings.CutSuffix(e.Name(), ticketSuffix)
		seq, err := strconv.ParseInt(digits, 10, 64)
		if ok && err == nil {
			out = append(out, queuedTicket{path: filepath.Join(q.queueDir(), e.Name()), seq: seq})
		}
	}
	return out, nil
}

// ticketLive reports whether a waiter still holds the ticket. A ticket anyone can lock belongs
// to a waiter that died without cleaning up, so it is removed here.
func ticketLive(path string) (bool, error) {
	lock, ok, err := flock.TryTake(path, true)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	err = os.Remove(path)
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
	}
	if relErr := lock.Release(); err == nil {
		err = relErr
	}
	return false, err
}

// probePlaces tries every place. Taking a place's lock is the only way to know it is free, so
// a free place comes back locked and the caller keeps it or releases it.
func (q queue) probePlaces() (free []*place, holders []record, err error) {
	for i := range q.capacity {
		path := q.placePath(i)
		lock, ok, err := flock.TryTake(path, true)
		if err != nil {
			releaseAll(free)
			return nil, nil, err
		}
		if ok {
			free = append(free, &place{lock: lock, path: path})
			continue
		}
		holders = append(holders, readRecord(path))
	}
	return free, holders, nil
}

func releaseAll(places []*place) {
	for _, p := range places {
		_ = p.release()
	}
}

// readRecord returns the zero record when the file cannot be read, which prints as an unknown
// holder rather than failing the run that only wanted to describe it.
func readRecord(path string) record {
	var rec record
	if data, err := os.ReadFile(path); err == nil {
		_ = json.Unmarshal(data, &rec)
	}
	return rec
}

// interval is the wait before the next pass: a waiter within reach of a place polls as often as
// takeCreateSlot does, and the rest every second, so a long queue costs little.
func (st waitState) interval(capacity int) time.Duration {
	if st.withinReach(capacity) {
		return 100 * time.Millisecond
	}
	return time.Second
}

var vmStatLine = regexp.MustCompile(`^Pages (free|inactive|speculative):\s+(\d+)\.?$`)
var vmStatPageSize = regexp.MustCompile(`page size of (\d+) bytes`)

// vmStatFreeMB is the memory dial's reading: free, inactive and speculative pages, which macOS
// hands out without swapping (SB-19).
func vmStatFreeMB() (int, error) {
	out, err := exec.Command("/usr/bin/vm_stat").Output()
	if err != nil {
		return 0, fmt.Errorf("vm_stat: %w", err)
	}
	return parseVMStat(string(out))
}

func parseVMStat(out string) (int, error) {
	size := vmStatPageSize.FindStringSubmatch(out)
	if size == nil {
		return 0, errors.New("vm_stat: no page size in its output")
	}
	pageBytes, _ := strconv.ParseInt(size[1], 10, 64)
	var pages int64
	found := 0
	for _, line := range strings.Split(out, "\n") {
		if m := vmStatLine.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			n, _ := strconv.ParseInt(m[2], 10, 64)
			pages += n
			found++
		}
	}
	if found != 3 {
		return 0, errors.New("vm_stat: free, inactive or speculative pages missing from its output")
	}
	return int(pages * pageBytes / (1 << 20)), nil
}
