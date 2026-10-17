package gate

import (
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"time"
)

var (
	pageSize  = regexp.MustCompile(`page size of (\d+) bytes`)
	pageCount = regexp.MustCompile(`(?m)^Pages (free|inactive|speculative):\s+(\d+)\.`)
)

// FreeMB reads vm_stat and returns free plus inactive plus speculative memory
// in MiB: the pages macOS can hand to a new process without swapping.
func FreeMB() (int, error) {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0, fmt.Errorf("vm_stat: %w", err)
	}
	return parseVMStat(string(out))
}

func parseVMStat(text string) (int, error) {
	size := pageSize.FindStringSubmatch(text)
	if size == nil {
		return 0, fmt.Errorf("vm_stat: no page size in the first line")
	}
	bytesPerPage, _ := strconv.Atoi(size[1])
	found := map[string]int{}
	for _, m := range pageCount.FindAllStringSubmatch(text, -1) {
		found[m[1]], _ = strconv.Atoi(m[2])
	}
	pages := 0
	for _, kind := range []string{"free", "inactive", "speculative"} {
		n, ok := found[kind]
		if !ok {
			return 0, fmt.Errorf("vm_stat: no %q pages line", kind)
		}
		pages += n
	}
	return pages * bytesPerPage / (1 << 20), nil
}

// WaitForMemory returns once FreeMB is at least min, sampling every sample. It
// logs one line when it starts to wait and one when it stops, and returns the
// error of a sample it cannot read rather than wait on a number it lacks.
func WaitForMemory(min int, sample time.Duration, log io.Writer) error {
	return waitFor(min, sample, log, FreeMB)
}

func waitFor(min int, sample time.Duration, log io.Writer, free func() (int, error)) error {
	start := time.Now()
	for waiting := false; ; waiting = true {
		mb, err := free()
		if err != nil {
			return err
		}
		if mb >= min {
			if waiting {
				_, _ = fmt.Fprintf(log, "gate: %d MB free, resuming after %.0fs\n", mb, time.Since(start).Seconds())
			}
			return nil
		}
		if !waiting {
			_, _ = fmt.Fprintf(log, "gate: %d MB free is under %d MB, waiting\n", mb, min)
		}
		time.Sleep(sample)
	}
}
