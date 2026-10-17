// Package settings adds and removes wk's hook entries, and the permission twins of the gates its
// hook wraps, in a Claude Code settings file.
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
	"slices"
	"strings"
)

// Entry is one hook wk registers. Verb is what follows the wk binary in its command. Sole
// marks an event whose hook gives Claude Code a single answer (a path, a removal), where a
// second tool's hook beside wk's would race it, so wk refuses to share the event. Guarded ends
// the command with guard. Matcher is the tool name Claude Code filters the event on. OnFailure
// "block" makes Claude Code refuse the tool call when the hook itself fails: a missing binary, a
// crash, a timeout (verified on 2.1.301, PLAN-P2-P4.md section 0).
type Entry struct {
	Event          string
	Verb           string
	TimeoutSeconds int
	Sole           bool
	Guarded        bool
	Matcher        string `json:"matcher,omitempty"`
	OnFailure      string
}

// Entries are the hooks `wk hooks install` writes. WorktreeCreate's timeout is Claude Code's
// own ceiling (PLAN.md 0.1), written out so a later default cannot cut short a workspace that
// waits on a template refresh. PreToolUse runs before every Bash call of every hooked session,
// so its timeout is short and it shares the event with the owner's other Bash hooks, such as
// the commit guard (SLOTS-PLAN.md SB-13, 0.2).
var Entries = []Entry{
	{Event: "WorktreeCreate", Verb: "hook worktree-create", TimeoutSeconds: 600, Sole: true},
	{Event: "WorktreeRemove", Verb: "hook worktree-remove", TimeoutSeconds: 120, Sole: true},
	{Event: "SubagentStop", Verb: "hook subagent-stop", TimeoutSeconds: 30},
	{Event: "PreToolUse", Verb: "hook pre-tool-use", TimeoutSeconds: 5, Guarded: true, Matcher: "Bash"},
}

// guard ends a Guarded entry's command. A binary older than the entry's verb prints its usage
// and exits 2, and exit 2 from a PreToolUse hook blocks the tool call, so a reinstall from an
// older commit would stop every Bash call of every hooked session; the guard turns that into no
// answer (SB-14). It relies on Claude Code running hook commands through a shell, which phase 3
// of SLOTS-PLAN.md checks.
const guard = " || true"

// leaseVerb starts the lease hook's verb; the main checkout's path follows it.
const leaseVerb = "hook lease --repo "

// LeaseEntries are the two PreToolUse entries of the lease hook for the repository whose main
// checkout is primary (D11, PLAN-P2-P4.md section 2). The edit tools' entry blocks on any
// failure of the hook, so a missing or broken binary refuses edits (D12). The Bash entry does
// not, and is guarded, because a missing or older binary would otherwise block every Bash call,
// the reinstall included; the hook never exits non-zero on purpose, it denies with JSON.
func LeaseEntries(primary string) []Entry {
	verb := leaseVerb + quote(primary)
	return []Entry{
		{Event: "PreToolUse", Verb: verb, TimeoutSeconds: 5, Matcher: "Edit|Write|MultiEdit|NotebookEdit", OnFailure: "block"},
		{Event: "PreToolUse", Verb: verb, TimeoutSeconds: 5, Matcher: "Bash", Guarded: true},
	}
}

// Command is the hook command that runs verb on the wk binary at exe.
func Command(exe, verb string) string {
	return quote(exe) + " " + verb
}

// quote single-quotes s for the shell when it holds a character the shell would read.
func quote(s string) string {
	if strings.ContainsAny(s, " '\"\\$`") {
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}
	return s
}

func (en Entry) command(exe string) string {
	if en.Guarded {
		return Command(exe, en.Verb) + guard
	}
	return Command(exe, en.Verb)
}

// hookGroup is one element of an event's list. The matcher comes first, as Claude Code's own
// files write it, and the hook stays a map, whose sorted keys are the bytes earlier wk versions
// wrote, so a reinstall over their entries changes nothing.
type hookGroup struct {
	Matcher string           `json:"matcher,omitempty"`
	Hooks   []map[string]any `json:"hooks"`
}

// Install makes path hold exactly one wk entry for each of Entries and extra, running the wk
// binary at exe; a wk entry of an event that is in neither is removed. It refuses a file that is
// not a JSON object, and another tool's hook on a Sole event. changed is false when the file
// already held those entries.
func Install(path, exe string, extra ...Entry) (changed bool, err error) {
	all := append(slices.Clone(Entries), extra...)
	return edit(path, func(root *object) error {
		return root.within("hooks", func(hooks *object) error {
			for _, ev := range events(all) {
				if err := put(hooks, ev, all, exe); err != nil {
					return err
				}
			}
			return nil
		})
	})
}

// events lists the events of entries once each, in first-seen order.
func events(entries []Entry) []string {
	var out []string
	for _, en := range entries {
		if !slices.Contains(out, en.Event) {
			out = append(out, en.Event)
		}
	}
	return out
}

// put replaces wk's hooks on event with one group for each of entries on that event, after
// every other tool's.
func put(hooks *object, event string, entries []Entry, exe string) error {
	kept, err := withoutWK(hooks.get(event), event)
	if err != nil {
		return err
	}
	for _, en := range entries {
		if en.Event != event {
			continue
		}
		if en.Sole && len(kept) > 0 {
			return fmt.Errorf("already has a %s hook that is not wk's; wk will not add a second", en.Event)
		}
		group, err := en.group(exe)
		if err != nil {
			return err
		}
		kept = append(kept, group)
	}
	return hooks.setOrRemove(event, kept)
}

func (en Entry) group(exe string) (json.RawMessage, error) {
	hook := map[string]any{"type": "command", "command": en.command(exe), "timeout": en.TimeoutSeconds}
	if en.OnFailure != "" {
		hook["onFailure"] = en.OnFailure
	}
	return marshal(hookGroup{Matcher: en.Matcher, Hooks: []map[string]any{hook}})
}

// Uninstall removes wk's entries from every event and every permission twin from allow, ask and
// deny (SB-17), and leaves every other hook and rule. A missing file is no change.
func Uninstall(path string) (changed bool, err error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return false, nil
	}
	return edit(path, func(root *object) error {
		if err := root.within("hooks", dropWK); err != nil {
			return err
		}
		return root.within("permissions", func(perms *object) error { return eachList(perms, withoutTwins) })
	})
}

func dropWK(hooks *object) error {
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
}

// edit applies change to the root object of path and writes the file only when its JSON
// changed. A file that does not exist and would come out empty is not created.
func edit(path string, change func(root *object) error) (bool, error) {
	before, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}
	root, err := parseObject(before)
	if err != nil {
		return false, fmt.Errorf("%s: %w; wk will not rewrite it", path, err)
	}
	if err := change(&root); err != nil {
		return false, fmt.Errorf("%s: %w", path, err)
	}
	after, err := root.indent()
	if err != nil {
		return false, err
	}
	if sameJSON(before, after) || (len(bytes.TrimSpace(before)) == 0 && len(root) == 0) {
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

// IsWK says whether command runs one of wk's hooks, plain or ending in guard. Leading VAR=value
// words are skipped, so the fallback proof's WK_HOOK_FAIL=1 prefix is still recognised and
// removed.
func IsWK(command string) bool {
	command = strings.TrimSuffix(strings.TrimSpace(command), guard)
	if rest, _, ok := strings.Cut(command, " "+leaseVerb); ok {
		return runsWK(rest)
	}
	for _, en := range Entries {
		if rest, ok := strings.CutSuffix(command, " "+en.Verb); ok {
			return runsWK(rest)
		}
	}
	return false
}

// runsWK says whether rest, the part of a hook command before its verb, is the wk binary.
func runsWK(rest string) bool {
	words := strings.Fields(rest)
	for len(words) > 1 && strings.Contains(words[0], "=") {
		words = words[1:]
	}
	return len(words) > 0 && filepath.Base(strings.Trim(strings.Join(words, " "), "'")) == "wk"
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

// within replaces key's object with what change makes of it, and leaves the key's bytes alone
// when change made nothing new. A missing key reads as an empty object, and an emptied object is
// removed rather than kept.
func (o *object) within(key string, change func(*object) error) error {
	before := o.get(key)
	inner, err := parseObject(before)
	if err != nil {
		return fmt.Errorf("%s: %w", key, err)
	}
	if err := change(&inner); err != nil {
		return err
	}
	after, err := inner.MarshalJSON()
	if err != nil || sameJSON(before, after) {
		return err
	}
	return o.setOrRemove(key, inner)
}

// setOrRemove sets key to value, in place when the key exists and at the end when it does not,
// or removes key when value is an empty list or object.
func (o *object) setOrRemove(key string, value any) error {
	data, err := marshal(value)
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
		k, err := marshal(m.key)
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

// marshal is json.Marshal without its HTML escaping, which rewrites every &, < and > in a kept
// value, so the owner's rule "Bash(cd web && bun run lint 2>&1)" would come back as
// "Bash(cd web \u0026\u0026 bun run lint 2\u003e\u00261)": the same JSON, but not the bytes
// the owner wrote.
func marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return nil, err
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n")), nil
}
