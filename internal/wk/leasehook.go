package wk

import (
	"cmp"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/zach-short/wk/internal/config"
	"github.com/zach-short/wk/internal/lease"
)

// leaseInput is the part of Claude Code's PreToolUse input the lease hook reads (verified on
// 2.1.301, PLAN-P2-P4.md section 0): Edit, Write and MultiEdit carry file_path, NotebookEdit
// carries notebook_path, Bash carries command.
type leaseInput struct {
	Cwd       string `json:"cwd"`
	ToolName  string `json:"tool_name"`
	ToolInput struct {
		FilePath     string `json:"file_path"`
		NotebookPath string `json:"notebook_path"`
		Command      string `json:"command"`
	} `json:"tool_input"`
}

// hookDenial is the PreToolUse deny shape verified on 2.1.301: Claude Code refuses the call and
// shows the reason to the model as the tool's error.
type hookDenial struct {
	HookSpecificOutput struct {
		HookEventName            string `json:"hookEventName"`
		PermissionDecision       string `json:"permissionDecision"`
		PermissionDecisionReason string `json:"permissionDecisionReason"`
	} `json:"hookSpecificOutput"`
}

// LeaseHook answers `wk hook lease --repo <primary>` for one PreToolUse call read from in. A
// deny is printed on out as JSON; an allow prints nothing. It never runs git (PD-2) and its
// caller always exits 0, so Claude Code blocks only on the printed deny; a panic is a deny too,
// because the guard fails closed (D12). The deny is printed before the log line is written, so a
// failure of the log cannot lose it.
func LeaseHook(home string, args []string, in io.Reader, out io.Writer) {
	start := time.Now()
	h := leaseHook{home: home}
	d := h.decide(args, in)
	if !d.Allow {
		LeaseDeny(out, d.Reason)
	}
	defer func() { _ = recover() }()
	h.logLine(d, time.Since(start))
}

// LeaseDeny prints Claude Code's PreToolUse deny with reason on out. The command line calls it for
// a panic outside LeaseHook, so every failure of the hook verb refuses the call.
func LeaseDeny(out io.Writer, reason string) {
	var o hookDenial
	o.HookSpecificOutput.HookEventName = "PreToolUse"
	o.HookSpecificOutput.PermissionDecision = "deny"
	o.HookSpecificOutput.PermissionDecisionReason = reason
	_ = json.NewEncoder(out).Encode(o)
}

// leaseHook carries one call's state, so the log line can name what was judged.
type leaseHook struct {
	home, name string
	input      leaseInput
	note       string
}

func (h *leaseHook) decide(args []string, in io.Reader) (d lease.Decision) {
	defer func() {
		if r := recover(); r != nil {
			d = lease.Decision{Reason: fmt.Sprintf("wk: the lease hook failed (%v); report it, and edit after `wk hooks uninstall` only if the owner agrees", r)}
		}
	}()
	primary, err := hookPrimary(args, h.home)
	if err != nil {
		return lease.Decision{Reason: "wk: " + err.Error()}
	}
	h.name = filepath.Base(primary)
	if err := json.NewDecoder(in).Decode(&h.input); err != nil {
		return lease.Decision{Reason: fmt.Sprintf("wk: cannot read the hook input: %v", err)}
	}
	if h.input.ToolName == "Bash" && h.skipsBash() {
		return lease.Decision{Allow: true}
	}
	return h.judge(primary)
}

// hookPrimary reads `--repo <primary>`; the hook takes its repository from the command line, never
// from git.
func hookPrimary(args []string, home string) (string, error) {
	if len(args) != 2 || args[0] != "--repo" || !filepath.IsAbs(args[1]) {
		return "", fmt.Errorf("the lease hook needs --repo <absolute main checkout>; got %q; run `wk hooks install` again", strings.Join(args, " "))
	}
	if !filepath.IsAbs(home) {
		return "", fmt.Errorf("WK_HOME %q is not absolute", home)
	}
	return args[1], nil
}

// skipsBash allows, before any file is read, a command that runs only the wk binary, so `wk claim`
// can never be locked out, and a command with no write target the walk recognises (owner
// amendments, 2026-10-16).
func (h *leaseHook) skipsBash() bool {
	cmd := h.input.ToolInput.Command
	return runsOnlyWK(cmd) || len(lease.WriteTargets(h.input.Cwd, cmd)) == 0
}

// runsOnlyWK is true for a single wk command: the first word is the wk binary and no operator,
// redirection or substitution follows, so `wk claim 3; rm web/x.ts` is judged like any command.
func runsOnlyWK(cmd string) bool {
	fields := strings.Fields(cmd)
	return len(fields) > 0 && filepath.Base(strings.Trim(fields[0], `'"`)) == "wk" &&
		!strings.ContainsAny(cmd, ";&|<>\n`") && !strings.Contains(cmd, "$(")
}

// judge reads the config and the store and judges the call. With leases off it allows and logs.
func (h *leaseHook) judge(primary string) lease.Decision {
	cfgPath := filepath.Join(h.home, h.name+".toml")
	cfg, _, err := config.Load(cfgPath)
	if err != nil {
		return lease.Decision{Reason: fmt.Sprintf("wk: cannot read %v; fix the file, then try again", err)}
	}
	if !cfg.Leases.Enabled {
		h.note = "leases.enabled is false"
		return lease.Decision{Allow: true}
	}
	f, err := loadLeases(filepath.Join(h.home, h.name))
	if err != nil {
		return lease.Decision{Reason: fmt.Sprintf("wk: cannot read the lease store: %v", err)}
	}
	if f.Primary != "" && f.Primary != primary {
		return lease.Decision{Reason: fmt.Sprintf("wk: %s belongs to %s, not %s; two repositories share the name %s", leasesPath(filepath.Join(h.home, h.name)), f.Primary, primary, h.name)}
	}
	return h.judgeFolded(f, cfg.Leases, primary)
}

// judgeFolded judges with every path folded to lower case on a volume that ignores case (audit 1,
// B1): there `WEB/Page.ts` is the file `web/page.ts` leases, and the judgment compares bytes.
func (h *leaseHook) judgeFolded(f leaseFile, l config.Leases, primary string) lease.Decision {
	fold := func(p string) string { return p }
	if caseInsensitive(primary) {
		fold = strings.ToLower
	}
	req, err := h.request()
	if err != nil {
		return lease.Decision{Reason: "wk: " + err.Error()}
	}
	roots := foldAll(h.roots(f, primary, req), fold)
	canon := req.Canon
	req.Path, req.Cwd = fold(req.Path), fold(req.Cwd)
	req.Canon = func(p string) string { return fold(canon(p)) }
	if d := checkTargets(roots, req, fold); !d.Allow {
		return d
	}
	store := foldStore(f.Store, fold)
	return lease.Judge(&store, lease.Rules{SingleWriter: foldAll(l.SingleWriter, fold), BashGuard: l.BashGuard}, roots, req)
}

// caseInsensitive reports whether the volume holding primary ignores letter case, as APFS and HFS+
// do by default: `.git` and `.GIT` name one entry. When it cannot tell it says yes, because
// folding on a case-sensitive volume only refuses more.
func caseInsensitive(primary string) bool {
	a, errA := os.Lstat(filepath.Join(primary, ".git"))
	b, errB := os.Lstat(filepath.Join(primary, ".GIT"))
	return errA != nil || errB == nil && os.SameFile(a, b)
}

func foldAll(paths []string, fold func(string) string) []string {
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = fold(p)
	}
	return out
}

func foldStore(s lease.Store, fold func(string) string) lease.Store {
	out := lease.Store{Workspaces: map[string]lease.Workspace{}}
	for root, ws := range s.Workspaces {
		ws.Leases = foldAll(ws.Leases, fold)
		out.Workspaces[fold(root)] = ws
	}
	return out
}

// request builds the judgment's request with the edit's path and the cwd read as the kernel reads
// them, and Canon doing the same for each Bash target; a link loop or an unreadable link is an
// error for an edit and an unplaceable target for Bash, so both deny.
func (h *leaseHook) request() (lease.Request, error) {
	ti := h.input.ToolInput
	canon := func(p string) string {
		k, ok := kernelPath(p)
		if !ok {
			return "<a symlink loop or an unreadable link in " + p + ">"
		}
		return k
	}
	req := lease.Request{Tool: h.input.ToolName, Cwd: canon(h.input.Cwd), Command: ti.Command, Canon: canon}
	if p := cmp.Or(ti.FilePath, ti.NotebookPath); p != "" {
		if !filepath.IsAbs(p) && filepath.IsAbs(h.input.Cwd) {
			p = h.input.Cwd + "/" + p
		}
		req.Path = canon(p)
		if strings.HasPrefix(req.Path, "<") {
			return req, fmt.Errorf("cannot tell which file this writes: %s", req.Path)
		}
	}
	return req, nil
}

// targets are the absolute paths the call writes, as the kernel reads them: the edit's file, or
// every placed target of the Bash walk.
func targets(req lease.Request) []string {
	var out []string
	if req.Tool != "Bash" {
		if filepath.IsAbs(req.Path) {
			out = append(out, req.Path)
		}
		return out
	}
	for _, t := range lease.WriteTargets(req.Cwd, req.Command) {
		if t = req.Canon(t); filepath.IsAbs(t) {
			out = append(out, t)
		}
	}
	return out
}

// roots are the store's roots, the main checkout, and any worktree of this repository found on
// disk around the session's directory or a target, so a worktree made since the last claim is
// its own root, and unclaimed, rather than part of the main checkout.
func (h *leaseHook) roots(f leaseFile, primary string, req lease.Request) []string {
	roots := append(slices.Clone(f.Roots), canonical(primary))
	common := f.CommonDir
	if common == "" {
		common = filepath.Join(primary, ".git")
	}
	for _, p := range append(targets(req), req.Cwd) {
		if !filepath.IsAbs(p) {
			continue
		}
		if r, ok := worktreeOf(p, canonical(common)); ok && !slices.Contains(roots, r) {
			roots = append(roots, r)
		}
	}
	return roots
}

// worktreeOf finds the nearest directory at or above p holding a .git entry, and reports
// whether that entry is the common git directory or a worktree's link into it.
func worktreeOf(p, common string) (string, bool) {
	for d := p; ; d = filepath.Dir(d) {
		gitPath := filepath.Join(d, ".git")
		if info, err := os.Lstat(gitPath); err == nil {
			return d, (info.IsDir() && canonical(gitPath) == common) || linksInto(gitPath, d, common)
		}
		if d == filepath.Dir(d) {
			return "", false
		}
	}
}

// linksInto reads a worktree's .git file, "gitdir: <common>/worktrees/<name>".
func linksInto(gitFile, dir, common string) bool {
	data, err := os.ReadFile(gitFile)
	if err != nil {
		return false
	}
	target, ok := strings.CutPrefix(strings.TrimSpace(string(data)), "gitdir: ")
	if !ok {
		return false
	}
	if !filepath.IsAbs(target) {
		target = filepath.Join(dir, target)
	}
	return strings.HasPrefix(canonical(target), common+string(filepath.Separator)+"worktrees"+string(filepath.Separator))
}

// checkTargets denies a write into another worktree than the session's own: the judgment keys a
// path to the root that holds it, so without this a session could edit another worktree's files
// under that worktree's claim (row 7 close-out). A session whose cwd is in no worktree of the
// repository may write no worktree at all (audit 1, B2). On a case-folding volume it also denies a
// non-ASCII path, whose other spellings (Unicode normalisation) it cannot compare.
func checkTargets(roots []string, req lease.Request, fold func(string) string) lease.Decision {
	own, _, _ := lease.Resolve(roots, "", req.Cwd)
	for _, t := range targets(req) {
		root, rel, in := lease.Resolve(roots, "", t)
		switch {
		case !in:
		case own == "":
			return lease.Decision{Reason: fmt.Sprintf("wk: %s is in the workspace %s, and this session works outside every workspace of the repository (%s); work from inside %s", t, root, req.Cwd, root)}
		case root != own:
			return lease.Decision{Reason: fmt.Sprintf("wk: %s is in the workspace %s, and this session works in %s; edit the copy in %s", t, root, own, own)}
		case fold("A") == "a" && strings.IndexFunc(rel, func(r rune) bool { return r > 127 }) >= 0:
			return lease.Decision{Reason: fmt.Sprintf("wk: cannot compare the non-ASCII path %s on a volume that ignores case; rename it in ASCII", rel)}
		}
	}
	return lease.Decision{Allow: true}
}

// logLine writes one line to hook.log for each deny and each allow with leases off. Other allows
// are not logged: the hook runs on every edit of every session.
func (h *leaseHook) logLine(d lease.Decision, took time.Duration) {
	if d.Allow && h.note == "" || h.name == "" {
		return
	}
	what := "deny: " + d.Reason
	if d.Allow {
		what = "allow: " + h.note
	}
	subject := cmp.Or(h.input.ToolInput.FilePath, h.input.ToolInput.NotebookPath)
	if subject == "" {
		subject = fmt.Sprintf("%.200q", h.input.ToolInput.Command)
	}
	log, closeLog := openHookLog(h.home, h.name)
	defer closeLog()
	_, _ = fmt.Fprintf(log, "%s lease %s %s cwd=%s (%dms) %s\n", time.Now().Format(time.RFC3339), h.input.ToolName, subject, h.input.Cwd, took.Milliseconds(), what)
}
