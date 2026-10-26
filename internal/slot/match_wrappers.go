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
// wrappers whose every word wk reads (matchWrapped), or a `<shell> -c` string whose spellings
// share one class. A non-empty reason is a gate that stays unslotted, for missed.log.
func (w *matchWalk) gate(c *syntax.CallExpr) (matchGate, string) {
	if g, reason, ok := w.spelledAt(c.Args); ok {
		return g, reason
	}
	if i, ok := matchWrapped(c.Args); ok {
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

// matchWrapperReaders read the words of one wrapper at args[i]: each returns the index of the
// command the wrapper runs, and false for a word it does not read. The insertion goes at that
// command, after the wrapper's own words, as it does under the `time` keyword, so runs.log names
// the gate. Claude Code strips time and command by name before it matches a Bash permission
// rule (code.claude.com/docs/en/permissions, "Wrappers", read 2026-10-26), so the wrapped gate
// still meets its twin there; it strips neither env nor a path such as /usr/bin/time, which the
// gate did not meet before the rewrite either.
//
// timeout and gtimeout are left out on purpose (row 25, decided by the coordinating session by
// risk, 2026-10-26): after the insertion their clock would also count the wait for a place, and the slot logs held 118 waits
// over 10 minutes, so `timeout 600 go test` would often exit 124 before the gate started, which
// agents read as a hang. wk run before timeout would lose the permission twin. A gate behind
// timeout stays a miss with matchTimeoutReason.
var matchWrapperReaders = map[string]func(args []*syntax.Word, i int) (int, bool){
	"env":     matchEnvArgs,
	"command": matchCommandArgs,
	"time":    matchTimeArgs,
	"gtime":   matchTimeArgs,
}

// matchTimeoutReason is the missed.log reason for a gate behind timeout, so wk stats shows it.
const matchTimeoutReason = "after the wrapper timeout: the wait for a place would count against its clock"

// matchWrapped returns the index of the first word after a chain of wrappers that wk reads
// whole, such as `command time -p env -u X go test`. ok is false when args starts with no such
// wrapper, or one of them holds a word wk does not read.
//
// A wrapper word is read only as a bare name (time, found on PATH) or an absolute path
// (/usr/bin/time, Homebrew's gtime, a nix store), known by its last path element (row 28, from
// the row 27 audit N1). A relative path such as ./time or bin/env names a file in the working
// directory, most likely a project script with arguments of its own, so wk does not insert
// wk run among them; wrapperReason logs such a line with matchRelativeWrapperReason. Before
// row 28 any path counted; reversal: read path.Base of every word here.
func matchWrapped(args []*syntax.Word) (int, bool) {
	i := 0
	for i < len(args) {
		name, _ := matchWrapperName(args[i].Lit())
		read, isWrapper := matchWrapperReaders[name]
		if !isWrapper {
			return i, i > 0
		}
		next, ok := read(args, i)
		if !ok {
			return 0, false
		}
		i = next
	}
	return 0, false
}

// matchRelativeWrapperReason is the missed.log reason for a gate behind a wrapper's name spelled
// as a relative path, which matchWrapped does not read.
const matchRelativeWrapperReason = "after a wrapper name with a relative path: it may be a project script"

// matchWrapperName gives the last path element of word, and relative is true when word is a
// path that is neither a bare name nor absolute (./time, bin/env); name is "" then.
func matchWrapperName(word string) (name string, relative bool) {
	if strings.Contains(word, "/") && !strings.HasPrefix(word, "/") {
		return "", true
	}
	return path.Base(word), false
}

// matchEnvArgs reads env's options and assignments, when env reads every word the same way on
// macOS and in GNU coreutils, so wk run sees the environment the gate sees. That is safe only
// while env leaves wk run's own environment alone, so it refuses -i and every other option,
// and any HOME, PATH or WK_ variable: wk finds its slots through HOME and WK_HOME, wk run finds
// the gate's program and its own helpers through PATH, and a nested run reads WK_SLOT_HELD.
func matchEnvArgs(args []*syntax.Word, at int) (int, bool) {
	i, ok := matchEnvOptions(args, at+1)
	if !ok {
		return 0, false
	}
	for ; i < len(args); i++ {
		name, isAssign := matchAssignName(args[i])
		if !isAssign {
			return i, true
		}
		if !matchEnvNameFree(name) || !matchAssignValueUnsplit(args[i]) {
			return 0, false
		}
	}
	return 0, false
}

// matchCommandArgs reads `command` with no option or a closing --. It refuses -p, which looks
// the next word up in a default PATH that wk run would not use for the gate, and -v and -V,
// which print a path and run nothing.
func matchCommandArgs(args []*syntax.Word, at int) (int, bool) {
	i := at + 1
	if i < len(args) && args[i].Lit() == "--" {
		i++
	}
	if i >= len(args) || strings.HasPrefix(args[i].Lit(), "-") {
		return 0, false
	}
	return i, true
}

// matchTimeArgs reads the options of the time utility, which runs where the keyword does not:
// /usr/bin/time, `command time`, and `time` after an assignment. It reads the flags of macOS
// (-a -h -l -p, -o FILE) and of GNU time (-a -p -q -v, -o FILE, -f FORMAT and their long
// forms). Its report goes to stderr or the -o file, as the keyword's does. After the insertion
// it times wk run, which waits for the gate. On macOS (measured 2026-10-26, row 27) -l's real,
// user and sys times and its maximum resident set size include the gate, because a process's
// usage holds the children it waited for, but its instructions retired, cycles elapsed and peak
// memory footprint describe wk run alone; real also counts the wait for a place. Any other
// option, such as -V or --help, which run nothing, is refused.
//
// Every option word and every option argument must be a matchFixedWord (row 25 audit N2). An
// empty $F in `-o $F` drops the word, so time writes to a file named after the next word and
// runs the rest: the rewritten line would run `run --slot ...` where the original ran the gate's
// second word. A glob or a brace can add words the same way. Such a line stays a miss.
func matchTimeArgs(args []*syntax.Word, at int) (int, bool) {
	for i := at + 1; i < len(args); i++ {
		opt := args[i].Lit()
		if strings.HasPrefix(opt, "-") && !matchFixedWord(args[i]) {
			return 0, false
		}
		switch {
		case opt == "--":
			return i + 1, i+1 < len(args)
		case matchTimeLongFlags[opt]:
		case opt == "--output" || opt == "--format":
			if i++; i >= len(args) || !matchFixedWord(args[i]) {
				return 0, false
			}
		case strings.HasPrefix(opt, "--output=") || strings.HasPrefix(opt, "--format="):
		case strings.HasPrefix(opt, "--"):
			return 0, false
		case len(opt) > 1 && opt[0] == '-':
			skip, ok := matchShortFlags(opt[1:], "ahlpqv", "of")
			if !ok {
				return 0, false
			}
			if i += skip; skip == 1 && (i >= len(args) || !matchFixedWord(args[i])) {
				return 0, false
			}
		default:
			return i, true
		}
	}
	return 0, false
}

// matchFixedWord is true for a word the shell passes on as exactly one word, whatever the
// variables and the files hold: plain text with no expansion (matchPlainText), and no glob or
// brace character outside quotes. A quoted format such as -f '%e %M' is one; $F, t*.txt and
// {a,b} are not. Nor is unquoted text that holds a $ (row 28): zsh reads -o $=F as F split into
// words, so an empty F drops the word as $F does in bash (seen in real zsh), and this parser
// reads $=F as plain text.
func matchFixedWord(word *syntax.Word) bool {
	if _, plain := matchPlainText(word); !plain {
		return false
	}
	for _, part := range word.Parts {
		if lit, ok := part.(*syntax.Lit); ok && strings.ContainsAny(lit.Value, "*?[{$") {
			return false
		}
	}
	return true
}

var matchTimeLongFlags = map[string]bool{"--append": true, "--portability": true, "--quiet": true, "--verbose": true}

// matchShortFlags reads one cluster of short flags such as -lp or -ao: each letter is in bare,
// or is the last letter and in withArg, so it takes the next word, or is in withArg and the rest
// of the cluster is its argument (-ofile). skip is 1 when the next word is an argument.
func matchShortFlags(cluster, bare, withArg string) (skip int, ok bool) {
	for j, r := range cluster {
		switch {
		case strings.ContainsRune(bare, r):
		case strings.ContainsRune(withArg, r) && j == len(cluster)-1:
			return 1, true
		case strings.ContainsRune(withArg, r):
			return 0, true
		default:
			return 0, false
		}
	}
	return 0, true
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
	return matchEnvName.MatchString(name) && name != "HOME" && name != "PATH" && !strings.HasPrefix(name, "WK_")
}

// matchAssignName reads NAME from a NAME=value word whose name is plain text; the value may be
// quoted or hold expansions, which matchAssignValueUnsplit reads for env.
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

// matchAssignValueUnsplit is true when an env NAME=value argument stays one word. To env it is
// an ordinary argument, so an unquoted expansion in its value is split (row 25 audit N2): with
// X="1 2", `env A=$X go test` sets A=1 and runs `2 go test`, and after the insertion env would
// run `2` with wk run's words as its arguments, so such a line stays a miss. An empty value
// cannot move the gate, since the word keeps its literal NAME=, and a glob or a brace only makes
// more NAME= words, so neither is refused.
//
// A quoted expansion is one word, except those that make one word per element (row 28, from the
// row 27 audit N2), checked in real bash and zsh: "$@", "${@:1}", "${arr[@]}", "${arr[@]/x/y}",
// "${X:-$@}" and "${!prefix@}" in both, and "$arr[@]" in zsh, which bash reads as $arr and the
// text [@]. "$*", "${arr[*]}" and "${#arr[@]}" stay one word in both, so they are kept. Unquoted
// text that holds a $ is refused too: zsh reads $=X as X split into words, which this parser
// reads as plain text.
func matchAssignValueUnsplit(word *syntax.Word) bool {
	for _, part := range word.Parts {
		switch p := part.(type) {
		case *syntax.Lit:
			if strings.ContainsRune(p.Value, '$') {
				return false
			}
		case *syntax.SglQuoted:
		case *syntax.DblQuoted:
			if matchQuotedSplits(p) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// matchQuotedSplits is true when double-quoted text may expand to more than one word: an
// expansion of every positional parameter or array element ($@, [@], ${!prefix@}) anywhere in
// it, outside a command substitution, whose output is one word, or a short $name followed by a
// subscript, which zsh reads as one.
func matchQuotedSplits(q *syntax.DblQuoted) bool {
	for i, part := range q.Parts {
		pe, ok := part.(*syntax.ParamExp)
		if !ok || !pe.Short || i+1 >= len(q.Parts) {
			continue
		}
		if lit, ok := q.Parts[i+1].(*syntax.Lit); ok && strings.HasPrefix(lit.Value, "[") {
			return true
		}
	}
	splits := false
	syntax.Walk(q, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.CmdSubst:
			return false
		case *syntax.ParamExp:
			if n.Length {
				return true
			}
			splits = splits || n.Names == syntax.NamesPrefixWords ||
				(n.Param != nil && n.Param.Value == "@") || matchIndexAll(n.Index)
		}
		return !splits
	})
	return splits
}

func matchIndexAll(index syntax.ArithmExpr) bool {
	word, ok := index.(*syntax.Word)
	return ok && word.Lit() == "@"
}

// shellGate wraps a whole `<shell> -c <string>` call in one place, because the string is one
// word to the outer parser and an insertion inside it would have to be quoted for the inner
// shell. It does so only when the string is plain text with no expansion, parses, and every
// spelling it mentions has one class, with the trims of all of them; a string holding a heavy
// and a check gate would otherwise run the heavy one in a check place. ok is false when c is no
// `<shell> -c` call or its words mention no spelling.
//
// A string that runs a gate behind timeout, `sh -c 'timeout 300 go test'`, is wrapped whole like
// any other (row 27, from the row 25 audit N1): wk run waits for its place before the shell
// starts, so timeout's clock starts after the wait, and the harm that keeps a gate behind
// timeout a miss is absent. `timeout 300 sh -c '...'` is the other way round; wrapperReason logs
// it with matchTimeoutReason.
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
