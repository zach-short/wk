package gate

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Every sample below is real output, captured on 2026-10-16 in a throwaway
// probe project (a Go module with a type error, a printf misuse and a badly
// formatted file; a TypeScript project with a type error and an import cycle).
// Only the probe's absolute prefix is replaced by ROOT, and tsc's colours are
// kept as the escapes it printed. Tool versions: go1.24.1, golangci-lint
// v2.12.2, tsc 5.8.3 (/opt/homebrew/bin), eslint 9.39.5 and madge 8.0.0 (the
// versions in ezhomesteading's root node_modules, whose lint scripts use
// eslint's default stylish formatter; eslint 9 no longer ships the compact one).
var pathSamples = []struct {
	name, output string
	want         []string
}{
	{
		name:   "go build",
		output: "# probe\n./main.go:6:14: cannot use \"s\" (untyped string constant) as int value in variable declaration\n",
		want:   []string{"main.go"},
	},
	{
		name:   "go vet",
		output: "# probe/sub\nsub/sub.go:6:3: fmt.Printf format %d has arg \"s\" of wrong type string\n",
		want:   []string{"sub/sub.go"},
	},
	{
		name:   "gofmt -l",
		output: "g.go\nsub/sub.go\n",
		want:   []string{"g.go", "sub/sub.go"},
	},
	{
		name: "golangci-lint",
		output: "sub/sub.go:6:15: printf: fmt.Printf format %d has arg \"s\" of wrong type string (govet)\n" +
			"  fmt.Printf(\"%d\\n\", \"s\")\n" +
			"              ^\n" +
			"1 issues:\n" +
			"* govet: 1\n",
		want: []string{"sub/sub.go"},
	},
	{
		name: "tsc without pretty",
		output: "src/a.ts(1,7): error TS2322: Type 'string' is not assignable to type 'number'.\n" +
			"src/b.ts(2,14): error TS7022: 'b' implicitly has type 'any' because it does not have a type annotation and is referenced directly or indirectly in its own initializer.\n" +
			"src/c.ts(2,14): error TS7022: 'a' implicitly has type 'any' because it does not have a type annotation and is referenced directly or indirectly in its own initializer.\n",
		want: []string{"src/a.ts", "src/b.ts", "src/c.ts"},
	},
	{
		name: "tsc pretty",
		output: "\x1b[96msrc/a.ts\x1b[0m:\x1b[93m1\x1b[0m:\x1b[93m7\x1b[0m - \x1b[91merror\x1b[0m\x1b[90m TS2322: \x1b[0mType 'string' is not assignable to type 'number'.\n" +
			"\n" +
			"\x1b[7m1\x1b[0m const n: number = \"s\";\n" +
			"\x1b[7m \x1b[0m \x1b[91m      ~\x1b[0m\n" +
			"\n" +
			"\x1b[96msrc/b.ts\x1b[0m:\x1b[93m2\x1b[0m:\x1b[93m14\x1b[0m - \x1b[91merror\x1b[0m\x1b[90m TS7022: \x1b[0m'b' implicitly has type 'any' because it does not have a type annotation and is referenced directly or indirectly in its own initializer.\n" +
			"\n" +
			"\x1b[7m2\x1b[0m export const b = a;\n" +
			"\x1b[7m \x1b[0m \x1b[91m             ~\x1b[0m\n" +
			"\n" +
			"\x1b[96msrc/c.ts\x1b[0m:\x1b[93m2\x1b[0m:\x1b[93m14\x1b[0m - \x1b[91merror\x1b[0m\x1b[90m TS7022: \x1b[0m'a' implicitly has type 'any' because it does not have a type annotation and is referenced directly or indirectly in its own initializer.\n" +
			"\n" +
			"\x1b[7m2\x1b[0m export const a = b;\n" +
			"\x1b[7m \x1b[0m \x1b[91m             ~\x1b[0m\n" +
			"\n" +
			"\n" +
			"Found 3 errors in 3 files.\n" +
			"\n" +
			"Errors  Files\n" +
			"     1  src/a.ts\x1b[90m:1\x1b[0m\n" +
			"     1  src/b.ts\x1b[90m:2\x1b[0m\n" +
			"     1  src/c.ts\x1b[90m:2\x1b[0m\n",
		want: []string{"src/a.ts", "src/b.ts", "src/c.ts"},
	},
	{
		name: "eslint stylish",
		output: "\n" +
			"ROOT/src/a.ts\n" +
			"  1:8  error  Parsing error: Unexpected token :\n" +
			"\n" +
			"ROOT/src/b.ts\n" +
			"  3:7  error  'unused' is assigned a value but never used  no-unused-vars\n" +
			"\n" +
			"ROOT/src/d.js\n" +
			"  1:5  error  'x' is never reassigned. Use 'const' instead  prefer-const\n" +
			"  3:7  error  'y' is assigned a value but never used        no-unused-vars\n" +
			"\n" +
			"\u2716 4 problems (4 errors, 0 warnings)\n" +
			"  1 error and 0 warnings potentially fixable with the `--fix` option.\n",
		want: []string{"src/a.ts", "src/b.ts", "src/d.js"},
	},
	{
		name: "madge from the root",
		output: "- Finding files\n" +
			"Processed 3 files (258ms) \n" +
			"\n" +
			"\u2716 Found 1 circular dependency!\n" +
			"\n" +
			"1) src/b.ts > src/c.ts\n",
		want: []string{"src/b.ts", "src/c.ts"},
	},
	{
		// madge prints paths relative to the directory it was given, here src/,
		// so from the root they name nothing and are dropped.
		name:   "madge from a subdirectory",
		output: "1) b.ts > c.ts\n",
		want:   nil,
	},
	{
		name: "outside root, missing, directory, duplicate",
		output: "/etc/hosts:1:1: outside\n" +
			"src/gone.ts(1,1): error\n" +
			"src\n" +
			"ROOT/../escape.go:1:1: up\n" +
			"src/a.ts(1,7): error\n" +
			"ROOT/src/a.ts\n",
		want: []string{"src/a.ts"},
	},
}

// probeFiles are the files the samples name, made under each test root.
var probeFiles = []string{"main.go", "g.go", "sub/sub.go", "src/a.ts", "src/b.ts", "src/c.ts", "src/d.js"}

func TestPaths(t *testing.T) {
	for _, s := range pathSamples {
		t.Run(s.name, func(t *testing.T) {
			root := t.TempDir()
			for _, f := range probeFiles {
				writeFile(t, filepath.Join(root, f), "x\n")
			}
			got := Paths(root, []byte(strings.ReplaceAll(s.output, "ROOT", root)))
			if !reflect.DeepEqual(got, s.want) {
				t.Errorf("Paths = %q, want %q", got, s.want)
			}
		})
	}
}

// eslint prints the resolved path, so a root given through a symlink (macOS's
// /var is /private/var) still matches.
func TestPathsThroughSymlinkedRoot(t *testing.T) {
	real := t.TempDir()
	writeFile(t, filepath.Join(real, "src/d.js"), "x\n")
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	resolved, err := filepath.EvalSymlinks(real)
	if err != nil {
		t.Fatal(err)
	}
	got := Paths(link, []byte(resolved+"/src/d.js\n  1:5  error  x  prefer-const\n"))
	if want := []string{"src/d.js"}; !reflect.DeepEqual(got, want) {
		t.Errorf("Paths = %q, want %q", got, want)
	}
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}
