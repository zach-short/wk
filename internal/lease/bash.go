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
// rm`; the git subcommands that rewrite the worktree (a write of its root, see gitWritesTree).
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
	openQuote        = "<open quote>"
	openHeredoc      = "<open heredoc>"
	unreadablePrefix = "<character "
)

// refusal returns the denial for a target that names no file.
func refusal(path string) (string, bool) {
	switch path {
	case openQuote:
		return "wk: cannot tell which file this writes: the command has an open quote, and bash runs the lines before it; close the quote", true
	case openHeredoc:
		return "wk: cannot tell which file this writes: a heredoc in the command has no end line; end it with its delimiter alone on a line", true
	}
	if c, ok := strings.CutPrefix(path, unreadablePrefix); ok {
		return fmt.Sprintf("wk: cannot tell which file this writes: the command holds `%s`, which the lease walk does not read; write it without that character or ask for the path in full", strings.TrimSuffix(c, ">")), true
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
// `--git-dir`, `--work-tree` and a GIT_ variable move it somewhere the walk does not follow. A
// `-c` value may be a command git runs (`alias.x=!rm f`, `core.pager`), from the repository root.
func (w *walker) git(args []word) {
	dirs := w.dirs
	if strings.Contains(w.whole, "GIT_") {
		dirs = []string{""}
	}
	for i := 0; i < len(args); i++ {
		switch a := args[i].text; {
		case a == "-C" && i+1 < len(args):
			i++
			dirs = moved(dirs, args[i])
		case a == "-c" && i+1 < len(args):
			i++
			_, v, _ := strings.Cut(args[i].text, "=")
			w.nested([]string{""}, strings.TrimPrefix(v, "!"))
		case strings.HasPrefix(a, "--git-dir") || strings.HasPrefix(a, "--work-tree"):
			dirs = []string{""}
			if !strings.Contains(a, "=") {
				i++
			}
		case a == "--namespace" || a == "--config-env" || a == "--super-prefix" || a == "--attr-source":
			i++
		case strings.HasPrefix(a, "-"):
		default:
			w.gitCommand(dirs, a, args[i+1:])
			return
		}
	}
}

func (w *walker) gitCommand(dirs []string, sub string, args []word) {
	switch {
	case sub == "mv" || sub == "rm":
		w.add(dirs, operands(args)...)
	case gitWritesTree(sub, args):
		for _, d := range dirs {
			w.targets = append(w.targets, target{path: place(d, word{text: "."}), repo: true})
		}
	}
}

// treeWriters are the git subcommands that rewrite files across the worktree whatever their
// arguments.
var treeWriters = map[string]bool{
	"clean": true, "switch": true, "restore": true, "merge": true, "pull": true, "rebase": true,
	"apply": true, "am": true, "cherry-pick": true, "revert": true, "bisect": true,
	"checkout-index": true, "read-tree": true, "sparse-checkout": true, "mergetool": true,
}

// gitWritesTree reports a subcommand that rewrites files across the worktree. A bare `git stash`
// is `git stash push`, which puts every changed file back.
func gitWritesTree(sub string, args []word) bool {
	ops := operands(args)
	first := ""
	if len(ops) > 0 {
		first = ops[0].text
	}
	switch sub {
	case "reset":
		return slices.ContainsFunc(args, func(a word) bool { return a.text == "--hard" || a.text == "--merge" || a.text == "--keep" })
	case "checkout":
		return checkoutWrites(args)
	case "stash":
		return first != "list" && first != "show"
	case "worktree":
		return first == "remove" || first == "move"
	}
	return treeWriters[sub]
}

// checkoutWrites: every `git checkout` rewrites the worktree (`<ref>`, `-`, `-- <path>`, `-f`)
// except `git checkout -b <new>` with no start point and no `-f`.
func checkoutWrites(args []word) bool {
	branch, other := false, false
	for i := 0; i < len(args); i++ {
		switch a := args[i].text; {
		case a == "-b" || a == "-B" || a == "--orphan":
			branch = true
			i++
		case a == "-f" || a == "--force" || a == "-" || !strings.HasPrefix(a, "-"):
			other = true
		}
	}
	return other || !branch
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
