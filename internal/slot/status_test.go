package slot

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestStatusTwoHoldersAndThreeWaiters(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "[slots.check]\ncapacity = 2\n\n[slots.heavy]\ncapacity = 1\n")
	h1, release1 := f.holder("check", "h1")
	h2, release2 := f.holder("check", "h2")
	var waiters []*proc
	for i := 1; i <= 3; i++ {
		waiters = append(waiters, start(t, f.spec("check"), "true"))
		eventually(t, "ticket "+strconv.Itoa(i), func() bool { return len(f.tickets("check")) == i })
	}
	st, err := ReadStatus(f.home)
	if err != nil {
		t.Fatal(err)
	}
	checkStatusShape(t, st, []int{h1.pid(), h2.pid()}, pids(waiters))
	checkStatusText(t, st.Text(), f, h1, waiters)
	checkStatusJSON(t, st, h1.pid())
	release1()
	release2()
	for _, p := range append([]*proc{h1, h2}, waiters...) {
		p.waitCode(0)
	}
}

func pids(ps []*proc) []int {
	out := make([]int, len(ps))
	for i, p := range ps {
		out[i] = p.pid()
	}
	return out
}

func checkStatusShape(t *testing.T, st Status, holders, waiters []int) {
	t.Helper()
	if len(st.Classes) != 2 || st.Classes[0].Name != "check" || st.Classes[1].Name != "heavy" {
		t.Fatalf("classes %+v, want check then heavy", st.Classes)
	}
	check := st.Classes[0]
	held := map[int]bool{}
	for _, p := range check.Places {
		if p.Holder == nil {
			t.Fatalf("check place %d is free; want both held", p.Index)
		}
		held[p.Holder.PID] = true
	}
	if !held[holders[0]] || !held[holders[1]] {
		t.Fatalf("check holders %v, want pids %v", held, holders)
	}
	if len(check.Waiters) != 3 {
		t.Fatalf("waiters %+v, want three", check.Waiters)
	}
	for i, w := range check.Waiters {
		if w.PID != waiters[i] || w.Command != "true" {
			t.Fatalf("waiter %d = %+v, want pid %d running true, in arrival order", i+1, w, waiters[i])
		}
	}
	if heavy := st.Classes[1]; heavy.Places[0].Holder != nil || len(heavy.Waiters) != 0 {
		t.Fatalf("heavy %+v, want one free place and no waiters", heavy)
	}
}

func checkStatusText(t *testing.T, text string, f fixture, h1 *proc, waiters []*proc) {
	t.Helper()
	for _, want := range []string{
		"machine file " + filepath.Join(f.home, machineFile) + ": places under " + f.slots + ", order fifo\n",
		"check: 2 places\n",
		fmt.Sprintf("pid %d in %s, sh -c ", h1.pid(), f.work),
		fmt.Sprintf("  check waiter 1: pid %d in %s, true, waiting ", waiters[0].pid(), f.work),
		fmt.Sprintf("  check waiter 3: pid %d in %s, true, waiting ", waiters[2].pid(), f.work),
		"heavy: 1 place\n  heavy place 0: free\n",
		"missed.log: no lines in the last 24 hours\n",
		"errors.log: no lines in the last 24 hours\n",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("status text lacks %q:\n%s", want, text)
		}
	}
}

func checkStatusJSON(t *testing.T, st Status, holderPID int) {
	t.Helper()
	data, err := json.Marshal(st)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Found   bool `json:"found"`
		Classes []struct {
			Name   string `json:"name"`
			Places []struct {
				Holder *struct {
					PID int `json:"pid"`
				} `json:"holder"`
			} `json:"places"`
			Waiters []struct {
				PID int `json:"pid"`
			} `json:"waiters"`
		} `json:"classes"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	check := doc.Classes[0]
	if !doc.Found || check.Name != "check" || len(check.Waiters) != 3 || check.Places[0].Holder == nil {
		t.Fatalf("status JSON %s", data)
	}
	if !strings.Contains(string(data), fmt.Sprintf(`"pid":%d`, holderPID)) {
		t.Fatalf("status JSON lacks holder pid %d: %s", holderPID, data)
	}
}

func TestStatusWithoutAMachineFile(t *testing.T) {
	t.Parallel()
	f := newFixture(t, "")
	st, err := ReadStatus(f.home)
	if err != nil || st.Found || len(st.Classes) != 0 {
		t.Fatalf("ReadStatus = %+v, %v", st, err)
	}
	if want := "no machine file at " + filepath.Join(f.home, machineFile) + ": every wk run runs unslotted\n"; !strings.HasPrefix(st.Text(), want) {
		t.Fatalf("text %q, want it to start %q", st.Text(), want)
	}
}

func TestStatusReadsTheLogs(t *testing.T) {
	t.Parallel()
	f := newFixture(t, capacityOne)
	old := time.Now().Add(-25 * time.Hour).Format(logTimeLayout)
	writeFile(t, filepath.Join(f.slots, errorsLog), old+"\trun\told failure\n")
	if err := appendLog(f.slots, errorsLog, "run\tnew failure"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(f.slots, "missed.log"), "a line with no stamp\n")
	for i := 1; i <= 7; i++ {
		line := fmt.Sprintf("class=heavy\tpid=%d\texit=0\twait=1.5s\trun=63.2s\tdir=/ws\tcmd=bun run build", i)
		if err := appendLog(f.slots, runsLog, line); err != nil {
			t.Fatal(err)
		}
	}
	st, err := ReadStatus(f.home)
	if err != nil {
		t.Fatal(err)
	}
	if st.Errors.Last24h != 1 || !strings.HasSuffix(st.Errors.Newest, "new failure") || st.Missed.Last24h != 0 {
		t.Fatalf("errors %+v, missed %+v; want one recent error and no stamped miss", st.Errors, st.Missed)
	}
	runs := st.Classes[0].LastRuns
	if len(runs) != lastRunsShown || runs[0].PID != 3 || runs[4].PID != 7 {
		t.Fatalf("last runs %+v, want pids 3 to 7", runs)
	}
	if r := runs[4]; r.Exit != "0" || r.WaitSeconds != 1.5 || r.RunSeconds != 63.2 || r.Workspace != "/ws" || r.Command != "bun run build" {
		t.Fatalf("parsed run %+v", r)
	}
	if !strings.Contains(st.Text(), "exit 0, waited 1.5s, ran 63.2s, pid 7 in /ws: bun run build\n") {
		t.Fatalf("text lacks the last run:\n%s", st.Text())
	}
}

func TestParseRunSkipsWhatItCannotRead(t *testing.T) {
	t.Parallel()
	for _, line := range []string{"", "no stamp\tclass=heavy", time.Now().Format(logTimeLayout) + "\tpid=1"} {
		if _, ok := parseRun(line); ok {
			t.Errorf("parseRun(%q) read a run", line)
		}
	}
}
