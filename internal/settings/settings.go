// Package settings adds and removes wk's hook entries in a Claude Code settings file.
//
// The file is shared: it holds the owner's permission list and keys other tools merge in, such
// as personal-config's availableModels. So every edit keeps each other key, in its order, and
// only ever touches the entries wk owns (BD-8).
package settings

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Entry is one hook wk registers. Verb is what follows the wk binary in its command. Sole
// marks an event whose hook gives Claude Code a single answer (a path, a removal), where a
// second tool's hook beside wk's would race it, so wk refuses to share the event.
type Entry struct {
	Event          string
	Verb           string
	TimeoutSeconds int
	Sole           bool
}

// Entries are the hooks `wk hooks install` writes. WorktreeCreate's timeout is Claude Code's
// own ceiling (PLAN.md 0.1), written out so a later default cannot cut short a workspace that
// waits on a template refresh.
var Entries = []Entry{
	{Event: "WorktreeCreate", Verb: "hook worktree-create", TimeoutSeconds: 600, Sole: true},
	{Event: "WorktreeRemove", Verb: "hook worktree-remove", TimeoutSeconds: 120, Sole: true},
	{Event: "SubagentStop", Verb: "hook subagent-stop", TimeoutSeconds: 30},
}

// Command is the hook command that runs verb on the wk binary at exe.
func Command(exe, verb string) string {
	if strings.ContainsAny(exe, " '\"\\$`") {
		exe = "'" + strings.ReplaceAll(exe, "'", `'\''`) + "'"
	}
	return exe + " " + verb
}

// Install makes path hold exactly one wk entry for each of Entries, running the wk binary at
// exe. It refuses a file that is not a JSON object, and another tool's hook on a Sole event.
// changed is false when the file already held those entries.
func Install(path, exe string) (changed bool, err error) {
	return edit(path, func(hooks *object) error {
		for _, en := range Entries {
			kept, err := withoutWK(hooks.get(en.Event), en.Event)
			if err != nil {
				return err
			}
			if en.Sole && len(kept) > 0 {
				return fmt.Errorf("%s already has a %s hook that is not wk's; wk will not add a second", path, en.Event)
			}
			entry, err := json.Marshal(map[string]any{
				"hooks": []map[string]any{{"type": "command", "command": Command(exe, en.Verb), "timeout": en.TimeoutSeconds}},
			})
			if err != nil {
				return err
			}
			if err := hooks.setOrRemove(en.Event, append(kept, entry)); err != nil {
				return err
			}
		}
		return nil
	})
}

// Uninstall removes wk's entries from every event and leaves every other hook. A missing file
// is no change.
func Uninstall(path string) (changed bool, err error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	return edit(path, func(hooks *object) error {
		events := make([]string, len(*hooks))
		for i, m := range *hooks {
			events[i] = m.key
		}
		for _, ev := range events {
			kept, err := withoutWK(hooks.get(ev), ev)
			if err != nil {
				return err
			}
			if err := hooks.setOrRemove(ev, kept); err != nil {
				return err
			}
		}
		return nil
	})
}

// edit applies change to the hooks object of path and writes the file only when its JSON
// changed. An empty event list, then an empty hooks object, are removed rather than kept.
func edit(path string, change func(*object) error) (bool, error) {
	before, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	root, err := parseObject(before)
	if err != nil {
		return false, fmt.Errorf("%s: %w; wk will not rewrite it", path, err)
	}
	hooks, err := parseObject(root.get("hooks"))
	if err != nil {
		return false, fmt.Errorf("%s: hooks: %w", path, err)
	}
	if err := change(&hooks); err != nil {
		return false, err
	}
	if err := root.setOrRemove("hooks", hooks); err != nil {
		return false, err
	}
	after, err := root.indent()
	if err != nil {
		return false, err
	}
	if sameJSON(before, after) {
		return false, nil
	}
	return true, writeAtomic(path, after)
}

// withoutWK reads an event's groups and drops wk's hooks from each, and every group left empty.
func withoutWK(raw json.RawMessage, event string) ([]json.RawMessage, error) {
	var groups []json.RawMessage
	if raw != nil {
		if err := json.Unmarshal(raw, &groups); err != nil {
			return nil, fmt.Errorf("hooks.%s: %w", event, err)
		}
	}
	var out []json.RawMessage
	for _, g := range groups {
		group, err := parseObject(g)
		if err != nil {
			return nil, fmt.Errorf("a %s group: %w", event, err)
		}
		var hooks []json.RawMessage
		if raw := group.get("hooks"); raw != nil {
			if err := json.Unmarshal(raw, &hooks); err != nil {
				return nil, fmt.Errorf("a %s group's hooks: %w", event, err)
			}
		}
		var kept []json.RawMessage
		for _, h := range hooks {
			var cmd struct {
				Command string `json:"command"`
			}
			if json.Unmarshal(h, &cmd) == nil && IsWK(cmd.Command) {
				continue
			}
			kept = append(kept, h)
		}
		if len(kept) == 0 {
			continue
		}
		if len(kept) == len(hooks) {
			out = append(out, g)
			continue
		}
		if err := group.setOrRemove("hooks", kept); err != nil {
			return nil, err
		}
		data, err := group.MarshalJSON()
		if err != nil {
			return nil, err
		}
		out = append(out, data)
	}
	return out, nil
}

// IsWK says whether command runs one of wk's hooks. Leading VAR=value words are skipped, so the
// fallback proof's WK_HOOK_FAIL=1 prefix is still recognised and removed.
func IsWK(command string) bool {
	command = strings.TrimSpace(command)
	for _, en := range Entries {
		rest, ok := strings.CutSuffix(command, " "+en.Verb)
		if !ok {
			continue
		}
		words := strings.Fields(rest)
		for len(words) > 1 && strings.Contains(words[0], "=") {
			words = words[1:]
		}
		return len(words) > 0 && filepath.Base(strings.Trim(strings.Join(words, " "), "'")) == "wk"
	}
	return false
}

func sameJSON(a, b []byte) bool {
	var ca, cb bytes.Buffer
	if json.Compact(&ca, a) != nil || json.Compact(&cb, b) != nil {
		return false
	}
	return bytes.Equal(ca.Bytes(), cb.Bytes())
}

func writeAtomic(path string, data []byte) error {
	mode := os.FileMode(0o644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".wk.tmp"
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// object is a JSON object that keeps its keys in file order, which encoding/json's maps lose.
type object []member

type member struct {
	key   string
	value json.RawMessage
}

// parseObject reads a JSON object. Empty or blank input is an empty object, as a settings file
// that does not exist yet is.
func parseObject(data []byte) (object, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return object{}, nil
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if tok, err := dec.Token(); err != nil || tok != json.Delim('{') {
		return nil, errors.New("not a JSON object")
	}
	var o object
	for dec.More() {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		key, _ := tok.(string)
		var value json.RawMessage
		if err := dec.Decode(&value); err != nil {
			return nil, err
		}
		o = append(o, member{key, value})
	}
	if _, err := dec.Token(); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err == nil {
		return nil, errors.New("trailing data after the JSON object")
	}
	return o, nil
}

func (o object) get(key string) json.RawMessage {
	for _, m := range o {
		if m.key == key {
			return m.value
		}
	}
	return nil
}

// setOrRemove sets key to value, in place when the key exists and at the end when it does not,
// or removes key when value is an empty list or object.
func (o *object) setOrRemove(key string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	empty := string(data) == "[]" || string(data) == "{}" || string(data) == "null"
	for i, m := range *o {
		if m.key != key {
			continue
		}
		if empty {
			*o = append((*o)[:i], (*o)[i+1:]...)
		} else {
			(*o)[i].value = data
		}
		return nil
	}
	if !empty {
		*o = append(*o, member{key, data})
	}
	return nil
}

func (o object) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, m := range o {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(m.key)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(m.value)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

func (o object) indent() ([]byte, error) {
	data, err := o.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var b bytes.Buffer
	if err := json.Indent(&b, data, "", "  "); err != nil {
		return nil, err
	}
	b.WriteByte('\n')
	return b.Bytes(), nil
}
