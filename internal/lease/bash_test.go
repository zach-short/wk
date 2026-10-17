package lease

import (
	"slices"
	"strings"
	"testing"
)

func TestWriteTargets(t *testing.T) {
	cases := []struct {
		name, command string
		want          []string
	}{
		{"sed -i", `sed -i 's/a/b/' x.ts`, []string{"/w/x.ts"}},
		{"cd then sed -i, judged at both directories", `cd web && sed -i 's/a/b/' x.ts`, []string{"/w/x.ts", "/w/web/x.ts"}},
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
		{"escaped space is refused", `touch my\ file.txt`, []string{refused(`\`)}},
		{"cp last argument", `cp -r src/ /tmp/dst`, []string{"/tmp/dst"}},
		{"cp ignores an input redirect", `cp a b < x`, []string{"/w/b"}},
		{"mv source and destination", `mv old.go new.go`, []string{"/w/old.go", "/w/new.go"}},
		{"rm", `rm -rf build dist`, []string{"/w/build", "/w/dist"}},
		{"rm after --", `rm -- -weird`, []string{"/w/-weird"}},
		{"git rm -r", `git rm -r dir`, []string{"/w/dir"}},
		{"git mv", `git mv a.go b.go`, []string{"/w/a.go", "/w/b.go"}},
		{"git -C", `git -C sub rm --cached x`, []string{"/w/sub/x"}},
		{"git commands that do not write", `git status && git diff`, nil},
		{"python3 -c is refused for its parentheses", `python3 -c "open('f','w')"`, []string{refused("(")}},
		{"python -c open with mode= is refused", `python -c "open('out/x', mode='a').write('1')"`, []string{refused("(")}},
		{"python -c open to read is refused", `python -c "print(open('f').read())"`, []string{refused("(")}},
		{"node -e is refused", `node -e "require('fs').writeFileSync('out.json', '{}')"`, []string{refused("(")}},
		{"heredoc body is walked", "cat <<'EOF'\nrm -rf web\necho x > y\nEOF", []string{"/w/web", "/w/y"}},
		{"heredoc with a redirect, then a command", "cat > notes.md <<EOF\nhello > there\nEOF\ntouch after", []string{"/w/notes.md", "/w/there", "/w/after"}},
		{"heredoc with tabs", "cat <<-EOF\n\trm x\n\tEOF\nrm y", []string{"/w/x", "/w/y"}},
		{"absolute cd keeps the old directory", `cd /abs/dir; rm x`, []string{"/w/x", "/abs/dir/x"}},
		{"subshell is refused", `(cd sub && rm x); rm y`, []string{refused("(")}},
		{"two lines", "touch a\ntouch b", []string{"/w/a", "/w/b"}},
		{"keyword before the command", `if true; then rm x; fi`, []string{"/w/x"}},
		{"comment", `ls -la && cat x # > y`, nil},
		{"open quote cannot be split", `echo "unterminated > x`, []string{openQuote}},
		{"open quote on the same line as a write", `rm web/x.ts "`, []string{openQuote}},
		{"open quote on a later line", "rm web/x.ts\necho \"", []string{openQuote}},
		{"open single quote on a later line", "rm web/x.ts\necho 'a", []string{openQuote}},
		{"variable is refused", `rm $DIR/x`, []string{refused("$")}},
		{"glob is refused", `rm web/*.ts`, []string{refused("*")}},
		{"glob in the working directory is refused", `rm *.log`, []string{refused("*")}},
		{"cd to a variable is refused", `cd $X && rm y`, []string{refused("$")}},
		{"single-quoted $ is refused", `touch 'a$b'`, []string{refused("$")}},
		{"find -delete writes its starting point", `find . -name x -delete`, []string{"/w"}},
		{"a target with .. keeps it for the caller to read through links", `rm sub/e/../web/x.ts /r/a/../b`, []string{"/w/sub/e/../web/x.ts", "/r/a/../b"}},
		{"audit 4: git rm out/../web/page.ts is unplaceable", `git rm out/../web/page.ts`, []string{pathspec("out/../web/page.ts")}},
		{"audit 4: git rm -r out/../web is unplaceable", `git rm -r out/../web`, []string{pathspec("out/../web")}},
		{"audit 4: git mv out/../web/page.ts web/moved.ts is unplaceable", `git mv out/../web/page.ts web/moved.ts`, []string{pathspec("out/../web/page.ts"), "/w/web/moved.ts"}},
		{"audit 4: git mv -f notes.txt out/../web/page.ts is unplaceable", `git mv -f notes.txt out/../web/page.ts`, []string{"/w/notes.txt", pathspec("out/../web/page.ts")}},
		{"audit 4: git rm --cached out/../web/page.ts is unplaceable", `git rm --cached out/../web/page.ts`, []string{pathspec("out/../web/page.ts")}},
		{"audit 4: git -C p rm o/../../a.md is unplaceable", `git -C p rm o/../../a.md`, []string{pathspec("o/../../a.md")}},
		{"audit 4: git rm :/docs/a.md is unplaceable", `git rm :/docs/a.md`, []string{pathspec(":/docs/a.md")}},
		{"audit 4: git rm :docs/a.md is unplaceable", `git rm :docs/a.md`, []string{pathspec(":docs/a.md")}},
		{"audit 4: git rm p/../lnk is unplaceable", `git rm p/../lnk`, []string{pathspec("p/../lnk")}},
		{"a cd with .. lands cleaned and raw", `cd sub/e/../web && touch x.ts`, []string{"/w/x.ts", "/w/sub/web/x.ts", "/w/sub/e/../web/x.ts"}},
		{"shift in $(( )) is refused", "echo $((1<<2))\nrm web/x.ts", []string{refused("$")}},
		{"shift in (( )) is refused", "(( x = 1 << 2 ))\nrm y", []string{refused("(")}},
		{"nested parentheses in arithmetic are refused", "echo $(( (1+2) << 3 ))\nrm y", []string{refused("$")}},
		{"heredoc in a subshell is refused", "(cat <<EOF\nrm x\nEOF\n)", []string{refused("(")}},
		{"<< with no delimiter is refused", "cat <<\nrm y\nrm z", []string{openHeredoc}},
		{"pushd is judged at both directories", `pushd web && rm x.ts`, []string{"/w/x.ts", "/w/web/x.ts"}},
		{"popd is not followed", `popd; rm x`, []string{"/w/x", "x"}},
		{"builtin cd is judged at both directories", `builtin cd web && rm x`, []string{"/w/x", "/w/web/x"}},
		{"command cd is judged at both directories", `command cd web; rm x`, []string{"/w/x", "/w/web/x"}},
		{"cd - is not followed", `cd - && rm x`, []string{"/w/x", "x"}},
		{"shift in $[ ] is refused", "echo $[1<<2]\nrm web/x.ts", []string{refused("$")}},
		{"heredoc never closed is refused", "cat <<EOF\nrm x\ntouch y", []string{openHeredoc}},
		{"second heredoc never closed is refused", "cat <<A <<B\nA\nrm x", []string{openHeredoc}},
		{"trailing backslash is refused", "rm web/x.ts \\", []string{refused(`\`)}},
		{"brace expansion is refused", `rm {web,docs}/x.ts`, []string{refused("{")}},
		{"brace expansion in a directory is refused", `rm web/{a,b}.ts`, []string{refused("{")}},
		{"cd in a pipeline", `cd web; cd .. | true; rm x.ts`, []string{"/w/x.ts", "/w/web/x.ts", "/x.ts", "/w/../x.ts", "/w/web/../x.ts"}},
		{"cd at the end of a pipeline", `true | cd web; rm x.ts`, []string{"/w/x.ts", "/w/web/x.ts"}},
		{"cd in the background", `cd web; cd .. & rm x.ts`, []string{"/w/x.ts", "/w/web/x.ts", "/x.ts", "/w/../x.ts", "/w/web/../x.ts"}},
		{"case is refused for its )", "(cd web; case $x in a) true;; esac; rm x.ts)", []string{refused("(")}},
		{"eval cd is not followed", `eval cd web; rm x.ts`, []string{"x.ts"}},
		{"hunt: arithmetic then a redirect", "echo $(( 1 << 2 )) > out\nrm a", []string{refused("$")}},
		{"hunt: nested $[ ]", "echo $[ $[1+1] << 2 ]\nrm a", []string{refused("$")}},
		{"hunt: let with an unquoted shift opens a heredoc with no end", "let x=1<<2\nrm a", []string{openHeredoc}},
		{"hunt: arithmetic for loop", "for ((i=0;i<2;i++)); do rm a; done", []string{refused("(")}},
		{"hunt: continuation inside a word", "rm a\\\nb", []string{refused(`\`)}},
		{"hunt: joined quotes", `rm "we"'b'/x.ts`, []string{"/w/web/x.ts"}},
		{"hunt: quoted variable mid-word", `rm w"$X"/a`, []string{refused("$")}},
		{"hunt: braced variable at the start", `rm ${X}/a`, []string{refused("$")}},
		{"hunt: quoted braces", `rm 'web/{a,b}'`, []string{refused("{")}},
		{"hunt: ANSI-C quoting", `rm $'web/x.ts'`, []string{refused("$")}},
		{"hunt: command substitution in quotes", `rm -rf -- "$(pwd)/web"`, []string{refused("$")}},
		{"hunt: backticks", "rm `pwd`/web", []string{refused("`")}},
		{"hunt: cd through ..", `cd ./web/../api && rm a`, []string{"/w/a", "/w/api/a", "/w/./web/../api/a"}},
		{"hunt: bare cd", `cd web; cd; rm a`, []string{"/w/a", "/w/web/a", "a"}},
		{"hunt: group is refused", `{ cd web; rm a; }; rm b`, []string{refused("{")}},
		{"hunt: command substitution is refused", `x=$(cd web; pwd); rm a`, []string{refused("$")}},
		{"hunt: heredoc piped to sh", "cat <<EOF | sh\nrm a\nEOF\nrm b", []string{"<standard input>", "/w/a", "/w/b"}},
		{"hunt: heredoc read by bash", "bash <<'EOF'\nrm a\nEOF", []string{"<standard input>", "/w/a"}},
		{"hunt: process substitution is refused", `tee >(cat > f) web/x.ts`, []string{refused("(")}},
		{"hunt: bash -c from a cd", `cd web && bash -c 'rm x.ts'`, []string{"/w/x.ts", "/w/web/x.ts"}},
		{"hunt: bash -lc with its own cd", `bash -lc "cd web && rm x.ts"`, []string{"/w/x.ts", "/w/web/x.ts"}},
		{"hunt: exec", `exec rm a`, []string{"/w/a"}},
		{"hunt: case pattern is refused", "case x in (a) cd web;; esac; rm a", []string{refused("(")}},
		{"hunt: operators without spaces", `echo a;rm b&&touch c||mv d e`, []string{"/w/b", "/w/c", "/w/d", "/w/e"}},

		// Rule 1: a byte outside the safe set refuses the whole command.
		{"rule 1: question mark", `rm web/x?.ts`, []string{refused("?")}},
		{"rule 1: brackets", `rm web/[ab].ts`, []string{refused("[")}},
		{"rule 1: dot-dot after a glob", `rm docs/*/../../web/x.ts`, []string{refused("*")}},
		{"rule 1: non-ASCII", `rm café`, []string{refused("é")}},
		{"rule 1: carriage return", "rm a\rb", []string{refused(`\r`)}},
		{"rule 1: zsh numeric glob", `rm web/x<1-9>.ts`, []string{refused("<1-9>")}},
		{"rule 1: tab is safe", "rm\tweb/x.ts", []string{"/w/web/x.ts"}},
		{"rule 1: an unquoted leading ~ cannot be placed", `rm ~/x`, []string{"~/x"}},
		{"rule 1: a quoted ~ is a name", `rm '~/x'`, []string{"/w/~/x"}},
		{"rule 1: zsh = expansion cannot be placed", `rm =x`, []string{"=x"}},
		{"rule 1: zsh extended glob cannot be placed", `rm web/x.ts~a web/x.t#`, []string{"web/x.ts~a", "web/x.t#"}},
		{"rule 1: zsh clobber redirections", "echo a >! web/x.ts; echo b >>& web/y.ts", []string{"/w/web/x.ts", "/w/web/y.ts"}},

		// Rule 3: every heredoc body is walked.
		{"rule 3: heredoc on a piped line, shell on a later one", "cat <<EOF |\nrm web/x.ts\nEOF\nsh", []string{"/w/web/x.ts", "<standard input>"}},
		{"rule 3: here-string is a read", `cat <<<word > out`, []string{"/w/out"}},
		{"rule 3: source of standard input", "source /dev/stdin <<EOF\nrm web/x.ts\nEOF", []string{"<standard input>", "/w/web/x.ts"}},
		{"rule 3: a shell on standard input", `echo rm a | sh`, []string{"<standard input>"}},
		{"rule 3: a script file is not read", `bash script.sh`, nil},

		{"rule 3 fix 1: an apostrophe in a body pairs with nothing outside it", "cat <<EOF > docs/a.md\ndon't\nEOF\nrm web/x.ts # it's gone", []string{"/w/docs/a.md", "/w/web/x.ts"}},
		{"rule 3 fix 1: two bodies with apostrophes", "cat <<EOF > docs/a.md\ndon't\nEOF\nrm web/x.ts\ncat <<EOF > docs/b.md\nwon't\nEOF", []string{"/w/docs/a.md", "/w/web/x.ts", "/w/docs/b.md"}},
		{"rule 3 fix 1: a double quote in a body", "cat <<EOF > docs/a.md\nsay \"hi\nEOF\ntouch web/x.ts\necho \"there\"", []string{"/w/docs/a.md", "/w/web/x.ts"}},
		{"rule 3 fix 1: the delimiter inside prose is no terminator", "cat <<EOF\nthe EOF isn't here\nEOF\nrm web/x.ts", []string{"/w/web/x.ts"}},
		{"rule 3 fix 1: a write in a body with an open quote", "cat <<EOF\nrm web/x.ts 'a\nb'\nEOF", []string{"/w/web/x.ts", "/w/a"}},
		{"rule 3 fix 1: quoted delimiter", "cat <<'EOF'\nit's\nEOF\nrm a", []string{"/w/a"}},
		{"rule 3 fix 1: double-quoted delimiter", "cat <<\"EOF\"\nit's\nEOF\nrm a", []string{"/w/a"}},
		{"rule 3 fix 1: <<- terminator after tabs", "cat <<-EOF\n\tit's\n\tEOF\nrm a", []string{"/w/a"}},
		{"rule 3 fix 1: << in a body opens nothing", "cat <<EOF\na << b\nEOF\nrm a", []string{"/w/a"}},
		{"rule 3 fix 2: a space before the delimiter", "cat << EOF > docs/a.md\ndon't\nEOF\nrm web/x.ts # it's gone", []string{"/w/docs/a.md", "/w/web/x.ts"}},
		{"rule 3 fix 2: <<- and a space", "cat <<- EOF > docs/a.md\n\tdon't\n\tEOF\nrm web/x.ts # it's gone", []string{"/w/docs/a.md", "/w/web/x.ts"}},
		{"rule 3 fix 2: a space before a quoted delimiter", "cat << 'EOF' > docs/a.md\ndon't\nEOF\nrm web/x.ts # it's gone", []string{"/w/docs/a.md", "/w/web/x.ts"}},
		{"rule 3 fix 2: a tab before the delimiter", "cat <<\tEOF > docs/a.md\ndon't\nEOF\nrm web/x.ts # it's gone", []string{"/w/docs/a.md", "/w/web/x.ts"}},
		{"rule 3 fix 2: a double quote across two bodies", "cat << EOF\nsay \"hi\nEOF\ntouch web/x.ts\ncat << EOF\nthere\"\nEOF", []string{"/w/web/x.ts"}},
		{"rule 3 fix 2: no space, as before", "cat <<EOF > docs/a.md\ndon't\nEOF\nrm web/x.ts", []string{"/w/docs/a.md", "/w/web/x.ts"}},
		{"rule 3 fix 2: << and a space, then a newline", "cat << \nrm y", []string{openHeredoc}},
		{"rule 3 fix 2: << then an operator", "cat << | rm y", []string{openHeredoc}},
		{"rule 3 fix 2: << at the end of the input", "cat <<", []string{openHeredoc}},
		{"rule 3 fix 1: heredoc at the end of the input", "cat <<EOF", []string{openHeredoc}},
		{"rule 3 fix 1: a body that never ends", "cat <<EOF > docs/a.md\ndon't\nrm web/x.ts", []string{openHeredoc}},

		// Rule 4: a relative target after a directory change is judged at every directory.
		{"rule 4: cd after && that may not run", `test -f docs/Makefile && cd docs; rm web/x.ts`, []string{"/w/web/x.ts", "/w/docs/web/x.ts"}},
		{"rule 4: cd to a missing directory", `cd nope; rm web/x.ts`, []string{"/w/web/x.ts", "/w/nope/web/x.ts"}},
		{"rule 4: cd after ||", `cd docs/web 2>/dev/null || cd web; rm x.ts`, []string{"/dev/null", "/w/x.ts", "/w/docs/web/x.ts", "/w/web/x.ts", "/w/docs/web/web/x.ts"}},
		{"rule 4: zsh two-word cd", `cd a b; rm x`, []string{"/w/x", "x"}},
		{"rule 4: too many directories", `cd a; cd b; cd c; cd d; cd e; rm x`, []string{"x"}},
		{"rule 4: eval is walked", `eval 'rm web/x.ts'`, []string{"/w/web/x.ts"}},
		{"rule 4: trap is walked from no directory", `trap 'rm web/x.ts' EXIT`, []string{"web/x.ts"}},
		{"rule 4: alias is walked from no directory", `alias x='rm /r/b/web/x.ts'`, []string{"/r/b/web/x.ts"}},

		// Rule 5: wrappers before the verb.
		{"rule 5: time -p", `time -p rm web/x.ts`, []string{"/w/web/x.ts"}},
		{"rule 5: env with an assignment", `env FOO=1 rm web/x.ts`, []string{"/w/web/x.ts"}},
		{"rule 5: env -i with assignments", `env -i FOO=1 BAR+=2 rm a`, []string{"/w/a"}},
		{"rule 5: leading NAME+=value", `FOO+=x rm a`, []string{"/w/a"}},
		{"rule 5: command", `command rm a`, []string{"/w/a"}},
		{"rule 5: nohup", `nohup rm a`, []string{"/w/a"}},
		{"rule 5: timeout", `timeout 5 rm a`, []string{"/w/a"}},
		{"rule 5: timeout with a signal", `timeout -s KILL 5 rm a`, []string{"/w/a"}},
		{"rule 5: sudo -u", `sudo -u root rm a`, []string{"/w/a"}},
		{"rule 5: exec -a", `exec -a name rm a`, []string{"/w/a"}},
		{"rule 5: coproc", `coproc rm a`, []string{"/w/a"}},
		{"rule 5: nice", `nice -n 5 rm a`, []string{"/w/a"}},
		{"rule 5: zsh precommand modifiers", `noglob rm a; - rm b; repeat 3 rm c`, []string{"/w/a", "/w/b", "/w/c"}},
		{"rule 5: stacked wrappers", `sudo env FOO=1 nohup /bin/rm a`, []string{"/w/a"}},
		{"rule 5: a sudo option the walk does not know", `sudo -D web rm a`, []string{"<sudo option>"}},
		{"rule 5: env -C", `env -C web rm a`, []string{"<env option>"}},
		{"rule 5: a long wrapper option", `timeout --foreground 5 rm a`, []string{"<timeout option>"}},
		{"rule 5: xargs rm", `ls | xargs rm`, []string{"<xargs input>"}},
		{"rule 5: xargs with options and sed -i", `ls | xargs -n 1 sed -i 's/a/b/'`, []string{"<xargs input>"}},
		{"rule 5: xargs grep writes nothing", `ls | xargs grep x`, nil},

		// Rule 6: verbs.
		{"rule 6: unlink", `unlink web/x.ts`, []string{"/w/web/x.ts"}},
		{"rule 6: sed -I", `sed -I '' 's/a/b/' web/x.ts`, []string{"/w/web/x.ts"}},
		{"rule 6: zsh -c", `zsh -c 'rm a'; dash -c 'rm b'; ksh -c 'rm c'`, []string{"/w/a", "/w/b", "/w/c"}},
		{"rule 6: git checkout a branch", `git checkout main`, []string{"/w"}},
		{"rule 6: git checkout --", `git checkout -- web/x.ts`, []string{"/w"}},
		{"rule 6: git checkout -", `git checkout -`, []string{"/w"}},
		{"rule 6: git checkout -b alone", `git checkout -b feature`, nil},
		{"rule 6: git checkout -b with a start", `git checkout -b feature origin/main`, []string{"/w"}},
		{"rule 6: git stash pop", `git stash pop`, []string{"/w"}},
		{"rule 6: bare git stash", `git stash`, []string{"/w"}},
		{"rule 6: git stash list", `git stash list`, nil},
		{"rule 6: git reset --hard", `git reset --hard HEAD`, []string{"/w"}},
		{"rule 6: git reset of the index", `git reset HEAD x`, nil},
		{"rule 6: the other tree writers", `git clean -fd; git switch main; git restore x; git merge main; git pull; git rebase main; git apply x.diff; git am x; git cherry-pick abc; git revert abc`, []string{"/w", "/w", "/w", "/w", "/w", "/w", "/w", "/w", "/w", "/w"}},
		{"rule 6: git worktree remove", `git worktree remove ../x; git worktree list`, []string{"/w"}},
		{"rule 6: git status", `git status`, nil},
		{"rule 6: git -C then a tree writer", `git -C sub checkout main`, []string{"/w/sub"}},
		{"rule 6: cd then git pull", `cd web && git pull`, []string{"/w", "/w/web"}},
		{"rule 6: git --work-tree", `git --work-tree=/r/b checkout main`, []string{"."}},
		{"rule 6: GIT_WORK_TREE", `GIT_WORK_TREE=/r/b git checkout main`, []string{"."}},
		{"rule 6: git -c alias with a shell command", `git -c alias.x='!rm web/x.ts' x`, []string{alias("x")}},
		{"rule 6: git -c that runs nothing", `git -c color.ui=false status`, nil},

		// Rule 7: the audits' gaps inside the safe set.
		{"rule 7: gofmt -w", `gofmt -w web/x.go; gofmt -l .`, []string{"/w/web/x.go"}},
		{"rule 7: prettier --write", `prettier --write web/x.ts`, []string{"/w/web/x.ts"}},
		{"rule 7: perl -pi", `perl -pi -e 's/a/b/' web/x.ts`, []string{"/w/s/a/b", "/w/web/x.ts"}},
		{"rule 7: awk -i inplace", `awk -i inplace 'NR==1' web/x.ts`, []string{"/w/inplace", "/w/NR==1", "/w/web/x.ts"}},
		{"rule 7: rsync", `rsync -a src/ web/`, []string{"/w/src", "/w/web"}},
		{"rule 7: tar -x -C", `tar -xzf a.tgz -C web`, []string{"/w/a.tgz", "/w/web", "/w"}},
		{"rule 7: tar -C attached", `tar -xf a.tar -C/r/web`, []string{"/w/a.tar", "/r/web", "/w"}},
		{"rule 7: tar create", `tar -czf out.tgz src`, []string{"/w/out.tgz", "/w/src"}},
		{"rule 7: find -delete in a directory", `find web -name x -delete`, []string{"/w/web"}},
		{"rule 7: find -exec", `find web -name x -exec rm x ';'`, []string{"<find -exec>"}},
		{"rule 7: find that writes nothing", `find web -name x`, nil},
		{"rule 7: dd of=", `dd if=a of=web/x.ts`, []string{"/w/web/x.ts"}},
		{"rule 7: cp -t", `cp -t web a b`, []string{"/w/web", "/w/a", "/w/b"}},
		{"rule 7: cp --target-directory=", `cp --target-directory=web a`, []string{"/w/a", "/w/web"}},
		{"rule 7: ln", `ln -s a web/x.ts`, []string{"/w/web/x.ts"}},
		{"rule 7: patch", `patch -p1 < x.diff`, []string{"<patch input>"}},
		{"rule 7: truncate", `truncate -s 0 web/x.ts`, []string{"/w/0", "/w/web/x.ts"}},

		// The builder's own hunt inside the safe set.
		{"hunt 2: here-string is walked", `source /dev/stdin <<<'rm web/x.ts'`, []string{"/w/web/x.ts", "<standard input>"}},
		{"hunt 2: here-string data", `wc -w <<<'hello world'`, nil},
		{"hunt 2: a pipe into source", `echo rm web/x.ts | . /dev/stdin`, []string{"<standard input>"}},
		{"hunt 2: bash -", `echo rm a | bash -`, []string{"<standard input>"}},
		{"hunt 2: git checkout -f", `git checkout -f`, []string{"/w"}},
		{"hunt 2: bare git checkout", `git checkout`, []string{"/w"}},
		{"hunt 2: git bisect", `git bisect good`, []string{"/w"}},
		{"hunt 2: relative cd beside CDPATH", `CDPATH=/r/b cd web; rm x.ts`, []string{"/w/x.ts", "x.ts"}},
		{"hunt 2: absolute cd beside CDPATH", `CDPATH=/r cd /r/b; rm x.ts`, []string{"/w/x.ts", "/r/b/x.ts"}},
		{"hunt 2: ln with one operand", `ln -s /tmp/x.ts`, []string{"/w/x.ts"}},
		{"hunt 2: chmod", `chmod +x web/x.ts`, []string{"/w/+x", "/w/web/x.ts"}},
		{"hunt 2: busybox", `busybox rm a`, []string{"/w/a"}},
		{"hunt 2: fish -c", `fish -c 'rm a'`, []string{"/w/a"}},
		{"hunt 2: a bare redirect", `>web/x.ts`, []string{"/w/web/x.ts"}},
		{"hunt 2: exec into a file", `exec 3>web/x.ts`, []string{"/w/web/x.ts"}},
		{"hunt 2: dup into a file", `echo x >&web/x.ts`, []string{"/w/web/x.ts"}},
		{"hunt 2: read-write", `exec 1<>web/x.ts`, []string{"/w/web/x.ts"}},
		{"hunt 2: a comment hides a redirect", `echo hi #> web/x.ts`, nil},
		{"hunt 2: # mid-word is no comment", `echo a#> web/x.ts`, []string{"/w/web/x.ts"}},
		{"hunt 2: newline after &&", "true &&\nrm web/x.ts", []string{"/w/web/x.ts"}},
		{"hunt 2: quoted verb", `"rm" web/x.ts; r''m web/y.ts`, []string{"/w/web/x.ts", "/w/web/y.ts"}},
		{"hunt 2: trap after --", `trap -- 'rm /r/b/web/x.ts' EXIT`, []string{"/r/b/web/x.ts"}},
		{"hunt 2: sudo -s", `sudo -s rm a`, []string{"<sudo option>"}},
		{"hunt 2: env -S", `env -S 'rm a'`, []string{"<env option>"}},
		{"hunt 2: cd -P", `cd -P web && rm x.ts`, []string{"/w/x.ts", "/w/web/x.ts"}},
		{"hunt 2: cd +1 in zsh", `cd +1; rm x`, []string{"/w/x", "x"}},
		{"hunt 2: cp to the working directory", `cp -r /tmp/web .`, []string{"/w"}},
		{"hunt 2: git -c alias run from no directory", `git -c alias.x='!rm /r/b/web/x.ts' x`, []string{alias("x")}},

		// Audit 5: git spellings the walk did not read (B12, B13).
		{"audit 5: git rm --pathspec-from-file= after a write of the list", `echo web/page.ts > l && git rm --pathspec-from-file=l`, []string{"/w/l", fromList("--pathspec-from-file=l")}},
		{"audit 5: git rm --pathspec-from-file=- from a pipe", `echo web/page.ts | git rm --pathspec-from-file=-`, []string{fromList("--pathspec-from-file=-")}},
		{"audit 5: git rm --pathspec-from-file=list.txt", `git rm --pathspec-from-file=list.txt`, []string{fromList("--pathspec-from-file=list.txt")}},
		{"audit 5: git rm --pathspec-from-file list.txt", `git rm --pathspec-from-file list.txt`, []string{fromList("--pathspec-from-file")}},
		{"audit 5: git add --pathspec-from-file", `git add --pathspec-from-file=l`, []string{fromList("--pathspec-from-file=l")}},
		{"audit 5: git checkout --pathspec-from-file", `git checkout --pathspec-from-file=l`, []string{fromList("--pathspec-from-file=l")}},
		{"audit 5: the other verbs that take --pathspec-from-file", `git restore --pathspec-from-file=l; git reset --pathspec-from-file=l; git stash push --pathspec-from-file=l; git commit --pathspec-from-file=l`, []string{fromList("--pathspec-from-file=l"), fromList("--pathspec-from-file=l"), fromList("--pathspec-from-file=l"), fromList("--pathspec-from-file=l")}},
		{"audit 5: --pathspec-from-file abbreviated as git allows", `git rm --pathspec-fr=l; git rm --pathspec-from l`, []string{fromList("--pathspec-fr=l"), fromList("--pathspec-from")}},
		{"audit 5: --pathspec-from-file after -- is a path", `git rm -- --pathspec-from-file=l`, []string{"/w/--pathspec-from-file=l"}},
		{"audit 5: git -c alias.x=rm", `git -c alias.x=rm x web/page.ts`, []string{alias("x")}},
		{"audit 5: git -c alias.x=mv", `git -c alias.x=mv x -f a b`, []string{alias("x")}},
		{"audit 5: git -c alias.x=checkout", `git -c alias.x=checkout x -- web/page.ts`, []string{alias("x")}},
		{"audit 5: git -c alias.x=!rm", `git -c 'alias.x=!rm' x`, []string{alias("x")}},
		{"audit 5: git reads an alias key in any case", `git -c ALIAS.Y=rm Y web/page.ts`, []string{alias("Y")}},
		{"audit 5: a stored alias is not a verb the walk reads", `git x web/page.ts`, []string{verb("x")}},
		{"audit 5: an alias from --config-env is not a verb the walk reads", `git --config-env=alias.y=X y web/page.ts`, []string{gitConfig("--config-env=alias.y=X"), verb("y")}},
		{"audit 5: an unknown verb after -C", `git -C sub x web/page.ts`, []string{verb("x")}},
		{"audit 5: read-only verbs stay allowed", `git status; git log --oneline -5; git diff; git fetch`, nil},
		{"audit 5: add, commit and push stay allowed", `git add -A && git commit -m msg && git push`, nil},
		{"audit 5: read forms of branch, tag, remote, reflog and config", `git branch feature; git branch -a; git tag v1; git remote -v; git remote show origin; git reflog; git config --get user.name; git config -l; git -c core.pager=less log`, nil},
		{"audit 5: branch -D and branch --del", `git branch -D old; git branch --del old`, []string{verb("branch"), verb("branch")}},
		{"audit 5: tag -d, remote add, reflog expire", `git tag -d v1; git remote add o u; git reflog expire --all`, []string{verb("tag"), verb("remote"), verb("reflog")}},
		{"audit 5: a config that stores an alias", `git config alias.x rm`, []string{verb("config")}},

		// Row 14 audit 1: git writes through options, transports and the environment.
		{"F1: a config value then -l is a write", `git config core.fsmonitor 'rm web/page.ts' -l && git status`, []string{verb("config")}},
		{"F1: a config value then --get is a write", `git config --global core.fsmonitor 'touch p' --get`, []string{verb("config")}},
		{"F1: a config value then --list is a write", `git config diff.external x --list && git diff HEAD~1`, []string{verb("config")}},
		{"F1: config reads stay allowed", `git config --get user.name; git config -l; git config --list; git config user.name; git config -f c.cfg --get x; git config get user.name`, nil},
		{"F1: config --unset and --edit are writes", `git config --unset user.name; git config -e`, []string{verb("config"), verb("config")}},
		{"F2: diff --output", `git diff --output=web/page.ts`, []string{"/w/web/page.ts"}},
		{"F2: log --output", `git log --output=web/page.ts`, []string{"/w/web/page.ts"}},
		{"F2: show --output", `git show --output=web/page.ts`, []string{"/w/web/page.ts"}},
		{"F2: shortlog --output", `git shortlog --output=web/page.ts HEAD`, []string{"/w/web/page.ts"}},
		{"F2: rev-list --output", `git rev-list --output=web/page.ts HEAD`, []string{"/w/web/page.ts"}},
		{"F2: blame --output", `git blame --output=web/page.ts go.mod`, []string{"/w/web/page.ts"}},
		{"F2: stash show --output", `git stash show --output=web/page.ts`, []string{"/w/web/page.ts"}},
		{"F2: stash list --output", `git stash list --output=web/page.ts`, []string{"/w/web/page.ts"}},
		{"F2: --output in the next word, after -C", `git log --output web/page.ts; git -C sub diff --output=x.ts`, []string{"/w/web/page.ts", "/w/sub/x.ts"}},
		{"F2: --output after -- is a path", `git log -- --output=x`, nil},
		{"F3: reset --har", `git reset --har`, []string{"/w"}},
		{"F3: reset --kee", `git reset --kee HEAD~1`, []string{"/w"}},
		{"F3: reset --merg", `git reset --merg HEAD~1`, []string{"/w"}},
		{"F3: checkout -b then --forc", `git checkout -b nb1 --forc`, []string{"/w"}},
		{"F3: checkout --forc then -b", `git checkout --forc -b nb1`, []string{"/w"}},
		{"F3: reset --mixed and --soft stay index writes", `git reset --mixed HEAD~1; git reset --soft HEAD~1`, nil},
		{"F4: grep -O attached", `git grep -Ovim x`, []string{grepPager("-Ovim")}},
		{"F4: grep -O and a word", `git grep -O vim x`, []string{grepPager("-O")}},
		{"F4: grep --open-files-in-pager=", `git grep --open-files-in-pager=vim x`, []string{grepPager("--open-files-in-pager=vim")}},
		{"F4: grep --open-files-in-pager and a word", `git grep --open-files-in-pager vim x`, []string{grepPager("--open-files-in-pager")}},
		{"F5: fetch --upload-pack", `git fetch --upload-pack='touch web/page.ts' .`, []string{transport("--upload-pack=touch web/page.ts")}},
		{"F5: push --receive-pack", `git push --receive-pack=x . HEAD`, []string{transport("--receive-pack=x")}},
		{"F5: push --exec", `git push --exec=x .`, []string{transport("--exec=x")}},
		{"F5: ls-remote --upload-pack and a word", `git ls-remote --upload-pack x .`, []string{transport("--upload-pack")}},
		{"F5: an ext:: URL", `git -c protocol.ext.allow=always fetch 'ext::sh -c x'`, []string{gitConfig("-c protocol.ext.allow=always"), transport("ext::sh -c x")}},
		{"F5: GIT_SSH_COMMAND", `GIT_SSH_COMMAND=x git fetch ssh://h/r`, []string{variable("GIT_SSH_COMMAND")}},
		{"F5: GIT_EXTERNAL_DIFF", `GIT_EXTERNAL_DIFF=x git diff`, []string{variable("GIT_EXTERNAL_DIFF")}},
		{"F5: GIT_CONFIG_COUNT", `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=core.fsmonitor GIT_CONFIG_VALUE_0=x git status`, []string{variable("GIT_CONFIG_COUNT")}},
		{"F5: an exported GIT_ variable", `export GIT_SSH_COMMAND=x; git fetch`, []string{variable("GIT_SSH_COMMAND")}},
		{"F6: worktree add", `git worktree add docs/x`, []string{"/w/docs/x"}},
		{"F6: worktree add -b", `git worktree add -b nb web/x`, []string{"/w/web/x"}},
		{"F6: worktree add --detach", `git worktree add --detach web/x HEAD`, []string{"/w/web/x"}},
		{"F6: worktree list, prune, lock and unlock read", `git worktree list; git worktree prune; git worktree lock x; git worktree unlock x`, nil},
		{"F7: --config-env=", `git --config-env=diff.external=X diff HEAD~1`, []string{gitConfig("--config-env=diff.external=X")}},
		{"F7: --config-env and a word", `git --config-env core.fsmonitor=X status`, []string{gitConfig("--config-env core.fsmonitor=X")}},
		{"F7: core.fsmonitor is still walked as a command", `git -c core.fsmonitor='rm web/page.ts' status`, []string{"web/page.ts"}},
		{"N1: read-only verbs", `git merge-tree --write-tree a b; git show-ref; git diff-tree HEAD; git check-ignore x; git show-branch; git range-diff a b c; git whatchanged; git var -l; git count-objects; git ls-remote origin`, nil},
		{"N1: submodule status and symbolic-ref reads", `git submodule status; git submodule; git symbolic-ref HEAD; git symbolic-ref --short HEAD`, nil},
		{"N1: symbolic-ref with two operands or -d", `git symbolic-ref HEAD refs/heads/x; git symbolic-ref -d HEAD`, []string{verb("symbolic-ref"), verb("symbolic-ref")}},
		{"N2: git mv has no --pathspec-from-file", `git mv --pathspec-from-file=l`, nil},

		// Row 14 audit 2: an option's value is never an option, and ext:: in a -c value.
		{"B1: --comment takes -l as its value", `git config --comment -l core.fsmonitor 'rm web/page.ts'`, []string{verb("config")}},
		{"B1: --comment takes --list as its value", `git config --comment --list core.fsmonitor x`, []string{verb("config")}},
		{"B1: --comment takes --get as its value", `git config --comment --get diff.external x`, []string{verb("config")}},
		{"B1: --global then --comment -l", `git config --global --comment -l core.fsmonitor x`, []string{verb("config")}},
		{"B1: -f with an attached value", `git config -f.git/logs/../config core.fsmonitor x`, []string{verb("config")}},
		{"B1: -f takes -l as its value", `git config -f -l x.y z`, []string{verb("config")}},
		{"B1: --file takes --get as its value", `git config --file --get x.y z`, []string{verb("config")}},
		{"B1: -fl is -f with the value l", `git config -fl x.y z`, []string{verb("config")}},
		{"B1: --comment=-l, the control", `git config --comment=-l core.fsmonitor x`, []string{verb("config")}},
		{"B2: protocol.allow and an ext:: remote URL", `git -c protocol.allow=always -c 'remote.o.url=ext::sh -c touch% PROOF1' fetch o`, []string{gitConfig("-c protocol.allow=always"), gitConfig("-c remote.o.url=ext::sh -c touch% PROOF1")}},
		{"B2: an ext:: insteadOf key", `git -c 'url.ext::sh -c touch% PROOF2 #.insteadOf=foo:' fetch foo:x`, []string{gitConfig("-c url.ext::sh -c touch% PROOF2 #.insteadOf=foo:")}},
		{"B2: protocol.allow and an ext:: URL operand", `git -c protocol.allow=always fetch 'ext::sh -c x'`, []string{gitConfig("-c protocol.allow=always"), transport("ext::sh -c x")}},
		{"B2: an ext:: insteadOf for https", `git -c url.ext::sh.insteadOf=https:// fetch https://x`, []string{gitConfig("-c url.ext::sh.insteadOf=https://")}},
		{"B3: --reason abbreviated to --reas", `git worktree add --lock --reas r docs/x`, []string{"/w/docs/x"}},
		{"B3: --reason abbreviated to --reaso", `git worktree add --lock --reaso r docs/x`, []string{"/w/docs/x"}},
		{"N-a: checkout -qf -b", `git checkout -qf -b x`, []string{"/w"}},
		{"N-a: checkout -fq -b", `git checkout -fq -b x`, []string{"/w"}},
		{"N-a: checkout -bfeat is a branch named feat", `git checkout -bfeat`, nil},

		// Rule 8: reads stay allowed.
		{"rule 8: ls -la", `ls -la docs`, nil},
		{"rule 8: cat", `cat web/x.ts`, nil},
		{"rule 8: echo to a file", `echo hi > docs/a.md`, []string{"/w/docs/a.md"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := WriteTargets("/w", c.command); !slices.Equal(got, c.want) {
				t.Errorf("WriteTargets(%q) = %q, want %q", c.command, got, c.want)
			}
		})
	}
}

func refused(c string) string { return unreadablePrefix + c + ">" }

func pathspec(p string) string { return pathspecPrefix + p + ">" }

func fromList(o string) string { return fromFilePrefix + o + ">" }

func alias(n string) string { return aliasPrefix + n + ">" }

func verb(s string) string { return gitVerbPrefix + s + ">" }

func gitConfig(s string) string { return gitConfigPrefix + s + ">" }

func variable(s string) string { return gitVariablePrefix + s + ">" }

func grepPager(s string) string { return grepPrefix + s + ">" }

func transport(s string) string { return transportPrefix + s + ">" }

func TestWriteTargetsGitProgramKeys(t *testing.T) {
	for _, kv := range []string{
		"include.path=/tmp/c", "includeIf.gitdir:/r/.path=/tmp/c", "core.hooksPath=/tmp/h",
		"core.sshCommand=x", "protocol.ext.allow=always", "core.editor=x", "sequence.editor=x",
		"diff.external=x", "merge.tool=x", "mergetool.x.cmd=y", "difftool.x.cmd=y",
		"filter.x.smudge=y", "credential.helper=x", "Core.HooksPath=/tmp/h", "protocol.allow=always",
		"protocol.ext.allow=always", "credential.https://h.helper=x", "remote.o.uploadpack=x", "gpg.ssh.program=x",
	} {
		got := WriteTargets("/w", "git -c "+kv+" status")
		if want := []string{gitConfig("-c " + kv)}; !slices.Equal(got, want) {
			t.Errorf("git -c %s: got %q, want %q", kv, got, want)
		}
	}
}

func TestWriteTargetsWithoutCwd(t *testing.T) {
	got := WriteTargets("", `touch a /abs/b`)
	if want := []string{"a", "/abs/b"}; !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRefusalNamesWhatItCouldNotRead(t *testing.T) {
	for target, want := range map[string]string{
		fromList("--pathspec-from-file=l"): "with `--pathspec-from-file=l` git reads the paths from a file or standard input",
		alias("x"):                         "`x` is a git alias, which the lease walk does not read; run the git verb directly",
		verb("x"):                          "`git x` is not a git verb the lease walk reads",
		gitConfig("-c core.hooksPath=h"):   "`-c core.hooksPath=h` is config that names a program or a file",
		variable("GIT_SSH_COMMAND"):        "`GIT_SSH_COMMAND` is set beside git, and a GIT_ variable can carry a command or config",
		grepPager("-Ovim"):                 "with `-Ovim` git grep runs a command on the matched files",
		transport("--upload-pack=x"):       "with `--upload-pack=x` git runs a command to reach the other repository",
		pathspec("out/../a"):               "so `out/../a` cannot be placed",
		refused("$"):                       "the command holds `$`",
	} {
		if got, ok := refusal(target); !ok || !strings.Contains(got, want) {
			t.Errorf("refusal(%q) = %q, %v; want it to hold %q", target, got, ok, want)
		}
	}
}
