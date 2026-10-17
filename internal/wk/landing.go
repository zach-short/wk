package wk

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/zach-short/wk/internal/changeset"
	"github.com/zach-short/wk/internal/flock"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/landing"
)

// queue is ~/.wk/<repo>/queue.jsonl (PD-8).
func (e *Env) queue() changeset.Queue {
	return changeset.Queue{Path: filepath.Join(e.State, "queue.jsonl")}
}

// LandingLogPath is where the detached worker writes.
func (e *Env) LandingLogPath() string { return filepath.Join(e.State, "landing.log") }

// LandLoop is `wk land --loop`, the worker: one at a time under landing.lock, it runs passes
// until no changeset is submitted or landing, then exits (files decision, 2026-10-16). A git
// failure ends the worker with the error on its log; the next submit or the timer starts one
// again, which predicts afresh. A second worker finds the lock held and leaves at once.
func (e *Env) LandLoop(out io.Writer) error {
	if !e.Cfg.Landing.Enabled {
		return fmt.Errorf("landing.enabled is false in %s; the worker moves nothing", e.configPath())
	}
	lock, ok, err := flock.TryTake(filepath.Join(e.State, "landing.lock"), true)
	if err != nil {
		return err
	}
	if !ok {
		e.say("another worker holds landing.lock; leaving the queue to it")
		return nil
	}
	defer func() { _ = lock.Release() }()
	for {
		more, err := e.landPass(out)
		if err != nil || !more {
			return err
		}
	}
}

// landPass puts the ref where the loop lands (landing.Ensure), then takes every submitted or
// landing changeset in queue order. more is whether a submit during the pass left work.
func (e *Env) landPass(out io.Writer) (more bool, err error) {
	tip, err := e.ensureLanding()
	if err != nil {
		return false, err
	}
	records, err := e.queue().Load()
	if err != nil {
		return false, err
	}
	for _, r := range records {
		if !pending(r) {
			continue
		}
		if tip, err = e.landOne(r, tip, out); err != nil {
			return false, err
		}
	}
	if records, err = e.queue().Load(); err != nil {
		return false, err
	}
	for _, r := range records {
		if pending(r) {
			return true, nil
		}
	}
	return false, nil
}

func pending(r changeset.Record) bool {
	return r.State == changeset.Submitted || r.State == changeset.Landing
}

// landOne predicts r onto tip and lands it, and returns the new tip. A tree already on the tip
// (a landing record whose swap went through before a crash, or a submit that changed nothing)
// is landed without a commit, so no duplicate commit is written (F18); a conflict is refused
// with its paths (option P); otherwise the predicted tree is journalled as a landing line and
// then committed by the compare-and-swap. A refused swap ends the pass with the error, and the
// next pass predicts again from the landing line.
func (e *Env) landOne(r changeset.Record, tip string, out io.Writer) (string, error) {
	tipTree, err := gitx.Run(e.Repo.Primary, "rev-parse", tip+"^{tree}")
	if err != nil {
		return "", err
	}
	if r.Tree == tipTree {
		return tip, e.markLanded(r, tip, out)
	}
	if e.Cfg.Landing.RequireReview && r.Review == "" {
		return tip, e.refuse(r, nil, "no review; landing.require_review is true, submit again with --review", out)
	}
	result, conflicts, err := landing.Predict(e.Repo.Primary, r.Base, tip, r.Tree)
	if err != nil {
		return "", err
	}
	if len(conflicts) > 0 {
		return tip, e.refuse(r, conflicts, "conflicts with the landing tip "+tip+"; run wk sync, fix the markers and submit again", out)
	}
	if result == tipTree {
		return tip, e.markLanded(r, tip, out)
	}
	r.State, r.Tree, r.At = changeset.Landing, result, e.clock()
	if err := e.queue().Append(r); err != nil {
		return "", err
	}
	commit, err := landing.Land(e.Repo.Primary, e.Cfg.Landing.Ref, tip, result, r.Message)
	if err != nil {
		return "", err
	}
	return commit, e.markLanded(r, commit, out)
}

func (e *Env) refuse(r changeset.Record, paths []string, reason string, out io.Writer) error {
	r.State, r.Paths, r.Reason, r.At = changeset.Refused, paths, reason, e.clock()
	_, _ = fmt.Fprintf(out, "refused %s: %s\n", r.ID, reason)
	return e.queue().Append(r)
}

// markLanded appends the landed line and drops the workspace's leases (option K).
func (e *Env) markLanded(r changeset.Record, commit string, out io.Writer) error {
	r.State, r.Commit, r.At = changeset.Landed, commit, e.clock()
	if err := e.queue().Append(r); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "landed %s as %s\n", r.ID, commit)
	e.releaseLanded(r.Workspace)
	return nil
}

// releaseLanded frees the leases of a landed workspace. A workspace that is gone was reaped;
// a store that cannot be changed is said, because the landing itself is done.
func (e *Env) releaseLanded(root string) {
	if !exists(root) {
		return
	}
	err := e.changeLeases(root, func(f *leaseFile) error {
		f.Release(root)
		return nil
	})
	if err != nil {
		e.say("leases of %s not released: %v", root, err)
	}
}

// ensureLanding is landing.Ensure against the base branch's hash.
func (e *Env) ensureLanding() (string, error) {
	main, err := e.baseCommit()
	if err != nil {
		return "", err
	}
	return landing.Ensure(e.Repo.Primary, e.Cfg.Landing.Ref, main)
}

func (e *Env) baseCommit() (string, error) {
	return gitx.Commit(e.Repo.Primary, "refs/heads/"+e.Cfg.Workspace.Base)
}

// landingTip is the hash the landing ref holds, or the base branch's when the ref does not
// exist yet, which is where the first pass creates it. It writes nothing.
func (e *Env) landingTip() (string, error) {
	if tip, err := gitx.Commit(e.Repo.Primary, e.Cfg.Landing.Ref); err == nil {
		return tip, nil
	}
	return e.baseCommit()
}

// Sync is `wk sync`: the workspace holding dir moves to the landing tip with its uncommitted
// work carried across (landing.Sync). A branch with commits of its own is first folded: the
// branch is moved back to its merge base with the tip, so its commits become uncommitted work
// in the tree and stay in the reflog (owner amendment, 2026-10-16; PD-7 refused it). Conflicts
// are printed as paths, with git's markers left in the tree, and come back as an error.
func (e *Env) Sync(dir string, out io.Writer) error {
	root, err := e.workspaceOnly(dir)
	if err != nil {
		return err
	}
	tip, err := e.landingTip()
	if err != nil {
		return err
	}
	if err := e.foldCommits(root, tip); err != nil {
		return err
	}
	base, err := changeset.Base(root, tip)
	if err != nil {
		return err
	}
	conflicts, err := landing.Sync(root, base, tip)
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "synced: %s at %s\n", root, tip)
	for _, p := range conflicts {
		_, _ = fmt.Fprintf(out, "conflict: %s\n", p)
	}
	if len(conflicts) > 0 {
		return fmt.Errorf("%s with markers in the working tree; fix them, then wk submit", count(len(conflicts), "path conflicts", "paths conflict"))
	}
	return nil
}

// foldCommits moves the workspace's branch from a HEAD that is not an ancestor of tip back to
// their merge base, by compare-and-swap, and says how many commits became uncommitted work.
// A detached HEAD is left to landing.Sync's refusal.
func (e *Env) foldCommits(root, tip string) error {
	head, branch, err := changeset.Head(root)
	if err != nil || branch == "" || gitx.IsAncestor(root, head, tip) {
		return err
	}
	base, err := changeset.Base(root, tip)
	if err != nil {
		return err
	}
	n, err := gitx.Run(root, "rev-list", "--count", base+".."+head)
	if err != nil {
		return err
	}
	if _, err := gitx.Run(root, "update-ref", "refs/heads/"+branch, base, head); err != nil {
		return err
	}
	e.say("folded %s commits of %s into the working tree; they stay in the branch's reflog at %s", n, branch, head)
	return nil
}

// Land is `wk land`, the dry run: the landing tip, the base branch, whether a fast-forward is
// possible, and the dirty paths in the main checkout that block it (DESIGN.md section 6).
func (e *Env) Land(out io.Writer) error {
	st, err := e.landingState()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "%s %s (%s)\n%s %s\n", e.Cfg.Landing.Ref, orMissing(st.tip), st.note, e.Cfg.Workspace.Base, st.main)
	if !st.ahead {
		return nil
	}
	blockers, err := landing.Blockers(e.Repo.Primary, e.Cfg.Workspace.Base, st.tip)
	if err != nil {
		return err
	}
	for _, p := range blockers {
		_, _ = fmt.Fprintf(out, "blocker: %s\n", p)
	}
	if len(blockers) > 0 {
		_, _ = fmt.Fprintf(out, "%s in %s would be overwritten; their owner commits or stashes them first\n", count(len(blockers), "dirty path", "dirty paths"), e.Repo.Primary)
		return nil
	}
	_, _ = fmt.Fprintf(out, "fast-forward possible: wk land --apply --expect %s\n", st.tip)
	return nil
}

// LandApply is `wk land --apply --expect <hash>`: the base branch is fast-forwarded in the main
// checkout to the landing ref while it still holds expect, then pushed when landing.push is set
// (D20). In P3 this is the only way the base branch moves (PD-6).
func (e *Env) LandApply(expect string, out io.Writer) error {
	blockers, err := landing.FastForward(e.Repo.Primary, e.Cfg.Workspace.Base, e.Cfg.Landing.Ref, expect)
	for _, p := range blockers {
		_, _ = fmt.Fprintf(out, "blocker: %s\n", p)
	}
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "%s is at %s\n", e.Cfg.Workspace.Base, expect)
	if !e.Cfg.Landing.Push {
		return nil
	}
	if err := landing.Push(e.Repo.Primary, e.Cfg.Landing.Remote, e.Cfg.Workspace.Base); err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "pushed %s to %s\n", e.Cfg.Workspace.Base, e.Cfg.Landing.Remote)
	return nil
}

// landingState is the read-only view `wk land` and `wk status` print: the two hashes and one
// note on how they stand. ahead is whether the base branch can fast-forward to the tip.
type landingState struct {
	main, tip, note string
	ahead           bool
}

func (e *Env) landingState() (landingState, error) {
	main, err := e.baseCommit()
	if err != nil {
		return landingState{}, err
	}
	st := landingState{main: main}
	st.tip, _ = gitx.Commit(e.Repo.Primary, e.Cfg.Landing.Ref)
	repo := e.Repo.Primary
	switch {
	case st.tip == "":
		st.note = "missing; the first worker pass creates it at " + e.Cfg.Workspace.Base
	case st.tip == main:
		st.note = "at " + e.Cfg.Workspace.Base + "; nothing to land"
	case gitx.IsAncestor(repo, main, st.tip):
		n, _ := gitx.Run(repo, "rev-list", "--count", main+".."+st.tip)
		st.note, st.ahead = n+" commits ahead of "+e.Cfg.Workspace.Base, true
	case gitx.IsAncestor(repo, st.tip, main):
		st.note = "behind " + e.Cfg.Workspace.Base + "; the next worker pass moves it there"
	default:
		st.note = fmt.Sprintf("diverged from %s: the worker stops; merge by hand, or reset it with git update-ref %s %s %s", e.Cfg.Workspace.Base, e.Cfg.Landing.Ref, main, st.tip)
	}
	return st, nil
}

func orMissing(hash string) string {
	if hash == "" {
		return "missing"
	}
	return hash
}

// shortHash is the first 12 hex of a hash for a status line, or the hash as it is.
func shortHash(h string) string {
	if len(h) > 12 && !strings.ContainsAny(h, " ") {
		return h[:12]
	}
	return h
}
