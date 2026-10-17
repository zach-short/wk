// Package batch is the pure policy of P4 (DESIGN.md D16, options T and U): which submitted
// changesets form a batch, how a red batch shrinks, and which changeset a red gate's paths
// blame. It runs no git and writes nothing; the daemon's landing loop calls it.
package batch

import (
	"time"

	"github.com/zach-short/wk/internal/changeset"
)

// Pick returns the first max records in state submitted, in queue order, once they are ready
// to gate: there are already max of them, or the newest of them is at least idle old at now.
// Waiting for quiet is what makes a burst of submits one batch, not several (option U). It
// returns nil when nothing is ready.
func Pick(records []changeset.Record, max int, idle time.Duration, now time.Time) []changeset.Record {
	if max < 1 {
		return nil
	}
	var picked []changeset.Record
	var newest time.Time
	for _, r := range records {
		if len(picked) == max {
			break
		}
		if r.State != changeset.Submitted {
			continue
		}
		picked = append(picked, r)
		if r.At.After(newest) {
			newest = r.At
		}
	}
	if len(picked) == 0 || (len(picked) < max && now.Sub(newest) < idle) {
		return nil
	}
	return picked
}

// Shrink drops the last changeset of a red batch so the rest re-gate without it (T1). ok is
// false on a batch of fewer than two, which comes back as kept: a lone red changeset is the
// one that failed, and there is nothing left to re-gate.
func Shrink(batch []changeset.Record) (kept []changeset.Record, dropped changeset.Record, ok bool) {
	if len(batch) < 2 {
		return batch, changeset.Record{}, false
	}
	last := len(batch) - 1
	// Capped at last, so a caller appending to kept cannot overwrite the dropped record in batch.
	return batch[:last:last], batch[last], true
}
