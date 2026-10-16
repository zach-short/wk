package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/zach-short/wk/internal/config"
)

// Choices is what the person picked in the wizard.
type Choices struct {
	Base      string
	CloneDirs []string
	CopyFiles []string
	Installs  []Install
	Schedule  bool // install the launchd agent that runs wk gc --apply
	Warm      bool // build the template before the wizard ends
}

// Render writes the config file the choices make. Every other dial keeps its default, and is
// written out anyway so the file shows what there is to change.
func Render(repo string, c Choices) string {
	d := config.Defaults()
	var b strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&b, format+"\n", args...) }

	line("# wk config for %s, written by wk setup.", repo)
	line("# Paths are relative to the main checkout.")
	line("")
	line("[template]")
	line("path = %s", quote(d.Template.Path))
	if len(c.Installs) > 0 {
		line("# The template rebuilds when this file changes.")
		line("refresh_on = %s", quote(c.Installs[0].Lockfile))
	}
	line("# Copy-on-write clones of these make a workspace warm.")
	line("clone_dirs = %s", list(c.CloneDirs))
	line("# Copied into each workspace, never printed.")
	line("copy_files = %s", list(c.CopyFiles))
	for _, in := range c.Installs {
		line("")
		line("[[template.post_clone]]")
		line("dir = %s", quote(in.Dir))
		line("run = %s", list(in.Run))
	}
	line("")
	line("[workspace]")
	line("dir = %s", quote(d.Workspace.Dir))
	line("branch = %s", quote(d.Workspace.Branch))
	line("base = %s", quote(c.Base))
	line("")
	line("[hook]")
	line("dir = %s", quote(d.Hook.Dir))
	line("branch = %s", quote(d.Hook.Branch))
	line("")
	line("[gc]")
	line("# Idle: no change for this long and no process in it.")
	line("quiet_minutes = %d", d.GC.QuietMinutes)
	line("apply_interval_minutes = %d", d.GC.ApplyIntervalMinutes)
	return b.String()
}

func quote(s string) string { return strconv.Quote(s) }

func list(items []string) string {
	q := make([]string, len(items))
	for i, s := range items {
		q[i] = quote(s)
	}
	return "[" + strings.Join(q, ", ") + "]"
}

// ErrExists is returned when the config is already there: setup never overwrites one, since a
// config holds hand-written notes.
var ErrExists = errors.New("the config already exists")

// Write checks the text loads as a config, then creates path with it. It refuses an existing
// file.
func Write(path, text string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".setup-*.toml")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.WriteString(text); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if _, _, err := config.Load(tmp.Name()); err != nil {
		return fmt.Errorf("the config setup made does not load: %w", err)
	}
	// A hard link fails when path exists, so a config made meanwhile is never replaced.
	if err := os.Link(tmp.Name(), path); err != nil {
		if errors.Is(err, os.ErrExist) {
			return fmt.Errorf("%s: %w", path, ErrExists)
		}
		return err
	}
	return nil
}
