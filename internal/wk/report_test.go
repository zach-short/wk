package wk

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/zach-short/landing-layer/internal/testrepo"
)

func TestGCJSONReportsTheVerdictsAndChangesNothing(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	dir := addWorktree(t, f, "candidate")
	var out bytes.Buffer
	if err := e.GCJSON(&out); err != nil {
		t.Fatal(err)
	}
	assertStays(t, dir)
	var r Report
	if err := json.Unmarshal(out.Bytes(), &r); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, out.String())
	}
	if r.Base != e.Cfg.Workspace.Base || r.Removable != 1 || len(r.Worktrees) != 1 {
		t.Fatalf("report = %+v", r)
	}
	item := r.Worktrees[0]
	if item.Path != ".claude/worktrees/candidate" || item.Verdict != Removable || !item.DeleteBranch {
		t.Fatalf("item = %+v", item)
	}
	if item.LastActivity == "" || item.Reasons == nil {
		t.Fatalf("item lacks activity or reasons: %+v", item)
	}
}

func TestGCJSONWithNoWorktreesIsAnEmptyList(t *testing.T) {
	f := testrepo.New(t)
	e := idle(t, f)
	var out bytes.Buffer
	if err := e.GCJSON(&out); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out.Bytes(), []byte(`"worktrees": []`)) {
		t.Fatalf("want an empty list, not null:\n%s", out.String())
	}
}
