package wk

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/zach-short/wk/internal/slot"
)

// The hook.log shapes of 2026-10-16 to 10-26: a claim from the pool, a template clone after
// "no ready sparse slot" (which names the pool but is not a claim), a plain fallback, a reuse,
// and a warm create on a day outside the range.
const hookLogFixture = `2026-10-20T14:11:19-04:00 WorktreeCreate name=a cwd=/p/ezh base=d534a262c7
wk: claim pool slot wk-pool-8b93e3c0   0.06s
wk: checkout -b worktree-a   0.53s
2026-10-20T14:11:20-04:00 warm /p/ezh/.claude/worktrees/a in 0.66s
2026-10-20T14:12:00-04:00 WorktreeCreate name=b cwd=/p/ezh base=d534a262c7
wk: pool: no ready sparse slot
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
2026-10-30T09:00:05-04:00 warm /p/ezh/.claude/worktrees/e in 5.00s
`

func TestReadStatsSplitsWarmCreatesByPoolAndTemplate(t *testing.T) {
	t.Parallel()
	home := t.TempDir()
	for repo, log := range map[string]string{"ezh": hookLogFixture, "quiet": "2026-10-20T14:11:19-04:00 SubagentStop x\n"} {
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
	if len(s.Warm) != 1 || s.Warm[0].Repo != "ezh" {
		t.Fatalf("warm %+v, want ezh alone: quiet has no warm create", s.Warm)
	}
	w := s.Warm[0]
	if w.Pool != (slot.Distribution{Count: 2, Total: 2.66, Median: 0.66, P90: 2}) {
		t.Errorf("pool %+v, want a and c", w.Pool)
	}
	if w.Template != (slot.Distribution{Count: 1, Total: 28.3, Median: 28.3, P90: 28.3}) {
		t.Errorf("template %+v, want b alone", w.Template)
	}
	if text := s.Text(); !strings.Contains(text, "  ezh\n    pool:     2.7s in 2, median 0.7s, p90 2.0s\n") {
		t.Errorf("text lacks the ezh pool line:\n%s", text)
	}
}
