package wk

import (
	"errors"
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/gitx"
)

// The branch pass. judge lets a stopped subagent's worktree go while its unlanded branch stays,
// and nothing looked at that branch again, so it stayed after it landed: ezhomesteading had 21
// such worktree-agent-* branches on 2026-10-21. This pass deletes the ones the base now holds.

// BranchJudgment is gc's answer for one local branch that no worktree has checked out.
type BranchJudgment struct {
	Branch  string
	Verdict Verdict
	Reason  string
}

// branchSurvey is what the branch pass reads once and asks every branch about.
type branchSurvey struct {
	base       string
	checkedOut map[string]bool
	now        time.Time
	quiet      time.Duration
}

func (e *Env) takeBranchSurvey() (branchSurvey, error) {
	list, err := gitx.Worktrees(e.Repo.Primary)
	if err != nil {
		return branchSurvey{}, err
	}
	// Every entry counts, locked, prunable, template and pool slots too: git refuses to delete
	// a branch any of them has checked out, and gc never asks it to.
	checkedOut := map[string]bool{}
	for _, w := range list {
		if w.Branch != "" {
			checkedOut[w.Branch] = true
		}
	}
	return branchSurvey{
		base:       e.Cfg.Workspace.Base,
		checkedOut: checkedOut,
		now:        e.clock(),
		quiet:      time.Duration(e.Cfg.GC.QuietMinutes) * time.Minute,
	}, nil
}

// judgeBranches judges every local branch that gc owns and no worktree has checked out.
func (e *Env) judgeBranches() ([]BranchJudgment, error) {
	s, err := e.takeBranchSurvey()
	if err != nil {
		return nil, err
	}
	out, err := gitx.Run(e.Repo.Primary, "for-each-ref", "--format=%(refname)", "refs/heads/")
	if err != nil {
		return nil, err
	}
	var judgments []BranchJudgment
	for ref := range strings.SplitSeq(out, "\n") {
		b, ok := strings.CutPrefix(ref, "refs/heads/")
		if ok && !s.checkedOut[b] && e.ownsBranch(b) {
			judgments = append(judgments, e.judgeBranch(s, b))
		}
	}
	return judgments, nil
}

// ownsBranch says whether gc may delete branch b once it is an orphan: a name the hook or
// wk new gives, or one a gc.branch_patterns glob matches. The base and the landing ref's branch
// are never gc's, whatever a pattern says.
func (e *Env) ownsBranch(b string) bool {
	if b == e.Cfg.Workspace.Base || "refs/heads/"+b == e.Cfg.Landing.Ref {
		return false
	}
	if fills(e.Cfg.Hook.Branch, config.NamePlaceholder, b, nil) || fills(e.Cfg.Workspace.Branch, config.RowPlaceholder, b, rowPattern) {
		return true
	}
	for _, p := range e.Cfg.GC.BranchPatterns {
		if ok, _ := path.Match(p, b); ok {
			return true
		}
	}
	return false
}

// fills says whether b is template with its placeholder filled by a non-empty value, which must
// also match valid when valid is set. The row must be a number, so wk/landing never reads as
// workspace wk/{row}.
func fills(template, placeholder, b string, valid *regexp.Regexp) bool {
	prefix, suffix, ok := strings.Cut(template, placeholder)
	if !ok || len(b) <= len(prefix)+len(suffix) || !strings.HasPrefix(b, prefix) || !strings.HasSuffix(b, suffix) {
		return false
	}
	return valid == nil || valid.MatchString(b[len(prefix):len(b)-len(suffix)])
}

// judgeBranch keeps a branch with commits the base does not hold, and one that moved inside
// gc.quiet_minutes: a pool claim writes its branch a moment before the slot's HEAD points at it,
// and in that moment the new branch is an orphan at the base.
func (e *Env) judgeBranch(s branchSurvey, b string) BranchJudgment {
	j := BranchJudgment{Branch: b, Verdict: Kept}
	ref := "refs/heads/" + b
	if !gitx.IsAncestor(e.Repo.Primary, ref, "refs/heads/"+s.base) {
		j.Reason = "no worktree has it; " + e.unheld(ref, s.base)
		return j
	}
	moved, err := refMoved(e.Repo.Primary, ref)
	switch {
	case err != nil:
		j.Reason = "its last update cannot be read: " + err.Error()
	case s.now.Sub(moved) < s.quiet:
		j.Reason = fmt.Sprintf("it moved %d minutes ago; a new workspace may be taking it", int(s.now.Sub(moved).Minutes()))
	default:
		j.Verdict = Removable
		j.Reason = "no worktree has it, and it is an ancestor of " + s.base
	}
	return j
}

// unheld says how many commits on ref the base does not hold.
func (e *Env) unheld(ref, base string) string {
	out, err := gitx.Run(e.Repo.Primary, "rev-list", "--count", "refs/heads/"+base+".."+ref)
	if n, convErr := strconv.Atoi(out); err == nil && convErr == nil {
		return "it has " + count(n, "commit", "commits") + " " + base + " does not hold"
	}
	return "it has commits " + base + " does not hold"
}

// refMoved is the time of ref's newest reflog entry. A branch with no reflog has no readable
// age, and judgeBranch keeps it.
func refMoved(dir, ref string) (time.Time, error) {
	out, err := gitx.Run(dir, "reflog", "-1", "--date=unix", "--format=%gd", ref)
	if err != nil {
		return time.Time{}, err
	}
	m := reflogTime.FindStringSubmatch(out)
	if m == nil {
		return time.Time{}, errors.New("it has no reflog")
	}
	sec, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return time.Time{}, err
	}
	return time.Unix(sec, 0), nil
}

// gcBranches is the dry run of the branch pass. It prints nothing when no branch is an orphan.
func (e *Env) gcBranches(out io.Writer) error {
	judgments, err := e.judgeBranches()
	if err != nil || len(judgments) == 0 {
		return err
	}
	removable := 0
	for _, j := range judgments {
		if j.Verdict == Removable {
			removable++
		}
		printBranch(out, j.Verdict, j.Branch, j.Reason)
	}
	_, _ = fmt.Fprintf(out, "%d of %d orphan branches removable; dry run, nothing deleted\n", removable, len(judgments))
	return nil
}

// applyBranches deletes every orphan branch gc judges removable, judging each again at the
// moment of deletion. A refusal from git is a kept line, as it is for a worktree's branch in
// removeOne, not a failure of the pass.
func (e *Env) applyBranches(out io.Writer) error {
	judgments, err := e.judgeBranches()
	if err != nil || len(judgments) == 0 {
		return err
	}
	deleted := 0
	var failures []error
	for _, j := range judgments {
		if j.Verdict != Removable {
			printBranch(out, j.Verdict, j.Branch, j.Reason)
			continue
		}
		ok, err := e.deleteBranch(out, j.Branch)
		if err != nil {
			failures = append(failures, err)
		}
		if ok {
			deleted++
		}
	}
	_, _ = fmt.Fprintf(out, "%d of %d orphan branches deleted\n", deleted, len(judgments))
	return errors.Join(failures...)
}

func (e *Env) deleteBranch(out io.Writer, b string) (bool, error) {
	s, err := e.takeBranchSurvey()
	if err != nil {
		printBranch(out, "kept", b, "the re-check failed: "+err.Error())
		return false, err
	}
	if s.checkedOut[b] {
		printBranch(out, "kept", b, "a worktree has checked it out since the pass began")
		return false, nil
	}
	if j := e.judgeBranch(s, b); j.Verdict != Removable {
		printBranch(out, j.Verdict, b, j.Reason)
		return false, nil
	}
	// No -D: git refuses a branch it calls unmerged, a second guard behind the re-check above.
	// It reads "merged" against the main checkout's HEAD, so it also refuses while that is not
	// the base.
	if _, err := gitx.Run(e.Repo.Primary, "branch", "-d", b); err != nil {
		printBranch(out, "kept", b, "git branch -d refused: "+err.Error())
		return false, nil
	}
	printBranch(out, "deleted", b, "it was an ancestor of "+s.base+" and no worktree had it")
	return true, nil
}

func printBranch(out io.Writer, verdict Verdict, b, reason string) {
	_, _ = fmt.Fprintf(out, "%-26s %-44s %s\n", verdict, "branch "+b, reason)
}
