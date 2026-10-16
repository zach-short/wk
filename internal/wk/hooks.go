package wk

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/zach-short/wk/internal/settings"
)

const settingsFile = ".claude/settings.local.json"

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

// HooksInstall writes wk's WorktreeCreate, WorktreeRemove and SubagentStop entries into the main
// checkout's settings file and the template's (BD-8). Never the user's ~/.claude/settings.json
// and never the tracked .claude/settings.json (D6): the hooks are this machine's, not the
// repository's.
func (e *Env) HooksInstall(exe string, out io.Writer) error {
	if err := checkInstalled(exe); err != nil {
		return err
	}
	return e.eachSettingsFile(out, func(path string) (bool, error) { return settings.Install(path, exe) })
}

// HooksUninstall removes wk's entries from the same two files and nothing else.
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
