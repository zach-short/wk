package wk

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/gitx"
)

// StoppedMarkerName is the file the SubagentStop hook leaves in a subagent worktree's private
// git directory. gc reads it as: the agent that owned this worktree has finished a turn, so a
// short quiet window is enough (gc.stopped_quiet_minutes).
const StoppedMarkerName = "wk-agent-stopped"

// agentWorktree is how Claude Code 2.1.301 names a subagent's worktree: "agent-" and the agent
// id, which is "a" and 16 hex digits (the regex in the binary; PLAN.md phase 3 As-built).
var agentWorktree = regexp.MustCompile(`^agent-a[0-9a-f]{16}$`)

func isAgentWorktree(path string) bool { return agentWorktree.MatchString(filepath.Base(path)) }

// SubagentStopInput is the part of the SubagentStop input wk reads. cwd is the subagent's own
// working directory: its worktree when it ran with isolation, its parent's otherwise.
type SubagentStopInput struct {
	AgentID string `json:"agent_id"`
	Cwd     string `json:"cwd"`
}

// SubagentStop marks the stopped subagent's worktree. It marks nothing unless the worktree
// that holds cwd is named for this very agent, so a subagent that ran without isolation never
// marks its parent's worktree. It never fails the stop: it returns what it did, for the log.
func SubagentStop(home string, in io.Reader) string {
	var input SubagentStopInput
	if err := json.NewDecoder(in).Decode(&input); err != nil {
		return "unreadable input: " + err.Error()
	}
	name := "agent-" + input.AgentID
	if !agentWorktree.MatchString(name) || !filepath.IsAbs(input.Cwd) {
		return fmt.Sprintf("agent %q in %q runs in no worktree of its own", input.AgentID, input.Cwd)
	}
	top, err := gitx.Run(input.Cwd, "rev-parse", "--show-toplevel")
	if err != nil || filepath.Base(top) != name {
		return fmt.Sprintf("%s is not %s's worktree; nothing marked", input.Cwd, name)
	}
	gd, err := privateGitDir(top)
	if err != nil {
		return err.Error()
	}
	log, closeLog := openHookLogFor(home, top)
	defer closeLog()
	stamp := time.Now().Format(time.RFC3339)
	if err := os.WriteFile(filepath.Join(gd, StoppedMarkerName), []byte(stamp+"\n"), 0o644); err != nil {
		_, _ = fmt.Fprintf(log, "%s SubagentStop %s: marker not written: %v\n", stamp, top, err)
		return err.Error()
	}
	_, _ = fmt.Fprintf(log, "%s SubagentStop marked %s\n", stamp, top)
	return "marked " + top
}

// WorktreeRemoveInput is the WorktreeRemove input wk reads (PLAN.md 0.1).
type WorktreeRemoveInput struct {
	WorktreePath string `json:"worktree_path"`
}

// WorktreeRemove answers Claude Code's WorktreeRemove hook, which it sends only when the owner
// discards a worktree explicitly. The owner has asked for the worktree to go, so it goes with
// --force; its branch goes only when the base already holds it (BD-11). It refuses the main
// checkout, anything that is not a worktree of the repository, and wk's own template and pool
// slots, none of which Claude Code made.
func WorktreeRemove(home string, in io.Reader) (string, error) {
	var input WorktreeRemoveInput
	if err := json.NewDecoder(in).Decode(&input); err != nil {
		return "", fmt.Errorf("WorktreeRemove input: %w", err)
	}
	path := input.WorktreePath
	if !filepath.IsAbs(path) {
		return "", fmt.Errorf("worktree_path %q is not absolute", path)
	}
	repo, err := gitx.Resolve(path)
	if err != nil {
		return "", err
	}
	w, found := worktreeAt(repo.Primary, path)
	switch {
	case physical(path) == physical(repo.Primary):
		return "", fmt.Errorf("%s is the main checkout; wk never removes it", path)
	case !found:
		return "", fmt.Errorf("%s is not a worktree of %s", path, repo.Primary)
	}
	if gd, err := privateGitDir(path); err == nil && (exists(filepath.Join(gd, MarkerName)) || exists(filepath.Join(gd, PoolMarkerName))) {
		return "", fmt.Errorf("%s belongs to wk itself, not to a session", path)
	}
	base := config.Defaults().Workspace.Base
	if env, err := Open(home, repo.Primary, io.Discard); err == nil {
		base = env.Cfg.Workspace.Base
	}
	log, closeLog := openHookLog(home, repo.Name)
	defer closeLog()
	if _, err := gitx.Run(repo.Primary, "worktree", "remove", "--force", path); err != nil {
		_, _ = fmt.Fprintf(log, "%s WorktreeRemove %s failed: %v\n", time.Now().Format(time.RFC3339), path, err)
		return "", err
	}
	what := "removed " + path
	if w.Branch != "" {
		what += "; " + deleteBranchIfHeld(repo.Primary, w.Branch, base)
	}
	_, _ = fmt.Fprintf(log, "%s WorktreeRemove %s\n", time.Now().Format(time.RFC3339), what)
	return what, nil
}

func deleteBranchIfHeld(primary, branch, base string) string {
	if !gitx.IsAncestor(primary, "refs/heads/"+branch, "refs/heads/"+base) {
		return "branch " + branch + " kept: " + base + " does not hold it"
	}
	if _, err := gitx.Run(primary, "branch", "-d", branch); err != nil {
		return "branch " + branch + " kept: " + err.Error()
	}
	return "branch " + branch + " deleted"
}

func worktreeAt(primary, dir string) (gitx.Worktree, bool) {
	list, err := gitx.Worktrees(primary)
	if err != nil {
		return gitx.Worktree{}, false
	}
	for _, w := range list {
		if physical(w.Path) == physical(dir) && !w.Prunable {
			return w, true
		}
	}
	return gitx.Worktree{}, false
}

// openHookLogFor opens hook.log for the repository that holds dir.
func openHookLogFor(home, dir string) (io.Writer, func()) {
	repo, err := gitx.Resolve(dir)
	if err != nil {
		return io.Discard, func() {}
	}
	return openHookLog(home, repo.Name)
}
