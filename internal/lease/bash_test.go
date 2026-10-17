package lease

import (
	"slices"
	"testing"
)

func TestWriteTargets(t *testing.T) {
	cases := []struct {
		name, command string
		want          []string
	}{
		{"sed -i", `sed -i 's/a/b/' x.ts`, []string{"/w/x.ts"}},
		{"cd then sed -i", `cd web && sed -i 's/a/b/' x.ts`, []string{"/w/web/x.ts"}},
		{"macOS sed -i with empty suffix and -e", `sed -i '' -e 's/a/b/' a.go b.go`, []string{"/w/a.go", "/w/b.go"}},
		{"sed -i with attached suffix", `sed -i.bak 's/a/b/' f`, []string{"/w/f"}},
		{"sed -Ei cluster after --", `sed -Ei -- 's/a/b/' f`, []string{"/w/f"}},
		{"sed without -i writes nothing", `sed 's/a/b/' f`, nil},
		{"sed to stdout redirected", `sed -n 's/a/b/p' f > out`, []string{"/w/out"}},
		{"assignment before tee -a", `FOO=1 tee -a log`, []string{"/w/log"}},
		{"tee in a pipe, two files", `echo x | tee a b`, []string{"/w/a", "/w/b"}},
		{"redirect", `echo hi > out.txt`, []string{"/w/out.txt"}},
		{"append", `echo hi >> logs/run.log`, []string{"/w/logs/run.log"}},
		{"redirect touching the word", `echo hi >out`, []string{"/w/out"}},
		{"stderr to a file", `go test 2>err.txt`, []string{"/w/err.txt"}},
		{"clobber and both streams", `echo x >| f; echo y &> both.log`, []string{"/w/f", "/w/both.log"}},
		{"dup to a descriptor", `go test ./... 2>&1 | tail -5`, nil},
		{"input redirect is not a target", `cat < in > out`, []string{"/w/out"}},
		{"> inside double quotes", `echo "a > b"`, nil},
		{"> quoted alone", `echo 'x' '>' y`, nil},
		{"quoted path with a space", `touch "my file.txt"`, []string{"/w/my file.txt"}},
		{"escaped space", `touch my\ file.txt`, []string{"/w/my file.txt"}},
		{"cp last argument", `cp -r src/ /tmp/dst`, []string{"/tmp/dst"}},
		{"cp ignores an input redirect", `cp a b < x`, []string{"/w/b"}},
		{"mv source and destination", `mv old.go new.go`, []string{"/w/old.go", "/w/new.go"}},
		{"rm", `rm -rf build dist`, []string{"/w/build", "/w/dist"}},
		{"rm after --", `rm -- -weird`, []string{"/w/-weird"}},
		{"git rm -r", `git rm -r dir`, []string{"/w/dir"}},
		{"git mv", `git mv a.go b.go`, []string{"/w/a.go", "/w/b.go"}},
		{"git -C", `git -C sub rm --cached x`, []string{"/w/sub/x"}},
		{"git commands that do not write", `git status && git diff`, nil},
		{"python3 -c open w", `python3 -c "open('f','w')"`, []string{"/w/f"}},
		{"python -c open with mode=", `python -c "open('out/x', mode='a').write('1')"`, []string{"/w/out/x"}},
		{"python -c open to read", `python -c "print(open('f').read())"`, nil},
		{"node -e writeFileSync", `node -e "require('fs').writeFileSync('out.json', '{}')"`, []string{"/w/out.json"}},
		{"heredoc body is no command", "cat <<'EOF'\nrm -rf web\necho x > y\nEOF", nil},
		{"heredoc with a redirect, then a command", "cat > notes.md <<EOF\nhello > there\nEOF\ntouch after", []string{"/w/notes.md", "/w/after"}},
		{"heredoc with tabs", "cat <<-EOF\n\trm x\n\tEOF\nrm y", []string{"/w/y"}},
		{"absolute cd", `cd /abs/dir; rm x`, []string{"/abs/dir/x"}},
		{"subshell cd ends at )", `(cd sub && rm x); rm y`, []string{"/w/sub/x", "/w/y"}},
		{"two lines", "touch a\ntouch b", []string{"/w/a", "/w/b"}},
		{"keyword before the command", `if true; then rm x; fi`, []string{"/w/x"}},
		{"comment", `ls -la && cat x # > y`, nil},
		{"open quote cannot be split", `echo "unterminated > x`, []string{openQuote}},
		{"open quote on the same line as a write", `rm web/x.ts "`, []string{openQuote}},
		{"open quote on a later line", "rm web/x.ts\necho \"", []string{openQuote}},
		{"open single quote on a later line", "rm web/x.ts\necho 'a", []string{openQuote}},
		{"variable at the start stays as written", `rm $DIR/x`, []string{"$DIR/x"}},
		{"glob narrows to its directory", `rm web/*.ts`, []string{"/w/web"}},
		{"glob in the working directory", `rm *.log`, []string{"/w"}},
		{"cd the walk cannot follow", `cd $X && rm y`, []string{"y"}},
		{"single-quoted $ is literal", `touch 'a$b'`, []string{"/w/a$b"}},
		{"unrecognised write passes", `find . -name x -delete`, nil},
		{"shift in $(( )) opens no heredoc", "echo $((1<<2))\nrm web/x.ts", []string{"/w/web/x.ts"}},
		{"shift in (( )) opens no heredoc", "(( x = 1 << 2 ))\nrm y", []string{"/w/y"}},
		{"nested parentheses in arithmetic", "echo $(( (1+2) << 3 ))\nrm y", []string{"/w/y"}},
		{"heredoc in a subshell is still skipped", "(cat <<EOF\nrm x\nEOF\n)", nil},
		{"<< with no delimiter keeps walking", "cat <<\nrm y\nrm z", []string{"/w/y", "/w/z"}},
		{"pushd is not followed", `pushd web && rm x.ts`, []string{"x.ts"}},
		{"popd is not followed", `popd; rm x`, []string{"x"}},
		{"builtin cd is not followed", `builtin cd web && rm x`, []string{"x"}},
		{"command cd is not followed", `command cd web; rm x`, []string{"x"}},
		{"cd - is not followed", `cd - && rm x`, []string{"x"}},
		{"shift in $[ ] opens no heredoc", "echo $[1<<2]\nrm web/x.ts", []string{"/w/web/x.ts"}},
		{"heredoc never closed is walked", "cat <<EOF\nrm x\ntouch y", []string{"/w/x", "/w/y"}},
		{"second heredoc never closed", "cat <<A <<B\nA\nrm x", []string{"/w/x"}},
		{"trailing backslash", "rm web/x.ts \\", []string{"/w/web/x.ts"}},
		{"brace expansion at the start", `rm {web,docs}/x.ts`, []string{"/w"}},
		{"brace expansion in a directory", `rm web/{a,b}.ts`, []string{"/w/web"}},
		{"cd in a pipeline is not followed", `cd web; cd .. | true; rm x.ts`, []string{"x.ts"}},
		{"cd at the end of a pipeline is not followed", `true | cd web; rm x.ts`, []string{"x.ts"}},
		{"cd in the background is not followed", `cd web; cd .. & rm x.ts`, []string{"x.ts"}},
		{"case forgets every directory", "(cd web; case $x in a) true;; esac; rm x.ts)", []string{"x.ts"}},
		{"eval is not followed", `eval cd web; rm x.ts`, []string{"x.ts"}},
		{"hunt: arithmetic then a redirect", "echo $(( 1 << 2 )) > out\nrm a", []string{"/w/out", "/w/a"}},
		{"hunt: nested $[ ]", "echo $[ $[1+1] << 2 ]\nrm a", []string{"/w/a"}},
		{"hunt: let with an unquoted shift", "let x=1<<2\nrm a", []string{"/w/a"}},
		{"hunt: arithmetic for loop", "for ((i=0;i<2;i++)); do rm a; done", []string{"/w/a"}},
		{"hunt: continuation inside a word", "rm a\\\nb", []string{"/w/ab"}},
		{"hunt: joined quotes", `rm "we"'b'/x.ts`, []string{"/w/web/x.ts"}},
		{"hunt: quoted variable mid-word", `rm w"$X"/a`, []string{"/w"}},
		{"hunt: braced variable at the start", `rm ${X}/a`, []string{"${X}/a"}},
		{"hunt: quoted braces are literal", `rm 'web/{a,b}'`, []string{"/w/web/{a,b}"}},
		{"hunt: ANSI-C quoting", `rm $'web/x.ts'`, []string{"$web/x.ts"}},
		{"hunt: command substitution in quotes", `rm -rf -- "$(pwd)/web"`, []string{"$(pwd)/web"}},
		{"hunt: backticks", "rm `pwd`/web", []string{"`pwd`/web"}},
		{"hunt: cd through ..", `cd ./web/../api && rm a`, []string{"/w/api/a"}},
		{"hunt: bare cd", `cd web; cd; rm a`, []string{"a"}},
		{"hunt: group keeps its cd", `{ cd web; rm a; }; rm b`, []string{"/w/web/a", "/w/web/b"}},
		{"hunt: cd in a command substitution ends there", `x=$(cd web; pwd); rm a`, []string{"/w/a"}},
		{"hunt: heredoc piped to sh is walked", "cat <<EOF | sh\nrm a\nEOF\nrm b", []string{"/w/a", "/w/b"}},
		{"hunt: heredoc read by bash is walked", "bash <<'EOF'\nrm a\nEOF", []string{"/w/a"}},
		{"hunt: process substitution keeps the outer command", `tee >(cat > f) web/x.ts`, []string{"/w/f", "/w/web/x.ts"}},
		{"hunt: bash -c from a cd", `cd web && bash -c 'rm x.ts'`, []string{"/w/web/x.ts"}},
		{"hunt: bash -lc with its own cd", `bash -lc "cd web && rm x.ts"`, []string{"/w/web/x.ts"}},
		{"hunt: exec", `exec rm a`, []string{"/w/a"}},
		{"hunt: case pattern with a leading (", "case x in (a) cd web;; esac; rm a", []string{"a"}},
		{"hunt: operators without spaces", `echo a;rm b&&touch c||mv d e`, []string{"/w/b", "/w/c", "/w/d", "/w/e"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := WriteTargets("/w", c.command); !slices.Equal(got, c.want) {
				t.Errorf("WriteTargets(%q) = %q, want %q", c.command, got, c.want)
			}
		})
	}
}

func TestWriteTargetsWithoutCwd(t *testing.T) {
	got := WriteTargets("", `touch a /abs/b`)
	if want := []string{"a", "/abs/b"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}
