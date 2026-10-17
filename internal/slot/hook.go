package slot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/gitx"
)

// HookDeps is what `wk hook pre-tool-use` hands HookPreToolUse, so a test can run it from a
// temporary home with a fake clock and resolver.
type HookDeps struct {
	Exe     string                                    // the absolute wk binary
	Home    string                                    // WK_HOME
	LogDir  string                                    // where missed.log and errors.log go
	Now     func() time.Time                          // nil means time.Now
	Resolve func(cwd string) (repo string, err error) // nil means one `git rev-parse --path-format=absolute --git-common-dir` and wk's naming rule
}

// Claude Code's foreground maximum for a Bash call while BASH_MAX_TIMEOUT_MS is unset. A
// synchronous subagent's gate that waits for a place past its call's timeout is moved to the
// background, where the subagent's final response kills it (SLOTS-PLAN.md 0.1, SB-16).
const hookBashTimeoutMS = 600000

type hookInput struct {
	ToolName  string                     `json:"tool_name"`
	Cwd       string                     `json:"cwd"`
	ToolInput map[string]json.RawMessage `json:"tool_input"`
}

type hookSpecific struct {
	HookEventName            string         `json:"hookEventName"`
	PermissionDecision       string         `json:"permissionDecision,omitempty"`
	PermissionDecisionReason string         `json:"permissionDecisionReason,omitempty"`
	UpdatedInput             map[string]any `json:"updatedInput,omitempty"`
}

type hookOutput struct {
	HookSpecificOutput hookSpecific `json:"hookSpecificOutput"`
}

// HookPreToolUse reads one PreToolUse input from in, resolves the repository name from
// tool_input's cwd (or the input's cwd) with deps.Resolve, loads <deps.Home>/<repo>.slots.toml,
// and prints at most one JSON document to out: updatedInput with the whole tool_input, command
// replaced and timeout set to 600000 when absent or lower (SB-16). It prints nothing when there
// is no file, no match, a parse error or the deadline passes, and it always returns 0; with
// FailOpen false and a broken file it prints a permissionDecision "deny" document, still 0
// (SB-14). Misses and errors go to <deps.LogDir>/missed.log and errors.log.
//
// It returns 0 even after a panic: exit 2 from a PreToolUse hook blocks the call, and this hook
// runs before every Bash call of every hooked session.
func HookPreToolUse(in io.Reader, out io.Writer, deps HookDeps) int {
	h := hookNewRun(deps)
	defer func() {
		if r := recover(); r != nil {
			h.logError(fmt.Sprintf("panic: %v", r))
		}
	}()
	if doc := h.decide(in); doc != nil {
		_, _ = out.Write(doc)
	}
	return 0
}

type hookRun struct {
	deps  HookDeps
	start time.Time
	repo  string
}

func hookNewRun(deps HookDeps) *hookRun {
	if deps.Now == nil {
		deps.Now = time.Now
	}
	if deps.Resolve == nil {
		deps.Resolve = hookResolveRepo
	}
	return &hookRun{deps: deps, start: deps.Now()}
}

func (h *hookRun) decide(in io.Reader) []byte {
	input, command, ok := h.read(in)
	if !ok {
		return nil
	}
	sf, found, err := h.load(input)
	if err != nil {
		return h.broken(sf, err)
	}
	if !found {
		return nil
	}
	doc := h.rewrite(input.ToolInput, command, sf)
	if doc == nil || h.late(sf.Hook) {
		return nil
	}
	return doc
}

func (h *hookRun) read(in io.Reader) (hookInput, string, bool) {
	var input hookInput
	if !filepath.IsAbs(h.deps.Exe) || !filepath.IsAbs(h.deps.Home) {
		h.logError(fmt.Sprintf("wk %q and WK_HOME %q must be absolute", h.deps.Exe, h.deps.Home))
		return input, "", false
	}
	if err := json.NewDecoder(in).Decode(&input); err != nil {
		h.logError(fmt.Sprintf("PreToolUse input: %v", err))
		return input, "", false
	}
	var command string
	if input.ToolName != "Bash" || json.Unmarshal(input.ToolInput["command"], &command) != nil {
		return input, "", false
	}
	return input, command, command != ""
}

func (h *hookRun) load(input hookInput) (SlotsFile, bool, error) {
	cwd := input.Cwd
	var toolCwd string
	if json.Unmarshal(input.ToolInput["cwd"], &toolCwd) == nil && toolCwd != "" {
		cwd = toolCwd
	}
	repo, err := h.deps.Resolve(cwd)
	if err != nil {
		return SlotsFile{Hook: slotsFileHookDefaults()}, false, fmt.Errorf("repository of %s: %w", cwd, err)
	}
	h.repo = repo
	return LoadSlotsFile(filepath.Join(h.deps.Home, repo+".slots.toml"))
}

func (h *hookRun) broken(sf SlotsFile, err error) []byte {
	h.logError(err.Error())
	if sf.Hook.FailOpen {
		return nil
	}
	reason := fmt.Sprintf("wk hook pre-tool-use: %v; hook.fail_open is false, so every Bash call is refused until the slots file is fixed", err)
	return h.encode(hookSpecific{HookEventName: "PreToolUse", PermissionDecision: "deny", PermissionDecisionReason: reason})
}

func (h *hookRun) rewrite(toolInput map[string]json.RawMessage, command string, sf SlotsFile) []byte {
	out, changed, missed, parseErr := Rewrite(command, h.deps.Exe, sf)
	for _, m := range missed {
		h.log("missed.log", fmt.Sprintf("%s %s: %q in %q", h.repo, m.Reason, m.Command, command))
	}
	if parseErr != nil || !changed {
		return nil
	}
	updated := make(map[string]any, len(toolInput)+1)
	for k, v := range toolInput {
		updated[k] = v
	}
	updated["command"] = out
	if hookTimeoutBelowMax(toolInput["timeout"]) {
		updated["timeout"] = hookBashTimeoutMS
	}
	return h.encode(hookSpecific{HookEventName: "PreToolUse", UpdatedInput: updated})
}

// An absent, null or non-numeric timeout counts as below the maximum: Claude Code's own default
// for an absent one is 120 s.
func hookTimeoutBelowMax(raw json.RawMessage) bool {
	var ms float64
	if json.Unmarshal(raw, &ms) != nil {
		return true
	}
	return ms < hookBashTimeoutMS
}

// late checks the deadline once, just before the single write, so a document is printed whole
// or not at all. The git run and the file read count against it; Claude Code's own 5 s timeout
// on the settings entry bounds a git run that hangs.
func (h *hookRun) late(hook HookConfig) bool {
	elapsed := h.deps.Now().Sub(h.start)
	if elapsed <= time.Duration(hook.TimeoutMS)*time.Millisecond {
		return false
	}
	h.logError(fmt.Sprintf("deadline of %d ms passed after %d ms; the command runs unslotted", hook.TimeoutMS, elapsed.Milliseconds()))
	return true
}

// encode leaves &, < and > unescaped so the printed command reads as the agent will see it.
func (h *hookRun) encode(specific hookSpecific) []byte {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(hookOutput{HookSpecificOutput: specific}); err != nil {
		h.logError(fmt.Sprintf("encode the hook output: %v", err))
		return nil
	}
	return buf.Bytes()
}

func (h *hookRun) logError(msg string) {
	h.log("errors.log", "hook pre-tool-use "+h.repo+": "+msg)
}

// log appends one line that starts with an RFC 3339 time, as hook.log's lines do, so `wk slots`
// can count the last 24 hours. A failure to log is dropped: the hook must still answer.
func (h *hookRun) log(name, line string) {
	if h.deps.LogDir == "" || os.MkdirAll(h.deps.LogDir, 0o755) != nil {
		return
	}
	f, err := os.OpenFile(filepath.Join(h.deps.LogDir, name), os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	_, _ = f.WriteString(h.deps.Now().Format(time.RFC3339) + " " + strings.ReplaceAll(line, "\n", " ") + "\n")
	_ = f.Close()
}

// hookResolveRepo runs only the first of gitx.Resolve's two git commands (SLOTS-PLAN.md 0.2),
// because the hook runs before every Bash call, and names the repository by the same rule, so
// the slots file sits beside the ~/.wk/<repo>.toml that wk reads.
func hookResolveRepo(cwd string) (string, error) {
	if !filepath.IsAbs(cwd) {
		return "", fmt.Errorf("%q is relative; wk takes absolute paths only", cwd)
	}
	common, err := gitx.Run(cwd, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return "", err
	}
	return filepath.Base(filepath.Dir(common)), nil
}
