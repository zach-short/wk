# Row 7, pass 2 audit (auditor 4), Opus 5.5, agent a844296530c0656a8

Launched 2026-10-16 22:50 EDT, returned 2026-10-16 22:58 EDT. Worktree `.claude/worktrees/ap-row-7-audit-4`, branch `ap/row-7-audit-4`, fast-forwarded to `f9ee256` (merge-base `a692538`).

## Verdict: blocking findings

B1 (blocking, a claimed form). A quote in a heredoc body pairs with a quote after the body, so the commands between them are never walked. The splitter (`internal/lease/bash.go:781-815`, `step`/`quote`) tokenizes heredoc bodies as shell text. An apostrophe in the body opens a quote that closes at the next `'` after the delimiter, and everything in between becomes one inert word. Scratch test, roots /r/a (row 1: web/, backend/routes/routes.go) and /r/b (row 2: docs/), cwd /r/b:

- `cat <<EOF > docs/a.md` NL `don't` NL `EOF` NL `rm web/x.ts # it's gone` gives ALLOW, targets `[/r/b/docs/a.md]`.
- The same body, then `rm web/x.ts`, then a second heredoc whose body holds `won't` gives ALLOW.
- The same with `"` (`say "hi` ... `touch web/x.ts` ... `there"`) gives ALLOW.

Real-shell check: the first form, with `touch hw` in place of `rm`, in /bin/bash 3.2.57 and zsh 5.9, created `hw`. The rm runs. The item file's over-refusal line ("an apostrophe in a heredoc body is an open quote, which refuses the command") holds only when the total quote count is odd. An apostrophe in prose is the common case for an agent that writes docs through a heredoc. Rule 3 claims heredoc bodies, and the hidden write is a plain `rm`.

## Non-blocking findings

- N1. `cd web; cd ..; rm docs/a.md` is DENIED, but the brief expected allowed. Targets are `/r/b/docs/a.md`, `/r/b/web/docs/a.md` and `/r/docs/a.md`; the deny names row 1. Rule 4 applied literally: the set keeps `/r/b/web` because the `cd ..` "may not run". A loud over-refusal.
- N2. Gaps inside the safe set, silently allowed. Listed by the item file: `go fmt ./web/...`, `npx prettier --write web/x.ts`, `bunx prettier --write web/x.ts`, `git format-patch -o web HEAD~1`, `unzip -o a.zip -d web`. Not listed: `golangci-lint run --fix ./web/...`, `mkdir -p web/components`, `git merge-file web/x.ts a b`, `git submodule update --init`, `chown me web/x.ts`. The `prettier --write` entry is close to dead code (prettier runs through `bunx` or `npx`, neither a wrapper).
- N3. Four new stdlib imports in bash.go: `regexp`, `strconv`, `unicode`, `unicode/utf8`. `go.mod` and `go.sum` unchanged.
- N4. The builder rewrote the brief's git hand-back line in passoff/7 (line 22) to "commit on your own worktree branch only; the lead merges...". Lead's note: intended, owner amendment 6 (2026-10-16 22:23 EDT) asked for that sentence in row 7's file.
- N5. Design note for row 3, outside the walk: `cd ..; touch a/web/x.ts` from cwd /r/b is ALLOWED. The file's own root /r/a leases it, and `Judge` has no idea of the caller's root. Pass-1 `Judge` behavior.

## Reproduction

- `main` moved to a692538 since the brief. `git merge --ff-only main`: already up to date. `git merge --no-edit ap/row-7` fast-forwarded to f9ee256. `go mod download` exit 0.
- `git diff --stat main...HEAD`: 7 files, +1859/-1 (bash.go 846, bash_test.go 257, judge.go 154, judge_test.go 150, lease_test.go 149, store.go 173, passoff/7 131). Matches the builder's report. Name-only list is those 7 paths. go.mod/go.sum diff 0 lines. f9ee256 carries no attribution line.

## Gates

1. `GOTOOLCHAIN=local sh -c 'test -z "$(gofmt -l .)" && go vet ./... && go test -race -count=1 ./... && golangci-lint run ./...'`: tail `ok .../internal/viz 1.706s` / `ok .../internal/wk 61.060s` / `0 issues.`, exit 0.
2. `GOTOOLCHAIN=local go test -race -count=1 -v ./internal/lease/...`: 295 PASS, 0 FAIL, exit 0.
3. `go list -deps ./internal/lease | grep zach-short` prints only the lease package.

## Decision table (cwd /r/b)

- Rule 1: all DENY with "cannot tell ... holds `X`" for `$((1<<2))` ($), trailing `\`, `{web,docs}` ({), `docs/*/../../web/x.ts` (*), `$'a\'b'` ($), `${x//<<EOF/}` ($), backtick, `ls (docs)` ((), `printf 'é'` (é).
- Rule 3: all DENY naming row 1: `cat <<EOF |` ... `sh` (targets web/x.ts and `<standard input>`); `cat <<EOF`, `rm web/x.ts`, `EOF`, `echo done` (body walked); `cat <<<"text" > web/x.ts`.
- Rule 4: DENY naming row 1: `test -f docs/Makefile && cd docs; rm web/x.ts`, `cd nope; rm web/x.ts`, `cd docs/web 2>/dev/null || cd web; rm x.ts`, `cd web && rm x.ts`, `pushd web && rm x.ts`. ALLOW: `cd docs && rm a.md` (targets /r/b/a.md, /r/b/docs/a.md). `cd web; cd ..; rm docs/a.md` DENY (N1).
- Rules 5 and 6: DENY with web/x.ts held by row 1: `time -p`, `env FOO=1`, `FOO+=1`, `nohup`, `timeout 5`, `sudo -u me`, `exec -a x`, `eval '...'`, `unlink`, `sed -I ''`, `sh -c '...'`. DENY with ". is leased by row 1": `git checkout main`, `git stash pop`, `git reset --hard HEAD~1`. ALLOW: `git status`, `git log --oneline -5`, `git diff`, `ls -la docs`, `cat web/x.ts`, `go test ./...`, `echo hi > docs/a.md`.
- Hunt: 40 spellings in the safe set judged correctly (cp -t, --target-directory, install -Dm644, ln -sf, nice -n, /bin/rm, `"r"m we''b/x.ts`, dd of=, `2>&1>web/x.ts`, `exec 3<>`, `while ...; done > web/x.ts`, `cd -P`, `command cd --`, `if ...; then touch ...; fi`, `git -C ../a checkout --`, `git --no-pager -c core.pager=less restore`, `cd /r/a && git checkout --`, `git commit -m "it's done" && git checkout main`, `sed -e .. -i.bak`, `perl -pi -e`, `tar xf a.tar -C web`, `find web -name x.ts -delete`, `bash -o pipefail -c`, `xargs -n1 touch` refused). Wrong allows: B1 (3 inputs) and N2 (10 inputs).
- Shell probes (touch only, scratch temp dir): every builder claim holds. zsh `>!` writes under noclobber; `>>&` appends both streams; `f<1-9>` globs; `repeat 2`, `noglob`, `nocorrect` and `-` each run the command; bash 3.2 `source /dev/stdin <<<"touch g1"` created g1.

## Lean

bash.go is 846 lines (653 after pass 1). Cuts worth about 35 lines, each reopening only its own exotic spelling: zshRange regexp (about 6), `printable` via `%q` (about 6), dead `case` branch (2), `alias` path in `deferred` (about 4), `dd` (11), git `--namespace`/`--config-env`/`--super-prefix`/`--attr-source` (2), zsh `noglob`/`nocorrect`/`-`/`repeat`/`busybox`/`coproc` (about 1), shells mksh/fish/csh/tcsh (entries), zsh-only redirections (about 1), git plumbing checkout-index/read-tree/sparse-checkout/mergetool (about 1). Duplicated: the rule for which words expand is coded twice (`literal` line 113, `step` line 797). Keep the wrappers, verbs, `--merge`/`--keep`, bare `stash`, `worktree move`, `bisect`, `checkout -f`, `fromStdin`, CDPATH and maxDirs. L1 (6 to 15 lines): `sedFiles` 31, `walker.git` 28; `writtenBy` 24, `command` 23, `step` 23, `unwrap` 22. Pass-1 maximum 41 (`end`) is gone.

## Diff eb0c622..f9ee256

Exported surface identical. New imports as N3. `judge.go` changes only `judgeBash` and the new unexported `judgeWrite`. No vacuous test. Item file: no em dashes, no contractions; `## Held` (line 94) before `## Pass 2` (line 98); only the git line edit (N4).

Committed nothing; did not write the builder's worktree. A first attempt to write the probe-case file through a Bash heredoc containing `rm` text was denied; not retried; written with the Write tool instead. A PostToolUse hook flagged contractions in that scratch data file (the apostrophes are the test input).
