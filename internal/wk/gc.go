package wk

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/procs"
)

// TrialMarkerName is the file the land-worktrees trial.sh leaves in a trial worktree's private
// git directory (lib.sh trial_marker). The trial belongs to that skill, so gc leaves it alone.
const TrialMarkerName = "land-worktrees-trial"

type Verdict string

const (
	Removable Verdict = "removable"
	Kept      Verdict = "kept"
	// KeptForLanding is a worktree whose commits main already holds but whose tree has
	// changes: sorting those is the judgment /land-worktrees keeps (D9, option E2).
	KeptForLanding Verdict = "kept, run /land-worktrees"
	Skipped        Verdict = "skip"
	Prunable       Verdict = "prunable"
)

// Judgment is gc's answer for one worktree.
type Judgment struct {
	Worktree gitx.Worktree
	Verdict  Verdict
	Reasons  []string
	// DeleteBranch is BD-11: the branch goes with its worktree only when it is an ancestor of
	// the base. A branch absorbed by content alone is kept for /land-worktrees.
	DeleteBranch bool
	// Dirty counts the changed paths and Activity is the newest activity time gc read. Both
	// feed the --json report; a zero Activity means it could not be read.
	Dirty    int
	Activity time.Time
}

// survey is what gc reads once per pass and asks every worktree about.
type survey struct {
	base          string
	procs         []procs.Proc
	now           time.Time
	quiet         time.Duration
	stoppedQuiet  time.Duration
	unmarkedQuiet time.Duration
	primary       string
	template      string
}

func (e *Env) takeSurvey() (survey, error) {
	base, err := gitx.Commit(e.Repo.Primary, "refs/heads/"+e.Cfg.Workspace.Base)
	if err != nil {
		return survey{}, fmt.Errorf("base %q: %w", e.Cfg.Workspace.Base, err)
	}
	list, err := procs.Snapshot()
	if err != nil {
		return survey{}, err
	}
	return survey{
		base:          base,
		procs:         list,
		now:           e.clock(),
		quiet:         time.Duration(e.Cfg.GC.QuietMinutes) * time.Minute,
		stoppedQuiet:  time.Duration(e.Cfg.GC.StoppedQuietMinutes) * time.Minute,
		unmarkedQuiet: time.Duration(e.Cfg.GC.UnmarkedAgentQuietHours) * time.Hour,
		primary:       physical(e.Repo.Primary),
		template:      physical(e.TemplateDir()),
	}, nil
}

// GC judges every worktree but the main checkout and prints one line for each. It changes
// nothing.
func (e *Env) GC(out io.Writer) error {
	judgments, err := e.judgeAll()
	if err != nil {
		return err
	}
	removable := 0
	for _, j := range judgments {
		if j.Verdict == Removable {
			removable++
		}
		e.printJudgment(out, j.Verdict, j)
	}
	_, _ = fmt.Fprintf(out, "%d of %d removable; dry run, nothing removed (wk gc --apply removes them)\n", removable, len(judgments))
	return nil
}

func (e *Env) judgeAll() ([]Judgment, error) {
	s, err := e.takeSurvey()
	if err != nil {
		return nil, err
	}
	list, err := gitx.Worktrees(e.Repo.Primary)
	if err != nil {
		return nil, err
	}
	var judgments []Judgment
	for _, w := range list {
		if w.Bare || physical(w.Path) == s.primary {
			continue
		}
		judgments = append(judgments, e.judge(s, w))
	}
	return judgments, nil
}

// judge applies every test of PLAN.md phase 2 item 1 and keeps every reason that fails, so a
// kept line says all of why, not just the first.
func (e *Env) judge(s survey, w gitx.Worktree) Judgment {
	j := Judgment{Worktree: w}
	if w.Prunable {
		j.Verdict = Prunable
		j.Reasons = []string{"its directory is gone; git worktree prune clears the entry"}
		return j
	}
	if skip := e.skipReason(s, w); skip != "" {
		j.Verdict = Skipped
		j.Reasons = []string{skip}
		return j
	}
	if w.Branch != "" && w.Branch == e.Cfg.Workspace.Base {
		j.Verdict = Kept
		j.Reasons = []string{"it has " + w.Branch + " checked out"}
		return j
	}

	var reasons []string
	keep := func(format string, args ...any) { reasons = append(reasons, fmt.Sprintf(format, args...)) }
	if w.Locked {
		keep("it is locked (%s)", lockText(w.LockReason))
	}
	dir := physical(w.Path)
	if live := procs.In(s.procs, dir); len(live) > 0 {
		keep("a process is working in it: %s", procList(live))
	}
	dirty, dirtyErr := gitx.DirtyPaths(w.Path)
	j.Dirty = len(dirty)
	quiet, stopped, why := s.quietFor(w.Path)
	if last, err := lastActivity(w.Path, dirty); err != nil {
		keep("its last activity cannot be read: %v", err)
	} else if age := s.now.Sub(last); age < quiet {
		j.Activity = last
		keep("it changed %d minutes ago and %s", int(age.Minutes()), why)
	} else {
		j.Activity = last
	}
	absorbed, ancestor := gitx.IsAbsorbed(e.Repo.Primary, w.Head, s.base)
	// A stopped subagent's commits are kept on its branch, which /land-worktrees lands from
	// refs/heads, so its worktree can go whether or not the base holds them yet.
	if !absorbed && !stopped {
		if w.Branch != "" {
			keep("branch %s has commits %s does not hold", w.Branch, e.Cfg.Workspace.Base)
		} else {
			keep("its detached HEAD has commits %s does not hold", e.Cfg.Workspace.Base)
		}
	}
	switch {
	case dirtyErr != nil:
		keep("git could not list its changes: %v", dirtyErr)
	case len(dirty) > 0:
		keep("%s", count(len(dirty), "changed path", "changed paths"))
	}

	switch {
	case len(reasons) == 0 && stopped && !absorbed:
		j.Verdict = Removable
		j.Reasons = []string{stoppedReason(e.Cfg.Workspace.Base, w.Branch)}
	case len(reasons) == 0:
		j.Verdict = Removable
		j.DeleteBranch = ancestor && w.Branch != ""
		j.Reasons = []string{removableReason(e.Cfg.Workspace.Base, w.Branch, ancestor)}
	case absorbed && (dirtyErr != nil || len(dirty) > 0):
		j.Verdict = KeptForLanding
		j.Reasons = reasons
	default:
		j.Verdict = Kept
		j.Reasons = reasons
	}
	return j
}

func (e *Env) skipReason(s survey, w gitx.Worktree) string {
	if physical(w.Path) == s.template {
		return "the wk template"
	}
	gd, err := gitx.GitDir(w.Path)
	if err != nil {
		return ""
	}
	if exists(filepath.Join(gd, MarkerName)) {
		return "a wk template"
	}
	if exists(filepath.Join(gd, PoolMarkerName)) {
		return "a wk pool slot; wk pool fill manages it"
	}
	if exists(filepath.Join(gd, TrialMarkerName)) {
		return "a land-worktrees trial; that skill removes it"
	}
	return ""
}

// quietFor is the quiet window a worktree needs, whether it is a stopped subagent's, and the
// reason a kept line gives. A running subagent leaves no process in its worktree between tool
// calls, so one without a stop marker gets the long window.
func (s survey) quietFor(path string) (time.Duration, bool, string) {
	if !isAgentWorktree(path) {
		return s.quiet, false, "a session may still be in it"
	}
	if gd, err := privateGitDir(path); err == nil && exists(filepath.Join(gd, StoppedMarkerName)) {
		return s.stoppedQuiet, true, fmt.Sprintf("a stopped subagent's worktree waits %d minutes", int(s.stoppedQuiet.Minutes()))
	}
	return max(s.quiet, s.unmarkedQuiet), false, fmt.Sprintf("a subagent worktree with no stop marker waits %d hours", int(s.unmarkedQuiet.Hours()))
}

func stoppedReason(base, branch string) string {
	if branch == "" {
		return "stopped subagent, clean and idle; its detached commits are not on " + base
	}
	return "stopped subagent, clean and idle; branch " + branch + " is kept, with commits " + base + " does not hold"
}

func removableReason(base, branch string, ancestor bool) string {
	how := "an ancestor of " + base
	if !ancestor {
		how = "absorbed by " + base + " by content only"
	}
	switch {
	case branch == "":
		return "detached HEAD is " + how + "; clean and idle"
	case ancestor:
		return "branch " + branch + " is " + how + "; clean and idle; the branch is deleted with it"
	default:
		return "branch " + branch + " is " + how + "; clean and idle; the branch is kept (BD-11)"
	}
}

var reflogTime = regexp.MustCompile(`@\{([0-9]+)\}`)

// lastActivity is BD-10: the newest of the HEAD reflog entry, the worktree's private HEAD and
// index, its top directory, and every changed path. An empty reflog never means "old": a
// worktree made a minute ago by some tools has none, and lib.sh read one such as 56 years idle.
func lastActivity(wt string, dirty []string) (time.Time, error) {
	var newest time.Time
	seen := func(t time.Time) {
		if t.After(newest) {
			newest = t
		}
	}
	if out, err := gitx.Run(wt, "reflog", "-1", "--date=unix", "--format=%gd", "HEAD"); err == nil {
		if m := reflogTime.FindStringSubmatch(out); m != nil {
			if sec, err := strconv.ParseInt(m[1], 10, 64); err == nil {
				seen(time.Unix(sec, 0))
			}
		}
	}
	gd, err := gitx.GitDir(wt)
	if err != nil {
		return time.Time{}, err
	}
	for _, p := range []string{filepath.Join(gd, "HEAD"), filepath.Join(gd, "index"), wt} {
		info, err := os.Lstat(p)
		if err != nil && !os.IsNotExist(err) {
			return time.Time{}, err
		}
		if err == nil {
			seen(info.ModTime())
		}
	}
	for _, rel := range dirty {
		if info, err := os.Lstat(filepath.Join(wt, filepath.FromSlash(rel))); err == nil {
			seen(info.ModTime())
		}
	}
	if newest.IsZero() {
		return time.Time{}, errors.New("no reflog entry and no file time")
	}
	return newest, nil
}

// GCApply removes every worktree gc judges removable, judging each again from a fresh survey
// at the moment of removal, because a session may have moved into it since the pass began. It
// takes the template lock only around `git worktree prune`, the one step that must not race a
// `wk new` between `worktree add` and its clones; holding it for the whole pass would make
// every new workspace wait for gc.
func (e *Env) GCApply(out io.Writer) error {
	_, _ = fmt.Fprintf(out, "wk gc --apply at %s\n", e.clock().Format(time.RFC3339))

	judgments, err := e.judgeAll()
	if err != nil {
		return err
	}
	var failures []error
	removed, pruned := 0, false
	for _, j := range judgments {
		switch j.Verdict {
		case Removable:
			ok, err := e.removeOne(out, j.Worktree)
			if err != nil {
				failures = append(failures, err)
			}
			if ok {
				removed++
			}
		case Prunable:
			if !pruned {
				if err := e.prune(); err != nil {
					failures = append(failures, err)
				}
				pruned = true
			}
			e.printJudgment(out, "pruned", j)
		default:
			e.printJudgment(out, j.Verdict, j)
		}
	}
	_, _ = fmt.Fprintf(out, "%d of %d removed\n", removed, len(judgments))
	return errors.Join(failures...)
}

func (e *Env) prune() error {
	lock, err := flock.Take(e.lockPath(), true, func() { e.say("waiting for the template lock to prune") })
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()
	_, err = gitx.Run(e.Repo.Primary, "worktree", "prune")
	return err
}

func (e *Env) removeOne(out io.Writer, w gitx.Worktree) (bool, error) {
	j, err := e.rejudge(w)
	if err != nil {
		e.printLine(out, "kept", w, "the re-check failed: "+err.Error())
		return false, err
	}
	if j.Verdict != Removable {
		e.printJudgment(out, j.Verdict, j)
		return false, nil
	}
	// No --force: git refuses a worktree with tracked changes or untracked files, a second
	// guard behind the re-check above. Ignored files go with the directory.
	if _, err := gitx.Run(e.Repo.Primary, "worktree", "remove", j.Worktree.Path); err != nil {
		e.printLine(out, "kept", w, "git worktree remove refused: "+err.Error())
		return false, err
	}
	if !j.DeleteBranch {
		e.printJudgment(out, "removed", j)
		return true, nil
	}
	if !gitx.IsAncestor(e.Repo.Primary, "refs/heads/"+j.Worktree.Branch, "refs/heads/"+e.Cfg.Workspace.Base) {
		e.printLine(out, "removed", w, "branch "+j.Worktree.Branch+" moved off "+e.Cfg.Workspace.Base+" during the pass, so it is kept")
		return true, nil
	}
	if _, err := gitx.Run(e.Repo.Primary, "branch", "-d", j.Worktree.Branch); err != nil {
		e.printLine(out, "removed", w, "branch "+j.Worktree.Branch+" kept: "+err.Error())
		return true, nil
	}
	e.printLine(out, "removed", w, "branch "+j.Worktree.Branch+" deleted")
	return true, nil
}

// rejudge reads the worktree list, the base and the processes again and judges w alone.
func (e *Env) rejudge(w gitx.Worktree) (Judgment, error) {
	s, err := e.takeSurvey()
	if err != nil {
		return Judgment{}, err
	}
	list, err := gitx.Worktrees(e.Repo.Primary)
	if err != nil {
		return Judgment{}, err
	}
	for _, now := range list {
		if physical(now.Path) == physical(w.Path) {
			return e.judge(s, now), nil
		}
	}
	return Judgment{}, fmt.Errorf("%s is no longer a worktree", w.Path)
}

func (e *Env) printJudgment(out io.Writer, verdict Verdict, j Judgment) {
	e.printLine(out, verdict, j.Worktree, strings.Join(j.Reasons, "; "))
}

func (e *Env) printLine(out io.Writer, verdict Verdict, w gitx.Worktree, reason string) {
	_, _ = fmt.Fprintf(out, "%-26s %-44s %s\n", verdict, e.display(w.Path), reason)
}

// display shows a worktree under the main checkout by its relative path, which is how the
// config names them, and any other by its absolute path.
func (e *Env) display(path string) string {
	if rel, err := filepath.Rel(e.Repo.Primary, path); err == nil && !strings.HasPrefix(rel, "..") {
		return rel
	}
	return path
}

func lockText(reason string) string {
	if reason == "" {
		return "git worktree unlock releases it"
	}
	return reason
}

func procList(list []procs.Proc) string {
	shown := list
	if len(shown) > 3 {
		shown = shown[:3]
	}
	parts := make([]string, len(shown))
	for i, p := range shown {
		parts[i] = p.String()
	}
	text := strings.Join(parts, ", ")
	if more := len(list) - len(shown); more > 0 {
		text += fmt.Sprintf(" and %d more", more)
	}
	return text
}

func count(n int, singular, plural string) string {
	if n == 1 {
		return "1 " + singular
	}
	return fmt.Sprintf("%d %s", n, plural)
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
