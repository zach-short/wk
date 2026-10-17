package lease

import (
	"path/filepath"
	"strings"
)

// WriteTargets returns the files a shell command writes, by the spellings the walk recognises
// (D11), without a shell parser (PD-4): `sed -i`, `tee`, the `>` family of redirections, `cp`
// (its last argument), `mv` (every argument: the sources disappear), `rm`, `touch`, `git mv`,
// `git rm`, and `python`, `python3` or `node` given a `-c` or `-e` program that opens a file for
// writing; the program of `sh -c` or `bash -c`, and a heredoc fed to a shell, are walked as
// commands. Leading `VAR=value` words, shell keywords and `exec` are skipped, `cd` moves the
// directory for what follows it, `git -C` for its own arguments, and a subshell's `cd` ends at
// its `)`.
//
// Targets are absolute. A word the shell would expand is narrowed to its literal directory
// (`web/*.ts` is `web`), and one that could be anywhere (`$X/a`, `~/a`, a relative path after a
// `cd` the walk cannot follow) is returned as written, not absolute, for the judgment to refuse.
// A word with a brace (`{web,docs}/x.ts`) narrows the same way. A command the walk cannot split
// returns the single target openQuote, which the judgment refuses: bash runs every complete line
// before it reports an open quote on a later one. Every other spelling passes: the walk fails
// closed only on what it recognises.
func WriteTargets(cwd, command string) []string {
	words, ok := split(command)
	if !ok {
		return []string{openQuote}
	}
	w := walker{dir: absOrUnknown(cwd)}
	w.run(words)
	return w.targets
}

// openQuote is the target of a command with a quote that never closes, the one way a split fails.
const openQuote = "<open quote>"

// word is one word after quote removal, or one operator when op is set. exp is the index in text
// of the first character the shell would expand, or -1 when the word is literal.
type word struct {
	text string
	op   string
	exp  int
}

// redirection kinds: a write names a target, a dup may (`>&file`) or may not (`>&2`), a read
// never does. Every operator not listed here ends a command.
const (
	read = iota + 1
	write
	dup
)

var redirections = map[string]int{
	">": write, ">>": write, ">|": write, "&>": write, "&>>": write, "<>": write,
	">&": dup, "<": read, "<&": read, "<<": read, "<<-": read, "<<<": read,
}

type walker struct {
	dir     string // "" when the walk cannot tell
	sep     string // the operator that ended the previous command
	frames  []frame
	argv    []word
	targets []string
}

// frame is one open `(`: the directory to restore at its `)`, and for a process substitution
// (`>(...)`, `<(...)`) the command it sits inside, which goes on after the `)`.
type frame struct {
	dir  string
	argv []word
	sub  bool
}

func (w *walker) run(words []word) {
	for i := 0; i < len(words); i++ {
		t := words[i]
		kind, isRedirect := redirections[t.op]
		switch {
		case t.op == "":
			w.argv = append(w.argv, t)
		case isRedirect && i+1 < len(words) && words[i+1].op == "(":
			i++
			w.frames = append(w.frames, frame{dir: w.dir, argv: w.argv, sub: true})
			w.argv = nil
		case isRedirect:
			if i+1 < len(words) && words[i+1].op == "" {
				i++
				w.redirect(kind, words[i])
			}
		default:
			w.end(t.op)
			w.subshell(t.op)
		}
	}
	w.end("")
}

func (w *walker) redirect(kind int, target word) {
	if kind == write || kind == dup && strings.Trim(target.text, "0123456789") != "" && target.text != "-" {
		w.add(w.dir, target)
	}
}

// subshell keeps a `cd` inside `( ... )` from reaching past its `)`, and takes up the command a
// process substitution sat inside.
func (w *walker) subshell(op string) {
	switch {
	case op == "(":
		w.frames = append(w.frames, frame{dir: w.dir})
	case op == ")" && len(w.frames) > 0:
		f := w.frames[len(w.frames)-1]
		w.frames = w.frames[:len(w.frames)-1]
		w.dir = f.dir
		if f.sub {
			w.argv = f.argv
		}
	}
}

// end judges the simple command gathered so far, which sep ended. `pushd`, `popd`, `eval`,
// `builtin cd`, `command cd` and a `cd` beside `|` or `&` (a subshell in bash, the current shell
// in zsh) move the directory somewhere the walk does not follow, so after them a relative target
// is returned as written and refused. A `case` pattern ends in a `)` that closes no subshell, so
// after `case` no directory is known, the saved ones included.
func (w *walker) end(sep string) {
	argv := skipPrefix(w.argv)
	async := isAsync(sep) || isAsync(w.sep)
	w.argv, w.sep = nil, sep
	if len(argv) == 0 {
		return
	}
	args := argv[1:]
	switch filepath.Base(argv[0].text) {
	case "cd":
		w.cd(args)
		if async {
			w.dir = ""
		}
	case "case":
		w.dir = ""
		for i := range w.frames {
			w.frames[i].dir = ""
		}
	case "pushd", "popd", "eval":
		w.dir = ""
	case "builtin", "command":
		if len(args) > 0 && args[0].text == "cd" {
			w.dir = ""
		}
	case "sed":
		w.add(w.dir, sedFiles(args)...)
	case "tee", "rm", "touch", "mv":
		w.add(w.dir, operands(args)...)
	case "cp":
		if ops := operands(args); len(ops) > 0 {
			w.add(w.dir, ops[len(ops)-1])
		}
	case "git":
		w.git(args)
	case "python", "python3", "node":
		w.add(w.dir, inlineWrites(args)...)
	case "sh", "bash", "zsh", "dash", "ksh":
		w.shell(args)
	}
}

// shell walks the program of `sh -c` and its kin, from the directory in effect. A flag cluster
// holding `c` (`-c`, `-lc`, `-ec`) makes the first operand after it the program.
func (w *walker) shell(args []word) {
	for i, a := range args {
		if len(a.text) > 1 && a.text[0] == '-' && a.text[1] != '-' && strings.ContainsRune(a.text, 'c') {
			if ops := operands(args[i+1:]); len(ops) > 0 {
				w.targets = append(w.targets, WriteTargets(w.dir, ops[0].text)...)
			}
			return
		}
	}
}

func isAsync(sep string) bool { return sep == "|" || sep == "|&" || sep == "&" }

func (w *walker) add(dir string, words ...word) {
	for _, t := range words {
		w.targets = append(w.targets, place(dir, t))
	}
}

func (w *walker) cd(args []word) {
	ops := operands(args)
	if len(ops) == 0 || ops[0].text == "-" || ops[0].exp >= 0 {
		w.dir = ""
		return
	}
	w.dir = absOrUnknown(place(w.dir, ops[0]))
}

// git walks past git's global options to `mv` or `rm`. `-C <dir>` moves where git reads paths.
func (w *walker) git(args []word) {
	dir := w.dir
	for i := 0; i < len(args); i++ {
		switch a := args[i].text; {
		case a == "-C" && i+1 < len(args):
			i++
			dir = absOrUnknown(place(dir, args[i]))
			if args[i].exp >= 0 {
				dir = ""
			}
		case a == "-c" || a == "--git-dir" || a == "--work-tree" || a == "--namespace":
			i++
		case strings.HasPrefix(a, "-"):
		case a == "mv" || a == "rm":
			w.add(dir, operands(args[i+1:])...)
			return
		default:
			return
		}
	}
}

// place makes a target absolute against dir, or returns it as written when it cannot.
func place(dir string, t word) string {
	s := t.text
	if t.exp == 0 && strings.ContainsRune("$`~", rune(s[0])) {
		return s
	}
	if t.exp >= 0 {
		s = s[:strings.LastIndexByte(s[:t.exp], '/')+1]
		if s == "" {
			s = "."
		}
	}
	if filepath.IsAbs(s) {
		return filepath.Clean(s)
	}
	if dir == "" {
		return s
	}
	return filepath.Join(dir, s)
}

func absOrUnknown(dir string) string {
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return ""
}

var keywords = map[string]bool{
	"!": true, "{": true, "if": true, "then": true, "else": true, "elif": true,
	"do": true, "while": true, "until": true, "time": true, "exec": true,
}

func skipPrefix(argv []word) []word {
	for len(argv) > 0 && (keywords[argv[0].text] || isAssignment(argv[0].text)) {
		argv = argv[1:]
	}
	return argv
}

func isAssignment(s string) bool {
	name, _, ok := strings.Cut(s, "=")
	if !ok || name == "" || name[0] >= '0' && name[0] <= '9' {
		return false
	}
	return strings.Trim(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") == ""
}

// operands drops flags; after `--` every word is an operand.
func operands(args []word) []word {
	var out []word
	for i, a := range args {
		if a.text == "--" {
			return append(out, args[i+1:]...)
		}
		if len(a.text) < 2 || a.text[0] != '-' {
			out = append(out, a)
		}
	}
	return out
}

// sedFiles returns the files `sed` edits in place, or nothing without `-i`. The first operand
// is the script unless `-e` or `-f` gave one. A bare `-i` followed by an empty word is the macOS
// BSD form, and the empty word is its backup suffix.
func sedFiles(args []word) []word {
	inPlace, script := false, false
	var rest []word
	for i := 0; i < len(args); i++ {
		a := args[i].text
		var in, s, next bool
		switch {
		case a == "--":
			rest = append(rest, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(a, "--"):
			in, s, next = sedLong(a)
		case len(a) > 1 && a[0] == '-':
			in, s, next = sedCluster(a[1:])
			next = next || a == "-i" && i+1 < len(args) && args[i+1].text == ""
		default:
			rest = append(rest, args[i])
		}
		inPlace, script = inPlace || in, script || s
		if next {
			i++
		}
	}
	if !inPlace {
		return nil
	}
	if !script && len(rest) > 0 {
		rest = rest[1:]
	}
	return rest
}

// sedLong reads one long option: `--in-place` edits in place, and `--expression` or `--file`
// give the script, from the next word when no `=` joins them.
func sedLong(a string) (inPlace, script, next bool) {
	switch {
	case strings.HasPrefix(a, "--in-place"):
		return true, false, false
	case a == "--expression" || a == "--file":
		return false, true, true
	case strings.HasPrefix(a, "--expression=") || strings.HasPrefix(a, "--file="):
		return false, true, false
	}
	return false, false, false
}

// sedCluster reads one short-flag cluster as sed does: `i` takes the rest as its suffix, and
// `e` or `f` take the rest, or the next word when they end the cluster.
func sedCluster(letters string) (inPlace, script, next bool) {
	for j := 0; j < len(letters); j++ {
		switch letters[j] {
		case 'i':
			return true, script, false
		case 'e', 'f':
			return inPlace, true, j == len(letters)-1
		}
	}
	return inPlace, script, false
}

// inlineWrites returns the program of `-c`, `-e`, `--eval`, `-p` or `--print` read for the
// files it opens for writing.
func inlineWrites(args []word) []word {
	for i := 0; i+1 < len(args); i++ {
		switch args[i].text {
		case "-c", "-e", "--eval", "-p", "--print":
			return codeWrites(args[i+1].text)
		}
	}
	return nil
}

// The calls whose first string literal is a file written. A call with a mode counts only when
// the literal after it holds `w`, `a`, `x` or `+`.
var writeCalls = []struct {
	name string
	mode bool
}{
	{"open(", true}, {"openSync(", true},
	{"writeFile(", false}, {"writeFileSync(", false}, {"appendFile(", false}, {"appendFileSync(", false},
}

func codeWrites(code string) []word {
	var out []word
	for _, c := range writeCalls {
		for rest := code; ; {
			i := strings.Index(rest, c.name)
			if i < 0 {
				break
			}
			rest = rest[i+len(c.name):]
			if f, ok := callTarget(rest, c.mode); ok {
				out = append(out, word{text: f, exp: -1})
			}
		}
	}
	return out
}

func callTarget(args string, mode bool) (string, bool) {
	name, rest, ok := literal(args)
	if !ok || !mode {
		return name, ok
	}
	rest = strings.TrimLeft(rest, " \t")
	if !strings.HasPrefix(rest, ",") {
		return "", false
	}
	rest = strings.TrimPrefix(strings.TrimLeft(rest[1:], " \t"), "mode=")
	m, _, ok := literal(rest)
	return name, ok && strings.ContainsAny(m, "wax+")
}

// literal reads one quoted string literal at the start of s, after spaces.
func literal(s string) (val, rest string, ok bool) {
	s = strings.TrimLeft(s, " \t")
	if s == "" || !strings.ContainsRune("'\"`", rune(s[0])) {
		return "", s, false
	}
	for i := 1; i < len(s); i++ {
		switch s[i] {
		case '\\':
			i++
		case s[0]:
			return s[1:i], s[i+1:], true
		}
	}
	return "", s, false
}

// Operators, longest first so `&&` is never read as two `&`.
var operators = []string{
	"&>>", "<<<", "<<-", "&&", "||", ";;", "|&", ">>", ">|", ">&", "&>", "<<", "<&", "<>",
	";", "&", "|", "<", ">", "(", ")",
}

// splitter turns a command into words and operators the way the shell reads quotes: single
// quotes keep everything, double quotes keep all but `\`, `$` and backticks, a backslash keeps
// the next character, and a backslash before a newline joins the lines. A newline ends a
// command, a `#` at the start of a word starts a comment, and a heredoc's body is skipped.
type splitter struct {
	s       string
	i       int
	out     []word
	cur     strings.Builder
	in      bool
	exp     int
	bodies  []heredoc
	pending *heredoc
	line    int // index in out of the first word of the current line
	arith   int // parentheses open inside `((` or `$((`, where `<<` is a shift
	square  int // brackets open inside `$[`
}

type heredoc struct {
	delim string
	tabs  bool
}

func split(s string) ([]word, bool) {
	p := splitter{s: s, exp: -1}
	for p.i < len(p.s) {
		if !p.step() {
			return nil, false
		}
	}
	p.i = len(p.s)
	p.flush()
	return p.out, true
}

func (p *splitter) step() bool {
	c := p.s[p.i]
	switch {
	case c == ' ' || c == '\t':
		p.flush()
		p.i++
	case c == '\n':
		p.newline()
	case c == '#' && !p.in:
		p.comment()
	case c == '\\':
		p.escape()
	case c == '\'':
		return p.single()
	case c == '"':
		return p.double()
	case strings.IndexByte(";&|<>()", c) >= 0:
		p.operator()
	default:
		p.bracket(c)
		p.char(c, strings.IndexByte("$`*?[{", c) >= 0 || c == '~' && !p.in)
		p.i++
	}
	return true
}

// bracket counts `$[ ... ]`, the older arithmetic form, where `<<` is a shift too.
func (p *splitter) bracket(c byte) {
	switch {
	case c == '[' && (p.square > 0 || strings.HasSuffix(p.cur.String(), "$")):
		p.square++
	case c == ']' && p.square > 0:
		p.square--
	}
}

// newline ends a command and skips the heredoc bodies opened on the line. A `<<` that never got
// its delimiter word is dropped, so the lines after it are still walked.
//
// A line that names a shell (`bash <<EOF`, `cat <<EOF | sh`) feeds the body to it as commands,
// so the body is walked too.
func (p *splitter) newline() {
	p.flush()
	p.pending = nil
	if p.namesShell() {
		p.bodies = nil
	}
	p.out = append(p.out, word{op: ";"})
	p.line = len(p.out)
	p.i++
	p.skipBodies()
}

func (p *splitter) namesShell() bool {
	for _, w := range p.out[p.line:] {
		switch filepath.Base(w.text) {
		case "sh", "bash", "zsh", "dash", "ksh":
			return w.op == ""
		}
	}
	return false
}

func (p *splitter) comment() {
	for p.i < len(p.s) && p.s[p.i] != '\n' {
		p.i++
	}
}

func (p *splitter) char(c byte, expands bool) {
	if expands && p.exp < 0 {
		p.exp = p.cur.Len()
	}
	p.cur.WriteByte(c)
	p.in = true
}

// escape keeps the next character. A backslash that ends the input is a continuation with
// nothing after it, as both shells run it.
func (p *splitter) escape() {
	if p.i+1 < len(p.s) && p.s[p.i+1] != '\n' {
		p.char(p.s[p.i+1], false)
	}
	p.i += 2
}

func (p *splitter) single() bool {
	end := strings.IndexByte(p.s[p.i+1:], '\'')
	if end < 0 {
		return false
	}
	p.cur.WriteString(p.s[p.i+1 : p.i+1+end])
	p.in = true
	p.i += end + 2
	return true
}

func (p *splitter) double() bool {
	p.in = true
	for p.i++; p.i < len(p.s); p.i++ {
		switch c := p.s[p.i]; {
		case c == '"':
			p.i++
			return true
		case c == '\\' && p.i+1 < len(p.s) && strings.IndexByte("$`\"\\\n", p.s[p.i+1]) >= 0:
			p.i++
			if p.s[p.i] != '\n' {
				p.char(p.s[p.i], false)
			}
		default:
			p.char(c, c == '$' || c == '`')
		}
	}
	return false
}

// operator reads one operator. Digits right before a redirection are its file descriptor
// (`2>err`), not a word.
func (p *splitter) operator() {
	op := ""
	for _, o := range operators {
		if strings.HasPrefix(p.s[p.i:], o) {
			op = o
			break
		}
	}
	text := p.cur.String()
	if (op[0] == '<' || op[0] == '>') && p.in && p.exp < 0 && strings.Trim(text, "0123456789") == "" && text != "" {
		p.reset()
	} else {
		p.flush()
	}
	p.out = append(p.out, word{op: op})
	p.heredocOrArith(op)
	p.i += len(op)
}

// heredocOrArith opens a heredoc on `<<` or `<<-`, except inside `((`, `$((` or `$[`, where `<<` is a
// shift and opening a heredoc would hide the lines after it. Any `((` counts as arithmetic: when
// it was two subshells, a real heredoc inside is walked as commands, which can only refuse more.
func (p *splitter) heredocOrArith(op string) {
	switch {
	case op == "(" && (p.arith > 0 || strings.HasPrefix(p.s[p.i+1:], "(")):
		p.arith++
	case op == ")" && p.arith > 0:
		p.arith--
	case (op == "<<" || op == "<<-") && p.arith == 0 && p.square == 0:
		p.pending = &heredoc{tabs: op == "<<-"}
	}
}

func (p *splitter) flush() {
	if !p.in {
		return
	}
	p.out = append(p.out, word{text: p.cur.String(), exp: p.exp})
	if p.pending != nil {
		p.pending.delim = p.cur.String()
		p.bodies = append(p.bodies, *p.pending)
		p.pending = nil
	}
	p.reset()
}

func (p *splitter) reset() {
	p.cur.Reset()
	p.in = false
	p.exp = -1
}

// skipBodies passes over the bodies of the heredocs opened on the line just ended, so a `>` or
// an `rm` in a body is never read as a command. A body whose delimiter never comes is walked as
// commands from its first line: whatever opened it by mistake, a missing heredoc can only
// refuse more.
func (p *splitter) skipBodies() {
	for _, h := range p.bodies {
		if !p.skipBody(h) {
			break
		}
	}
	p.bodies = nil
}

func (p *splitter) skipBody(h heredoc) bool {
	start := p.i
	for p.i < len(p.s) {
		line, _, _ := strings.Cut(p.s[p.i:], "\n")
		p.i = min(p.i+len(line)+1, len(p.s))
		if h.tabs {
			line = strings.TrimLeft(line, "\t")
		}
		if line == h.delim {
			return true
		}
	}
	p.i = start
	return false
}
