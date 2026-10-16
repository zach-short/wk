package procs

import (
	"os"
	"testing"
)

func TestParseAndIn(t *testing.T) {
	list := parse("p10\ncclaude\nfcwd\nn/repo/.claude/worktrees/a\np11\ncsleep\nfcwd\nn/repo/.claude/worktrees/a/web\np12\nczsh\nfcwd\nn/repo/.claude/worktrees/ab\n")
	if len(list) != 3 || list[0].PID != 10 || list[0].Command != "claude" || list[2].Cwd != "/repo/.claude/worktrees/ab" {
		t.Fatalf("parse = %+v", list)
	}
	inside := In(list, "/repo/.claude/worktrees/a")
	if len(inside) != 2 || inside[0].PID != 10 || inside[1].PID != 11 {
		t.Fatalf("In = %+v; want the worktree and its subdirectory, never the sibling ab", inside)
	}
}

func TestSnapshotHoldsThisProcess(t *testing.T) {
	list, err := Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range list {
		if p.PID == os.Getpid() {
			if p.Cwd == "" || p.Cwd[0] != '/' {
				t.Fatalf("own entry %+v has no absolute cwd", p)
			}
			return
		}
	}
	t.Fatal("the snapshot does not list this process")
}
