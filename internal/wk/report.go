package wk

import (
	"encoding/json"
	"io"
	"time"
)

// Report is the machine-readable form of GC. The visualizer reads it, so a field name here is a
// contract: rename one only with the page that reads it.
type Report struct {
	Base      string       `json:"base"`
	Generated string       `json:"generated"`
	Removable int          `json:"removable"`
	Worktrees []ReportItem `json:"worktrees"`
}

// ReportItem is one worktree's verdict as Report lists it.
type ReportItem struct {
	Path         string   `json:"path"`
	Branch       string   `json:"branch,omitempty"`
	Head         string   `json:"head"`
	Verdict      Verdict  `json:"verdict"`
	Reasons      []string `json:"reasons"`
	Dirty        int      `json:"dirty"`
	LastActivity string   `json:"last_activity,omitempty"`
	DeleteBranch bool     `json:"delete_branch"`
	// Held counts the changed paths that are untracked files the base holds byte for byte;
	// --apply deletes them with the worktree. It is 0 unless every changed path is held.
	Held int `json:"held"`
}

// GCJSON writes the dry-run judgments as one JSON document. Like GC, it changes nothing.
func (e *Env) GCJSON(out io.Writer) error {
	judgments, err := e.judgeAll()
	if err != nil {
		return err
	}
	r := Report{
		Base:      e.Cfg.Workspace.Base,
		Generated: e.clock().Format(time.RFC3339),
		Worktrees: make([]ReportItem, 0, len(judgments)),
	}
	for _, j := range judgments {
		if j.Verdict == Removable {
			r.Removable++
		}
		item := ReportItem{
			Path:         e.display(j.Worktree.Path),
			Branch:       j.Worktree.Branch,
			Head:         j.Worktree.Head,
			Verdict:      j.Verdict,
			Reasons:      j.Reasons,
			Dirty:        j.Dirty,
			DeleteBranch: j.DeleteBranch,
			Held:         len(j.held),
		}
		if !j.Activity.IsZero() {
			item.LastActivity = j.Activity.Format(time.RFC3339)
		}
		if item.Reasons == nil {
			item.Reasons = []string{}
		}
		r.Worktrees = append(r.Worktrees, item)
	}
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}
