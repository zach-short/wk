package lease

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// safeSet is every byte the walk reads, the allowlist of the commit guard's `raw_words`. Outside
// it are `$`, the backtick, `(`, `)`, `{`, `}`, `[`, `]`, `*`, `?`, the backslash and every
// non-ASCII byte: where the shell expands, substitutes, globs or escapes, and a word walk cannot
// follow it.
const safeSet = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 \t\n-_./=:,+@~%'\"|&;<>#!"

// zshRange is zsh's numeric glob (`x<1-9>.ts`), the one glob spelled in safe characters.
var zshRange = regexp.MustCompile(`<[0-9]*-[0-9]*>`)

// WriteTargets returns the files a shell command writes, by the spellings the walk recognises
// (D11), without a shell parser (PD-4).
//
// The walk reads a command only when every byte is in the safe set: ASCII letters and digits,
// space, tab, newline and - _ . / = : , + @ ~ % ' " | & ; < > # !. Any other byte (`$`, a
// backtick, `(`, `)`, `{`, `}`, `[`, `]`, `*`, `?`, a backslash, any non-ASCII byte), or a zsh
// numeric glob, refuses the whole command: the one target names it, and the judgment denies. In
// the safe set nothing expands inside quotes, so quotes only join words.
//
// The spellings: the `>` family of redirections (zsh's `>!` and `>>&` too); `rm`, `unlink`,
// `rmdir`, `touch`, `tee`, `truncate`, `rsync`, `chmod` and `mv` (every operand); `cp`, `ln` and
// `install` (the last operand, or every one with `-t`); `sed -i` and `sed -I`; `dd of=`; `find
// -delete` (its starting points); `tar` (its operands, and where it extracts); `gofmt -w`,
// `goimports -w`, `prettier --write`, `perl -i` and `awk -i` (every operand); `git mv` and `git
// rm`; the git subcommands that rewrite the worktree (a write of its root, see gitWritesTree). A
// git verb outside the walk's list (see gitReads), a `git -c alias.` key and the git options and
// variables that read paths or run a command (see gitRuns, gitConfig, gitVariable) refuse the
// command; `--output` and `git worktree add` are writes of the path they name.
// `patch`, `find -exec`, `xargs`, and a shell or `source` reading standard input write what the
// walk cannot see. The programs of `sh -c` and its kin, `eval`, `trap`, `alias` and `git -c`, every
// heredoc body and every here-string are walked as commands. Keywords, `NAME=value` and
// `NAME+=value`, and the wrappers of `wrappers` are skipped before the verb.
//
// Targets are absolute, one for each directory the command may run in: every `cd` and `pushd`
// adds a directory and keeps the ones before it. A target the walk cannot place (an unknown
// directory, a word the shell rewrites, git beside a GIT_ variable) is returned as written, which
// the judgment refuses. A quote that never closes returns the one target openQuote, and a heredoc
// with no end line the one target openHeredoc. Every other
// verb passes.
func WriteTargets(cwd, command string) []string {
	var out []string
	for _, t := range writes(cwd, command) {
		out = append(out, t.path)
	}
	return out
}

// target is one write. A repo write (`git checkout`, `git reset --hard`) rewrites the whole
// worktree that holds path, so the judgment widens it to the root.
type target struct {
	path string
	repo bool
}

func writes(cwd, command string) []target {
	if i := strings.IndexFunc(command, outsideSafeSet); i >= 0 {
		r, _ := utf8.DecodeRuneInString(command[i:])
		return []target{{path: unreadablePrefix + printable(r) + ">"}}
	}
	if m := zshRange.FindString(command); m != "" {
		return []target{{path: unreadablePrefix + m + ">"}}
	}
	w := walker{whole: command}
	w.nested([]string{absOrUnknown(cwd)}, command)
	return w.targets
}

func outsideSafeSet(r rune) bool { return r >= utf8.RuneSelf || !strings.ContainsRune(safeSet, r) }

func printable(r rune) string {
	if unicode.IsPrint(r) {
		return string(r)
	}
	return strings.Trim(strconv.QuoteRune(r), "'")
}

// The targets that name no file but a reason the walk cannot read the command.
const (
	openQuote         = "<open quote>"
	openHeredoc       = "<open heredoc>"
	unreadablePrefix  = "<character "
	pathspecPrefix    = "<git pathspec "
	fromFilePrefix    = "<git paths from "
	aliasPrefix       = "<git alias "
	gitVerbPrefix     = "<git verb "
	gitConfigPrefix   = "<git config "
	gitVariablePrefix = "<git variable "
	grepPrefix        = "<git grep pager "
	transportPrefix   = "<git transport "
)

// prefixed are the denials for the targets that carry what the walk could not read.
var prefixed = []struct{ prefix, format string }{
	{pathspecPrefix, "wk: cannot tell which file this writes: git removes a `..` in a pathspec by text and reads a leading `:` as magic, so `%s` cannot be placed; give the path from the worktree's top without either"},
	{fromFilePrefix, "wk: cannot tell which file this writes: with `%s` git reads the paths from a file or standard input, which the lease walk does not read; give the paths on the command line"},
	{aliasPrefix, "wk: cannot tell which file this writes: `%s` is a git alias, which the lease walk does not read; run the git verb directly"},
	{gitVerbPrefix, "wk: cannot tell which file this writes: `git %s` is not a git verb the lease walk reads (an alias, an unlisted command, or a form that changes refs or config); run a git verb it reads"},
	{gitConfigPrefix, "wk: cannot tell which file this writes: `%s` is config that names a program or a file, or takes its value from the environment, which the lease walk does not read; run git without it"},
	{gitVariablePrefix, "wk: cannot tell which file this writes: `%s` is set beside git, and a GIT_ variable can carry a command or config, which the lease walk does not read; run git without it"},
	{grepPrefix, "wk: cannot tell which file this writes: with `%s` git grep runs a command on the matched files, which the lease walk does not read; run git grep without it"},
	{transportPrefix, "wk: cannot tell which file this writes: with `%s` git runs a command to reach the other repository, which the lease walk does not read; run git without it"},
	{unreadablePrefix, "wk: cannot tell which file this writes: the command holds `%s`, which the lease walk does not read; write it without that character or ask for the path in full"},
}

// refusal returns the denial for a target that names no file.
func refusal(path string) (string, bool) {
	switch path {
	case openQuote:
		return "wk: cannot tell which file this writes: the command has an open quote, and bash runs the lines before it; close the quote", true
	case openHeredoc:
		return "wk: cannot tell which file this writes: a heredoc in the command has no end line; end it with its delimiter alone on a line", true
	}
	for _, p := range prefixed {
		if v, ok := strings.CutPrefix(path, p.prefix); ok {
			return fmt.Sprintf(p.format, strings.TrimSuffix(v, ">")), true
		}
	}
	return "", false
}

// word is one word after quote removal, or one operator when op is set. expands marks a word the
// shell rewrites: a leading `~` or `=` (zsh), or a `~` or `#` anywhere outside quotes (zsh's
// extended glob), which the walk cannot place.
type word struct {
	text    string
	op      string
	expands bool
}

// literal is a word cut out of another, such as the value of `of=`.
func literal(s string) word {
	return word{text: s, expands: s != "" && (s[0] == '=' || strings.ContainsAny(s, "~#"))}
}

// unseen is a write the walk knows of but cannot name, which the judgment refuses.
func unseen(what string) word { return word{text: "<" + what + ">", expands: true} }

// unread is a target that names what the walk could not read, under one of the prefixes refusal
// knows.
func unread(prefix, what string) word { return word{text: prefix + what + ">", expands: true} }

// redirection kinds: a write names a target, a dup may (`>&file`) or may not (`>&2`), a read
// never does, and a here-string's word is walked as a command, like a heredoc body, since it may
// be a program (`source /dev/stdin <<<'rm f'`). Every operator not listed here ends a command.
const (
	read = iota + 1
	write
	dup
	here
)

var redirections = map[string]int{
	">": write, ">>": write, ">|": write, ">!": write, ">>|": write, ">>!": write,
	"&>": write, "&>>": write, "&>|": write, "&>!": write, ">>&": write, ">&|": write, ">&!": write,
	"<>": write, ">&": dup, "<": read, "<&": read, "<<": read, "<<-": read, "<<<": here,
}

type walker struct {
	dirs    []string // every directory the command may be in; "" for one the walk cannot tell
	whole   string   // the whole command, for the variables that move git and cd (GIT_, CDPATH)
	argv    []word
	targets []target
}

func (w *walker) run(words []word) {
	for i := 0; i < len(words); i++ {
		t := words[i]
		kind, isRedirect := redirections[t.op]
		switch {
		case t.op == "":
			w.argv = append(w.argv, t)
		case isRedirect && i+1 < len(words) && words[i+1].op == "":
			i++
			w.redirect(kind, words[i])
		case !isRedirect:
			w.end()
		}
	}
	w.end()
}

func (w *walker) redirect(kind int, target word) {
	if kind == here {
		w.nested(w.dirs, target.text)
	}
	if kind == write || kind == dup && strings.Trim(target.text, "0123456789") != "" && target.text != "-" {
		w.add(w.dirs, target)
	}
}

// end judges the simple command gathered so far.
func (w *walker) end() {
	argv := w.unwrap(w.argv)
	w.argv = nil
	if len(argv) > 0 {
		w.command(filepath.Base(argv[0].text), argv[1:])
	}
}

func (w *walker) command(verb string, args []word) {
	switch verb {
	case "cd", "pushd", "popd":
		w.cd(verb, args)
	case "case":
		w.dirs = []string{""}
	case "eval":
		w.nested(w.dirs, joined(args))
		w.dirs = []string{""}
	case "trap", "alias":
		w.deferred(verb, args)
	case "sh", "bash", "zsh", "dash", "ksh", "mksh", "fish", "csh", "tcsh":
		w.shell(args)
	case "source", ".":
		if ops := operands(args); len(ops) > 0 && fromStdin(ops) {
			w.add(w.dirs, unseen("standard input"))
		}
	case "git":
		w.git(args)
	default:
		w.add(w.dirs, writtenBy(verb, args)...)
	}
}

// add records each word as a target at every directory, once when it needs none.
func (w *walker) add(dirs []string, words ...word) {
	for _, t := range words {
		for _, d := range dirs {
			w.targets = append(w.targets, target{path: place(d, t)})
			if t.expands || filepath.IsAbs(t.text) {
				break
			}
		}
	}
}

// place makes a target absolute against dir, or returns it as written when it cannot. A path
// with a `..`, in the target or in dir, is not cleaned: the kernel applies the `..` after
// following any symlink before it, so only a caller that reads the disk (Request.Canon) may
// remove it.
func place(dir string, t word) string {
	switch {
	case t.expands:
		return t.text
	case dotDot(t.text) || dotDot(dir) && !filepath.IsAbs(t.text):
		if filepath.IsAbs(t.text) || dir == "" {
			return t.text
		}
		return strings.TrimSuffix(dir, "/") + "/" + t.text
	case filepath.IsAbs(t.text):
		return filepath.Clean(t.text)
	case dir == "":
		return t.text
	}
	return filepath.Join(dir, t.text)
}

func dotDot(p string) bool { return slices.Contains(strings.Split(p, "/"), "..") }

func absOrUnknown(dir string) string {
	if filepath.IsAbs(dir) {
		return filepath.Clean(dir)
	}
	return ""
}

// maxDirs bounds the directories one command tracks; past it the walk cannot tell.
const maxDirs = 16

// cd adds the directory a `cd` or `pushd` moves to and keeps every one before it, so a later
// relative target is judged at each: the change may not run (`test -f x && cd docs`, `cd nope`),
// or may run in a subshell (`cd web | true`). A change the walk cannot follow (`popd`, `cd -`, a
// bare `cd`, `pushd +1`, zsh's `cd old new`, a relative `cd` beside CDPATH) adds an unknown
// directory.
func (w *walker) cd(verb string, args []word) {
	ops := operands(args)
	next := []string{""}
	if verb != "popd" && len(ops) == 1 && w.follows(ops[0].text) {
		next = moved(w.dirs, ops[0])
	}
	all := slices.Clone(w.dirs)
	for _, d := range next {
		if !slices.Contains(all, d) {
			all = append(all, d)
		}
	}
	if len(all) > maxDirs {
		all = []string{""}
	}
	w.dirs = all
}

func (w *walker) follows(dir string) bool {
	switch {
	case dir == "" || strings.ContainsRune("-+", rune(dir[0])):
		return false
	case strings.Contains(w.whole, "CDPATH"):
		return filepath.IsAbs(dir)
	}
	return true
}

// moved is where a `cd` to `to` lands from each of dirs. A path with a `..` lands at two places:
// the text-cleaned one, where bash's logical `cd` goes, and the raw one, which the caller reads
// through symlinks, where `cd -P` goes and where bash goes when the logical path does not exist.
func moved(dirs []string, to word) []string {
	var out []string
	for _, d := range dirs {
		p := place(d, to)
		out = append(out, absOrUnknown(p))
		if dotDot(p) && filepath.IsAbs(p) {
			out = append(out, p)
		}
	}
	return out
}

// nested walks a program the command runs (`sh -c`, `eval`) from dirs. A quote that never
// closes refuses the program: bash runs every complete line before it reports the open quote.
func (w *walker) nested(dirs []string, program string) {
	words, failed := split(program)
	if failed != "" {
		w.targets = append(w.targets, target{path: failed})
		return
	}
	sub := walker{dirs: slices.Clone(dirs), whole: w.whole}
	sub.run(words)
	w.targets = append(w.targets, sub.targets...)
}

// deferred walks a program the shell keeps to run later, a trap's or an alias's, from a
// directory the walk cannot know.
func (w *walker) deferred(verb string, args []word) {
	for _, a := range operands(args) {
		program := a.text
		if verb == "alias" {
			_, program, _ = strings.Cut(program, "=")
		}
		w.nested([]string{""}, program)
		if verb == "trap" {
			return
		}
	}
}

// shell walks the program of `sh -c` and its kin: a flag cluster holding `c` (`-c`, `-lc`) makes
// the first operand after it the program. Without `-c` the shell reads its program from a file,
// which the walk does not read, or from standard input (`echo rm x | sh`), which it cannot see.
func (w *walker) shell(args []word) {
	for i, a := range args {
		if len(a.text) > 1 && a.text[0] == '-' && a.text[1] != '-' && strings.ContainsRune(a.text, 'c') {
			if ops := operands(args[i+1:]); len(ops) > 0 {
				w.nested(w.dirs, ops[0].text)
			}
			return
		}
	}
	if fromStdin(operands(args)) {
		w.add(w.dirs, unseen("standard input"))
	}
}

func fromStdin(ops []word) bool {
	return len(ops) == 0 || slices.Contains([]string{"-", "/dev/stdin", "/dev/fd/0"}, ops[0].text)
}

func joined(args []word) string {
	var texts []string
	for _, a := range args {
		texts = append(texts, a.text)
	}
	return strings.Join(texts, " ")
}

var keywords = map[string]bool{
	"!": true, "if": true, "then": true, "else": true, "elif": true, "do": true, "while": true, "until": true,
}

// wrapper is a command that runs the command after it: the option letters that take a value, the
// flag letters it may carry, and the words it reads before the command. Any other option refuses
// the command, because it may change what runs or where (`env -C dir`, `sudo -D dir`).
type wrapper struct {
	values, flags string
	skip          int  // `timeout`'s duration, zsh's `repeat` count
	assigns       bool // `env`: NAME=value words before the command
	input         bool // `xargs`: the command gets operands the walk cannot see
}

var wrappers = map[string]wrapper{
	"command": {flags: "pvV"}, "builtin": {}, "exec": {values: "a", flags: "cl"}, "nohup": {},
	"time": {flags: "p"}, "coproc": {}, "noglob": {}, "nocorrect": {}, "-": {}, "repeat": {skip: 1}, "busybox": {},
	"nice":    {values: "n"},
	"timeout": {values: "sk", flags: "fpv", skip: 1},
	"env":     {values: "u", flags: "i", assigns: true},
	"sudo":    {values: "u", flags: "nEH"},
	"xargs":   {values: "EILJPRSadns", flags: "0oprtx", input: true},
}

// unwrap drops what comes before the verb: keywords, assignments, and wrappers with their
// options. A wrapper option the walk does not know becomes an unseen write.
func (w *walker) unwrap(argv []word) []word {
	for {
		argv = skipPrefix(argv)
		if len(argv) == 0 {
			return nil
		}
		name := filepath.Base(argv[0].text)
		wr, ok := wrappers[name]
		if !ok {
			return argv
		}
		rest, ok := wr.strip(argv[1:])
		if !ok {
			w.add(w.dirs, unseen(name+" option"))
			return nil
		}
		if wr.input && len(rest) > 0 {
			rest = append(slices.Clip(rest), unseen("xargs input"))
		}
		argv = rest
	}
}

// strip drops the wrapper's options and the words it reads before the command. ok is false for
// an option it does not know, a long one included.
func (wr wrapper) strip(args []word) ([]word, bool) {
	for len(args) > 0 {
		a := args[0].text
		if a == "--" {
			return wr.after(args[1:]), true
		}
		if a == "" || a[0] != '-' {
			return wr.after(args), true
		}
		n, ok := wr.option(a[1:])
		if !ok {
			return nil, false
		}
		args = args[min(n, len(args)):]
	}
	return nil, true
}

// option reads one cluster and returns how many words it takes, its value included.
func (wr wrapper) option(cluster string) (int, bool) {
	for i := 0; i < len(cluster); i++ {
		switch c := cluster[i]; {
		case strings.IndexByte(wr.values, c) >= 0 && i == len(cluster)-1:
			return 2, true
		case strings.IndexByte(wr.values, c) >= 0:
			return 1, true
		case strings.IndexByte(wr.flags, c) < 0:
			return 0, false
		}
	}
	return 1, true
}

func (wr wrapper) after(args []word) []word {
	for wr.assigns && len(args) > 0 && strings.Contains(args[0].text, "=") {
		args = args[1:]
	}
	return args[min(wr.skip, len(args)):]
}

func skipPrefix(argv []word) []word {
	for len(argv) > 0 && (keywords[argv[0].text] || isAssignment(argv[0].text)) {
		argv = argv[1:]
	}
	return argv
}

func isAssignment(s string) bool {
	name, _, ok := strings.Cut(s, "=")
	name = strings.TrimSuffix(name, "+")
	if !ok || name == "" || name[0] >= '0' && name[0] <= '9' {
		return false
	}
	return strings.Trim(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_") == ""
}

// writtenBy returns the operands a file-writing verb writes, or nothing for any other verb.
func writtenBy(verb string, args []word) []word {
	switch verb {
	case "rm", "unlink", "rmdir", "touch", "tee", "truncate", "rsync", "chmod":
		return operands(args)
	case "cp", "ln", "install", "mv":
		return copied(verb, args)
	case "sed":
		return sedFiles(args)
	case "dd":
		return ddFiles(args)
	case "find":
		return findWrites(args)
	case "tar":
		return tarWrites(args)
	case "patch":
		return []word{unseen("patch input")}
	}
	if f, ok := flagWriters[verb]; ok {
		if found, _ := flagged(args, f); found {
			return operands(args)
		}
	}
	return nil
}

// flag is a short flag letter, found in any cluster, and its long name.
type flag struct {
	short byte
	long  string
}

// flagWriters write every operand when they carry the flag.
var flagWriters = map[string]flag{
	"gofmt": {'w', ""}, "goimports": {'w', ""}, "prettier": {'w', "write"},
	"perl": {'i', ""}, "awk": {'i', "include"}, "gawk": {'i', "include"},
}

// flagged reports whether args carry f, and returns the values written into the same word
// (`-tweb`, `--target-directory=web`).
func flagged(args []word, f flag) (found bool, values []word) {
	for _, a := range args {
		t := a.text
		if t == "--" {
			break
		}
		if long, ok := strings.CutPrefix(t, "--"); ok {
			name, v, _ := strings.Cut(long, "=")
			if f.long != "" && name == f.long {
				found, values = true, appendValue(values, v)
			}
		} else if j := strings.IndexByte(t, f.short); len(t) > 1 && t[0] == '-' && j > 0 {
			found, values = true, appendValue(values, t[j+1:])
		}
	}
	return found, values
}

func appendValue(values []word, v string) []word {
	if v == "" {
		return values
	}
	return append(values, literal(v))
}

// copied returns what `cp`, `ln`, `install` and `mv` write: the last operand, or the directory
// `-t` names, and for `mv` every source too, since a moved file leaves its place. `ln` with one
// operand links it into the working directory under its own name.
func copied(verb string, args []word) []word {
	ops := operands(args)
	t, dirs := flagged(args, flag{'t', "target-directory"})
	switch {
	case verb == "mv" || t:
		return append(ops, dirs...)
	case len(ops) == 0:
		return nil
	case verb == "ln" && len(ops) == 1:
		return []word{literal(filepath.Base(ops[0].text))}
	}
	return ops[len(ops)-1:]
}

func ddFiles(args []word) []word {
	var out []word
	for _, a := range args {
		if f, ok := strings.CutPrefix(a.text, "of="); ok {
			out = append(out, literal(f))
		}
	}
	return out
}

// findWrites: `-delete` removes what find matches under its starting points; `-exec` and its
// kin run a command the walk cannot see, and `-fprint` and its kin write a file.
func findWrites(args []word) []word {
	is := func(names ...string) bool {
		return slices.ContainsFunc(args, func(a word) bool { return slices.Contains(names, a.text) })
	}
	switch {
	case is("-exec", "-execdir", "-ok", "-okdir", "-fprint", "-fprint0", "-fprintf", "-fls"):
		return []word{unseen("find -exec")}
	case is("-delete"):
		return startingPoints(args)
	}
	return nil
}

// startingPoints are the words before find's expression, after its own options; "." when none.
func startingPoints(args []word) []word {
	for len(args) > 0 && len(args[0].text) == 2 && args[0].text[0] == '-' {
		args = args[1:]
	}
	end := slices.IndexFunc(args, func(a word) bool { return strings.HasPrefix(a.text, "-") || a.text == "!" })
	if end < 0 {
		end = len(args)
	}
	if end == 0 {
		return []word{{text: "."}}
	}
	return args[:end]
}

// tarWrites returns every operand (an archive tar creates is one) and, when it extracts, the
// directory it runs in and one `-C` names in the same word (`-Cdir`, `--directory=dir`).
func tarWrites(args []word) []word {
	out := operands(args)
	extract := false
	for i, a := range args {
		t := a.text
		short := strings.HasPrefix(t, "-") != strings.HasPrefix(t, "--") || i == 0 && !strings.HasPrefix(t, "-")
		extract = extract || t == "--extract" || t == "--get" || short && strings.Contains(t, "x")
		if v, ok := strings.CutPrefix(t, "--directory="); ok {
			out = append(out, literal(v))
		}
		if j := strings.IndexByte(t, 'C'); short && j >= 0 && j+1 < len(t) {
			out = append(out, literal(t[j+1:]))
		}
	}
	if extract {
		out = append(out, word{text: "."})
	}
	return out
}

// git walks past git's global options to the subcommand. `-C <dir>` moves where git runs;
// `--git-dir`, `--work-tree` and a GIT_ variable move it where the walk does not follow, and a
// GIT_ variable (gitVariable) or a `--config-env`, whose value is in the environment, refuses it.
func (w *walker) git(args []word) {
	dirs := w.dirs
	if strings.Contains(w.whole, "GIT_") {
		dirs = []string{""}
	}
	from := len(w.targets)
	for i := 0; i < len(args); i++ {
		switch a := args[i].text; {
		case a == "-C" && i+1 < len(args):
			i++
			dirs = moved(dirs, args[i])
		case a == "-c" && i+1 < len(args):
			i++
			if !w.gitConfig(dirs, args[i].text) {
				return
			}
		case strings.HasPrefix(a, "--config-env"):
			if !strings.Contains(a, "=") && i+1 < len(args) {
				i++
				a += " " + args[i].text
			}
			w.add(dirs, unread(gitConfigPrefix, a))
		case strings.HasPrefix(a, "--git-dir") || strings.HasPrefix(a, "--work-tree"):
			dirs = []string{""}
			if !strings.Contains(a, "=") {
				i++
			}
		case a == "--namespace" || a == "--super-prefix" || a == "--attr-source":
			i++
		case strings.HasPrefix(a, "-"):
		default:
			w.gitCommand(dirs, a, args[i+1:])
			w.gitVariable(from)
			return
		}
	}
}

// gitConfig judges one `-c` key and value, in any case, and reports whether the walk goes on. An
// `alias.` key makes a verb the walk cannot read (audit 5, B13) and stops it. A programKeys key, or
// an `ext::` URL in the key or value (audit 2, B2), refuses; any other value is walked as a command.
func (w *walker) gitConfig(dirs []string, kv string) bool {
	k, v, _ := strings.Cut(kv, "=")
	key := strings.ToLower(k)
	switch {
	case strings.HasPrefix(key, "alias."):
		w.add(dirs, unread(aliasPrefix, k[len("alias."):]))
		return false
	case programKeys.MatchString(key) || strings.Contains(kv, "ext::"):
		w.add(dirs, unread(gitConfigPrefix, "-c "+kv))
	default:
		w.nested([]string{""}, strings.TrimPrefix(v, "!"))
	}
	return true
}

// programKeys are the config keys, in lower case, that name a program git runs or a file it reads.
var programKeys = regexp.MustCompile(`^(include\.path|includeif\..*|core\.(hookspath|sshcommand|editor|askpass|gitproxy)|` +
	`sequence\.editor|protocol\.(.*\.)?allow|diff\.(external|.*\.(command|textconv))|merge\.(tool|.*\.driver)|` +
	`(mergetool|difftool|filter)\..*|credential\..*helper|remote\..*\.(uploadpack|receivepack)|gpg\.(.*\.)?program)$`)

var gitVariableName = regexp.MustCompile(`GIT_[A-Za-z0-9_]*`)

// gitVariable refuses a git command beside a GIT_ variable (GIT_SSH_COMMAND, GIT_CONFIG_COUNT)
// unless a target since from, one the walk could not place, refuses it already.
func (w *walker) gitVariable(from int) {
	name := gitVariableName.FindString(w.whole)
	if name == "" || slices.ContainsFunc(w.targets[from:], func(t target) bool { return !filepath.IsAbs(t.path) }) {
		return
	}
	w.add(w.dirs, unread(gitVariablePrefix, name))
}

// gitCommand judges one git verb. A verb the walk does not know, such as a stored alias (`git
// config alias.x rm`, then `git x`), refuses the command.
func (w *walker) gitCommand(dirs []string, sub string, args []word) {
	if t, ok := gitRuns(sub, args); ok {
		w.add(dirs, t)
		return
	}
	ops := operands(args)
	switch {
	case sub == "mv" || sub == "rm":
		w.add(dirs, pathspecs(sub, ops)...)
	case sub == "worktree" && len(ops) > 0 && ops[0].text == "add":
		w.add(dirs, worktreePath(args)...)
	case gitWritesTree(sub, args):
		for _, d := range dirs {
			w.targets = append(w.targets, target{path: place(d, word{text: "."}), repo: true})
		}
	case !gitReads(sub, args):
		w.add(dirs, unread(gitVerbPrefix, sub))
	default:
		w.add(dirs, outputs(args)...)
	}
}

// pathspecVerbs are the git verbs that take `--pathspec-from-file` (`git mv` does not).
var pathspecVerbs = []string{"rm", "add", "checkout", "restore", "reset", "stash", "commit"}

// transports are the git verbs that reach another repository and take a command for it.
var transports = []string{"fetch", "pull", "push", "ls-remote", "clone", "archive"}

// gitRuns returns the target of an option through which git reads paths from a file (audit 5,
// B12) or runs a command the walk does not read: `git grep -O`, a transport's `--upload-pack`,
// `--receive-pack` or `--exec`, and an `ext::` URL.
func gitRuns(sub string, args []word) (word, bool) {
	if o, ok := option(args, "", "--pathspec-from-file"); ok && slices.Contains(pathspecVerbs, sub) {
		return unread(fromFilePrefix, o), true
	}
	if o, ok := option(args, "O", "--open-files-in-pager"); ok && sub == "grep" {
		return unread(grepPrefix, o), true
	}
	if !slices.Contains(transports, sub) {
		return word{}, false
	}
	if o, ok := option(args, "", "--upload-pack", "--receive-pack", "--exec"); ok {
		return unread(transportPrefix, o), true
	}
	ext := func(a word) bool { return strings.HasPrefix(a.text, "ext::") || strings.Contains(a.text, "=ext::") }
	if i := slices.IndexFunc(args, ext); i >= 0 {
		return unread(transportPrefix, args[i].text), true
	}
	return word{}, false
}

// worktreePath is the directory `git worktree add` checks a whole tree out into.
func worktreePath(args []word) []word {
	if _, ops := worktreeValues.split(args); len(ops) > 1 {
		return ops[1:2]
	}
	return nil
}

// outputs returns the files `--output` writes on a git verb that otherwise only reads (`git log
// --output=web/x.ts`), placed like a redirection. Git 2.49.0 takes the option only whole.
func outputs(args []word) []word {
	var out []word
	opts, _ := valueOptions{longs: []string{"--output"}}.split(args)
	for _, o := range opts {
		if v, ok := strings.CutPrefix(o.text, "--output="); ok {
			out = append(out, literal(v))
		}
	}
	return out
}

// pathspecs marks as unplaceable each `git rm` or `git mv` operand git reads by text, not where
// the kernel resolves: one with a `..`, which git removes against its physical directory (audit 4,
// B9 and B10), and a `git rm` operand starting with `:`, pathspec magic (B11).
func pathspecs(sub string, ops []word) []word {
	out := slices.Clone(ops)
	for i, op := range out {
		if dotDot(op.text) || sub == "rm" && strings.HasPrefix(op.text, ":") {
			out[i] = unread(pathspecPrefix, op.text)
		}
	}
	return out
}

// gitReaders are the git verbs that write no worktree file but through `--output` (outputs): read
// porcelain and plumbing, and `add`, `commit` and `push`, whose hooks the walk cannot see. With
// gitWritesTree's verbs and gitReads' forms, they are the edge of the walk's allowlist.
var gitReaders = map[string]bool{
	"status": true, "log": true, "diff": true, "show": true, "blame": true, "grep": true,
	"ls-files": true, "ls-tree": true, "rev-parse": true, "rev-list": true, "cat-file": true,
	"describe": true, "shortlog": true, "fetch": true, "merge-base": true, "name-rev": true,
	"for-each-ref": true, "help": true, "version": true, "merge-tree": true, "show-ref": true,
	"diff-tree": true, "check-ignore": true, "show-branch": true, "range-diff": true,
	"whatchanged": true, "var": true, "count-objects": true, "ls-remote": true,
	"add": true, "commit": true, "push": true,
}

// gitReadForms are the first operands with which a git verb reads only; with no operand it lists.
var gitReadForms = map[string][]string{
	"remote":    {"show", "get-url"},
	"reflog":    {"show", "list", "exists"},
	"submodule": {"status"},
}

// gitReads reports a git verb, past gitWritesTree, that writes no worktree file and changes no
// branch or tag name, remote, reflog, submodule, symbolic ref or config (where a verb can hide).
func gitReads(sub string, args []word) bool {
	ops := operands(args)
	switch sub {
	case "reset", "checkout", "stash", "worktree":
		return true
	case "branch":
		return !hasOption(args, "dDmM", "--delete", "--move")
	case "tag":
		return !hasOption(args, "d", "--delete")
	case "config":
		return configReads(args)
	case "symbolic-ref":
		return len(ops) == 1 && !hasOption(args, "d", "--delete")
	case "remote", "reflog", "submodule":
		return len(ops) == 0 || slices.Contains(gitReadForms[sub], ops[0].text)
	}
	return gitReaders[sub]
}

// configReads reports a `git config` that only reads: `get` or `list`, a get or list option, or
// one key and no writing option. Its options end at the first operand (`NAME VALUE -l` stores
// VALUE; row 14 audit 1, F1), and a value is no option (`--comment -l`, `-f.git/x`; audit 2, B1).
func configReads(args []word) bool {
	opts, ops := configValues.split(args)
	switch {
	case len(ops) > 0 && (ops[0].text == "get" || ops[0].text == "list"):
		return true
	case hasOption(opts, "l", "--get-all", "--get-regexp", "--get-urlmatch", "--get-colorbool", "--list"):
		return true
	}
	return len(ops) == 1 && !hasOption(opts, "e", "--unset-all", "--remove-section", "--rename-section", "--replace-all", "--add", "--edit")
}

// valueOptions are a git verb's options that take a value: short letters, each ending its cluster
// and taking the rest of it or the next word, and long names, whole or abbreviated as git allows,
// taking the next word without `=`. stop ends the options at the first operand.
type valueOptions struct {
	shorts string
	longs  []string
	stop   bool
}

var (
	configValues   = valueOptions{shorts: "ft", longs: []string{"--file", "--blob", "--type", "--default", "--comment"}, stop: true}
	worktreeValues = valueOptions{shorts: "bB", longs: []string{"--reason"}}
	checkoutValues = valueOptions{shorts: "bB", longs: []string{"--orphan", "--conflict"}}
)

// split returns a verb's options and operands as git reads them. A value is in neither: a short
// option keeps its cluster up to its letter (`-f`), a long one joins it (`--file=v`).
func (v valueOptions) split(args []word) (opts, ops []word) {
	for i := 0; i < len(args); i++ {
		a := args[i]
		name, _, joined := strings.Cut(a.text, "=")
		switch {
		case a.text == "--":
			return opts, append(ops, args[i+1:]...)
		case len(a.text) < 2 || a.text[0] != '-':
			if ops = append(ops, a); v.stop {
				return opts, append(ops, args[i+1:]...)
			}
			continue
		case strings.HasPrefix(name, "--"):
			if !joined && i+1 < len(args) && slices.ContainsFunc(v.longs, func(l string) bool { return len(name) > 2 && strings.HasPrefix(l, name) }) {
				i++
				a.text += "=" + args[i].text
			}
		case strings.ContainsAny(a.text[1:], v.shorts):
			if a.text = a.text[:strings.IndexAny(a.text[1:], v.shorts)+2]; a.text == args[i].text {
				i++
			}
		}
		opts = append(opts, a)
	}
	return opts, ops
}

func hasOption(args []word, shorts string, longs ...string) bool {
	_, ok := option(args, shorts, longs...)
	return ok
}

// option returns the first argument before any `--` that is a short option of shorts, alone or
// in a cluster, or one of longs, whole or abbreviated as git allows (`--del` for `--delete`).
func option(args []word, shorts string, longs ...string) (string, bool) {
	for _, a := range args {
		name, _, _ := strings.Cut(a.text, "=")
		switch {
		case a.text == "--":
			return "", false
		case len(name) > 2 && strings.HasPrefix(name, "--"):
			if slices.ContainsFunc(longs, func(l string) bool { return strings.HasPrefix(l, name) }) {
				return a.text, true
			}
		case len(name) > 1 && name[0] == '-' && strings.ContainsAny(name[1:], shorts):
			return a.text, true
		}
	}
	return "", false
}

// treeWriters are the git subcommands that rewrite files across the worktree whatever their
// arguments.
var treeWriters = map[string]bool{
	"clean": true, "switch": true, "restore": true, "merge": true, "pull": true, "rebase": true,
	"apply": true, "am": true, "cherry-pick": true, "revert": true, "bisect": true,
	"checkout-index": true, "read-tree": true, "sparse-checkout": true, "mergetool": true,
}

// gitWritesTree reports a subcommand that rewrites files across the worktree. A bare `git stash`
// is `git stash push`. `reset` options match abbreviated (`--har`, and `--m`, which git refuses).
func gitWritesTree(sub string, args []word) bool {
	first := append(operands(args), word{})[0].text // the first operand, or ""
	switch sub {
	case "reset":
		return hasOption(args, "", "--hard", "--merge", "--keep")
	case "checkout":
		return checkoutWrites(args)
	case "stash":
		return first != "list" && first != "show"
	case "worktree":
		return first == "remove" || first == "move"
	}
	return treeWriters[sub]
}

// checkoutWrites: every `git checkout` rewrites the worktree (`<ref>`, `-`, `-- <path>`) except
// `git checkout -b <new>` with no start point and no `-f` (`-qf`) or `--force` (`--forc`).
func checkoutWrites(args []word) bool {
	opts, ops := checkoutValues.split(args)
	return len(ops) > 0 || hasOption(opts, "f", "--force") || !hasOption(opts, "bB", "--orphan")
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

// sedFiles returns the files `sed` edits in place, or nothing without `-i` or `-I`. The first
// operand is the script unless `-e` or `-f` gave one. A bare `-i` or `-I` followed by an empty
// word is the macOS form, and the empty word is its backup suffix.
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
			next = next || (a == "-i" || a == "-I") && i+1 < len(args) && args[i+1].text == ""
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

// sedCluster reads one short-flag cluster as sed does: `i` or `I` takes the rest as its suffix,
// and `e` or `f` take the rest, or the next word when they end the cluster.
func sedCluster(letters string) (inPlace, script, next bool) {
	for j := 0; j < len(letters); j++ {
		switch letters[j] {
		case 'i', 'I':
			return true, script, false
		case 'e', 'f':
			return inPlace, true, j == len(letters)-1
		}
	}
	return inPlace, script, false
}

// Operators, longest first so `&&` is never read as two `&`.
var operators = []string{
	"&>>", ">>&", ">>|", ">>!", "&>|", "&>!", ">&|", ">&!", "<<<", "<<-",
	"&&", "||", "|&", ">>", ">|", ">!", ">&", "&>", "<<", "<&", "<>", ";", "&", "|", "<", ">",
}

// splitter turns a command into words and operators. Quotes join a word and keep what they hold.
// A newline ends a command, and a `#` at the start of a word starts a comment. A heredoc body is
// not shell text to the command around it: its lines, up to the terminator, are cut out first, so
// no quote in them pairs with one outside, and then each line is read as commands, never skipped.
// A data body that looks like a write refuses, and a body fed to a shell (`cat <<EOF | sh`) is
// walked.
type splitter struct {
	s       string
	i       int
	out     []word
	cur     strings.Builder
	in      bool // a word is open, even an empty quoted one
	quoted  bool
	expands bool
	body    bool     // one heredoc body line, where `<<` opens nothing
	pending *heredoc // a `<<` waiting for its delimiter word
	open    []heredoc
	failed  string // openQuote or openHeredoc
}

type heredoc struct {
	delim string
	tabs  bool
}

// split returns the words of s, or the target that refuses it.
func split(s string) ([]word, string) {
	p := splitter{s: s}
	return p.run()
}

func (p *splitter) run() ([]word, string) {
	for p.i < len(p.s) {
		if !p.step() {
			break
		}
	}
	p.flush()
	if p.failed == "" && (len(p.open) > 0 || p.pending != nil) {
		p.failed = openHeredoc
	}
	if p.failed != "" {
		return nil, p.failed
	}
	return p.out, ""
}

func (p *splitter) step() bool {
	switch c := p.s[p.i]; {
	case c == ' ' || c == '\t':
		p.flush()
		p.i++
	case c == '\n':
		p.flush()
		if p.pending != nil {
			p.failed = openHeredoc
			return false
		}
		p.out = append(p.out, word{op: ";"})
		p.i++
		return p.bodies()
	case c == '#' && !p.in:
		p.i += strings.IndexByte(p.s[p.i:]+"\n", '\n')
	case c == '\'' || c == '"':
		return p.quote(c)
	case strings.IndexByte(";&|<>", c) >= 0:
		p.operator()
		return p.failed == ""
	default:
		p.expands = p.expands || c == '~' || c == '#' || c == '=' && !p.in
		p.cur.WriteByte(c)
		p.in = true
		p.i++
	}
	return true
}

// quote reads a quoted run into the open word. A quote that never closes fails the split.
func (p *splitter) quote(q byte) bool {
	end := strings.IndexByte(p.s[p.i+1:], q)
	if end < 0 {
		p.failed = openQuote
		return false
	}
	p.cur.WriteString(p.s[p.i+1 : p.i+1+end])
	p.in, p.quoted = true, true
	p.i += end + 2
	return true
}

// operator reads one operator. Unquoted digits right before a redirection are its file
// descriptor (`2>err`), not a word.
func (p *splitter) operator() {
	op := ""
	for _, o := range operators {
		if strings.HasPrefix(p.s[p.i:], o) {
			op = o
			break
		}
	}
	if text := p.cur.String(); (op[0] == '<' || op[0] == '>') && p.in && !p.quoted && strings.Trim(text, "0123456789") == "" {
		p.reset()
	} else {
		p.flush()
	}
	if p.pending != nil {
		p.failed = openHeredoc
	}
	p.out = append(p.out, word{op: op})
	p.i += len(op)
	if (op == "<<" || op == "<<-") && !p.body {
		p.pending = &heredoc{tabs: op == "<<-"}
	}
}

// flush ends the open word. The first word after `<<`, spaces and tabs allowed between them, is
// its delimiter. A `<<` followed by an operator, a newline or the end of the input has none, and
// refuses the command like a heredoc with no end.
func (p *splitter) flush() {
	if p.in {
		p.out = append(p.out, word{text: p.cur.String(), expands: p.expands})
		if p.pending != nil {
			p.open = append(p.open, heredoc{delim: p.cur.String(), tabs: p.pending.tabs})
		}
		p.pending = nil
	}
	p.reset()
}

// bodies cuts out the bodies of the heredocs opened on the line just ended, in order, each up to
// its terminator: the delimiter alone on a line, after leading tabs for `<<-`.
func (p *splitter) bodies() bool {
	for _, h := range p.open {
		if !p.cutBody(h) {
			p.failed = openHeredoc
			return false
		}
	}
	p.open = nil
	return true
}

func (p *splitter) cutBody(h heredoc) bool {
	for p.i < len(p.s) {
		line, _, _ := strings.Cut(p.s[p.i:], "\n")
		p.i = min(p.i+len(line)+1, len(p.s))
		if h.tabs {
			line = strings.TrimLeft(line, "\t")
		}
		if line == h.delim {
			return true
		}
		p.out = append(append(p.out, bodyLine(line)...), word{op: ";"})
	}
	return false
}

// bodyLine reads one body line as commands. A quote that does not close on the line is taken out,
// and the line read again.
func bodyLine(line string) []word {
	q := splitter{s: line, body: true}
	if words, failed := q.run(); failed == "" {
		return words
	}
	q = splitter{s: strings.NewReplacer("'", "", `"`, "").Replace(line), body: true}
	words, _ := q.run()
	return words
}

func (p *splitter) reset() {
	p.cur.Reset()
	p.in, p.quoted, p.expands = false, false, false
}
