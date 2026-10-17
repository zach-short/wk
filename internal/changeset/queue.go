package changeset

import (
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"
)

// The states a changeset moves through, one appended line each (PD-8). P3 writes the first
// four; P4 adds gated and failed. Landing is the worker's journal line before its
// compare-and-swap (owner amendment of 2026-10-16, the kill test's F18): its Tree is the
// predicted tree, so a crash between the swap and the landed line is found on the next pass.
const (
	Submitted = "submitted"
	Landing   = "landing"
	Refused   = "refused"
	Landed    = "landed"
	Gated     = "gated"
	Failed    = "failed"
)

// Record is one line of the queue: a changeset at one state. Commit is set on landed, Paths
// on refused (the conflicts) and failed (the paths the gate named), Gate and Reason on gated
// and failed. Review names the changeset's review, which landing.require_review demands.
type Record struct {
	ID        string    `json:"id"`
	State     string    `json:"state"`
	Tree      string    `json:"tree"`
	Base      string    `json:"base"`
	Message   string    `json:"message"`
	Row       string    `json:"row,omitempty"`
	Workspace string    `json:"workspace"`
	Review    string    `json:"review,omitempty"`
	Commit    string    `json:"commit,omitempty"`
	Paths     []string  `json:"paths,omitempty"`
	Gate      string    `json:"gate,omitempty"`
	Reason    string    `json:"reason,omitempty"`
	At        time.Time `json:"at"`
}

// Queue is the JSONL file at Path. `wk submit` appends the submitted line and the landing
// worker every later one (PD-8 as amended); a reader folds it with Load.
type Queue struct {
	Path string
}

// Append writes r as one line and syncs it, so the line is whole on disk before the daemon
// answers the request that caused it.
func (q Queue) Append(r Record) error {
	line, err := json.Marshal(r)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(q.Path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return err
	}
	if _, err = f.Write(append(line, '\n')); err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	return err
}

// Load folds the file to the current state of every changeset, its last line, in the order
// the ids were first seen, which is queue order. A missing file is an empty queue; a line
// that does not parse is an error naming its number.
func (q Queue) Load() ([]Record, error) {
	data, err := os.ReadFile(q.Path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []Record
	at := map[string]int{}
	for n, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		var r Record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			return nil, fmt.Errorf("%s line %d: %w", q.Path, n+1, err)
		}
		if i, seen := at[r.ID]; seen {
			out[i] = r
			continue
		}
		at[r.ID] = len(out)
		out = append(out, r)
	}
	return out, nil
}

// NewID names a changeset by the first 12 hex of the SHA-1 of its tree, base and submit time
// joined by newlines (PD-13), so the same tree from the same base submitted twice is two
// changesets.
func NewID(tree, base string, at time.Time) string {
	sum := sha1.Sum([]byte(tree + "\n" + base + "\n" + at.UTC().Format(time.RFC3339Nano)))
	return hex.EncodeToString(sum[:])[:12]
}
