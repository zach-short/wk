package settings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// A Twin is one wrapped prefix a gate spelling implies: for "bun run build" in class heavy with
// trim web/.next/cache, Prefix is exe + " run --slot heavy --trim web/.next/cache -- ".
type Twin struct {
	Spelling string
	Prefix   string
}

// ruleLists are the lists a twin can go in. A twin lands in its original's own list, so a
// wrapped gate is allowed, asked or refused exactly as the unwrapped one was (SB-17).
var ruleLists = []string{"allow", "ask", "deny"}

// InstallTwins makes the file at path hold the twin of each Bash allow, ask and deny rule in
// sources that covers a twin's Spelling: the rule "Bash(" + Prefix + <the original pattern body>
// + ")" in the original's own list, once (SB-17). Claude Code matches rules against the command
// the hook rewrote (SLOTS-PLAN.md 0.2), so without its twin a wrapped gate loses the rule its
// original had, and a deny rule stops refusing it.
//
// sources are the user's ~/.claude/settings.json, the tracked .claude/settings.json and the file
// at path, as the caller read them; a nil source is an empty one. twins is the whole set: a twin
// already in path that this call does not compute, because its original rule was since removed,
// is removed, so revoking a rule revokes its twin at the next install. Every other rule keeps its
// place and its bytes. Never writes any file but path.
func InstallTwins(path string, sources [][]byte, twins []Twin) (changed bool, err error) {
	if err := checkTwins(twins); err != nil {
		return false, err
	}
	want, err := twinRules(sources, twins)
	if err != nil {
		return false, err
	}
	return edit(path, func(root *object) error {
		return root.within("permissions", func(perms *object) error {
			return eachList(perms, func(list string, rules []json.RawMessage) ([]json.RawMessage, error) {
				return withTwins(rules, want[list])
			})
		})
	})
}

// checkTwins refuses a twin Uninstall could not find again, because a twin it never removes
// outlives wk's hook.
func checkTwins(twins []Twin) error {
	for _, t := range twins {
		if strings.TrimSpace(t.Spelling) == "" || !isTwin("Bash("+t.Prefix+")") {
			return fmt.Errorf("twin %q for spelling %q: needs a spelling and a prefix that starts with an absolute path to wk and \" run --slot \"", t.Prefix, t.Spelling)
		}
	}
	return nil
}

// twinRules lists, per permission list, the twin of each source rule that needs one, in source
// order and without repeats.
func twinRules(sources [][]byte, twins []Twin) (map[string][]string, error) {
	want := map[string][]string{}
	for i, src := range sources {
		lists, err := ruleStrings(src)
		if err != nil {
			return nil, fmt.Errorf("settings source %d: %w", i+1, err)
		}
		for _, list := range ruleLists {
			for _, rule := range lists[list] {
				want[list] = appendNew(want[list], twinsOf(rule, twins))
			}
		}
	}
	return want, nil
}

// ruleStrings reads the string rules of a settings file's allow, ask and deny lists.
func ruleStrings(src []byte) (map[string][]string, error) {
	root, err := parseObject(src)
	if err != nil {
		return nil, err
	}
	perms, err := parseObject(root.get("permissions"))
	if err != nil {
		return nil, fmt.Errorf("permissions: %w", err)
	}
	lists := map[string][]string{}
	for _, list := range ruleLists {
		rules, err := rulesOf(perms, list)
		if err != nil {
			return nil, err
		}
		for _, r := range rules {
			if s, ok := ruleString(r); ok {
				lists[list] = append(lists[list], s)
			}
		}
	}
	return lists, nil
}

func twinsOf(rule string, twins []Twin) []string {
	body, ok := bashBody(rule)
	if !ok || isTwin(rule) {
		return nil
	}
	var out []string
	for _, t := range twins {
		if needsTwin(body, t) {
			out = append(out, "Bash("+t.Prefix+body+")")
		}
	}
	return out
}

func bashBody(rule string) (string, bool) {
	rest, ok := strings.CutPrefix(rule, "Bash(")
	if !ok {
		return "", false
	}
	body, ok := strings.CutSuffix(rest, ")")
	return body, ok && body != ""
}

// needsTwin says whether a rule with this body can match a command the hook wraps with t, one
// whose words start with t.Spelling. That is wider than "the rule matches the spelling": a deny
// rule narrower than the spelling, such as Bash(bun run build --prod), must still refuse the
// wrapped command. Claude Code reads a trailing ":*" as a trailing " *", and a "*" anywhere as
// any text (code.claude.com/docs/en/permissions). A rule whose text before its first "*" begins
// t.Prefix needs no twin: it already matches the wrapped command, and its twin would draw Claude
// Code's startup warning about a "*" before the rest of a command.
func needsTwin(body string, t Twin) bool {
	if p, ok := strings.CutSuffix(body, ":*"); ok {
		body = p + " *"
	}
	literal, _, wild := strings.Cut(body, "*")
	if !wild {
		return literal == t.Spelling || strings.HasPrefix(literal, t.Spelling+" ")
	}
	if strings.HasPrefix(t.Prefix, literal) {
		return false
	}
	return strings.HasPrefix(t.Spelling, literal) || strings.HasPrefix(literal, t.Spelling+" ")
}

// isTwin says whether rule is one wk writes: a Bash rule that starts with an absolute path to a
// binary named wk and " run --slot ". Uninstall gets no exe, so the path is matched by its
// shape; wk installs only absolute paths, so a rule naming a relative wk is the owner's and
// stays.
func isTwin(rule string) bool {
	body, ok := strings.CutPrefix(rule, "Bash(")
	if !ok {
		return false
	}
	bin, _, ok := strings.Cut(body, " run --slot ")
	return ok && wkBinary(bin)
}

// wkBinary says whether word is an absolute path to a binary named wk, plain or quoted the way
// Command quotes it.
func wkBinary(word string) bool {
	if len(word) >= 2 && word[0] == '\'' && word[len(word)-1] == '\'' {
		word = strings.ReplaceAll(word[1:len(word)-1], `'\''`, `'`)
	} else if strings.ContainsAny(word, " \t") {
		return false
	}
	return filepath.IsAbs(word) && filepath.Base(word) == "wk"
}

// eachList replaces each of allow, ask and deny with what change makes of it, and leaves a list
// alone when change returns the same rules, so an empty list the owner wrote stays.
func eachList(perms *object, change func(list string, rules []json.RawMessage) ([]json.RawMessage, error)) error {
	for _, list := range ruleLists {
		rules, err := rulesOf(*perms, list)
		if err != nil {
			return err
		}
		out, err := change(list, rules)
		if err != nil {
			return err
		}
		if slices.EqualFunc(rules, out, func(a, b json.RawMessage) bool { return bytes.Equal(a, b) }) {
			continue
		}
		if err := perms.setOrRemove(list, out); err != nil {
			return err
		}
	}
	return nil
}

func rulesOf(perms object, list string) ([]json.RawMessage, error) {
	var rules []json.RawMessage
	if raw := perms.get(list); raw != nil {
		if err := json.Unmarshal(raw, &rules); err != nil {
			return nil, fmt.Errorf("permissions.%s: %w", list, err)
		}
	}
	return rules, nil
}

// withTwins keeps every rule that is not a twin in its place, keeps each wanted twin already
// there once, drops the twins no longer wanted, and appends the wanted ones still missing.
func withTwins(rules []json.RawMessage, want []string) ([]json.RawMessage, error) {
	var out []json.RawMessage
	have := map[string]bool{}
	for _, r := range rules {
		if s, ok := ruleString(r); ok && isTwin(s) {
			if have[s] || !slices.Contains(want, s) {
				continue
			}
			have[s] = true
		}
		out = append(out, r)
	}
	for _, s := range want {
		if have[s] {
			continue
		}
		raw, err := marshal(s)
		if err != nil {
			return nil, err
		}
		out = append(out, raw)
	}
	return out, nil
}

func withoutTwins(_ string, rules []json.RawMessage) ([]json.RawMessage, error) {
	var kept []json.RawMessage
	for _, r := range rules {
		if s, ok := ruleString(r); !ok || !isTwin(s) {
			kept = append(kept, r)
		}
	}
	return kept, nil
}

func ruleString(raw json.RawMessage) (string, bool) {
	var s string
	err := json.Unmarshal(raw, &s)
	return s, err == nil
}

func appendNew(list, rules []string) []string {
	for _, r := range rules {
		if !slices.Contains(list, r) {
			list = append(list, r)
		}
	}
	return list
}
