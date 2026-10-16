package wk

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Launchd is where the gc agent goes and how launchctl is reached. The command line fills it
// with the real ~/Library/LaunchAgents and launchctl; the self-test with a directory and a
// recorder, so no test ever loads an agent into the owner's session.
type Launchd struct {
	AgentsDir string
	Domain    string
	Control   func(args ...string) error
}

// Label names the agent per repository, so two repositories get two agents.
func (e *Env) Label() string { return "com.github.zach-short.wk.gc." + e.Repo.Name }

func (e *Env) GCLogPath() string { return filepath.Join(e.State, "gc.log") }

func (l Launchd) plistPath(label string) string {
	return filepath.Join(l.AgentsDir, label+".plist")
}

// ScheduleInstall writes the launchd agent that runs `wk gc --apply` every
// gc.apply_interval_minutes (D9, GATE 2 question 2) and loads it, replacing any earlier copy.
// exe is the wk binary the agent runs, and must be an installed one: a `go run` binary lives in
// a build cache that is deleted under it.
func (e *Env) ScheduleInstall(exe string, l Launchd, out io.Writer) error {
	if err := checkInstalled(exe); err != nil {
		return err
	}
	if err := os.MkdirAll(l.AgentsDir, 0o755); err != nil {
		return err
	}
	path := l.plistPath(e.Label())
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, e.plist(exe), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		return err
	}
	// bootout fails when the agent is not loaded, which is the usual case on a first install.
	_ = l.Control("bootout", l.Domain+"/"+e.Label())
	if err := l.Control("bootstrap", l.Domain, path); err != nil {
		return fmt.Errorf("launchctl bootstrap %s: %w; the plist is written but not loaded", path, err)
	}
	_, _ = fmt.Fprintf(out, "%s: wk gc --apply every %d minutes, log %s\n", path, e.Cfg.GC.ApplyIntervalMinutes, e.GCLogPath())
	return nil
}

// ScheduleUninstall unloads the agent and deletes its plist. It is the reversal of install,
// and P2's daemon will call it when it takes the schedule over.
func (e *Env) ScheduleUninstall(l Launchd, out io.Writer) error {
	path := l.plistPath(e.Label())
	_ = l.Control("bootout", l.Domain+"/"+e.Label())
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	_, _ = fmt.Fprintf(out, "%s: removed\n", path)
	return nil
}

// plist runs gc from / so that the agent's own cwd is inside no worktree, with the PATH of the
// shell that installed it, because launchd's own PATH holds no Homebrew git.
func (e *Env) plist(exe string) []byte {
	path := os.Getenv("PATH")
	if path == "" {
		path = "/usr/bin:/bin:/usr/sbin:/sbin"
	}
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
`)
	key := func(k string) { fmt.Fprintf(&b, "\t<key>%s</key>\n", k) }
	str := func(indent, v string) {
		fmt.Fprintf(&b, "%s<string>", indent)
		_ = xml.EscapeText(&b, []byte(v))
		b.WriteString("</string>\n")
	}
	key("Label")
	str("\t", e.Label())
	key("ProgramArguments")
	b.WriteString("\t<array>\n")
	for _, a := range []string{exe, "--repo", e.Repo.Primary, "gc", "--apply"} {
		str("\t\t", a)
	}
	b.WriteString("\t</array>\n")
	key("EnvironmentVariables")
	b.WriteString("\t<dict>\n")
	for _, kv := range [][2]string{{"PATH", path}, {"WK_HOME", filepath.Dir(e.State)}} {
		fmt.Fprintf(&b, "\t\t<key>%s</key>\n", kv[0])
		str("\t\t", kv[1])
	}
	b.WriteString("\t</dict>\n")
	key("WorkingDirectory")
	str("\t", "/")
	key("StartInterval")
	fmt.Fprintf(&b, "\t<integer>%d</integer>\n", e.Cfg.GC.ApplyIntervalMinutes*60)
	key("RunAtLoad")
	b.WriteString("\t<false/>\n")
	key("ProcessType")
	str("\t", "Background")
	key("StandardOutPath")
	str("\t", e.GCLogPath())
	key("StandardErrorPath")
	str("\t", e.GCLogPath())
	b.WriteString("</dict>\n</plist>\n")
	return b.Bytes()
}
