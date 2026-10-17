package wk

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/changeset"
)

// Status is `wk status`: the leases, then the landing ref against the base branch, the gate,
// and every changeset with its state and reason (PLAN-P2-P4.md section 3 item 5, section 4
// item 4).
func (e *Env) Status(out io.Writer) error {
	if err := e.LeaseStatus(out); err != nil {
		return err
	}
	return e.QueueStatus(out)
}

// QueueStatus prints the landing half of the status. The queue is read even when landing is
// off, so a changeset submitted before the switch is still seen.
func (e *Env) QueueStatus(out io.Writer) error {
	_, _ = fmt.Fprintf(out, "landing.enabled = %v\n", e.Cfg.Landing.Enabled)
	st, err := e.landingState()
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(out, "%s %s (%s), %s %s\n%s", e.Cfg.Landing.Ref, orMissing(shortHash(st.tip)), st.note, e.Cfg.Workspace.Base, shortHash(st.main), st.remote)
	records, err := e.queue().Load()
	if err != nil {
		return err
	}
	if err := e.printGates(out, st, records); err != nil {
		return err
	}
	for _, r := range records {
		printRecord(out, r)
	}
	if len(records) == 0 {
		_, _ = fmt.Fprintln(out, "no changesets")
	}
	return nil
}

func printRecord(out io.Writer, r changeset.Record) {
	_, _ = fmt.Fprintf(out, "%s  %-9s  %s  %s\n", r.ID, r.State, r.Workspace, r.Message)
	if r.Commit != "" {
		_, _ = fmt.Fprintf(out, "  commit: %s\n", r.Commit)
	}
	if r.Reason != "" {
		_, _ = fmt.Fprintf(out, "  reason: %s\n", r.Reason)
	}
	if r.Gate != "" {
		_, _ = fmt.Fprintf(out, "  gate: %s\n", r.Gate)
	}
	label := "conflict"
	if r.State == changeset.Failed {
		label = "named"
	}
	for _, p := range r.Paths {
		_, _ = fmt.Fprintf(out, "  %s: %s\n", label, p)
	}
}

// printGates is the P4 half of the status: the last gate run, or the batch in flight; why the
// base branch is behind the landing ref; and a warning when more than two batches wait.
func (e *Env) printGates(out io.Writer, st landingState, records []changeset.Record) error {
	l, err := e.readGateLog()
	if err != nil {
		return err
	}
	printGateRun(out, l)
	if st.ahead {
		_, _ = fmt.Fprintln(out, e.whyMainBehind(l))
	}
	if n := countPending(records); n > 2*e.Cfg.Gates.BatchMax {
		_, _ = fmt.Fprintf(out, "warning: %d changesets pending, more than two batches of gates.batch_max = %d; a queue this deep for long means the gate is busy more than half the time, and gates.batch_max may rise (decision 20)\n", n, e.Cfg.Gates.BatchMax)
	}
	return nil
}

func printGateRun(out io.Writer, l gateLog) {
	switch l.State {
	case "":
		return
	case "gating":
		_, _ = fmt.Fprintf(out, "batch in flight: %s, gating tree %s since %s\n", strings.Join(l.Batch, " "), shortHash(l.Tree), l.At.Format(time.RFC3339))
		return
	}
	_, _ = fmt.Fprintf(out, "last gate run: %s, %s on tree %s at %s\n", l.State, strings.Join(l.Batch, " "), shortHash(l.Tree), l.At.Format(time.RFC3339))
	for _, g := range l.Gates {
		_, _ = fmt.Fprintf(out, "  %s\n", g)
	}
	if l.Error != "" {
		_, _ = fmt.Fprintf(out, "  error: %s\n", l.Error)
	}
}

// whyMainBehind says why the base branch has not reached the landing tip (PD-11).
func (e *Env) whyMainBehind(l gateLog) string {
	base := e.Cfg.Workspace.Base
	switch {
	case !e.gatesOn():
		return fmt.Sprintf("%s did not move: %s has no [[gates.run]], so the worker lands onto %s and never fast-forwards (PD-11); wk land --apply moves it", base, e.configPath(), e.Cfg.Landing.Ref)
	case !e.Cfg.Landing.FFMain:
		return fmt.Sprintf("%s did not move: landing.ff_main is false; wk land --apply moves it", base)
	case l.Main != "":
		return fmt.Sprintf("%s is behind the landing ref; the last move, at %s: %s", base, l.MainAt.Format(time.RFC3339), l.Main)
	}
	return fmt.Sprintf("%s is behind the landing ref; the worker fast-forwards it after its next green batch, or wk land --apply moves it", base)
}

func countPending(records []changeset.Record) int {
	n := 0
	for _, r := range records {
		if pending(r) {
			n++
		}
	}
	return n
}
