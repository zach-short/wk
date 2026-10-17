package slot

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendLogStampsOneLinePerCall(t *testing.T) {
	t.Parallel()
	dir := filepath.Join(t.TempDir(), "not", "made", "yet")
	for _, line := range []string{"first", "second\nwith a newline"} {
		if err := appendLog(dir, errorsLog, line); err != nil {
			t.Fatal(err)
		}
	}
	lines, err := readLines(filepath.Join(dir, errorsLog))
	if err != nil || len(lines) != 2 {
		t.Fatalf("lines %q, %v; want two", lines, err)
	}
	for i, want := range []string{"first", "second with a newline"} {
		stamp, text, _ := strings.Cut(lines[i], "\t")
		at, err := time.Parse(logTimeLayout, stamp)
		if err != nil || time.Since(at) > time.Minute || text != want {
			t.Errorf("line %d = %q; want a fresh stamp, a tab, then %q", i, lines[i], want)
		}
	}
}
