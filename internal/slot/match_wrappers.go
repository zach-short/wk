package slot

import (
	"path"
	"regexp"
	"slices"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// matchGate is where Rewrite inserts the prefix in one simple command, and the entry it takes
// the class and trims from. A nil word means the command runs no gate.
type matchGate struct {
	word  *syntax.Word
	entry MatchEntry
}

// gate finds the gate a simple command runs: a spelling as its first words, a spelling behind
// env's options and assignments, or a `<shell> -c` string whose spellings share one class. A
// non-empty reason is a gate that stays unslotted, for missed.log.
func (w *matchWalk) gate(c *syntax.CallExpr) (matchGate, string) {
	if g, reason, ok := w.spelledAt(c.Args); ok {
		return g, reason
	}
	if i, ok := matchEnvGate(c.Args); ok {
		if g, reason, ok := w.spelledAt(c.Args[i:]); ok {
			return g, reason
		}
	}
	if g, reason, ok := w.shellGate(c); ok {
		return g, reason
	}
	reason, _ := w.unmatchedReason(c)
	return matchGate{}, reason
}

func (w *matchWalk) spelledAt(args []*syntax.Word) (matchGate, string, bool) {
	s, literal, ok := w.lookup(args)
	if !ok {
		return matchGate{}, "", false
	}
	if !literal {
		return matchGate{}, "quoted spelling", true
	}
	return matchGate{word: args[0], entry: s.entry}, "", true
}

// matchEnvGate returns the index of the first word after env's options and assignments, when
// env reads every word before it the same way on macOS and in GNU coreutils. The insertion goes
// there, not before env, so wk run sees the environment the gate sees and runs.log names the
// gate. That is safe only while env leaves wk run's own environment alone, so it refuses -i and
// every other option, and any HOME or WK_ variable: wk finds its slots through HOME and WK_HOME,
// and a nested run reads WK_SLOT_HELD.
func matchEnvGate(args []*syntax.Word) (int, bool) {
	if len(args) < 2 || path.Base(args[0].Lit()) != "env" {
		return 0, false
	}
	i, ok := matchEnvOptions(args, 1)
	if !ok {
		return 0, false
	}
	for ; i < len(args); i++ {
		name, isAssign := matchAssignName(args[i])
		if !isAssign {
			return i, true
		}
		if !matchEnvNameFree(name) {
			return 0, false
		}
	}
	return 0, false
}

// matchEnvOptions reads -u NAME, -uNAME, --unset=NAME and a closing --, from args[i] on.
func matchEnvOptions(args []*syntax.Word, i int) (int, bool) {
	for ; i < len(args); i++ {
		opt := args[i].Lit()
		var name string
		switch {
		case opt == "--":
			return i + 1, true
		case opt == "-u" && i+1 < len(args):
			i++
			name = args[i].Lit()
		case strings.HasPrefix(opt, "--unset="):
			name = strings.TrimPrefix(opt, "--unset=")
		case strings.HasPrefix(opt, "-u") && len(opt) > 2:
			name = opt[2:]
		case strings.HasPrefix(opt, "-"):
			return 0, false
		default:
			return i, true
		}
		if !matchEnvNameFree(name) {
			return 0, false
		}
	}
	return i, true
}

var matchEnvName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func matchEnvNameFree(name string) bool {
	return matchEnvName.MatchString(name) && name != "HOME" && !strings.HasPrefix(name, "WK_")
}

// matchAssignName reads NAME from a NAME=value word whose name is plain text; the value may be
// quoted or hold expansions.
func matchAssignName(word *syntax.Word) (string, bool) {
	if len(word.Parts) == 0 {
		return "", false
	}
	lit, ok := word.Parts[0].(*syntax.Lit)
	if !ok {
		return "", false
	}
	name, _, found := strings.Cut(lit.Value, "=")
	return name, found && matchEnvName.MatchString(name)
}

// shellGate wraps a whole `<shell> -c <string>` call in one place, because the string is one
// word to the outer parser and an insertion inside it would have to be quoted for the inner
// shell. It does so only when the string is plain text with no expansion, parses, and every
// spelling it mentions has one class, with the trims of all of them; a string holding a heavy
// and a check gate would otherwise run the heavy one in a check place. ok is false when c is no
// `<shell> -c` call or its words mention no spelling.
func (w *matchWalk) shellGate(c *syntax.CallExpr) (matchGate, string, bool) {
	name, at, reason := matchShellString(c.Args)
	if name == "" || !matchMentions(w.text(c.Args[1].Pos(), c.End()), w.spellings) {
		return matchGate{}, "", false
	}
	why := func(r string) (matchGate, string, bool) { return matchGate{}, "inside " + name + " -c: " + r, true }
	if reason != "" {
		return why(reason)
	}
	text, plain := matchPlainText(c.Args[at])
	if !plain {
		return why("the string is not plain text")
	}
	if _, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(text), ""); err != nil {
		return why("the string does not parse")
	}
	entry, one := matchOneClass(matchMentioned(text, w.spellings))
	if !one {
		return why("spellings of more than one class")
	}
	if entry.Class == "" {
		return matchGate{}, "", false
	}
	return matchGate{word: c.Args[0], entry: entry}, "", true
}

// matchShellString finds the string of a `<shell> -c` call: the first word after the shell's
// short options, where -o and -O take the next word. name is "" when the call is no `-c` call;
// reason is set when it is one whose options wk does not read.
func matchShellString(args []*syntax.Word) (name string, at int, reason string) {
	if len(args) < 2 {
		return "", 0, ""
	}
	name = path.Base(args[0].Lit())
	if !matchShells[name] || !matchHasCFlag(args[1:]) {
		return "", 0, ""
	}
	for i := 1; i < len(args); i++ {
		opt := args[i].Lit()
		switch {
		case opt == "--":
			return name, i + 1, matchNoString(args, i+1)
		case strings.HasPrefix(opt, "--"):
			return name, 0, "an option wk does not read"
		case len(opt) < 2 || (opt[0] != '-' && opt[0] != '+'):
			return name, i, ""
		case strings.ContainsAny(opt[1:], "oO"):
			i++
		}
	}
	return name, 0, "no string after -c"
}

func matchHasCFlag(args []*syntax.Word) bool {
	return slices.ContainsFunc(args, func(a *syntax.Word) bool {
		flag := a.Lit()
		return len(flag) > 1 && flag[0] == '-' && flag[1] != '-' && strings.ContainsRune(flag, 'c')
	})
}

func matchNoString(args []*syntax.Word, at int) string {
	if at < len(args) {
		return ""
	}
	return "no string after -c"
}

// matchPlainText is a word's value when it holds no expansion and no backslash outside single
// quotes, so the value is exactly what the inner shell reads.
func matchPlainText(word *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			if strings.ContainsRune(p.Value, '\\') {
				return "", false
			}
			b.WriteString(p.Value)
		case *syntax.SglQuoted:
			if p.Dollar {
				return "", false
			}
			b.WriteString(p.Value)
		case *syntax.DblQuoted:
			if !matchPlainDouble(p, &b) {
				return "", false
			}
		default:
			return "", false
		}
	}
	return b.String(), true
}

func matchPlainDouble(p *syntax.DblQuoted, b *strings.Builder) bool {
	for _, inner := range p.Parts {
		lit, ok := inner.(*syntax.Lit)
		if !ok || strings.ContainsRune(lit.Value, '\\') {
			return false
		}
		b.WriteString(lit.Value)
	}
	return true
}

// matchOneClass merges the entries of the spellings found: one class, every trim path once in
// the order found. ok is false for two classes; an empty list gives an empty entry.
func matchOneClass(found []matchSpelling) (MatchEntry, bool) {
	var merged MatchEntry
	for _, s := range found {
		if merged.Class != "" && s.entry.Class != merged.Class {
			return MatchEntry{}, false
		}
		merged.Class = s.entry.Class
		for _, p := range s.entry.TrimAfter {
			if !slices.Contains(merged.TrimAfter, p) {
				merged.TrimAfter = append(merged.TrimAfter, p)
			}
		}
	}
	return merged, true
}
