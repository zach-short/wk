package wk

import (
	"fmt"
	"io"

	"github.com/zach-short/wk/internal/changeset"
)

// Status is `wk status`: the leases, then the landing ref against the base branch and every
// changeset with its state and reason (PLAN-P2-P4.md section 3, item 5).
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
	for _, p := range r.Paths {
		_, _ = fmt.Fprintf(out, "  conflict: %s\n", p)
	}
}
