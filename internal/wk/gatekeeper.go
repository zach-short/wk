package wk

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/batch"
	"github.com/zach-short/wk/internal/changeset"
	"github.com/zach-short/wk/internal/gitx"
	"github.com/zach-short/wk/internal/landing"
)

// gatesOn is whether the worker gates before it lands. With no [[gates.run]] it lands onto the
// ref as P3 did and never fast-forwards the base branch (PD-11).
func (e *Env) gatesOn() bool { return len(e.Cfg.Gates.Run) > 0 }

// gatePass is one batch of P4 (PLAN-P2-P4.md section 4): what a stopped worker left is queued
// again, batch.Pick forms the batch, resolve gates and lands it, and the base branch follows the
// landing tip when anything landed. A batch that is not ready waits for its quiet.
func (e *Env) gatePass(records []changeset.Record, tip string, out io.Writer) error {
	queued, err := e.requeue(records, tip, out)
	if err != nil {
		return err
	}
	idle := time.Duration(e.Cfg.Gates.BatchIdleSeconds) * time.Second
	picked := batch.Pick(queued, e.Cfg.Gates.BatchMax, idle, e.clock())
	if picked == nil {
		e.waitForBatch(queued, idle)
		return nil
	}
	next, err := e.resolve(picked, tip, out)
	if next != tip {
		e.advanceMain(next, out)
	}
	return err
}

// requeue is the queue as batch.Pick reads it. A gated or landing record that a stopped worker
// left is pending again as submitted, so it is gated again before it lands (row 12's finding);
// one whose prediction is already the tip's tree had its swap go through, and is landed at the
// tip without a commit, as P3 does.
func (e *Env) requeue(records []changeset.Record, tip string, out io.Writer) ([]changeset.Record, error) {
	tipTree, err := e.treeOf(tip)
	if err != nil {
		return nil, err
	}
	var queued []changeset.Record
	for _, r := range records {
		if !pending(r) {
			continue
		}
		if r.State != changeset.Submitted && r.Predicted == tipTree {
			if err := e.markLanded(r, tip, out); err != nil {
				return nil, err
			}
			continue
		}
		r.State = changeset.Submitted
		queued = append(queued, r)
	}
	return queued, nil
}

// waitForBatch sleeps until the newest queued changeset has been quiet for idle, a second at a
// time, so a submit in between can still fill the batch to gates.batch_max.
func (e *Env) waitForBatch(queued []changeset.Record, idle time.Duration) {
	var newest time.Time
	for _, r := range queued {
		if r.At.After(newest) {
			newest = r.At
		}
	}
	if wait := newest.Add(idle).Sub(e.clock()); wait > 0 {
		e.sleep(min(wait, time.Second))
	}
}

// resolve gates recs as one batch on tip and lands it when green. On red, batch.Blame is a first
// guess: the batch without the blamed changeset is gated again, and the blamed one is failed only
// when that gate is green (row 12's audit, probe B11). Otherwise the batch is bisected; a red
// batch of one is failed. It returns the landing tip, which moves only by a green gate's trees.
func (e *Env) resolve(recs []changeset.Record, tip string, out io.Writer) (string, error) {
	p, err := e.predictBatch(recs, tip, out)
	if err != nil || len(p.members) == 0 {
		return tip, err
	}
	v, err := e.gatePlan(p, out)
	switch {
	case err != nil:
		return tip, err
	case v.green():
		return e.landPlan(p, v, out)
	case v.red == nil:
		return tip, fmt.Errorf("every gate skipped over %s (%s); a skip is never a pass, so nothing lands", strings.Join(p.ids(), " "), v.summary())
	case len(p.members) == 1:
		return tip, e.markFailed(p.members[0].rec, v, out)
	}
	if next, done, err := e.tryBlame(p, v, out); done || err != nil {
		return next, err
	}
	return e.bisect(p.records(), tip, out)
}

// tryBlame tests batch.Blame's guess. done is true when the batch without the blamed changeset
// was green, landed, and the blamed one is failed; false sends the caller to bisect the whole
// batch, the blamed changeset still in it.
func (e *Env) tryBlame(p plan, v verdict, out io.Writer) (string, bool, error) {
	diffs, err := e.diffs(p)
	if err != nil {
		return p.tip, false, err
	}
	id, unique := batch.Blame(v.paths, diffs)
	if !unique {
		return p.tip, false, nil
	}
	_, _ = fmt.Fprintf(out, "gate %s names %s; gating the batch again without it\n", v.red.Name, id)
	q, err := e.predictBatch(p.without(id), p.tip, out)
	if err != nil {
		return p.tip, false, err
	}
	qv, err := e.gatePlan(q, out)
	if err != nil || !qv.green() {
		return p.tip, false, err
	}
	if err := e.markFailed(p.member(id), v, out); err != nil {
		return p.tip, false, err
	}
	next, err := e.landPlan(q, qv, out)
	return next, true, err
}

// bisect splits a red batch in two in queue order and resolves each half as a batch of its own,
// the second predicted afresh on whatever the first landed (D16 as amended 2026-10-16).
func (e *Env) bisect(recs []changeset.Record, tip string, out io.Writer) (string, error) {
	first, second, _ := batch.Halves(recs)
	_, _ = fmt.Fprintf(out, "bisecting: %s | %s\n", strings.Join(recordIDs(first), " "), strings.Join(recordIDs(second), " "))
	tip, err := e.resolve(first, tip, out)
	if err != nil {
		return tip, err
	}
	return e.resolve(second, tip, out)
}

// landPlan lands a green plan in queue order. Every member is journalled gated first, then each
// is journalled landing and swapped by landing.Land, one compare-and-swap each, as P3's landOne
// does; a member that changes nothing lands at the tip without a commit.
func (e *Env) landPlan(p plan, v verdict, out io.Writer) (string, error) {
	for _, m := range p.members {
		r := m.rec
		r.State, r.Predicted, r.Reason, r.At = changeset.Gated, m.predicted, v.summary(), e.clock()
		if err := e.queue().Append(r); err != nil {
			return p.tip, err
		}
	}
	tip := p.tip
	for _, m := range p.members {
		next, err := e.landMember(m, v, tip, out)
		if err != nil {
			return tip, err
		}
		tip = next
	}
	return tip, nil
}

func (e *Env) landMember(m member, v verdict, tip string, out io.Writer) (string, error) {
	r := m.rec
	r.Predicted, r.Reason = m.predicted, v.summary()
	if m.predicted == m.prev {
		return tip, e.markLanded(r, tip, out)
	}
	r.State, r.At = changeset.Landing, e.clock()
	if err := e.queue().Append(r); err != nil {
		return tip, err
	}
	commit, err := landing.Land(e.Repo.Primary, e.Cfg.Landing.Ref, tip, m.predicted, r.Message)
	if err != nil {
		return tip, err
	}
	return commit, e.markLanded(r, commit, out)
}

// markFailed records r as failed by the red gate of v, with the paths that gate named, which may
// be none. Its leases stay: the workspace fixes the change and submits again.
func (e *Env) markFailed(r changeset.Record, v verdict, out io.Writer) error {
	r.State, r.Gate, r.Paths, r.At = changeset.Failed, v.red.Name, v.paths, e.clock()
	r.Reason = v.red.Reason + "; fix it in the workspace and wk submit again"
	_, _ = fmt.Fprintf(out, "failed %s: gate %s, %s\n", r.ID, r.Gate, v.red.Reason)
	return e.queue().Append(r)
}

// diffs maps each member to the paths its changeset changes against its own base. -r, or git
// prints top-level entries only and batch.Blame matches nothing; -z, so no path is quoted.
func (e *Env) diffs(p plan) (map[string][]string, error) {
	out := map[string][]string{}
	for _, m := range p.members {
		names, err := gitx.Run(e.Repo.Primary, "diff-tree", "-r", "-z", "--name-only", m.rec.Base, m.rec.Tree)
		if err != nil {
			return nil, err
		}
		for _, n := range strings.Split(names, "\x00") {
			if n != "" {
				out[m.rec.ID] = append(out[m.rec.ID], n)
			}
		}
	}
	return out, nil
}

// advanceMain fast-forwards the base branch to the landing tip after a green batch landed, and
// pushes it (D14, D20). The landing stands either way, so a refusal is said and recorded for
// wk status, never returned.
func (e *Env) advanceMain(tip string, out io.Writer) {
	note := e.fastForward(tip)
	_, _ = fmt.Fprintln(out, note)
	e.noteGates(func(l *gateLog) { l.Main, l.MainAt = note, e.clock() })
}

func (e *Env) fastForward(tip string) string {
	base, remote := e.Cfg.Workspace.Base, e.Cfg.Landing.Remote
	if !e.Cfg.Landing.FFMain {
		return fmt.Sprintf("%s did not move: landing.ff_main is false; wk land --apply moves it", base)
	}
	blockers, err := landing.FastForward(e.Repo.Primary, base, e.Cfg.Landing.Ref, tip)
	if err != nil {
		return fmt.Sprintf("%s did not move: %v %s", base, err, strings.Join(blockers, " "))
	}
	if !e.Cfg.Landing.Push {
		return fmt.Sprintf("%s fast-forwarded to %s; landing.push is false", base, tip)
	}
	if err := landing.Push(e.Repo.Primary, remote, base); err != nil {
		return fmt.Sprintf("%s fast-forwarded to %s; the push to %s failed: %v", base, tip, remote, err)
	}
	return fmt.Sprintf("%s fast-forwarded to %s and pushed to %s", base, tip, remote)
}

func (e *Env) treeOf(commit string) (string, error) {
	return gitx.Run(e.Repo.Primary, "rev-parse", commit+"^{tree}")
}

func recordIDs(recs []changeset.Record) []string {
	ids := make([]string, len(recs))
	for i, r := range recs {
		ids[i] = r.ID
	}
	return ids
}
