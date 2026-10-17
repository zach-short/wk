package slot

import (
	"errors"
	"fmt"
	"io/fs"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// MatchEntry is one [[slots.match]] entry: a gate spelling as an agent types it, the class of
// place it takes, and the paths `wk run` trims after it (SB-12).
type MatchEntry struct {
	Spelling  string   `toml:"spelling"`
	Class     string   `toml:"class"`
	TrimAfter []string `toml:"trim_after"`
}

// HookConfig is the [hook] table. FailOpen false turns a slots file the hook cannot use into a
// deny of every Bash call (SB-14); TimeoutMS is the hook's own deadline (SB-13).
type HookConfig struct {
	FailOpen  bool `toml:"fail_open"`
	TimeoutMS int  `toml:"timeout_ms"`
}

// SlotsFile is ~/.wk/<repo>.slots.toml (SB-4). It is a file of its own, not a table in
// ~/.wk/<repo>.toml, so that an older wk binary, which never reads it, cannot refuse the
// repository file over a key it does not know and stop every WorktreeCreate.
type SlotsFile struct {
	Match []MatchEntry
	Hook  HookConfig
}

type slotsFileDoc struct {
	Slots struct {
		Match []MatchEntry `toml:"match"`
	} `toml:"slots"`
	Hook HookConfig `toml:"hook"`
}

// The scope's dials (SLOTS-SCOPE.md section 4: option G1 for fail_open, DESIGN.md section 5
// for the 200 ms). The spellings and classes have no default: they are a repository's facts and
// live in its file.
func slotsFileHookDefaults() HookConfig {
	return HookConfig{FailOpen: true, TimeoutMS: 200}
}

// A class names a directory under slots.dir and is a word of the rewritten command, so it may
// hold nothing a path or a shell would read specially.
var slotsFileClass = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// LoadSlotsFile reads path. A missing file is not an error: found is false. An unknown key or a
// bad entry is an error, as in config.Load, so a typo never silently drops a gate. Past the TOML
// syntax, an error still returns the file's [hook] table in sf.Hook, so the hook can honour
// fail_open = false for a file it cannot use; a file that does not decode keeps the defaults.
func LoadSlotsFile(path string) (sf SlotsFile, found bool, err error) {
	doc := slotsFileDoc{Hook: slotsFileHookDefaults()}
	md, err := toml.DecodeFile(path, &doc)
	if errors.Is(err, fs.ErrNotExist) {
		return SlotsFile{Hook: slotsFileHookDefaults()}, false, nil
	}
	if err != nil {
		return SlotsFile{Hook: slotsFileHookDefaults()}, true, fmt.Errorf("%s: %w", path, err)
	}
	sf = SlotsFile{Match: doc.Slots.Match, Hook: doc.Hook}
	if err := slotsFileCheck(md, sf); err != nil {
		return SlotsFile{Hook: doc.Hook}, true, fmt.Errorf("%s: %w", path, err)
	}
	return sf, true, nil
}

func slotsFileCheck(md toml.MetaData, sf SlotsFile) error {
	if err := slotsFileUnknownKeys(md); err != nil {
		return err
	}
	if sf.Hook.TimeoutMS <= 0 {
		return fmt.Errorf("hook.timeout_ms is %d; it must be above 0", sf.Hook.TimeoutMS)
	}
	seen := map[string]bool{}
	for i, e := range sf.Match {
		if err := slotsFileCheckEntry(e); err != nil {
			return fmt.Errorf("slots.match[%d]: %w", i, err)
		}
		key := strings.Join(strings.Fields(e.Spelling), " ")
		if seen[key] {
			return fmt.Errorf("slots.match[%d]: spelling %q is listed twice", i, key)
		}
		seen[key] = true
	}
	return nil
}

func slotsFileUnknownKeys(md toml.MetaData) error {
	undecoded := md.Undecoded()
	if len(undecoded) == 0 {
		return nil
	}
	keys := make([]string, len(undecoded))
	for i, k := range undecoded {
		keys[i] = k.String()
	}
	sort.Strings(keys)
	return fmt.Errorf("unknown keys: %s", strings.Join(keys, ", "))
}

func slotsFileCheckEntry(e MatchEntry) error {
	if len(strings.Fields(e.Spelling)) == 0 {
		return errors.New("spelling is empty")
	}
	if !slotsFileClass.MatchString(e.Class) {
		return fmt.Errorf("class %q must be letters, digits, - or _", e.Class)
	}
	for _, p := range e.TrimAfter {
		if err := slotsFileCheckTrim(p); err != nil {
			return err
		}
	}
	return nil
}

// The same refusal `wk run` makes (SB-12), made here too so a bad path shows up in errors.log
// at the first Bash call instead of as a refused trim after a build.
func slotsFileCheckTrim(p string) error {
	if p == "" || path.IsAbs(p) || strings.ContainsRune(p, 0) {
		return fmt.Errorf("trim_after %q must be a relative path", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == ".." {
			return fmt.Errorf("trim_after %q may not hold a .. segment", p)
		}
	}
	return nil
}
