package wk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/slot"
)

// The hook.log shapes of 2026-10-16 to 10-26: a claim from the pool; a template clone after
// "no ready sparse slot" (which names the pool but is not a claim); a plain fallback; a reuse;
// a warm create on a day outside the range.
const hookLogFixture = `2026-10-20T14:11:19-04:00 WorktreeCreate name=a cwd=/p/ezh base=d534a262c7
wk: claim pool slot wk-pool-8b93e3c0   0.06s
wk: checkout -b worktree-a   0.53s
2026-10-20T14:11:20-04:00 warm /p/ezh/.claude/worktrees/a in 0.66s
2026-10-20T14:12:00-04:00 WorktreeCreate name=b cwd=/p/ezh base=d534a262c7
wk: pool: no ready sparse slot
wk: worktree add --no-checkout         0.07s
wk: clone node_modules   1.20s
2026-10-20T14:12:30-04:00 warm /p/ezh/.claude/worktrees/b in 28.30s
2026-10-20T14:13:00-04:00 WorktreeCreate name=c cwd=/p/ezh base=d534a262c7
wk: claim pool slot wk-pool-11111111   0.05s
2026-10-20T14:13:02-04:00 warm /p/ezh/.claude/worktrees/c in 2.00s
2026-10-20T14:14:00-04:00 WorktreeCreate name=d cwd=/p/ezh base=d534a262c7
warm workspace failed: WK_HOOK_FAIL is set
2026-10-20T14:14:02-04:00 plain fallback /p/ezh/.claude/worktrees/d in 2.33s
2026-10-20T14:15:00-04:00 reused /p/ezh/.claude/worktrees/a in 0.01s
2026-10-20T14:16:00-04:00 warm not-a-number in 1.2.3s
2026-10-30T09:00:00-04:00 WorktreeCreate name=e cwd=/p/ezh base=d534a262c7
wk: claim pool slot wk-pool-22222222   0.05s
2026-10-30T09:00:05-04:00 warm /p/ezh/.claude/worktrees/e in 5.00s
`

// Two creates whose lines cross: p takes the pool, t clones the template, and the claim line
// comes before the template clone line, so a single "claimed" flag called both of them pool.
// Neither can be attributed. A warm line with no WorktreeCreate line is not attributed either.
const crossedHookLog = `2026-10-21T09:00:00-04:00 WorktreeCreate name=p cwd=/p/ezh base=d534a262c7
2026-10-21T09:00:00-04:00 WorktreeCreate name=t cwd=/p/ezh base=d534a262c7
wk: claim pool slot wk-pool-33333333   0.05s
wk: pool: no ready sparse slot
wk: worktree add --no-checkout         0.07s
2026-10-21T09:00:01-04:00 warm /p/ezh/.claude/worktrees/p in 1.00s
2026-10-21T09:00:30-04:00 warm /p/ezh/.claude/worktrees/t in 30.00s
2026-10-21T09:01:00-04:00 warm /p/ezh/.claude/worktrees/orphan in 3.00s
2026-10-21T09:02:00-04:00 WorktreeCreate name=after cwd=/p/ezh base=d534a262c7
wk: claim pool slot wk-pool-44444444   0.05s
2026-10-21T09:02:01-04:00 warm /p/ezh/.claude/worktrees/after in 0.50s
`

func warmFixture(t *testing.T, logs map[string]string) []WarmStats {
	t.Helper()
	home := t.TempDir()
	for repo, log := range logs {
		if err := os.MkdirAll(filepath.Join(home, repo), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(home, repo, "hook.log"), []byte(log), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	r, err := slot.ParseRange("2026-10-17", "2026-10-26")
	if err != nil {
		t.Fatal(err)
	}
	s, err := ReadStats(home, r)
	if err != nil {
		t.Fatal(err)
	}
	if text := s.Text(); len(s.Warm) > 0 && !strings.Contains(text, "  "+s.Warm[0].Repo+"\n    pool:         ") {
		t.Errorf("text lacks the %s pool line:\n%s", s.Warm[0].Repo, text)
	}
	return s.Warm
}

func TestReadStatsSplitsWarmCreatesByPoolAndTemplate(t *testing.T) {
	t.Parallel()
	warm := warmFixture(t, map[string]string{"ezh": hookLogFixture, "quiet": "2026-10-20T14:11:19-04:00 SubagentStop x\n"})
	if len(warm) != 1 || warm[0].Repo != "ezh" {
		t.Fatalf("warm %+v, want ezh alone: quiet has no warm create", warm)
	}
	w := warm[0]
	if w.Pool != (slot.Distribution{Count: 2, Total: 2.66, Median: 0.66, P90: 2}) {
		t.Errorf("pool %+v, want a and c", w.Pool)
	}
	if w.Template != (slot.Distribution{Count: 1, Total: 28.3, Median: 28.3, P90: 28.3}) {
		t.Errorf("template %+v, want b alone", w.Template)
	}
	if w.Unattributed.Count != 0 {
		t.Errorf("unattributed %+v, want none", w.Unattributed)
	}
}

func TestCrossedCreatesAreUnattributed(t *testing.T) {
	t.Parallel()
	warm := warmFixture(t, map[string]string{"ezh": crossedHookLog})
	if len(warm) != 1 {
		t.Fatalf("warm %+v, want ezh", warm)
	}
	w := warm[0]
	if w.Pool.Count != 1 || w.Pool.Total != 0.5 {
		t.Errorf("pool %+v, want the create after the crossing alone", w.Pool)
	}
	if w.Template.Count != 0 {
		t.Errorf("template %+v, want none", w.Template)
	}
	if w.Unattributed != (slot.Distribution{Count: 3, Total: 34, Median: 3, P90: 30}) {
		t.Errorf("unattributed %+v, want p, t and the orphan", w.Unattributed)
	}
}
