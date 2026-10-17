package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Spawn starts exe with args from / in a session of its own, appends its stdout and stderr to
// logPath, and lets it go: the caller neither waits for it nor is its terminal's session.
func Spawn(exe string, args []string, logPath string) error {
	if transientBinary(exe) {
		return fmt.Errorf("%s is a go run build, which its build cache deletes; run `go install ./cmd/wk` and start that binary", exe)
	}
	log, err := os.OpenFile(logPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer func() { _ = log.Close() }()
	cmd := exec.Command(exe, args...)
	cmd.Dir = "/"
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	return cmd.Process.Release()
}

// transientBinary is wk.TransientBinary, copied so this package does not import internal/wk:
// a `go run` or `go test` build lives in a cache that is deleted under it.
func transientBinary(exe string) bool {
	return strings.Contains(exe, string(filepath.Separator)+"go-build")
}
