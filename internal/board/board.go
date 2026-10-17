// Package board reads one row's "Files it owns" cell from a board and turns it
// into the paths a lease can hold. It reads no file and runs no git: the caller
// passes the board text and the tracked-file list.
//
// The parse is lossy by design, and loud about it. Whatever it cannot read as a
// path comes back as unread, verbatim, so the caller can print it: a lossy parse
// that stays quiet is a lease that does not exist (DESIGN.md D10, option G).
package board

import (
	"fmt"
	"path"
	"strings"
	"unicode"
)

// Cell returns the last cell, trimmed, of the first markdown table line whose
// first cell, trimmed, equals row.
func Cell(text, row string) (cell string, err error) {
	for _, line := range strings.Split(text, "\n") {
		cells, ok := tableCells(line)
		if !ok || strings.TrimSpace(cells[0]) != row {
			continue
		}
		if len(cells) < 2 {
			return "", fmt.Errorf("board row %q has %d cell, want at least 2", row, len(cells))
		}
		return strings.TrimSpace(cells[len(cells)-1]), nil
	}
	return "", fmt.Errorf("board has no table row %q", row)
}

// tableCells splits a markdown table line on every pipe. Neither live board
// escapes a pipe inside a cell (checked 2026-10-16), so `\|` is not handled.
func tableCells(line string) ([]string, bool) {
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "|") {
		return nil, false
	}
	line = strings.TrimSuffix(strings.TrimPrefix(line, "|"), "|")
	return strings.Split(line, "|"), true
}

// Parse reads every backtick-quoted token of cell. paths holds, in cell order
// and without repeats: exact paths, directory prefixes ending in one "/", and
// globs for Expand. unread holds every token that is absolute, starts with "~",
// holds "..", or holds whitespace, and every run of text outside backticks
// that is more than punctuation and whitespace, each as written.
func Parse(cell string) (paths, unread []string) {
	p := parsed{seen: map[string]bool{}}
	parts := strings.Split(cell, "`")
	for i, part := range parts {
		switch {
		case i%2 == 0:
			p.text(part)
		case i == len(parts)-1:
			p.text("`" + part) // an unclosed backtick: show it as written
		default:
			p.token(part)
		}
	}
	return p.paths, p.unread
}

// parsed collects what Parse has read so far.
type parsed struct {
	paths, unread []string
	seen          map[string]bool
}

func (p *parsed) text(s string) {
	if hasWords(s) {
		p.unread = append(p.unread, strings.TrimSpace(s))
	}
}

func (p *parsed) token(tok string) {
	read, ok := classify(tok)
	switch {
	case tok == "":
	case !ok:
		p.unread = append(p.unread, tok)
	case !p.seen[read]:
		p.seen[read] = true
		p.paths = append(p.paths, read)
	}
}

// hasWords reports whether s holds a rune that is neither punctuation nor
// whitespace. A symbol such as an arrow counts as a word: it may mean a range.
func hasWords(s string) bool {
	for _, r := range s {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			return true
		}
	}
	return false
}

// classify turns one token into a path, a directory prefix or a glob, or
// reports that it cannot be read.
func classify(tok string) (string, bool) {
	if strings.HasPrefix(tok, "/") || strings.HasPrefix(tok, "~") ||
		strings.Contains(tok, "..") || strings.IndexFunc(tok, unicode.IsSpace) >= 0 {
		return tok, false
	}
	if base, ok := trimDir(tok); ok {
		return dirPrefix(base)
	}
	return cleaned(tok)
}

// trimDir strips every trailing "/", "/**" and "/**/*" from tok and reports
// whether it stripped one, so "dir/", "dir/**", "dir/**/" and "dir/**/*" all
// name the directory "dir".
func trimDir(tok string) (string, bool) {
	for _, suffix := range []string{"/**/*", "/**", "/"} {
		if strings.HasSuffix(tok, suffix) {
			base, _ := trimDir(strings.TrimSuffix(tok, suffix))
			return base, true
		}
	}
	return tok, false
}

// dirPrefix returns base as a directory prefix with one trailing "/". A base
// that is itself a glob cannot be a literal prefix, so it becomes the glob of
// every file below it, for Expand.
func dirPrefix(base string) (string, bool) {
	p, ok := cleaned(base)
	if isGlob(p) {
		return p + "/**/*", ok
	}
	return p + "/", ok
}

// cleaned returns tok in clean form; path.Clean keeps "*", "?" and "[", so a
// glob is cleaned like a path. A token that cleans to the repository root
// would lease everything or nothing, so it is not read.
func cleaned(tok string) (string, bool) {
	p := path.Clean(tok)
	return p, p != "."
}

func isGlob(s string) bool {
	return strings.ContainsAny(s, "*?[")
}

// Expand returns the files, in files order, that any pattern matches, and the
// patterns that match none. A pattern matches a file when path.Match holds
// segment by segment, with a "**" segment matching any number of segments. A
// pattern ending in "/" matches every file below that directory, so every form
// Parse returns can be checked against the tracked files.
func Expand(patterns, files []string) (matched, unmatched []string) {
	hit := make([]bool, len(files))
	for _, p := range patterns {
		if !mark(p, files, hit) {
			unmatched = append(unmatched, p)
		}
	}
	for i, f := range files {
		if hit[i] {
			matched = append(matched, f)
		}
	}
	return matched, unmatched
}

// mark sets hit for every file pattern matches and reports whether it matched one.
func mark(pattern string, files []string, hit []bool) bool {
	if strings.HasSuffix(pattern, "/") {
		pattern += "**/*"
	}
	segs := strings.Split(path.Clean(pattern), "/")
	found := false
	for i, f := range files {
		if matchSegments(segs, strings.Split(f, "/")) {
			hit[i] = true
			found = true
		}
	}
	return found
}

// matchSegments matches pattern segments against path segments. A malformed
// segment matches nothing, so its pattern comes back unmatched.
func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			return matchAnyDepth(pat[1:], name)
		}
		if len(name) == 0 {
			return false
		}
		if ok, err := path.Match(pat[0], name[0]); err != nil || !ok {
			return false
		}
		pat, name = pat[1:], name[1:]
	}
	return len(name) == 0
}

// matchAnyDepth matches rest against name after skipping any number of
// leading segments, which is what a "**" segment means.
func matchAnyDepth(rest, name []string) bool {
	for i := 0; i <= len(name); i++ {
		if matchSegments(rest, name[i:]) {
			return true
		}
	}
	return false
}
