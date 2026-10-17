// Package batch is the pure policy of P4 (DESIGN.md D16, options T and U): which submitted
// changesets form a batch, how a red batch splits, and which changeset a red gate's paths
// blame. It runs no git and writes nothing; the landing worker calls it.
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

// Halves splits a red batch in two in queue order, the first half holding the extra record of
// an odd batch, so each half re-gates as a batch of its own (bisection, D16 as amended
// 2026-10-16; it replaces the drop-the-last fallback T1). ok is false on a batch of fewer than
// two: a lone red changeset is the one that failed, and there is nothing left to split.
func Halves(batch []changeset.Record) (first, second []changeset.Record, ok bool) {
	if len(batch) < 2 {
		return batch, nil, false
	}
	mid := (len(batch) + 1) / 2
	// Capped at mid, so a caller appending to first cannot overwrite second in batch.
	return batch[:mid:mid], batch[mid:], true
}
