package slot

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	runsLog   = "runs.log"
	errorsLog = "errors.log"
)

// logTimeLayout starts every log line, so ReadStatus can count the lines of the last 24 hours
// in any log of the slots directory, whoever wrote them.
const logTimeLayout = time.RFC3339

// appendLog stamps line with the time and appends it to <dir>/<name> in a single write, so the
// lines of concurrent runs never interleave. The logs are never rotated in this row (SB-20).
func appendLog(dir, name, line string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(filepath.Join(dir, name), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	_, err = f.WriteString(time.Now().Format(logTimeLayout) + "\t" + oneLine(line) + "\n")
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	return err
}

func oneLine(s string) string {
	return strings.NewReplacer("\r", " ", "\n", " ").Replace(s)
}
