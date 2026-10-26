package slot

import (
	"path"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"mvdan.cc/sh/v3/syntax"
)

// Missed is one gate spelling Rewrite left alone, for missed.log, so the matcher grows from
// evidence (SB-15). Command is the source text of the simple command that holds the spelling,
// or the whole line when it does not parse.
type Missed struct{ Command, Reason string }

// Rewrite inserts "<wkExe> run --slot <class> [--trim <p>]... -- " at the byte offset of each
// matched simple command's first word (SB-15, every statement position, never in a function
// body) and changes no other byte. Statement positions include the command of `time` and the
// commands inside $( ) and backticks; behind env, command and the time utility the insertion
// goes after the wrapper's words (a gate behind timeout stays a miss), and a `<shell> -c` call
// is wrapped whole (match_wrappers.go). Text that does not parse comes back unchanged with
// parseErr set. Each literal spelling left alone is a Missed with its reason.
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
	// backquotes counts the backtick substitutions around the walk. Inside one the shell reads a
	// backslash, a backtick and a dollar sign once more before it parses the command.
	backquotes int
	// stderrCaptured is the miss reason while the walk is inside a substitution that captures
	// stderr, and "" otherwise.
	stderrCaptured string
	inserts        []matchInsert
	missed         []Missed
}

func (w *matchWalk) stmts(list []*syntax.Stmt) {
	for _, s := range list {
		w.stmt(s)
	}
}

// stmt visits one statement position. The cases are the positions GATE 2 answered "every
// statement" for. `coproc` is left out because the wrapper's wait would change what it starts.
// `time` was left out for the same reason until 2026-10-26, when it held 14 of ezhomesteading's
// missed.log lines from 2026-10-17 to 2026-10-25, each a gate an agent timed. Under
// `time <wk> run` the real time also counts the wait for a place, which wk run reports on
// stderr; a gate that runs unslotted for an honest time is the overload the slot exists to stop.
// Claude Code strips `time` before it matches a permission rule, so the gate's twin still applies.
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
		if c.Stmt != nil {
			w.stmt(c.Stmt)
		}
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
	g, reason := w.gate(c)
	switch {
	case (reason != "" || g.word != nil) && w.stderrCaptured != "":
		// Row 28, from the row 27 audit N4: inside a substitution that captures stderr no gate
		// is rewritten whatever else holds it back, so that reason is logged, and wk stats counts
		// `x=$(timeout 300 sh -c 'go test' 2>&1)` under the capture, not under timeout. Row 29
		// (N-D): on a line that holds wk run too, where it logged that reason before.
		w.miss(c, w.stderrCaptured)
	case reason != "":
		w.miss(c, reason)
	case g.word == nil:
	case w.wrapped:
		w.miss(c, "the line already holds wk run")
	default:
		w.insert(c, g)
	}
}

// words looks inside the words of a non-statement part, where a command can only sit in a
// substitution. A command substitution holds statements of its own, so they are walked as
// statements: the insertion lands at the start of a command inside it. wk run writes nothing to
// stdout, but it writes its waiting, place and trim lines to stderr, so the captured text stays
// the same only while stderr stays out of the capture; subst makes every gate in a substitution
// that sends stderr into it a miss. A process substitution stays a miss: its command runs
// beside the outer one, and wk does not follow what waits for it.
func (w *matchWalk) words(n syntax.Node) {
	syntax.Walk(n, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.CmdSubst:
			w.subst(n)
			return false
		case *syntax.ProcSubst:
			w.missAll(n, "inside a process substitution")
			return false
		}
		return true
	})
}

// subst walks a substitution's statements. One that sends stderr into its capture anywhere
// inside (row 22 audit B2: `[ -z "$(bun run lint 2>&1)" ]` would see wk run's lines and turn
// red) holds every gate in it, nested substitutions included, as a miss.
func (w *matchWalk) subst(c *syntax.CmdSubst) {
	if c.Backquotes {
		w.backquotes++
		defer func() { w.backquotes-- }()
	}
	outer := w.stderrCaptured
	defer func() { w.stderrCaptured = outer }()
	if outer == "" && matchCapturesStderr(c) {
		w.stderrCaptured = "inside $( ) that captures stderr"
		if c.Backquotes {
			w.stderrCaptured = "inside backticks that capture stderr"
		}
	}
	w.stmts(c.Stmts)
}

// matchCapturesStderr is true when a command inside c may write its stderr where c's stdout
// goes. It errs toward true, because a wrong true costs one unslotted gate and a wrong false
// changes the captured text (row 22 audits B2 and A): |&, &> and &>> anywhere, even to a file;
// any dup of fd 2 (>& or <&) to anything but a plain `-`, so 2>&3 after 3>&1 and 2>&1- count;
// any fd-2 redirect whose target is not plain literal text, such as 2>&"1" or 2> >(cat), or is
// under /dev/stdout or /dev/fd once the path is cleaned; >&word with no fd number whose word is
// not an fd, which bash reads as &>word; and an exec that dups any fd, which lasts for the
// commands after it. 2>/dev/null, 2>&-, >&1 and >&2 stay out of the capture.
func matchCapturesStderr(c *syntax.CmdSubst) bool {
	found := false
	syntax.Walk(c, func(n syntax.Node) bool {
		switch n := n.(type) {
		case *syntax.Stmt:
			found = found || matchExecDups(n)
		case *syntax.Redirect:
			found = found || matchStderrToStdout(n)
		case *syntax.BinaryCmd:
			found = found || n.Op == syntax.PipeAll
		}
		return !found
	})
	return found
}

func matchStderrToStdout(r *syntax.Redirect) bool {
	if r.Op == syntax.RdrAll || r.Op == syntax.AppAll {
		return true
	}
	if r.N == nil && r.Op == syntax.DplOut && r.Word != nil {
		// Bash reads >&word with no fd number as &>word unless word is digits or `-` (row 22
		// audit 3): >&/dev/stdout sends stderr into the capture as &>/dev/stdout does.
		target, plain := matchLiteral(r.Word)
		return !plain || !matchFDWord(target)
	}
	if r.N == nil || !matchFDIs(r.N.Value, 2) || r.Word == nil {
		return false
	}
	target, plain := matchLiteral(r.Word)
	switch {
	case !plain:
		return true
	case r.Op == syntax.DplOut || r.Op == syntax.DplIn:
		return target != "-"
	}
	target = path.Clean(target)
	return target == "/dev/stdout" || strings.HasPrefix(target, "/dev/fd/")
}

// matchFDIs is true when the digits of a redirect's fd number name fd; bash reads 02 and 002
// as fd 2 (row 22 audit 4).
func matchFDIs(digits string, fd int) bool {
	n, err := strconv.Atoi(digits)
	return err == nil && n == fd && strings.Trim(digits, "0123456789") == ""
}

// matchFDWord is true for the word of a dup that names a file descriptor: `-`, or digits with
// an optional trailing `-` that moves the fd (>&2-).
func matchFDWord(word string) bool {
	if word == "-" {
		return true
	}
	digits := strings.TrimSuffix(word, "-")
	if digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func matchExecDups(s *syntax.Stmt) bool {
	call, ok := s.Cmd.(*syntax.CallExpr)
	if !ok || len(call.Args) == 0 || call.Args[0].Lit() != "exec" {
		return false
	}
	for _, r := range s.Redirs {
		if r.Op == syntax.DplOut || r.Op == syntax.DplIn {
			return true
		}
	}
	return false
}

// missAll records every spelling under n with the reason of the outermost place that kept it
// from being rewritten. Inside a substitution that captures stderr that place is the capture,
// whose reason call logs over any other (row 29, from the row 28 audit N-D: a function body, a
// coproc or a process substitution inside such a capture logged its own reason before).
func (w *matchWalk) missAll(n syntax.Node, reason string) {
	if w.stderrCaptured != "" {
		reason = w.stderrCaptured
	}
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
// `eval` or a `<shell> -c` runs, or after a wrapper word, where gate did not route it. It is
// for missed.log only (SB-15), under the outer reason when there is one.
func (w *matchWalk) unmatched(c *syntax.CallExpr, outer string) {
	reason, ok := w.unmatchedReason(c)
	if !ok {
		return
	}
	if outer != "" {
		reason = outer
	}
	w.miss(c, reason)
}

func (w *matchWalk) unmatchedReason(c *syntax.CallExpr) (string, bool) {
	if reason, ok := w.nestedShellReason(c); ok {
		return reason, true
	}
	return w.wrapperReason(c)
}

// nestedShellReason logs a `<shell> -c` call or an eval whose string runs a gate. The string is
// one word to this parser, so runsGateAt parses it as a line of its own (row 29, the row 28
// builder's open item: `eval 'echo go test'` runs no gate and is not logged); a string that is
// not plain text or does not parse is searched as text, as before.
func (w *matchWalk) nestedShellReason(c *syntax.CallExpr) (string, bool) {
	reason, ok := matchNestedShell(c.Args)
	return reason, ok && w.runsGateAt(c, 0)
}

// A wrapper runs a command made of its later words, so a gate behind one runs unslotted. It is
// logged, not rewritten, because the wrapper's own options sit between it and the gate; env,
// command and the time utility are routed when matchWrapped can read every word before the
// gate. A gate with timeout anywhere before it is logged with matchTimeoutReason, which
// match_wrappers.go explains. The set is closed on purpose: any word before a spelling would
// also log `grep eslint`.
//
// Row 27 (from the row 25 audit): the gate behind a wrapper may also sit in the string of a
// `<shell> -c` call or an eval, so `timeout 300 sh -c 'go test'` is logged with
// matchTimeoutReason; and a quoted wrapper word, `"/usr/bin/time" -p go test`, is logged as a
// quoted spelling, the reason a quoted gate word gets, since matchWrapped reads plain words only.
// Row 28: a wrapper name spelled as a relative path (./time, bin/env) is logged with
// matchRelativeWrapperReason, since matchWrapped reads a bare name or an absolute path only.
var matchWrappers = map[string]bool{
	"exec": true, "nohup": true, "nice": true, "env": true, "command": true, "xargs": true,
	"timeout": true, "gtimeout": true, "time": true, "gtime": true,
}

func (w *matchWalk) wrapperReason(c *syntax.CallExpr) (string, bool) {
	if len(c.Args) < 2 {
		return "", false
	}
	name, _ := matchUnquoted(c.Args[0])
	name = path.Base(name)
	if !matchWrappers[name] {
		return "", false
	}
	for i := 1; i < len(c.Args); i++ {
		if w.runsGateAt(c, i) {
			return matchWrapperMiss(name, c.Args[:i]), true
		}
	}
	return "", false
}

// runsGateAt is true when the words of c from i on spell a gate, or are a `<shell> -c` call or
// an eval whose string runs one. The string is parsed and walked as a line of its own, so a gate
// counts only where Rewrite would find it: as a command's first words, behind a wrapper, or in a
// nested string (row 28, from the row 27 audit N3: `timeout 30 sh -c 'echo go test'` runs no
// gate). Where the string is not plain text or does not parse, its text is searched as before,
// which errs toward logging.
func (w *matchWalk) runsGateAt(c *syntax.CallExpr, i int) bool {
	args := c.Args[i:]
	if _, _, found := w.lookup(args); found {
		return true
	}
	if _, nested := matchNestedShell(args); !nested {
		return false
	}
	if text, plain := matchNestedText(args); plain {
		if runs, parsed := w.nestedGates(text); parsed {
			return runs
		}
	}
	return matchMentions(w.text(args[1].Pos(), c.End()), w.spellings)
}

// nestedGates parses text, the string of a `<shell> -c` call or an eval, and walks it as a line
// of its own. runs is true when the walk finds a gate it would rewrite or log, or a wk run whose
// later words run one: the walk leaves a wk run alone, but behind a wrapper the bare form
// `timeout 30 wk run -- go test` is logged, so `timeout 30 sh -c 'wk run -- go test'` is too
// (row 29, from the row 28 audit N-B). parsed is false when text does not parse. It decides only
// whether a gate runs; shellGate takes the class from every spelling the text mentions.
func (w *matchWalk) nestedGates(text string) (runs, parsed bool) {
	file, err := syntax.NewParser(syntax.Variant(syntax.LangBash)).Parse(strings.NewReader(text), "")
	if err != nil {
		return false, false
	}
	inner := matchWalk{src: text, exe: w.exe, spellings: w.spellings}
	inner.wrapped = inner.holdsWKRun(file)
	inner.stmts(file.Stmts)
	runs = len(inner.inserts) > 0 || len(inner.missed) > 0
	syntax.Walk(file, func(n syntax.Node) bool {
		c, ok := n.(*syntax.CallExpr)
		if runs || !ok || !inner.isWKRun(c) {
			return !runs
		}
		for j := 2; j < len(c.Args) && !runs; j++ {
			runs = inner.runsGateAt(c, j)
		}
		return !runs
	})
	return runs, true
}

// matchNestedText is the text a `<shell> -c` call or an eval runs, when every word of it is
// plain text: the string after -c, or eval's words joined by spaces, as eval joins them.
func matchNestedText(args []*syntax.Word) (string, bool) {
	if path.Base(args[0].Lit()) == "eval" {
		words := make([]string, 0, len(args)-1)
		for _, a := range args[1:] {
			text, plain := matchPlainText(a)
			if !plain {
				return "", false
			}
			words = append(words, text)
		}
		return strings.Join(words, " "), true
	}
	name, at, reason := matchShellString(args)
	if name == "" || reason != "" {
		return "", false
	}
	return matchPlainText(args[at])
}

// matchWrapperMiss gives timeout's reason first, wherever it sits before the gate, so a gate
// behind timeout is always counted as one; ./timeout counts too. Then a quoted wrapper word
// anywhere before the gate, then a wrapper name spelled as a relative path (row 28) or under a
// home directory (row 29) anywhere before the gate, which matchWrapped does not read. An
// assignment word is no wrapper word: env X=/a/timeout runs no timeout.
//
// Row 29, from the row 28 audit N-D: before it only the first word was checked for quotes, so
// `env -u X "time" -p go test` was logged after the wrapper env, and an assignment whose value
// ended in /timeout took timeout's reason. Labels only; no rewrite changed.
func matchWrapperMiss(name string, before []*syntax.Word) string {
	words := make([]*syntax.Word, 0, len(before))
	for _, word := range before {
		if _, isAssign := matchAssignName(word); !isAssign {
			words = append(words, word)
		}
	}
	for _, word := range words {
		v, _ := matchUnquoted(word)
		if base := path.Base(v); base == "timeout" || base == "gtimeout" {
			return matchTimeoutReason
		}
	}
	for _, word := range words {
		v, unquoted := matchUnquoted(word)
		if _, literal := matchLiteral(word); unquoted && !literal && matchWrappers[path.Base(v)] {
			return "quoted spelling"
		}
	}
	for _, word := range words {
		lit := word.Lit()
		if _, relative := matchWrapperName(lit); relative && matchWrappers[path.Base(lit)] {
			if strings.HasPrefix(lit, "~") {
				return matchHomeWrapperReason
			}
			return matchRelativeWrapperReason
		}
	}
	return "after the wrapper " + name
}

var matchShells = map[string]bool{"bash": true, "sh": true, "zsh": true, "dash": true, "ksh": true}

func matchNestedShell(args []*syntax.Word) (string, bool) {
	if len(args) < 2 {
		return "", false
	}
	name := path.Base(args[0].Lit())
	if name == "eval" {
		return "inside eval", true
	}
	if !matchShells[name] {
		return "", false
	}
	for _, a := range args[1:] {
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

func (w *matchWalk) insert(c *syntax.CallExpr, g matchGate) {
	prefix, ok := matchPrefix(w.exe, g.entry)
	if !ok {
		w.miss(c, "the wk path or the entry cannot be quoted")
		return
	}
	// call logs a gate inside a capture of stderr before it reaches insert.
	if w.backquotes > 0 && strings.ContainsAny(prefix, "\\`$") {
		w.miss(c, "inside backticks")
		return
	}
	w.inserts = append(w.inserts, matchInsert{at: g.word.Pos().Offset(), text: prefix})
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

func matchMentions(text string, spellings []matchSpelling) bool {
	return len(matchMentioned(text, spellings)) > 0
}

// matchMentioned lists the spelling at each place text spells one, the longest where two start
// at one word. It searches text that was not parsed into commands, so its tokens break at shell
// punctuation as well as at spaces.
func matchMentioned(text string, spellings []matchSpelling) []matchSpelling {
	tokens := strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(";&|()<>'\"`$", r)
	})
	var found []matchSpelling
	for i := 0; i < len(tokens); i++ {
		for _, s := range spellings {
			if len(tokens)-i >= len(s.words) && slices.Equal(tokens[i:i+len(s.words)], s.words) {
				found = append(found, s)
				i += len(s.words) - 1
				break
			}
		}
	}
	return found
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
