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

// Template is the tree every workspace clones from. With FollowBase, a base branch that moved
// without a RefreshOn change gets a light refresh (checkout and post-clone, no build), so a
// workspace made at the base's head can skip its own post-clone commands.
type Template struct {
	Path       string    `toml:"path"`
	RefreshOn  string    `toml:"refresh_on"`
	FollowBase bool      `toml:"follow_base"`
	CloneDirs  []string  `toml:"clone_dirs"`
	CopyFiles  []string  `toml:"copy_files"`
	PostClone  []Command `toml:"post_clone"`
	Build      []Command `toml:"build"`
}

type Gates struct {
	Env map[string]string `toml:"env"`
}

// Workspace names and shapes a `wk new` workspace. Each SparseExclude entry is a directory, or
// a pattern when it holds a glob character, such as docs/incomplete/**/*.png.
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

// GC holds gc's windows. A subagent worktree with a stop marker needs only StoppedQuietMinutes
// of quiet; one without a marker needs UnmarkedAgentQuietHours, because a running subagent
// leaves no process inside its worktree between tool calls.
type GC struct {
	QuietMinutes            int `toml:"quiet_minutes"`
	StoppedQuietMinutes     int `toml:"stopped_quiet_minutes"`
	UnmarkedAgentQuietHours int `toml:"unmarked_agent_quiet_hours"`
	ApplyIntervalMinutes    int `toml:"apply_interval_minutes"`
}

// New holds MaxParallel, the number of workspaces that clone and check out at once; the rest
// queue. APFS runs concurrent clones and file writes on one volume one after another, so more
// at once only makes every one of them finish late. The other fields are proof thresholds
// for PLAN.md, never timeouts.
type New struct {
	MaxParallel       int     `toml:"max_parallel"`
	MaxSecondsCold    float64 `toml:"max_seconds_cold"`
	MaxMBPerWorkspace int     `toml:"max_mb_per_workspace"`
	BurstSize         int     `toml:"burst_size"`
	BurstMaxSeconds   float64 `toml:"burst_max_seconds"`
}

// Pool is the warm workspaces made ahead under neutral names. Size counts the full ones and
// SparseSize the sparse ones; a sparse request never takes a full slot, because writing the
// sparse patterns over a full tree deletes every excluded file one by one.
type Pool struct {
	Size       int    `toml:"size"`
	SparseSize int    `toml:"sparse_size"`
	Dir        string `toml:"dir"`
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
	SlotPlaceholder = "{slot}"
)

func Defaults() Config {
	return Config{
		Template: Template{
			Path:       ".claude/worktrees/wk-template",
			FollowBase: true,
			CloneDirs:  []string{"node_modules"},
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
		GC:   GC{QuietMinutes: 60, StoppedQuietMinutes: 15, UnmarkedAgentQuietHours: 24, ApplyIntervalMinutes: 60},
		New:  New{MaxParallel: 2, MaxSecondsCold: 10, MaxMBPerWorkspace: 120, BurstSize: 10, BurstMaxSeconds: 60},
		Pool: Pool{Dir: ".claude/worktrees/wk-pool-" + SlotPlaceholder},
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
		"pool.dir":            c.Pool.Dir,
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
	if !strings.Contains(c.Pool.Dir, SlotPlaceholder) {
		return fmt.Errorf("pool.dir must hold %s", SlotPlaceholder)
	}
	if c.Pool.Size < 0 || c.Pool.SparseSize < 0 {
		return fmt.Errorf("pool.size and pool.sparse_size must not be negative")
	}
	// A quiet window of 0 would let gc remove a worktree a session made a second ago, before
	// the session's process has moved into it.
	for key, v := range map[string]int{
		"gc.quiet_minutes":              c.GC.QuietMinutes,
		"gc.stopped_quiet_minutes":      c.GC.StoppedQuietMinutes,
		"gc.unmarked_agent_quiet_hours": c.GC.UnmarkedAgentQuietHours,
		"gc.apply_interval_minutes":     c.GC.ApplyIntervalMinutes,
		"new.max_parallel":              c.New.MaxParallel,
	} {
		if v < 1 {
			return fmt.Errorf("%s is %d; it must be at least 1", key, v)
		}
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
