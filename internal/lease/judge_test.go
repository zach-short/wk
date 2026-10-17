package lease

import "testing"

const (
	mainRoot = "/r/main"
	rootA    = "/r/main/.claude/worktrees/a" // nested under the main checkout, as wk's worktrees are
	rootB    = "/r/b"
	rootC    = "/r/c"
)

var roots = []string{mainRoot, rootA, rootB, rootC}

func TestResolve(t *testing.T) {
	cases := []struct {
		name, cwd, path, root, rel string
		ok                         bool
	}{
		{"absolute", "", "/r/b/web/x.ts", rootB, "web/x.ts", true},
		{"relative to cwd", "/r/b/web", "x.ts", rootB, "web/x.ts", true},
		{"dot-dot cleaned", "/r/b", "api/../web/x.ts", rootB, "web/x.ts", true},
		{"nested root wins", "", rootA + "/web/x.ts", rootA, "web/x.ts", true},
		{"main checkout", "", "/r/main/web/x.ts", mainRoot, "web/x.ts", true},
		{"the root itself", "/r/b", ".", rootB, ".", true},
		{"outside every root", "", "/tmp/x", "", "", false},
		{"shared name prefix is outside", "", "/r/bb/x", "", "", false},
		{"dot-dot out of a root", "/r/b", "../../tmp/x", "", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root, rel, ok := Resolve(roots, c.cwd, c.path)
			if root != c.root || rel != c.rel || ok != c.ok {
				t.Errorf("Resolve(%q, %q) = %q, %q, %v", c.cwd, c.path, root, rel, ok)
			}
		})
	}
}

func TestJudge(t *testing.T) {
	var s Store
	mustClaim(t, &s, rootA, "7", "web/x.ts", "docs/")
	mustClaim(t, &s, rootB, "8", "api/")
	mustClaim(t, &s, rootC, "9", "PASSOFF.md")
	rules := Rules{SingleWriter: []string{"backend/main.go", "PASSOFF.md"}, BashGuard: true}

	const (
		unclaimed  = "wk: this workspace has not claimed a row; run `wk claim <row>` or `wk claim --none` in /r/main"
		heldByA    = "wk: web/x.ts is leased by row 7 in /r/main/.claude/worktrees/a; leave it to that workspace or wait until it releases"
		singleMain = "wk: backend/main.go is single-writer (leases.single_writer) and this workspace does not lease it; run `wk lease add backend/main.go` in /r/main/.claude/worktrees/a"
		heldAtB    = heldByA + " (the command writes /r/b/web/x.ts)"
		rootOfB    = "wk: . is leased by row 9 in /r/c; leave it to that workspace or wait until it releases (the command writes /r/b)"
	)
	cases := []struct {
		name string
		req  Request
		want Decision
	}{
		{"outside every root", edit("", "/tmp/x"), allow},
		{"unclaimed workspace", edit("", "/r/main/README.md"), Decision{Reason: unclaimed}},
		{"leased by another root", edit("", "/r/b/web/x.ts"), Decision{Reason: heldByA}},
		{"leased by another root, relative path", edit("/r/b/web", "x.ts"), Decision{Reason: heldByA}},
		{"leased by another root, through ..", edit("", "/r/b/api/../web/x.ts"), Decision{Reason: heldByA}},
		{"inside another root's directory lease", edit("", "/r/b/docs/a.md"), Decision{Reason: "wk: docs/a.md is leased by row 7 in /r/main/.claude/worktrees/a; leave it to that workspace or wait until it releases"}},
		{"own lease", edit("", rootA+"/web/x.ts"), allow},
		{"unleased path nobody holds", edit("", rootA+"/README.md"), allow},
		{"single-writer path not leased", edit("", rootA+"/backend/main.go"), Decision{Reason: singleMain}},
		{"single-writer path leased", edit("", rootC+"/PASSOFF.md"), allow},
		{"relative path with no cwd", edit("", "web/x.ts"), Decision{Reason: `wk: cannot tell which file this writes ("web/x.ts"); give the path in full`}},
		{"no path", edit("/r/b", ""), Decision{Reason: `wk: cannot tell which file this writes (""); give the path in full`}},
		{"Bash write to another's lease", bash("/r/b", `cd web && sed -i 's/a/b/' x.ts`), Decision{Reason: heldByA + " (the command writes /r/b/web/x.ts)"}},
		{"Bash rm -r over another's lease", bash("/r/b", `rm -r web`), Decision{Reason: "wk: web is leased by row 7 in /r/main/.claude/worktrees/a; leave it to that workspace or wait until it releases (the command writes /r/b/web)"}},
		{"Bash rm -r over a single-writer file", bash(rootA, `rm -r backend`), Decision{Reason: singleMain + " (the command writes /r/main/.claude/worktrees/a/backend)"}},
		{"Bash write in an unclaimed workspace", bash("/r/main", `echo x > notes.txt`), Decision{Reason: unclaimed + " (the command writes /r/main/notes.txt)"}},
		{"Bash write with a variable", bash("/r/b", `rm -rf $X/web`), Decision{Reason: holds("$")}},
		{"Bash write the walk cannot place", bash("/r/b", `rm -rf ~/web`), Decision{Reason: `wk: cannot tell which file this writes ("~/web"); give the path in full (the command writes ~/web)`}},
		{"Bash with no write", bash("/r/main", `git status && ls -la`), allow},
		{"Bash write outside every root", bash("/r/b", `go test ./... > /tmp/out 2>&1`), allow},
		{"Bash write to an own lease", bash(rootB, `touch api/x.go`), allow},
		{"Bash write after an arithmetic shift", bash("/r/b", "echo $((1<<2))\nrm web/x.ts"), Decision{Reason: holds("$")}},
		{"Bash write after pushd", bash("/r/b", `pushd web && rm x.ts`), Decision{Reason: heldByA + " (the command writes /r/b/web/x.ts)"}},
		{"Bash write after popd", bash("/r/b", `popd; rm x.ts`), Decision{Reason: `wk: cannot tell which file this writes ("x.ts"); give the path in full (the command writes x.ts)`}},
		{"Bash open quote after a write", bash("/r/b", "rm web/x.ts\necho \""), Decision{Reason: "wk: cannot tell which file this writes: the command has an open quote, and bash runs the lines before it; close the quote"}},
		{"Bash write after a $[ ] shift", bash("/r/b", "echo $[1<<2]\nrm web/x.ts"), Decision{Reason: holds("$")}},
		{"Bash write with a trailing backslash", bash("/r/b", "rm web/x.ts \\"), Decision{Reason: holds(`\`)}},
		{"Bash brace expansion over a lease", bash("/r/b", `rm {web,docs}/x.ts`), Decision{Reason: holds("{")}},
		{"Bash process substitution then a lease", bash("/r/b", `tee >(cat) web/x.ts`), Decision{Reason: holds("(")}},
		{"Bash glob then dot-dot", bash("/r/b", `rm docs/*/../../web/x.ts`), Decision{Reason: holds("*")}},
		{"Bash non-ASCII", bash("/r/b", `rm café`), Decision{Reason: holds("é")}},
		{"Bash cd after && that may not run", bash("/r/b", `test -f docs/Makefile && cd docs; rm web/x.ts`), Decision{Reason: heldAtB}},
		{"Bash cd to a missing directory", bash("/r/b", `cd nope; rm web/x.ts`), Decision{Reason: heldAtB}},
		{"Bash heredoc piped to sh", bash("/r/b", "cat <<EOF |\nrm web/x.ts\nEOF\nsh"), Decision{Reason: heldAtB}},
		{"Bash eval", bash("/r/b", `eval 'rm web/x.ts'`), Decision{Reason: heldAtB}},
		{"Bash time -p", bash("/r/b", `time -p rm web/x.ts`), Decision{Reason: heldAtB}},
		{"Bash env with an assignment", bash("/r/b", `env FOO=1 rm web/x.ts`), Decision{Reason: heldAtB}},
		{"Bash unlink", bash("/r/b", `unlink web/x.ts`), Decision{Reason: heldAtB}},
		{"Bash sed -I", bash("/r/b", `sed -I '' 's/a/b/' web/x.ts`), Decision{Reason: heldAtB}},
		{"Bash xargs", bash("/r/b", `ls | xargs rm`), Decision{Reason: `wk: cannot tell which file this writes ("<xargs input>"); give the path in full (the command writes <xargs input>)`}},
		{"Bash git checkout is a write of the root", bash("/r/b", `git checkout main`), Decision{Reason: rootOfB}},
		{"Bash git stash pop from a subdirectory", bash("/r/b/web", `git stash pop`), Decision{Reason: rootOfB}},
		{"Bash git status", bash("/r/b", `git status`), allow},
		{"Bash ls", bash("/r/b", `ls -la docs`), allow},
		{"Bash cat of a leased file", bash("/r/b", `cat web/x.ts`), allow},
		{"Bash write to an own directory lease", bash(rootA, `echo hi > docs/a.md`), allow},
		{"Bash sh -c over a lease", bash("/r/b", `sh -c 'rm web/x.ts'`), Decision{Reason: heldByA + " (the command writes /r/b/web/x.ts)"}},
		{"lower-case bash is not Bash", Request{Tool: "bash", Cwd: "/r/b", Command: "rm web/x.ts"}, Decision{Reason: `wk: cannot tell which file the tool "bash" writes; give the path in full`}},
		{"unknown tool with a path is judged as an edit", Request{Tool: "Frobnicate", Path: "/r/b/web/x.ts"}, Decision{Reason: heldByA}},
		{"unknown tool with nothing to judge", Request{Tool: "Frobnicate", Cwd: "/r/b"}, Decision{Reason: `wk: cannot tell which file the tool "Frobnicate" writes; give the path in full`}},
		{"NotebookEdit", Request{Tool: "NotebookEdit", Path: "/r/b/web/x.ts"}, Decision{Reason: heldByA}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Judge(&s, rules, roots, c.req); got != c.want {
				t.Errorf("Judge = %+v\nwant    %+v", got, c.want)
			}
		})
	}
}

// A write of the whole worktree is allowed while no other root holds a lease, single-writer
// paths aside.
func TestJudgeRepoWrite(t *testing.T) {
	var s Store
	mustClaim(t, &s, rootB, "8", "api/")
	rules := Rules{SingleWriter: []string{"PASSOFF.md"}, BashGuard: true}
	if got := Judge(&s, rules, roots, bash("/r/b/web", `git merge main`)); got != allow {
		t.Errorf("alone, Judge = %+v", got)
	}
	mustClaim(t, &s, rootC, "9", "docs/")
	want := "wk: . is leased by row 9 in /r/c; leave it to that workspace or wait until it releases (the command writes /r/b)"
	if got := Judge(&s, rules, roots, bash("/r/b/web", `git merge main`)); got.Reason != want {
		t.Errorf("beside a lease, Judge = %+v", got)
	}
}

func holds(c string) string {
	return "wk: cannot tell which file this writes: the command holds `" + c + "`, which the lease walk does not read; write it without that character or ask for the path in full"
}

func TestJudgeBashGuardOff(t *testing.T) {
	var s Store
	mustClaim(t, &s, rootA, "7", "web/x.ts")
	got := Judge(&s, Rules{}, roots, bash("/r/b", `rm web/x.ts`))
	if got != allow {
		t.Errorf("with the guard off, Judge = %+v", got)
	}
}

func edit(cwd, path string) Request { return Request{Tool: "Edit", Cwd: cwd, Path: path} }

func bash(cwd, command string) Request { return Request{Tool: "Bash", Cwd: cwd, Command: command} }
