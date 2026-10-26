package slot

import (
	"path"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode"

	"mvdan.cc/sh/v3/syntax"
)

// Missed is one gate spelling Rewrite left alone, for missed.log, so the matcher grows from
// evidence (SB-15). Command is the source text of the simple command that holds the spelling,
// or the whole line when it does not parse.
type Missed struct{ Command, Reason string }

// Rewrite inserts "<wkExe> run --slot <class> [--trim <p>]... -- " at the byte offset of each
// matched simple command's first word (SB-15, every statement position, never inside a word or
// a function body) and changes no other byte. Text that does not parse comes back unchanged
// with parseErr set. Each literal spelling left alone is a Missed with its reason.
//
// A simple command matches when its first words, after any VAR=value words, are unquoted
// literals equal to the words of a spelling; the longest spelling wins. A line that already
// holds a `wk run` is left whole, so a hand-wrapped line and Rewrite's own output never change.
func Rewrite(command, wkExe string, sf SlotsFile) (out string, changed bool, missed []Missed, parseErr error) {
	spellings := matchSpellings(sf)
	if len(spellings) == 0 {
		return command, false, nil, nil
	}
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(command), "")
	if err != nil {
		return command, false, matchUnparsed(command, spellings, err), err
	}
	w := matchWalk{src: command, exe: wkExe, spellings: spellings}
	w.wrapped = w.holdsWKRun(file)
	w.stmts(file.Stmts)
	if len(w.inserts) == 0 {
		return command, false, w.missed, nil
	}
	return matchApply(command, w.inserts), true, w.missed, nil
}

type matchSpelling struct {
	words []string
	entry MatchEntry
}

func matchSpellings(sf SlotsFile) []matchSpelling {
	list := make([]matchSpelling, 0, len(sf.Match))
	for _, e := range sf.Match {
		if words := strings.Fields(e.Spelling); len(words) > 0 {
			list = append(list, matchSpelling{words: words, entry: e})
		}
	}
	sort.SliceStable(list, func(i, j int) bool { return len(list[i].words) > len(list[j].words) })
	return list
}

type matchInsert struct {
	at   uint
	text string
}

type matchWalk struct {
	src       string
	exe       string
	spellings []matchSpelling
	wrapped   bool
	inserts   []matchInsert
	missed    []Missed
}

func (w *matchWalk) stmts(list []*syntax.Stmt) {
	for _, s := range list {
		w.stmt(s)
	}
}

// stmt visits one statement position. The cases are the positions GATE 2 answered "every
// statement" for; `time` and `coproc` are left out because the wrapper's wait would change what
// they measure or start.
func (w *matchWalk) stmt(s *syntax.Stmt) {
	for _, r := range s.Redirs {
		w.words(r)
	}
	switch c := s.Cmd.(type) {
	case nil:
	case *syntax.CallExpr:
		w.call(c)
	case *syntax.BinaryCmd:
		w.stmt(c.X)
		w.stmt(c.Y)
	case *syntax.Subshell:
		w.stmts(c.Stmts)
	case *syntax.Block:
		w.stmts(c.Stmts)
	case *syntax.IfClause:
		w.ifClause(c)
	case *syntax.WhileClause:
		w.stmts(c.Cond)
		w.stmts(c.Do)
	case *syntax.ForClause:
		w.words(c.Loop)
		w.stmts(c.Do)
	case *syntax.CaseClause:
		w.caseClause(c)
	default:
		w.compound(c)
	}
}

func (w *matchWalk) compound(c syntax.Command) {
	switch c := c.(type) {
	case *syntax.FuncDecl:
		w.missAll(c.Body, "inside a function body")
	case *syntax.TimeClause:
		w.missAll(c, "inside time")
	case *syntax.CoprocClause:
		w.missAll(c, "inside coproc")
	default:
		w.words(c)
	}
}

func (w *matchWalk) ifClause(c *syntax.IfClause) {
	for ; c != nil; c = c.Else {
		w.stmts(c.Cond)
		w.stmts(c.Then)
	}
}

func (w *matchWalk) caseClause(c *syntax.CaseClause) {
	w.words(c.Word)
	for _, item := range c.Items {
		for _, p := range item.Patterns {
			w.words(p)
		}
		w.stmts(item.Stmts)
	}
}

func (w *matchWalk) call(c *syntax.CallExpr) {
	w.words(c)
	if w.isWKRun(c) {
		return
	}
	s, literal, ok := w.lookup(c.Args)
	switch {
	case !ok:
		w.unmatched(c, "")
	case !literal:
		w.miss(c, "quoted spelling")
	case w.wrapped:
		w.miss(c, "the line already holds wk run")
	default:
		w.insert(c, s.entry)
	}
}

// words looks inside the words of a non-statement part, where a command can only sit in a
// substitution; nothing there is rewritten, because an insertion inside a word changes the word.
func (w *matchWalk) words(n syntax.Node) {
	syntax.Walk(n, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.CmdSubst:
			w.missAll(n, matchSubstReason(n))
			return false
		case *syntax.ProcSubst:
			w.missAll(n, "inside a process substitution")
			return false
		}
		return true
	})
}

func matchSubstReason(c *syntax.CmdSubst) string {
	if c.Backquotes {
		return "inside backticks"
	}
	return "inside $( )"
}

// missAll records every spelling under n with the reason of the outermost place that kept it
// from being rewritten.
func (w *matchWalk) missAll(n syntax.Node, reason string) {
	syntax.Walk(n, func(n syntax.Node) bool {
		if c, ok := n.(*syntax.CallExpr); ok && !w.isWKRun(c) {
			if _, _, found := w.lookup(c.Args); found {
				w.miss(c, reason)
			} else {
				w.unmatched(c, reason)
			}
		}
		return true
	})
}

// unmatched records a spelling in a command whose first words spell none: in the string an
// `eval` or a `<shell> -c` runs, or after a wrapper word. Neither is rewritten, for missed.log
// only (SB-15).
func (w *matchWalk) unmatched(c *syntax.CallExpr, outer string) {
	reason, ok := w.nestedShellReason(c)
	if !ok {
		reason, ok = w.wrapperReason(c)
	}
	if !ok {
		return
	}
	if outer != "" {
		reason = outer
	}
	w.miss(c, reason)
}

// The string a nested shell runs is one word to this parser, so it is searched as text.
func (w *matchWalk) nestedShellReason(c *syntax.CallExpr) (string, bool) {
	reason, ok := matchNestedShell(c)
	return reason, ok && matchMentions(w.text(c.Args[1].Pos(), c.End()), w.spellings)
}

// A wrapper runs a command made of its later words, so a gate behind one runs unslotted. It is
// logged, not rewritten, because the wrapper's own options sit between it and the gate. The set
// is closed on purpose: any word before a spelling would also log `grep eslint`.
var matchWrappers = map[string]bool{"exec": true, "nohup": true, "nice": true, "env": true, "command": true, "timeout": true, "xargs": true}

func (w *matchWalk) wrapperReason(c *syntax.CallExpr) (string, bool) {
	if len(c.Args) < 2 {
		return "", false
	}
	name := path.Base(c.Args[0].Lit())
	if !matchWrappers[name] {
		return "", false
	}
	for i := 1; i < len(c.Args); i++ {
		if _, _, found := w.lookup(c.Args[i:]); found {
			return "after the wrapper " + name, true
		}
	}
	return "", false
}

var matchShells = map[string]bool{"bash": true, "sh": true, "zsh": true, "dash": true, "ksh": true}

func matchNestedShell(c *syntax.CallExpr) (string, bool) {
	if len(c.Args) < 2 {
		return "", false
	}
	name := path.Base(c.Args[0].Lit())
	if name == "eval" {
		return "inside eval", true
	}
	if !matchShells[name] {
		return "", false
	}
	for _, a := range c.Args[1:] {
		if flag := a.Lit(); len(flag) > 1 && flag[0] == '-' && flag[1] != '-' && strings.ContainsRune(flag, 'c') {
			return "inside " + name + " -c", true
		}
	}
	return "", false
}

func (w *matchWalk) isWKRun(c *syntax.CallExpr) bool {
	return len(c.Args) >= 2 && w.isWKWord(c.Args[0]) && c.Args[1].Lit() == "run"
}

func (w *matchWalk) isWKWord(word *syntax.Word) bool {
	v, ok := matchUnquoted(word)
	return ok && (v == w.exe || path.Base(v) == "wk")
}

// holdsWKRun reads the parsed words, never the raw text, so a comment that mentions wk run does
// not hold the line back. Two words "wk" and "run" in a row count wherever they sit, and so does
// "wk run" inside one quoted word, whatever punctuation follows it, except in the arguments of a
// text filter such as grep: agents write `| grep -v '^wk run'` to drop wk's waiting lines, and a
// pattern never runs. From 2026-10-17 to 2026-10-25, 63 of the 66 lines held back for this reason
// in ezhomesteading were such filters, so their gates ran unslotted.
func (w *matchWalk) holdsWKRun(file *syntax.File) bool {
	found := false
	syntax.Walk(file, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.CallExpr:
			found = found || w.callHoldsWKRun(n)
			if !found && matchTextFilter(n) {
				return false
			}
		case *syntax.Word:
			found = found || matchWKRun.MatchString(w.text(n.Pos(), n.End()))
		}
		return !found
	})
	return found
}

// matchTextFilters are the commands whose arguments are patterns and file names: none of them
// runs an argument as a command. awk and sed are left out, since each can run one.
var matchTextFilters = map[string]bool{"grep": true, "egrep": true, "fgrep": true, "rg": true}

func matchTextFilter(c *syntax.CallExpr) bool {
	if len(c.Args) == 0 {
		return false
	}
	v, ok := matchUnquoted(c.Args[0])
	return ok && matchTextFilters[path.Base(v)]
}

func (w *matchWalk) callHoldsWKRun(c *syntax.CallExpr) bool {
	for i := 0; i+1 < len(c.Args); i++ {
		if w.isWKWord(c.Args[i]) && c.Args[i+1].Lit() == "run" {
			return true
		}
	}
	return false
}

// lookup finds the longest spelling the first words spell once quotes and escapes are removed,
// and reports whether they spell it as plain literals too. Choosing by the unquoted words first
// keeps a quoted long spelling from being rewritten under a shorter spelling's class.
func (w *matchWalk) lookup(args []*syntax.Word) (s matchSpelling, literal, ok bool) {
	for _, s := range w.spellings {
		if len(args) >= len(s.words) && matchWordsEqual(args, s.words, matchUnquoted) {
			return s, matchWordsEqual(args, s.words, matchLiteral), true
		}
	}
	return matchSpelling{}, false, false
}

func matchWordsEqual(args []*syntax.Word, words []string, value func(*syntax.Word) (string, bool)) bool {
	for i, want := range words {
		if got, ok := value(args[i]); !ok || got != want {
			return false
		}
	}
	return true
}

func matchLiteral(w *syntax.Word) (string, bool) {
	lit := w.Lit()
	return lit, lit != "" && !strings.ContainsRune(lit, '\\')
}

func matchUnquoted(w *syntax.Word) (string, bool) {
	var b strings.Builder
	for _, part := range w.Parts {
		text, ok := matchUnquotedPart(part)
		if !ok {
			return "", false
		}
		b.WriteString(text)
	}
	return b.String(), true
}

func matchUnquotedPart(part syntax.WordPart) (string, bool) {
	switch p := part.(type) {
	case *syntax.Lit:
		return matchUnescape(p.Value), true
	case *syntax.SglQuoted:
		return p.Value, !p.Dollar
	case *syntax.DblQuoted:
		var b strings.Builder
		for _, inner := range p.Parts {
			text, ok := matchUnquotedPart(inner)
			if !ok {
				return "", false
			}
			b.WriteString(text)
		}
		return b.String(), true
	}
	return "", false
}

func matchUnescape(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	escaped := false
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && !escaped {
			escaped = true
			continue
		}
		escaped = false
		b.WriteByte(s[i])
	}
	return b.String()
}

func (w *matchWalk) insert(c *syntax.CallExpr, e MatchEntry) {
	prefix, ok := matchPrefix(w.exe, e)
	if !ok {
		w.miss(c, "the wk path or the entry cannot be quoted")
		return
	}
	w.inserts = append(w.inserts, matchInsert{at: c.Args[0].Pos().Offset(), text: prefix})
}

// Prefix is the text Rewrite inserts before a command that matches e. `wk hooks install` builds
// the permission twins from it (SB-17), so a twin and the rewritten command can never differ by
// a byte. ok is false when a word cannot be quoted for the shell.
func Prefix(wkExe string, e MatchEntry) (string, bool) { return matchPrefix(wkExe, e) }

func matchPrefix(exe string, e MatchEntry) (string, bool) {
	words := []string{exe, "run", "--slot", e.Class}
	for _, p := range e.TrimAfter {
		words = append(words, "--trim", p)
	}
	words = append(words, "--")
	for i, word := range words {
		if words[i] = matchQuote(word); words[i] == "" {
			return "", false
		}
	}
	return strings.Join(words, " ") + " ", true
}

// matchQuote is "" only for a string no shell can hold (a NUL byte), which no path or valid
// slots file has.
func matchQuote(s string) string {
	q, err := syntax.Quote(s, syntax.LangBash)
	if err != nil {
		return ""
	}
	return q
}

func (w *matchWalk) miss(c *syntax.CallExpr, reason string) {
	w.missed = append(w.missed, Missed{Command: w.text(c.Pos(), c.End()), Reason: reason})
}

func (w *matchWalk) text(from, to syntax.Pos) string {
	end := min(to.Offset(), uint(len(w.src)))
	return w.src[min(from.Offset(), end):end]
}

var matchWKRun = regexp.MustCompile(`(^|[^A-Za-z0-9_.-])wk\s+run([^A-Za-z0-9_.-]|$)`)

func matchUnparsed(command string, spellings []matchSpelling, err error) []Missed {
	if !matchMentions(command, spellings) {
		return nil
	}
	return []Missed{{Command: command, Reason: "does not parse: " + err.Error()}}
}

// matchMentions searches text that was not parsed into commands, so its tokens break at shell
// punctuation as well as at spaces.
func matchMentions(text string, spellings []matchSpelling) bool {
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(";&|()<>'\"`$", r)
	})
	for i := range tokens {
		for _, s := range spellings {
			if len(tokens)-i >= len(s.words) && slices.Equal(tokens[i:i+len(s.words)], s.words) {
				return true
			}
		}
	}
	return false
}

func matchApply(src string, inserts []matchInsert) string {
	sort.SliceStable(inserts, func(i, j int) bool { return inserts[i].at < inserts[j].at })
	var b strings.Builder
	last := uint(0)
	for _, in := range inserts {
		b.WriteString(src[last:in.at])
		b.WriteString(in.text)
		last = in.at
	}
	b.WriteString(src[last:])
	return b.String()
}
