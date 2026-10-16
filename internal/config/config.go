// Package config reads the per-repository dials from ~/.wk/<repo>.toml.
//
// Every dial has a generic default, so nothing specific to one repository is compiled in; the
// repository's own values live in its file.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// Command is one program run inside a checkout, with Dir relative to the checkout's root.
type Command struct {
	Dir string   `toml:"dir"`
	Run []string `toml:"run"`
}

type Template struct {
	Path      string    `toml:"path"`
	RefreshOn string    `toml:"refresh_on"`
	CloneDirs []string  `toml:"clone_dirs"`
	CopyFiles []string  `toml:"copy_files"`
	PostClone []Command `toml:"post_clone"`
	Build     []Command `toml:"build"`
}

type Gates struct {
	Env map[string]string `toml:"env"`
}

type Workspace struct {
	Dir           string   `toml:"dir"`
	Branch        string   `toml:"branch"`
	Base          string   `toml:"base"`
	Sparse        bool     `toml:"sparse"`
	SparseExclude []string `toml:"sparse_exclude"`
}

type Hook struct {
	Dir    string `toml:"dir"`
	Branch string `toml:"branch"`
}

type GC struct {
	QuietMinutes         int `toml:"quiet_minutes"`
	ApplyIntervalMinutes int `toml:"apply_interval_minutes"`
}

type New struct {
	MaxSecondsFull   float64 `toml:"max_seconds_full"`
	MaxSecondsSparse float64 `toml:"max_seconds_sparse"`
}

type Pool struct {
	Size int `toml:"size"`
}

type Config struct {
	Template  Template  `toml:"template"`
	Gates     Gates     `toml:"gates"`
	Workspace Workspace `toml:"workspace"`
	Hook      Hook      `toml:"hook"`
	GC        GC        `toml:"gc"`
	New       New       `toml:"new"`
	Pool      Pool      `toml:"pool"`
}

const (
	RowPlaceholder  = "{row}"
	NamePlaceholder = "{name}"
)

func Defaults() Config {
	return Config{
		Template: Template{
			Path:      ".claude/worktrees/wk-template",
			CloneDirs: []string{"node_modules"},
		},
		Gates: Gates{Env: map[string]string{}},
		Workspace: Workspace{
			Dir:    ".claude/worktrees/wk-" + RowPlaceholder,
			Branch: "wk/" + RowPlaceholder,
			Base:   "main",
		},
		Hook: Hook{
			Dir:    ".claude/worktrees/" + NamePlaceholder,
			Branch: "worktree-" + NamePlaceholder,
		},
		GC:   GC{QuietMinutes: 60, ApplyIntervalMinutes: 60},
		New:  New{MaxSecondsFull: 8, MaxSecondsSparse: 5},
		Pool: Pool{Size: 2},
	}
}

// Load reads path over the defaults. A missing file is not an error: found is false and the
// defaults come back. An unknown key is an error, so a typo never silently keeps a default.
func Load(path string) (cfg Config, found bool, err error) {
	cfg = Defaults()
	md, err := toml.DecodeFile(path, &cfg)
	if errors.Is(err, fs.ErrNotExist) {
		return Defaults(), false, nil
	}
	if err != nil {
		return Config{}, true, fmt.Errorf("%s: %w", path, err)
	}
	if undecoded := md.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		sort.Strings(keys)
		return Config{}, true, fmt.Errorf("%s: unknown keys: %s", path, strings.Join(keys, ", "))
	}
	if err := cfg.validate(); err != nil {
		return Config{}, true, fmt.Errorf("%s: %w", path, err)
	}
	return cfg, true, nil
}

func (c Config) validate() error {
	paths := map[string]string{
		"template.path":       c.Template.Path,
		"workspace.dir":       c.Workspace.Dir,
		"hook.dir":            c.Hook.Dir,
		"template.refresh_on": c.Template.RefreshOn,
	}
	for _, list := range []struct {
		key   string
		paths []string
	}{
		{"template.clone_dirs", c.Template.CloneDirs},
		{"template.copy_files", c.Template.CopyFiles},
		{"workspace.sparse_exclude", c.Workspace.SparseExclude},
	} {
		for i, p := range list.paths {
			paths[fmt.Sprintf("%s[%d]", list.key, i)] = p
		}
	}
	for _, cmds := range []struct {
		key  string
		list []Command
	}{{"template.post_clone", c.Template.PostClone}, {"template.build", c.Template.Build}} {
		for i, cmd := range cmds.list {
			if len(cmd.Run) == 0 {
				return fmt.Errorf("%s[%d].run is empty", cmds.key, i)
			}
			paths[fmt.Sprintf("%s[%d].dir", cmds.key, i)] = cmd.Dir
		}
	}
	for key, p := range paths {
		if err := checkRelative(p); err != nil {
			return fmt.Errorf("%s: %w", key, err)
		}
	}
	for key, v := range map[string]string{"template.path": c.Template.Path, "workspace.base": c.Workspace.Base} {
		if v == "" {
			return fmt.Errorf("%s is empty", key)
		}
	}
	if !strings.Contains(c.Workspace.Dir, RowPlaceholder) || !strings.Contains(c.Workspace.Branch, RowPlaceholder) {
		return fmt.Errorf("workspace.dir and workspace.branch must both hold %s", RowPlaceholder)
	}
	if !strings.Contains(c.Hook.Dir, NamePlaceholder) || !strings.Contains(c.Hook.Branch, NamePlaceholder) {
		return fmt.Errorf("hook.dir and hook.branch must both hold %s", NamePlaceholder)
	}
	return nil
}

// checkRelative refuses an absolute path or one that climbs out: every path in the file is
// relative to the repository's main checkout and must stay inside it.
func checkRelative(p string) error {
	if p == "" {
		return nil
	}
	if filepath.IsAbs(p) {
		return fmt.Errorf("%q is absolute; paths are relative to the main checkout", p)
	}
	for _, part := range strings.Split(filepath.ToSlash(p), "/") {
		if part == ".." {
			return fmt.Errorf("%q leaves the main checkout", p)
		}
	}
	return nil
}
