package slot

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func testQueue(t *testing.T, f fixture, class string) queue {
	t.Helper()
	m, found, err := LoadMachine(f.home)
	if err != nil || !found {
		t.Fatalf("LoadMachine: found %v, %v", found, err)
	}
	q := newQueue(m, class, nil)
	if err := q.prepare(); err != nil {
		t.Fatal(err)
	}
	return q
}

func TestQueueRemovesADeadTicketAhead(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	q := testQueue(t, f, "heavy")
	dead, err := q.enqueue(record{PID: 1})
	if err != nil {
		t.Fatal(err)
	}
	// The dead waiter's lock goes with its process; its file stays in queue/.
	if err := dead.lock.Release(); err != nil {
		t.Fatal(err)
	}
	live, err := q.enqueue(record{PID: 2})
	if err != nil {
		t.Fatal(err)
	}
	p, st, err := q.tryAdmit(live, record{PID: 2})
	if err != nil || p == nil || st.ahead != 0 {
		t.Fatalf("tryAdmit: place %v, %d ahead, %v; want a place with nobody ahead", p, st.ahead, err)
	}
	defer func() { _ = p.release() }()
	if exists(dead.path) || exists(live.path) {
		t.Fatalf("tickets left: dead %v, live %v", exists(dead.path), exists(live.path))
	}
}

func TestQueueALiveTicketAheadWaits(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "[slots.check]\ncapacity = 2\n")
	q := testQueue(t, f, "check")
	first, _ := q.enqueue(record{PID: 1})
	second, _ := q.enqueue(record{PID: 2})
	third, _ := q.enqueue(record{PID: 3})
	defer first.cancel()
	defer second.cancel()
	defer third.cancel()
	p, st, err := q.tryAdmit(third, record{PID: 3})
	if err != nil || p != nil || st.ahead != 2 {
		t.Fatalf("third with two places free and two ahead: place %v, %d ahead, %v; want a wait", p, st.ahead, err)
	}
	if st.interval(q.capacity) != time.Second {
		t.Fatalf("interval %v for a waiter out of reach, want 1s", st.interval(q.capacity))
	}
	p, st, err = q.tryAdmit(second, record{PID: 2})
	if err != nil || p == nil || st.ahead != 1 {
		t.Fatalf("second with two free and one ahead: place %v, %d ahead, %v; want a place", p, st.ahead, err)
	}
	_ = p.release()
}

func TestQueueSeqSurvivesRuns(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	q := testQueue(t, f, "heavy")
	for want := int64(1); want <= 3; want++ {
		tk, err := q.enqueue(record{})
		if err != nil || tk.seq != want {
			t.Fatalf("ticket %d: seq %v, %v", want, tk, err)
		}
		tk.cancel()
	}
	writeFile(t, q.seqPath(), "not a number\n")
	if _, err := q.enqueue(record{}); err == nil {
		t.Fatal("a broken seq counter gave a ticket; it must fail so the run goes on unslotted")
	}
}

// orderedCmd appends its name to order when it starts, holds briefly, and exits.
func orderedCmd(order, name string) []string {
	return []string{"sh", "-c", `echo "$2" >> "$1"; sleep 0.2`, "sh", order, name}
}

func TestQueueFiveWaitersStartInArrivalOrder(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	h, release := f.holder("heavy", "holder")
	order := f.path("order")
	var waiters []*proc
	for i := 1; i <= 5; i++ {
		waiters = append(waiters, start(t, f.spec("heavy"), orderedCmd(order, strconv.Itoa(i))...))
		eventually(t, "ticket "+strconv.Itoa(i), func() bool { return len(f.tickets("heavy")) == i })
	}
	release()
	h.waitCode(0)
	for _, w := range waiters {
		w.waitCode(0)
	}
	if got := readFile(t, order); got != "1\n2\n3\n4\n5\n" {
		t.Fatalf("start order %q, want arrival order", got)
	}
}

func TestQueueSixRunsAtCapacityTwoNeverOverlapMoreThanTwo(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "[slots.check]\ncapacity = 2\n")
	running, counts := f.path("running"), f.path("counts")
	if err := os.Mkdir(running, 0o755); err != nil {
		t.Fatal(err)
	}
	script := `touch "$1/$$"; ls "$1" | wc -l >> "$2"; sleep 0.5; rm "$1/$$"`
	var runs []*proc
	for range 6 {
		runs = append(runs, start(t, f.spec("check"), "sh", "-c", script, "sh", running, counts))
	}
	for _, r := range runs {
		r.waitCode(0)
	}
	most := 0
	for _, field := range strings.Fields(readFile(t, counts)) {
		n, _ := strconv.Atoi(field)
		most = max(most, n)
	}
	if most != 2 {
		t.Fatalf("at most %d ran at once, want exactly 2 at capacity 2", most)
	}
}

func TestQueueAKilledWaiterLeavesATicketTheNextOneRemoves(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	h, release := f.holder("heavy", "holder")
	order := f.path("order")
	w1 := start(t, f.spec("heavy"), orderedCmd(order, "1")...)
	eventually(t, "ticket 1", func() bool { return len(f.tickets("heavy")) == 1 })
	w2 := start(t, f.spec("heavy"), orderedCmd(order, "2")...)
	eventually(t, "ticket 2", func() bool { return len(f.tickets("heavy")) == 2 })
	w3 := start(t, f.spec("heavy"), orderedCmd(order, "3")...)
	eventually(t, "ticket 3", func() bool { return len(f.tickets("heavy")) == 3 })
	if err := syscall.Kill(w2.pid(), syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	if _, sig := w2.wait(); sig != syscall.SIGKILL {
		t.Fatalf("waiter 2 died of %v", sig)
	}
	release()
	h.waitCode(0)
	w1.waitCode(0)
	w3.waitCode(0)
	if got := readFile(t, order); got != "1\n3\n" {
		t.Fatalf("start order %q, want 1 then 3", got)
	}
	if got := f.tickets("heavy"); len(got) != 0 {
		t.Fatalf("tickets %v left after the queue drained", got)
	}
}

func TestQueueAKilledHolderFreesItsPlace(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	h, _ := f.holder("heavy", "holder")
	ran := f.path("waiter-ran")
	w := start(t, f.spec("heavy"), "touch", ran)
	eventually(t, "the waiter's ticket", func() bool { return len(f.tickets("heavy")) == 1 })
	if err := syscall.Kill(h.pid(), syscall.SIGKILL); err != nil {
		t.Fatal(err)
	}
	w.waitCode(0)
	if !exists(ran) {
		t.Fatal("the waiter's command did not run after the holder died")
	}
}

func TestParseVMStat(t *testing.T) {
	t.Parallel()
	out := `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                                     6746.
Pages active:                                 208533.
Pages inactive:                               198353.
Pages speculative:                              9316.
Pages throttled:                                   0.
`
	got, err := parseVMStat(out)
	if want := (6746 + 198353 + 9316) * 16384 / (1 << 20); err != nil || got != want {
		t.Fatalf("parseVMStat = %d, %v; want %d", got, err, want)
	}
	if _, err := parseVMStat("Pages free: 1.\n"); err == nil {
		t.Fatal("parseVMStat read output with no page size")
	}
	if _, err := parseVMStat(strings.SplitAfterN(out, "\n", 3)[0] + "Pages free: 1.\n"); err == nil {
		t.Fatal("parseVMStat read output missing two counts")
	}
}

func TestVMStatReadsThisMachine(t *testing.T) {
	t.Parallel()
	if _, err := os.Stat("/usr/bin/vm_stat"); err != nil {
		t.Skip("no /usr/bin/vm_stat on this system")
	}
	if mb, err := vmStatFreeMB(); err != nil || mb <= 0 {
		t.Fatalf("vmStatFreeMB = %d, %v", mb, err)
	}
}

func TestTicketNamesSortBySeq(t *testing.T) {
	t.Parallel()
	if a, b := ticketName(9), ticketName(10); a >= b || filepath.Ext(a) != ticketSuffix {
		t.Fatalf("ticket names %q and %q do not sort by seq", a, b)
	}
}
