package gate

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// Captured from `vm_stat` on the owner's machine (macOS, arm64) on 2026-10-16.
const vmStatSample = `Mach Virtual Memory Statistics: (page size of 16384 bytes)
Pages free:                                    85360.
Pages active:                                 292939.
Pages inactive:                               274815.
Pages speculative:                             17767.
Pages throttled:                                   0.
Pages wired down:                             146877.
Pages purgeable:                                6404.
"Translation faults":                     3144410742.
Pages copy-on-write:                       435094039.
Pages zero filled:                         880294966.
Pages reactivated:                         156934067.
Pages purged:                               10546811.
File-backed pages:                            185395.
Anonymous pages:                              400126.
Pages stored in compressor:                   503324.
Pages occupied by compressor:                 189249.
Decompressions:                            118831217.
Compressions:                              143002428.
Pageins:                                    50573201.
Pageouts:                                     207998.
Swapins:                                      408932.
Swapouts:                                     671054.
`

func TestParseVMStat(t *testing.T) {
	got, err := parseVMStat(vmStatSample)
	if err != nil {
		t.Fatal(err)
	}
	// (85360 + 274815 + 17767) pages * 16384 bytes / 1 MiB = 5905 MiB.
	if got != 5905 {
		t.Errorf("parseVMStat = %d, want 5905", got)
	}
}

func TestParseVMStatRefusesMissingLines(t *testing.T) {
	for name, text := range map[string]string{
		"no page size":   strings.Replace(vmStatSample, "page size of 16384 bytes", "", 1),
		"no speculative": strings.Replace(vmStatSample, "Pages speculative:", "Pages other:", 1),
	} {
		if _, err := parseVMStat(text); err == nil {
			t.Errorf("%s: parseVMStat gave no error", name)
		}
	}
}

func TestWaitForLogsStartAndStop(t *testing.T) {
	samples := []int{1000, 1500, 2048}
	free := func() (int, error) {
		mb := samples[0]
		samples = samples[1:]
		return mb, nil
	}
	var log strings.Builder
	if err := waitFor(2048, time.Millisecond, &log, free); err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(log.String()), "\n")
	if len(lines) != 2 || !strings.Contains(lines[0], "1000 MB free is under 2048 MB") || !strings.Contains(lines[1], "2048 MB free") {
		t.Errorf("log = %q, want one wait line and one resume line", log.String())
	}
	if len(samples) != 0 {
		t.Errorf("%d samples left unread", len(samples))
	}
}

func TestWaitForQuietWhenAboveFloor(t *testing.T) {
	var log strings.Builder
	if err := waitFor(100, time.Millisecond, &log, func() (int, error) { return 200, nil }); err != nil {
		t.Fatal(err)
	}
	if log.Len() != 0 {
		t.Errorf("log = %q, want nothing", log.String())
	}
}

func TestWaitForReturnsReadError(t *testing.T) {
	want := errors.New("no vm_stat")
	err := waitFor(100, time.Millisecond, &strings.Builder{}, func() (int, error) { return 0, want })
	if !errors.Is(err, want) {
		t.Errorf("err = %v, want %v", err, want)
	}
}
