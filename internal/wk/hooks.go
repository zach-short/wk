package wk

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/zach-short/wk/internal/settings"
	"github.com/zach-short/wk/internal/slot"
)

const (
	settingsFile        = ".claude/settings.local.json"
	trackedSettingsFile = ".claude/settings.json"
)

// TransientBinary says whether exe is a `go run` or `go test` build, which lives in a build
// cache that is deleted under it, so nothing may be installed to run it or be forked from it.
func TransientBinary(exe string) bool {
	return strings.Contains(exe, string(filepath.Separator)+"go-build")
}

func checkInstalled(exe string) error {
	if !filepath.IsAbs(exe) {
		return fmt.Errorf("%q is relative; wk installs only the wk binary's absolute path", exe)
	}
	if TransientBinary(exe) {
		return fmt.Errorf("%s is a go run build; run `go install ./cmd/wk` and install from that binary", exe)
	}
	return nil
}

// HooksInstall writes wk's WorktreeCreate, WorktreeRemove, SubagentStop and PreToolUse entries,
// and the lease hook's two when leases are on, into the main checkout's settings file and the
// template's (BD-8), with the permission twin of
// each rule that covers a gate the PreToolUse hook wraps (SLOTS-PLAN.md SB-17). Never the user's
// ~/.claude/settings.json and never the tracked .claude/settings.json (D6): the hooks are this
// machine's, not the repository's. Both are read, because their rules need twins too.
func (e *Env) HooksInstall(exe string, out io.Writer) error {
	if err := checkInstalled(exe); err != nil {
		return err
	}
	twins, err := e.slotTwins(exe)
	if err != nil {
		return err
	}
	shared, err := e.sharedSources()
	if err != nil {
		return err
	}
	leases, err := e.leaseEntries()
	if err != nil {
		return err
	}
	return e.eachSettingsFile(out, func(path string) (bool, error) {
		return installWithTwins(path, exe, shared, twins, leases...)
	})
}

// leaseEntries are the lease hook's two entries when leases.enabled is true, and none otherwise,
// so an install with leases off also removes them (owner amendments, 2026-10-16). The hook reads
// its state under the main checkout's base name, which must be the name wk files the
// repository under, or the hook would read another file and allow.
func (e *Env) leaseEntries() ([]settings.Entry, error) {
	if !e.Cfg.Leases.Enabled {
		return nil, nil
	}
	if base := filepath.Base(e.Repo.Primary); base != e.Repo.Name {
		return nil, fmt.Errorf("the main checkout's name %q is not the repository's name %q; the lease hook cannot find its state", base, e.Repo.Name)
	}
	return settings.LeaseEntries(e.Repo.Primary), nil
}

// installWithTwins hands InstallTwins the whole twin set and the file's own rules among the
// sources, because it removes every twin it does not compute (row 213 audit, N2). The twins go
// in before the hook, so a refused source stops the install before anything wraps a gate: a
// wrapped gate without its deny twin would run with no sign.
func installWithTwins(path, exe string, shared [][]byte, twins []settings.Twin, leases ...settings.Entry) (bool, error) {
	local, err := readSource(path)
	if err != nil {
		return false, err
	}
	twinsChanged, err := settings.InstallTwins(path, append(slices.Clone(shared), local), twins)
	if err != nil {
		return false, err
	}
	hooksChanged, err := settings.Install(path, exe, leases...)
	return twinsChanged || hooksChanged, err
}

// slotTwins carry the exact text the hook inserts, from the same function, so each twin matches
// the rewritten command byte for byte. No slots file means no twins, which also removes those of
// an earlier install. A broken file stops the install: once it is fixed, the hook would wrap
// gates whose twins were never written.
func (e *Env) slotTwins(exe string) ([]settings.Twin, error) {
	sf, _, err := slot.LoadSlotsFile(filepath.Join(e.home, e.Repo.Name+".slots.toml"))
	if err != nil {
		return nil, err
	}
	twins := make([]settings.Twin, 0, len(sf.Match))
	for _, m := range sf.Match {
		prefix, ok := slot.Prefix(exe, m)
		if !ok {
			return nil, fmt.Errorf("spelling %q: the wk path or the entry cannot be quoted for the shell", m.Spelling)
		}
		twins = append(twins, settings.Twin{Spelling: m.Spelling, Prefix: prefix})
	}
	return twins, nil
}

// sharedSources are the user's settings and the tracked project settings, which every written
// file takes its twins from.
func (e *Env) sharedSources() ([][]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	var sources [][]byte
	for _, path := range []string{filepath.Join(home, ".claude", "settings.json"), e.inPrimary(trackedSettingsFile)} {
		src, err := readSource(path)
		if err != nil {
			return nil, err
		}
		sources = append(sources, src)
	}
	return sources, nil
}

// readSource reads a missing settings file as an empty source.
func readSource(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	return data, err
}

// HooksUninstall removes wk's entries and every twin from the same two files and nothing else.
func (e *Env) HooksUninstall(out io.Writer) error {
	return e.eachSettingsFile(out, settings.Uninstall)
}

func (e *Env) eachSettingsFile(out io.Writer, change func(string) (bool, error)) error {
	roots := []string{e.Repo.Primary}
	if _, err := os.Stat(e.TemplateDir()); err == nil {
		roots = append(roots, e.TemplateDir())
	} else {
		e.say("skip the template's settings: there is no template yet")
	}
	for _, root := range roots {
		path := filepath.Join(root, filepath.FromSlash(settingsFile))
		changed, err := change(path)
		if err != nil {
			return err
		}
		what := "unchanged"
		if changed {
			what = "updated"
		}
		_, _ = fmt.Fprintf(out, "%s: %s\n", path, what)
	}
	return nil
}
