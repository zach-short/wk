// Command wk makes warm git workspaces for parallel agents.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/daemon"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/hive"
	"github.com/zach-short/wk/internal/setup"
	"github.com/zach-short/wk/internal/slot"
	"github.com/zach-short/wk/internal/viz"
	"github.com/zach-short/wk/internal/wk"
)

const usage = `usage:
  wk version
  wk [--repo <absolute dir>]
  wk [--repo <absolute dir>] setup --yes [--print]
  wk [--repo <absolute dir>] template refresh
  wk [--repo <absolute dir>] new <row> [--sparse|--full]
  wk [--repo <absolute dir>] gc [--apply|--json]
  wk [--repo <absolute dir>] gc schedule install|uninstall
  wk [--repo <absolute dir>] viz [--addr <host:port>]
  wk [--repo <absolute dir>] hive
  wk [--repo <absolute dir>] pool fill|status
  wk [--repo <absolute dir>] hooks install|uninstall
  wk [--repo <absolute dir>] claim <row>|--none
  wk [--repo <absolute dir>] lease add <path>...
  wk [--repo <absolute dir>] release
  wk [--repo <absolute dir>] status
  wk [--repo <absolute dir>] config show
  wk [--repo <absolute dir>] submit -m <message> [--row <n>] [--review <ref>]
  wk [--repo <absolute dir>] sync
  wk [--repo <absolute dir>] land [--loop | --apply --expect <hash>]
  wk hook worktree-create|worktree-remove|subagent-stop|pre-tool-use < <Claude Code hook input>
  wk hook lease --repo <absolute main checkout> < <Claude Code hook input>
  wk run [--slot <class>] [--keep] [--trim <path>]... -- <command> [args...]
  wk slots [--json]
  wk stats [--since YYYY-MM-DD] [--until YYYY-MM-DD] [--json]

wk alone, in a terminal, opens setup: it reads the repository, writes its config and warms the
template. Once the config exists it opens a home screen instead, and never overwrites the file.
wk setup --yes is that setup with no terminal and no questions: it writes the config with each
option the guided setup starts on, and nothing else. It prints the config's path on stdout and
each choice it made on stderr, and exits 1 without writing when the config exists. --print writes
nothing and prints the config on stdout. It builds no template and installs no hook or schedule.

The repository is the one holding --repo, or the current directory. Config is read from
$WK_HOME/<repo>.toml, where WK_HOME defaults to ~/.wk. wk new prints the workspace's absolute
path on stdout and everything else on stderr. Without a flag it follows workspace.sparse;
--sparse and --full override it.

wk gc lists every worktree with its verdict, then every orphan branch gc owns, and changes
nothing; --apply removes the removable worktrees and deletes the removable branches; --json
prints the worktree verdicts as one JSON document for tools.
wk gc schedule install writes and loads a launchd agent that runs wk gc --apply every
gc.apply_interval_minutes; uninstall unloads and deletes it.

wk viz serves the hive page on --addr (default 127.0.0.1:8787). The page reads the live
verdicts on every load. Its remove button removes one dead worktree, judged again first, and
nothing else.
wk hive draws the same hive in the terminal and reads the verdicts again every 3 seconds.
Arrow keys select a cell, q quits. Like wk gc without --apply, it changes nothing.

wk new takes a ready pool slot when there is one and then tops the pool up in the background
(pool.size full and pool.sparse_size sparse slots); wk pool fill does the topping up in the
foreground and wk pool status lists the slots. Its log is $WK_HOME/<repo>/pool.log.
wk hooks install adds the WorktreeCreate, WorktreeRemove, SubagentStop and PreToolUse hooks to
the main checkout's and the template's .claude/settings.local.json, with a wrapped twin of each
permission rule that covers a gate of $WK_HOME/<repo>.slots.toml, and keeps every other key;
uninstall removes only those entries and twins.
wk hook worktree-create reads Claude Code's JSON on stdin, makes a warm worktree, falls back to
a plain git worktree when that fails, and prints the path. wk hook subagent-stop marks a
stopped subagent's own worktree for gc, and never blocks the stop. wk hook worktree-remove
removes a worktree the owner discarded. Their log is $WK_HOME/<repo>/hook.log.
wk hook pre-tool-use routes each gate of a Bash command listed in $WK_HOME/<repo>.slots.toml
through wk run --slot, and always exits 0; its misses and errors go to missed.log and
errors.log in the slots directory.

wk claim reads row <row>'s "Files it owns" cell from leases.board, prints every token it cannot
read, and leases the paths it read to the workspace holding the directory; it refuses a path
another workspace leases and names that workspace's row. --none claims no path, for a session
that does not edit. wk lease add widens the claim, wk release drops it, and wk status lists
every claim. The store is $WK_HOME/<repo>/leases.json.
wk config show prints the effective config of $WK_HOME/<repo>.toml as TOML, and whether the
file was found.

wk submit, in a workspace with landing.enabled true, writes the working tree as a changeset
(the row from the claim, or --row), appends it to $WK_HOME/<repo>/queue.jsonl, prints its id
as the only line on stdout, and starts the landing worker. wk land --loop is that worker: one
at a time under landing.lock, it lands every submitted changeset onto landing.ref in order by
compare-and-swap, refuses one that conflicts, and exits when the queue is empty; its log is
$WK_HOME/<repo>/landing.log. With [[gates.run]] in the config, it gates each batch of up to
gates.batch_max changesets once in the trial at gates.trial_dir, lands a green batch, marks the
changeset a red gate traces to failed, and fast-forwards and pushes the base branch when
landing.ff_main and landing.push are true; a skipped gate never counts as a pass. wk sync moves the workspace to the landing tip with its work
carried across; a conflict leaves git's markers in the tree. wk land prints the landing tip
against the base branch and the dirty paths in the main checkout that block a fast-forward;
wk land --apply --expect <hash> fast-forwards the base branch to the landing ref while it
still holds <hash>, then pushes it to landing.remote when landing.push is true. wk status
lists every changeset after the leases, the last gate run, and why the base branch did not move.
wk hook lease judges one Edit, Write, MultiEdit, NotebookEdit or Bash call against the leases
when leases.enabled is true. It prints Claude Code's deny on stdout, logs each deny to
$WK_HOME/<repo>/hook.log, and always exits 0. wk hooks install adds its two PreToolUse entries
only while leases.enabled is true.

wk run runs the command after -- with its input, output, signals and exit status unchanged.
With --slot it first takes a place of that class from $WK_HOME/machine.toml, waiting in
arrival order while every place is held; without that file, or on any failure of its own, it
runs the command unslotted with one line on stderr. Each --trim path, relative to the git top
level, is removed after the command ends, except with --keep and in the main checkout, the
template or a pool slot.
wk slots lists each class's places, waiters and last runs, and the last day of missed.log and
errors.log; --json prints the same as one JSON document.
wk stats reports from wk's own logs how the places and the warm creates performed between two
local dates, both included (every date by default): per class and per command, the runs, nonzero
exits, wait and run times; how the waits overlapped; the waits over 10 minutes and the commands
that held the places during them; commands issued again in the same directory within 10 minutes
of the last one ending; missed.log by reason and the hook's deadline misses in errors.log; and
each repository's warm creates in hook.log, from a pool slot or the template as the warm line
says, or, for a line older than 2026-10-26 that names no source, unattributed when another create
overlapped it. --json prints the
same as one JSON document, with every row the text leaves out.
`

// version is set at release time with -ldflags "-X main.version=<tag>".
var version = "dev"

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "version" || args[0] == "--version") {
		_, _ = fmt.Fprintln(stdout, "wk", version)
		return exitOK
	}
	if rest := skipRepoFlag(args); len(rest) > 0 && (rest[0] == "run" || isPreToolUse(rest)) {
		return runInFront(rest, stdout, stderr)
	}
	if len(args) >= 2 && args[0] == "hook" && args[1] == "lease" {
		return runLeaseHook(args[2:], stdout)
	}
	repoDir, rest, err := splitRepoFlag(args)
	if err != nil || (len(rest) == 0 && !interactive()) {
		return fail(stderr, err, exitUsage)
	}
	home, err := wkHome()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	switch {
	case len(rest) == 0:
		return runSetup(home, repoDir, stderr)
	case rest[0] == "setup":
		return runSetupYes(home, repoDir, rest[1:], stdout, stderr)
	case len(rest) == 2 && rest[0] == "template" && rest[1] == "refresh":
		env, err := wk.Open(home, repoDir, stderr)
		if err != nil {
			return fail(stderr, err, exitError)
		}
		return fail(stderr, env.RefreshTemplate(), exitError)
	case rest[0] == "new":
		row, checkout, err := parseNew(rest[1:])
		if err != nil {
			return fail(stderr, err, exitUsage)
		}
		env, err := wk.Open(home, repoDir, stderr)
		if err != nil {
			return fail(stderr, err, exitError)
		}
		env.Refill = spawnFill(env)
		path, err := env.New(row, checkout)
		if err != nil {
			return fail(stderr, err, exitError)
		}
		_, _ = fmt.Fprintln(stdout, path)
		return exitOK
	case rest[0] == "gc":
		return runGC(home, repoDir, rest[1:], stdout, stderr)
	case rest[0] == "viz":
		return runViz(home, repoDir, rest[1:], stderr)
	case rest[0] == "hive":
		return runHive(home, repoDir, rest[1:], stderr)
	case len(rest) == 2 && rest[0] == "pool":
		return runPool(home, repoDir, rest[1], stdout, stderr)
	case len(rest) == 2 && rest[0] == "hooks":
		return runHooks(home, repoDir, rest[1], stdout, stderr)
	case rest[0] == "slots":
		return runSlots(home, rest[1:], stdout, stderr)
	case rest[0] == "stats":
		return runStats(home, rest[1:], stdout, stderr)
	case rest[0] == "claim" || rest[0] == "lease" || rest[0] == "release" || rest[0] == "status":
		return runLeases(home, repoDir, rest, stdout, stderr)
	case rest[0] == "submit":
		return runSubmit(home, repoDir, rest[1:], stdout, stderr)
	case len(rest) == 1 && rest[0] == "sync":
		return runLanding(home, repoDir, stderr, func(env *wk.Env) error { return env.Sync(repoDir, stdout) })
	case rest[0] == "land":
		return runLand(home, repoDir, rest[1:], stdout, stderr)
	case len(rest) == 2 && rest[0] == "config" && rest[1] == "show":
		return runConfigShow(home, repoDir, stdout, stderr)
	case len(rest) == 2 && rest[0] == "hook" && rest[1] == "worktree-create":
		return runWorktreeCreate(home, stdout, stderr)
	case len(rest) == 2 && rest[0] == "hook" && rest[1] == "worktree-remove":
		what, err := wk.WorktreeRemove(home, stdin)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "wk hook: %v\n", err)
			return exitError
		}
		_, _ = fmt.Fprintln(stderr, "wk hook: "+what)
		return exitOK
	case len(rest) == 2 && rest[0] == "hook" && rest[1] == "subagent-stop":
		// Exit 2 from a SubagentStop hook would keep the subagent running, so this one always
		// exits 0 and prints nothing on stdout, which Claude Code would read as a decision.
		_ = wk.SubagentStop(home, stdin)
		return exitOK
	}
	return fail(stderr, nil, exitUsage)
}

// stdin is swapped by the self-test, which feeds the hook its JSON.
var stdin io.Reader = os.Stdin

// runWorktreeCreate exits 2 on failure, which Claude Code reads as a blocking hook error, and
// prints nothing on stdout then, because any stdout line would be taken as the path.
func runWorktreeCreate(home string, stdout, stderr io.Writer) int {
	path, err := wk.WorktreeCreate(home, stdin, stderr, func(env *wk.Env) { env.Refill = spawnFill(env) })
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "wk hook: %v\n", err)
		return exitUsage
	}
	_, _ = fmt.Fprintln(stdout, path)
	return exitOK
}

func runPool(home, repoDir, what string, stdout, stderr io.Writer) int {
	if what != "fill" && what != "status" {
		return fail(stderr, fmt.Errorf("unexpected argument to wk pool: %q", what), exitUsage)
	}
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	if what == "status" {
		return fail(stderr, env.PoolStatus(stdout), exitError)
	}
	_, _ = fmt.Fprintf(stderr, "wk pool fill at %s\n", time.Now().Format(time.RFC3339))
	return fail(stderr, env.FillPool(), exitError)
}

func runHooks(home, repoDir, what string, stdout, stderr io.Writer) int {
	if what != "install" && what != "uninstall" {
		return fail(stderr, fmt.Errorf("unexpected argument to wk hooks: %q", what), exitUsage)
	}
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	if what == "uninstall" {
		return fail(stderr, env.HooksUninstall(stdout), exitError)
	}
	exe, err := installedExe()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	return fail(stderr, env.HooksInstall(exe, stdout), exitError)
}

// runLeaseHook is wk hook lease. It exits 0 on every path, a panic included: the hook denies by
// printing JSON, and exit 2 would block a call by accident (owner amendments, 2026-10-16). A
// panic outside wk.LeaseHook prints the deny, because the guard fails closed (audit 1, N1).
func runLeaseHook(args []string, stdout io.Writer) (code int) {
	defer func() {
		if r := recover(); r != nil {
			wk.LeaseDeny(stdout, fmt.Sprintf("wk: the lease hook failed (%v); report it", r))
			code = exitOK
		}
	}()
	home, err := wkHome()
	if err != nil {
		home = os.Getenv("WK_HOME")
	}
	wk.LeaseHook(home, args, stdin, stdout)
	return exitOK
}

// runLeases serves claim, lease add, release and status for the workspace holding repoDir.
func runLeases(home, repoDir string, rest []string, stdout, stderr io.Writer) int {
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	switch {
	case len(rest) == 2 && rest[0] == "claim" && rest[1] == "--none":
		err = env.ClaimNone(repoDir, stdout)
	case len(rest) == 2 && rest[0] == "claim" && !strings.HasPrefix(rest[1], "-"):
		err = env.Claim(repoDir, rest[1], stdout)
	case len(rest) > 2 && rest[0] == "lease" && rest[1] == "add":
		err = env.LeaseAdd(repoDir, rest[2:], stdout)
	case len(rest) == 1 && rest[0] == "release":
		err = env.Release(repoDir, stdout)
	case len(rest) == 1 && rest[0] == "status":
		err = env.Status(stdout)
	default:
		return fail(stderr, fmt.Errorf("unexpected arguments: %q", strings.Join(rest, " ")), exitUsage)
	}
	return fail(stderr, err, exitError)
}

// runLanding opens the repository and runs one landing command through it.
func runLanding(home, repoDir string, stderr io.Writer, cmd func(*wk.Env) error) int {
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	return fail(stderr, cmd(env), exitError)
}

// runSubmit is wk submit: the id is the only line on stdout, and the worker is started after
// the line is appended, so a submit never waits on a landing.
func runSubmit(home, repoDir string, args []string, stdout, stderr io.Writer) int {
	flags, err := parseSubmit(args)
	if err != nil {
		return fail(stderr, err, exitUsage)
	}
	return runLanding(home, repoDir, stderr, func(env *wk.Env) error {
		if err := env.Submit(repoDir, flags["-m"], flags["--row"], flags["--review"], stdout); err != nil {
			return err
		}
		spawnLand(env, stderr)
		return nil
	})
}

// parseSubmit reads -m, --row and --review, each with one value that does not start with "-".
func parseSubmit(args []string) (map[string]string, error) {
	flags := map[string]string{}
	for i := 0; i < len(args); i += 2 {
		a := args[i]
		if (a != "-m" && a != "--row" && a != "--review") || i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
			return nil, fmt.Errorf("unexpected argument to wk submit: %q", strings.Join(args[i:], " "))
		}
		flags[a] = args[i+1]
	}
	if flags["-m"] == "" {
		return nil, errors.New("wk submit needs -m with a message")
	}
	return flags, nil
}

// runLand is wk land: the dry run alone, --loop for the worker, --apply --expect <hash> for
// the fast-forward. --apply without --expect is a usage error, never a move.
func runLand(home, repoDir string, args []string, stdout, stderr io.Writer) int {
	what := strings.Join(args, " ")
	switch {
	case what == "":
		return runLanding(home, repoDir, stderr, func(env *wk.Env) error { return env.Land(stdout) })
	case what == "--loop":
		return runLanding(home, repoDir, stderr, func(env *wk.Env) error {
			env.WKExe = slotExe()
			return env.LandLoop(stdout)
		})
	case len(args) == 3 && args[0] == "--apply" && args[1] == "--expect" && args[2] != "" && args[2][0] != '-':
		return runLanding(home, repoDir, stderr, func(env *wk.Env) error { return env.LandApply(args[2], stdout) })
	}
	return fail(stderr, fmt.Errorf("unexpected arguments to wk land: %q", what), exitUsage)
}

// spawnLand starts `wk land --loop` detached, as spawnFill does, with its output in
// landing.log. A test binary never forks itself; a failure to start is said, because the
// launchd timer runs the worker later and the changeset is already in the queue.
func spawnLand(env *wk.Env, stderr io.Writer) {
	exe, err := installedExe()
	if err != nil || wk.TransientBinary(exe) {
		return
	}
	if err := daemon.Spawn(exe, []string{"--repo", env.Repo.Primary, "land", "--loop"}, env.LandingLogPath()); err != nil {
		_, _ = fmt.Fprintf(stderr, "wk: the landing worker did not start (%v); the timer runs it, or run wk land --loop\n", err)
	}
}

// slotExe is the binary the worker routes a gate through with wk run --slot (PD-10), or ""
// for a go run build, whose path is gone once it exits; the gates then run as they are.
func slotExe() string {
	exe, err := installedExe()
	if err != nil || wk.TransientBinary(exe) {
		return ""
	}
	return exe
}

// runConfigShow prints the effective config, or why the file cannot be read (D26).
func runConfigShow(home, repoDir string, stdout, stderr io.Writer) int {
	repo, err := gitx.Resolve(repoDir)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	path := filepath.Join(home, repo.Name+".toml")
	cfg, found, err := config.Load(path)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	return fail(stderr, config.Show(stdout, cfg, path, found), exitError)
}

// skipRepoFlag drops a leading --repo and its value unchecked. wk run and wk hook pre-tool-use
// take no repository, and a bad flag or a deleted working directory must not stop either one.
func skipRepoFlag(args []string) []string {
	if len(args) >= 2 && args[0] == "--repo" {
		return args[2:]
	}
	return args
}

func isPreToolUse(rest []string) bool {
	return len(rest) == 2 && rest[0] == "hook" && rest[1] == "pre-tool-use"
}

// runInFront serves the two verbs that stand in front of every gate and every Bash call, so wk's
// own state may not stop them: a bad WK_HOME goes to internal/slot as it is, where wk run fails
// open on it (SLOTS-PLAN.md SB-9) and the hook answers nothing.
func runInFront(rest []string, stdout, stderr io.Writer) int {
	home, err := wkHome()
	if err != nil {
		home = os.Getenv("WK_HOME")
	}
	if rest[0] == "run" {
		return runSlot(home, rest[1:], stderr)
	}
	return runPreToolUse(home, stdout)
}

type runArgs struct {
	class string
	keep  bool
	trim  []string
	argv  []string
}

// runSlot is wk run. The command gets this process's own three files, never the writers run()
// is handed: slot.Run passes *os.File through untouched and would copy anything else through a
// pipe. A bad argument list exits 2, which is a red gate, never a green one (SB-8).
func runSlot(home string, args []string, stderr io.Writer) int {
	ra, err := parseRun(args)
	if err != nil {
		return fail(stderr, err, exitUsage)
	}
	dir, err := os.Getwd()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	opts := slot.RunOptions{
		Class: ra.class, Trim: ra.trim, Keep: ra.keep, Dir: dir, Home: home,
		Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Env: os.Environ(),
	}
	opts.TrimAllowed = len(ra.trim) > 0 && !ra.keep && wk.TrimAllowed(home, dir)
	return slot.Run(opts, ra.argv)
}

func parseRun(args []string) (runArgs, error) {
	var ra runArgs
	for i := 0; i < len(args); i++ {
		switch a := args[i]; {
		case a == "--":
			ra.argv = args[i+1:]
			return ra, ra.checkCommand()
		case a == "--keep":
			ra.keep = true
		case (a == "--slot" || a == "--trim") && i+1 < len(args):
			i++
			if err := ra.set(a, args[i]); err != nil {
				return ra, err
			}
		default:
			return ra, fmt.Errorf("unexpected argument to wk run: %q", a)
		}
	}
	return ra, errors.New("wk run needs -- before the command")
}

// set refuses a value that starts with "-", because a missing value would otherwise swallow the
// next flag or the --, and the run would go unslotted or untrimmed with one easily missed line.
func (ra *runArgs) set(flag, value string) error {
	switch {
	case value == "" || strings.HasPrefix(value, "-"):
		return fmt.Errorf("%s needs a value, not %q", flag, value)
	case flag == "--trim":
		ra.trim = append(ra.trim, value)
	case ra.class != "":
		return errors.New("--slot is given twice")
	default:
		ra.class = value
	}
	return nil
}

func (ra runArgs) checkCommand() error {
	if len(ra.argv) == 0 {
		return errors.New("wk run needs a command after --")
	}
	return nil
}

func runSlots(home string, args []string, stdout, stderr io.Writer) int {
	asJSON := len(args) == 1 && args[0] == "--json"
	if len(args) > 0 && !asJSON {
		return fail(stderr, fmt.Errorf("unexpected arguments to wk slots: %q", strings.Join(args, " ")), exitUsage)
	}
	st, err := slot.ReadStatus(home)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	if !asJSON {
		_, _ = fmt.Fprint(stdout, st.Text())
		return exitOK
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return fail(stderr, enc.Encode(st), exitError)
}

func runStats(home string, args []string, stdout, stderr io.Writer) int {
	r, asJSON, err := parseStats(args)
	if err != nil {
		return fail(stderr, err, exitUsage)
	}
	st, err := wk.ReadStats(home, r)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	if !asJSON {
		_, _ = fmt.Fprint(stdout, st.Text())
		return exitOK
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	return fail(stderr, enc.Encode(st), exitError)
}

func parseStats(args []string) (r slot.Range, asJSON bool, err error) {
	dates := map[string]string{}
	seen := map[string]bool{} // a given empty value still counts, so it cannot be given twice
	for i := 0; i < len(args); i++ {
		switch flag := args[i]; {
		case flag == "--json" && !asJSON:
			asJSON = true
		case (flag == "--since" || flag == "--until") && !seen[flag] && i+1 < len(args):
			seen[flag] = true
			i++
			dates[flag] = args[i]
		default:
			return slot.Range{}, false, fmt.Errorf("unexpected argument to wk stats: %q", flag)
		}
	}
	r, err = slot.ParseRange(dates["--since"], dates["--until"])
	return r, asJSON, err
}

// runPreToolUse is wk hook pre-tool-use. It exits 0 on every path, a panic included: exit 2 from
// a PreToolUse hook blocks the Bash call it stands in front of (SB-14).
func runPreToolUse(home string, stdout io.Writer) (code int) {
	defer func() {
		if recover() != nil {
			code = exitOK
		}
	}()
	exe, _ := installedExe()
	slot.HookPreToolUse(stdin, stdout, slot.HookDeps{Exe: exe, Home: home, LogDir: slotsLogDir(home)})
	return exitOK
}

// slotsLogDir puts the hook's missed.log and errors.log beside wk run's logs, where wk slots
// counts them. A relative home logs nowhere, never into the session's working directory.
func slotsLogDir(home string) string {
	if !filepath.IsAbs(home) {
		return ""
	}
	if m, found, err := slot.LoadMachine(home); found && err == nil {
		return m.Dir
	}
	return filepath.Join(home, "slots")
}

// spawnFill starts `wk pool fill` in a session of its own, from /, with its output in
// pool.log: it outlives the wk new or hook that started it, nobody waits on it, and its cwd is
// inside no worktree. A test binary never forks itself.
func spawnFill(env *wk.Env) func() {
	return func() {
		exe, err := installedExe()
		if err != nil || wk.TransientBinary(exe) {
			return
		}
		log, err := os.OpenFile(env.PoolLogPath(), os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
		if err != nil {
			return
		}
		defer func() { _ = log.Close() }()
		cmd := exec.Command(exe, "--repo", env.Repo.Primary, "pool", "fill")
		cmd.Dir = "/"
		cmd.Stdout, cmd.Stderr = log, log
		cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
		if cmd.Start() == nil {
			_ = cmd.Process.Release()
		}
	}
}

func installedExe() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return "", err
	}
	return wk.StablePath(exe), nil
}

// launchd is swapped by the self-test, so no test loads an agent into the owner's session.
var launchd = realLaunchd

func realLaunchd() (wk.Launchd, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return wk.Launchd{}, err
	}
	return wk.Launchd{
		AgentsDir: filepath.Join(home, "Library", "LaunchAgents"),
		Domain:    fmt.Sprintf("gui/%d", os.Getuid()),
		Control: func(args ...string) error {
			out, err := exec.Command("launchctl", args...).CombinedOutput()
			if err != nil {
				return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
			}
			return nil
		},
	}, nil
}

func runGC(home, repoDir string, args []string, stdout, stderr io.Writer) int {
	what := strings.Join(args, " ")
	switch what {
	case "", "--apply", "--json", "schedule install", "schedule uninstall":
	default:
		return fail(stderr, fmt.Errorf("unexpected arguments to wk gc: %q", what), exitUsage)
	}
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	switch what {
	case "":
		return fail(stderr, env.GC(stdout), exitError)
	case "--apply":
		return fail(stderr, env.GCApply(stdout), exitError)
	case "--json":
		return fail(stderr, env.GCJSON(stdout), exitError)
	}
	l, err := launchd()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	if what == "schedule uninstall" {
		return fail(stderr, env.ScheduleUninstall(l, stdout), exitError)
	}
	return fail(stderr, scheduleInstall(env, l, stdout), exitError)
}

func scheduleInstall(env *wk.Env, l wk.Launchd, out io.Writer) error {
	exe, err := installedExe()
	if err != nil {
		return err
	}
	return env.ScheduleInstall(exe, l, out)
}

// interactive is true when both ends are a terminal, so a bare wk may draw setup. A pipe or a
// test gets the usage text, as before setup existed. The self-test swaps it.
var interactive = func() bool {
	for _, f := range []*os.File{os.Stdin, os.Stdout} {
		info, err := f.Stat()
		if err != nil || info.Mode()&os.ModeCharDevice == 0 {
			return false
		}
	}
	return true
}

// runSetup opens setup for the repository holding repoDir: the wizard when it has no config,
// the home screen when it has one. Leaving with "Open the hive" opens wk hive.
func runSetup(home, repoDir string, stderr io.Writer) int {
	plan, err := setupPlan(home, repoDir)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	acts := setup.Actions{Warm: func(log io.Writer) error {
		env, err := wk.Open(home, plan.Primary, log)
		if err != nil {
			return err
		}
		return env.RefreshTemplate()
	}}
	if runtime.GOOS == "darwin" {
		acts.Schedule = func(log io.Writer) error {
			env, err := wk.Open(home, plan.Primary, log)
			if err != nil {
				return err
			}
			l, err := launchd()
			if err != nil {
				return err
			}
			return scheduleInstall(env, l, log)
		}
	}
	final, err := tea.NewProgram(setup.New(plan, acts), tea.WithAltScreen()).Run()
	if err != nil {
		return fail(stderr, err, exitError)
	}
	if final.(setup.Model).Result() == setup.OpenHive {
		return runHive(home, plan.Primary, nil, stderr)
	}
	return exitOK
}

// runSetupYes is the guided setup with its defaults taken and no terminal: the config is the one
// the wizard would write with every key left alone. It never overwrites a config. The written
// path is the only line on stdout, as with wk new; the choices go to stderr.
func runSetupYes(home, repoDir string, args []string, stdout, stderr io.Writer) int {
	printOnly, err := parseSetup(args)
	if err != nil {
		return fail(stderr, err, exitUsage)
	}
	repo, err := gitx.Resolve(repoDir)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	path := filepath.Join(home, repo.Name+".toml")
	choices := setup.Defaults(setup.Detect(repo.Primary))
	text := setup.Render(repo.Name, choices)
	if printOnly {
		_, _ = fmt.Fprint(stdout, text)
		return exitOK
	}
	if err := setup.Write(path, text); err != nil {
		return fail(stderr, err, exitError)
	}
	for _, l := range setup.Describe(choices) {
		_, _ = fmt.Fprintln(stderr, "wk setup: "+l)
	}
	_, _ = fmt.Fprintln(stdout, path)
	return exitOK
}

// parseSetup reports whether --print was given. wk setup needs --yes or --print: the guided
// setup is a bare wk.
func parseSetup(args []string) (printOnly bool, err error) {
	var yes bool
	for _, a := range args {
		switch a {
		case "--yes":
			yes = true
		case "--print":
			printOnly = true
		default:
			return false, fmt.Errorf("unexpected argument to wk setup: %q", a)
		}
	}
	if !yes && !printOnly {
		return false, fmt.Errorf("wk setup needs --yes or --print; the guided setup is a bare wk")
	}
	return printOnly, nil
}

// setupPlan reads what setup starts from. The repository is only read for the wizard; the home
// screen needs the config alone.
func setupPlan(home, repoDir string) (setup.Plan, error) {
	repo, err := gitx.Resolve(repoDir)
	if err != nil {
		return setup.Plan{}, err
	}
	path := filepath.Join(home, repo.Name+".toml")
	cfg, found, loadErr := config.Load(path)
	plan := setup.Plan{Repo: repo.Name, Primary: repo.Primary, ConfigPath: path, Existing: found, LoadErr: loadErr}
	if !found {
		plan.Found = setup.Detect(repo.Primary)
		return plan, nil
	}
	if loadErr == nil {
		installs := make([]string, len(cfg.Template.PostClone))
		for i, c := range cfg.Template.PostClone {
			installs[i] = strings.Join(c.Run, " ")
		}
		plan.Summary = []setup.Fact{
			{Label: "base", Value: cfg.Workspace.Base},
			{Label: "template", Value: cfg.Template.Path},
			{Label: "clones", Value: orNone(strings.Join(cfg.Template.CloneDirs, ", "))},
			{Label: "copies", Value: orNone(strings.Join(cfg.Template.CopyFiles, ", "))},
			{Label: "installs", Value: orNone(strings.Join(installs, "; "))},
			{Label: "workspaces", Value: cfg.Workspace.Dir},
		}
	}
	return plan, nil
}

func orNone(s string) string {
	if s == "" {
		return "none"
	}
	return s
}

const defaultVizAddr = "127.0.0.1:8787"

// runViz serves the hive page until the process stops. Its report is built per request, so the
// page never shows a stale snapshot.
func runViz(home, repoDir string, args []string, stderr io.Writer) int {
	addr := defaultVizAddr
	switch {
	case len(args) == 0:
	case len(args) == 2 && args[0] == "--addr" && args[1] != "":
		addr = args[1]
	default:
		return fail(stderr, fmt.Errorf("unexpected arguments to wk viz: %q", strings.Join(args, " ")), exitUsage)
	}
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	_, _ = fmt.Fprintf(stderr, "wk viz: http://%s (Ctrl-C stops it)\n", addr)
	srv := &http.Server{Addr: addr, Handler: viz.Handler(env.GCJSON, env.RemoveDead), ReadHeaderTimeout: 5 * time.Second}
	return fail(stderr, srv.ListenAndServe(), exitError)
}

const hiveEvery = 3 * time.Second

// runHive draws the terminal hive until q. It reads through hiveLoader only, which is the dry
// run, so the view cannot remove anything.
func runHive(home, repoDir string, args []string, stderr io.Writer) int {
	if len(args) != 0 {
		return fail(stderr, fmt.Errorf("unexpected arguments to wk hive: %q", strings.Join(args, " ")), exitUsage)
	}
	env, err := wk.Open(home, repoDir, stderr)
	if err != nil {
		return fail(stderr, err, exitError)
	}
	_, err = tea.NewProgram(hive.New(hiveLoader(env), hiveEvery), tea.WithAltScreen()).Run()
	return fail(stderr, err, exitError)
}

// hiveLoader reads the report the page reads, through the same JSON, so both views keep one
// contract.
func hiveLoader(env *wk.Env) hive.Loader {
	return func() (wk.Report, error) {
		var buf bytes.Buffer
		if err := env.GCJSON(&buf); err != nil {
			return wk.Report{}, err
		}
		var r wk.Report
		err := json.Unmarshal(buf.Bytes(), &r)
		return r, err
	}
}

// fail prints err and returns code, or returns exitOK when err is nil and code is not a usage
// error. A usage error always prints the usage text.
func fail(stderr io.Writer, err error, code int) int {
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "wk: %v\n", err)
	}
	if code == exitUsage {
		_, _ = fmt.Fprint(stderr, usage)
		return code
	}
	if err == nil {
		return exitOK
	}
	return code
}

func splitRepoFlag(args []string) (string, []string, error) {
	if len(args) >= 2 && args[0] == "--repo" {
		if !filepath.IsAbs(args[1]) {
			return "", nil, fmt.Errorf("--repo %q is relative; wk takes absolute paths only", args[1])
		}
		return args[1], args[2:], nil
	}
	cwd, err := os.Getwd()
	return cwd, args, err
}

func parseNew(args []string) (row string, checkout wk.Checkout, err error) {
	for _, a := range args {
		switch {
		case (a == "--sparse" || a == "--full") && checkout != wk.DefaultCheckout:
			return "", 0, fmt.Errorf("--sparse and --full exclude each other")
		case a == "--sparse":
			checkout = wk.SparseCheckout
		case a == "--full":
			checkout = wk.FullCheckout
		case row == "" && len(a) > 0 && a[0] != '-':
			row = a
		default:
			return "", 0, fmt.Errorf("unexpected argument %q", a)
		}
	}
	if row == "" {
		return "", 0, fmt.Errorf("wk new needs a row")
	}
	return row, checkout, nil
}

func wkHome() (string, error) {
	if h := os.Getenv("WK_HOME"); h != "" {
		if !filepath.IsAbs(h) {
			return "", fmt.Errorf("WK_HOME %q is relative; wk takes absolute paths only", h)
		}
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".wk"), nil
}
