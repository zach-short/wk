package wk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"mvdan.cc/sh/v3/syntax"

	"github.com/zach-short/wk/internal/changeset"
	"github.com/zach-short/wk/internal/gate"
	"github.com/zach-short/wk/internal/landing"
	"github.com/zach-short/wk/internal/slot"
)

// member is one changeset of a batch: predicted is the tree after it, merged on top of prev,
// the tree after the member before it (or the tip's tree for the first).
type member struct {
	rec             changeset.Record
	prev, predicted string
}

// plan is a batch predicted onto the landing tip; its final tree is the one the gates see.
type plan struct {
	tip, tipTree string
	members      []member
}

func (p plan) final() string {
	if len(p.members) == 0 {
		return p.tipTree
	}
	return p.members[len(p.members)-1].predicted
}

// changes is whether any member changes the tree. A batch that changes nothing lands without
// a gate run, because its trees are the tip's.
func (p plan) changes() bool {
	for _, m := range p.members {
		if m.predicted != m.prev {
			return true
		}
	}
	return false
}

func (p plan) records() []changeset.Record {
	recs := make([]changeset.Record, len(p.members))
	for i, m := range p.members {
		recs[i] = m.rec
	}
	return recs
}

func (p plan) ids() []string { return recordIDs(p.records()) }

func (p plan) without(id string) []changeset.Record {
	var recs []changeset.Record
	for _, m := range p.members {
		if m.rec.ID != id {
			recs = append(recs, m.rec)
		}
	}
	return recs
}

func (p plan) member(id string) changeset.Record {
	for _, m := range p.members {
		if m.rec.ID == id {
			return m.rec
		}
	}
	return changeset.Record{}
}

// predictBatch predicts each of recs on top of the one before it, starting at tip.
func (e *Env) predictBatch(recs []changeset.Record, tip string, out io.Writer) (plan, error) {
	tipTree, err := e.treeOf(tip)
	if err != nil {
		return plan{}, err
	}
	p := plan{tip: tip, tipTree: tipTree}
	for _, r := range recs {
		if err := e.addMember(&p, r, out); err != nil {
			return plan{}, err
		}
	}
	return p, nil
}

// addMember predicts r on top of the plan so far. A changeset without the review
// landing.require_review asks for, or one that conflicts with the tip itself, is refused, as in
// P3; one that conflicts only with an earlier member waits, still submitted, for the next batch,
// because that member may yet fail its gate.
func (e *Env) addMember(p *plan, r changeset.Record, out io.Writer) error {
	if e.Cfg.Landing.RequireReview && r.Review == "" {
		return e.refuse(r, nil, "no review; landing.require_review is true, submit again with --review", out)
	}
	prev := p.final()
	result, conflicts, err := landing.Predict(e.Repo.Primary, r.Base, prev, r.Tree)
	if err != nil {
		return err
	}
	if len(conflicts) == 0 {
		p.members = append(p.members, member{rec: r, prev: prev, predicted: result})
		return nil
	}
	if prev != p.tipTree {
		if _, conflicts, err = landing.Predict(e.Repo.Primary, r.Base, p.tip, r.Tree); err != nil {
			return err
		}
		if len(conflicts) == 0 {
			_, _ = fmt.Fprintf(out, "deferred %s: it conflicts with an earlier changeset of the batch\n", r.ID)
			return nil
		}
	}
	return e.refuse(r, conflicts, "conflicts with the landing tip "+p.tip+"; run wk sync, fix the markers and submit again", out)
}

// verdict is one gate run over a plan's final tree. red is the gate that failed, and paths the
// repository-relative paths it named; unchanged marks a plan that needed no run.
type verdict struct {
	results   []gate.Result
	red       *gate.Result
	paths     []string
	unchanged bool
}

// green is a run with no fail and at least one pass. A run whose gates all skipped is not green:
// a skip is never a pass (DESIGN.md section 6), so an all-skip batch lands nothing.
func (v verdict) green() bool {
	if v.unchanged {
		return true
	}
	if v.red != nil {
		return false
	}
	for _, r := range v.results {
		if r.Status == gate.Pass {
			return true
		}
	}
	return false
}

func (v verdict) summary() string {
	if v.unchanged {
		return "no gate run: the batch changes nothing"
	}
	parts := make([]string, len(v.results))
	for i, r := range v.results {
		parts[i] = r.Name + " " + r.Status
		if r.Reason != "" {
			parts[i] += " (" + r.Reason + ")"
		}
	}
	return "gates: " + strings.Join(parts, ", ")
}

// gatePlan gates p's final tree in the trial and records the run for wk status: the batch in
// flight while it runs, then its outcome.
func (e *Env) gatePlan(p plan, out io.Writer) (verdict, error) {
	if !p.changes() {
		return verdict{unchanged: true}, nil
	}
	tree := p.final()
	_, _ = fmt.Fprintf(out, "gating %s on %s: tree %s\n", strings.Join(p.ids(), " "), shortHash(p.tip), tree)
	e.noteGates(func(l *gateLog) {
		l.Batch, l.Tree, l.State, l.Gates, l.Error, l.At = p.ids(), tree, "gating", nil, "", e.clock()
		l.PID = os.Getpid()
	})
	var v verdict
	err := e.withTrial(tree, func(dir string) (err error) {
		if v, err = e.refreshDeps(dir, tree, out); err != nil || v.red != nil {
			return err
		}
		v, err = e.runGates(dir, out)
		return err
	})
	e.noteGates(func(l *gateLog) { l.record(v, err, e.clock()) })
	return v, err
}

// runGates runs the gates in order over the trial at root, waiting for gates.min_free_mb of free
// memory before each, and stops at the first fail. Each gate run shares one set of fresh caches.
func (e *Env) runGates(root string, out io.Writer) (verdict, error) {
	env, cleanup, err := e.gateEnv()
	defer cleanup()
	if err != nil {
		return verdict{}, err
	}
	var v verdict
	for _, s := range e.gateList() {
		if err := e.waitForMemory(out); err != nil {
			return v, err
		}
		// A gate that cannot start is an error, not a fail: it says nothing about the tree, so it
		// is never blamed on a changeset (row 11). gate.Run says so for a gate run as it is; a
		// slotted gate runs under /bin/sh and wk run, which turn a missing command into an exit
		// status, so canStart and wrapperFailed say it for those (fix round 1, B1).
		if err := s.canStart(root, env); err != nil {
			return v, err
		}
		r, err := runGate(root, s, env, out)
		if err != nil {
			return v, err
		}
		v.results = append(v.results, r)
		if r.Status == gate.Fail {
			v.red, v.paths = &r, failedPaths(root, s.g, r)
			return v, nil
		}
	}
	return v, nil
}

// runGate is gate.Run over one gate, with its echo held back until wrapperFailed has read the
// result: a start error is logged as the start error with the output's tail, never as the
// `fail, exit status 75` line gate.Run would have written first.
func runGate(root string, s gateSpec, env map[string]string, out io.Writer) (gate.Result, error) {
	var held bytes.Buffer
	rs, err := gate.Run(root, []gate.Gate{s.g}, env, nil, &held)
	if err != nil {
		_, _ = io.Copy(out, &held)
		return gate.Result{}, err
	}
	if err := s.wrapperFailed(rs[0]); err != nil {
		_, _ = fmt.Fprintf(out, "%v\n", err)
		for _, line := range gate.Tail(rs[0].Output) {
			_, _ = fmt.Fprintf(out, "  %s\n", line)
		}
		return gate.Result{}, err
	}
	_, _ = io.Copy(out, &held)
	return rs[0], nil
}

// failedPaths is gate.Paths over the gate's own directory with that directory joined back on,
// because the tools print paths relative to where they ran (row 11's finding).
func failedPaths(root string, g gate.Gate, r gate.Result) []string {
	var paths []string
	for _, p := range gate.Paths(filepath.Join(root, filepath.FromSlash(g.Dir)), r.Output) {
		paths = append(paths, path.Join(g.Dir, p))
	}
	return paths
}

// waitForMemory is gate.WaitForMemory at gates.min_free_mb, sampled every second; 0 turns the
// pause off, for a machine without vm_stat.
func (e *Env) waitForMemory(out io.Writer) error {
	if e.Cfg.Gates.MinFreeMB == 0 {
		return nil
	}
	return gate.WaitForMemory(e.Cfg.Gates.MinFreeMB, time.Second, out)
}

// gateEnv is gates.env with every gates.fresh_caches variable pointed at a new empty directory,
// which cleanup removes. The caches win over the same names in gates.env.
func (e *Env) gateEnv() (map[string]string, func(), error) {
	env := map[string]string{}
	for k, v := range e.Cfg.Gates.Env {
		env[k] = v
	}
	var made []string
	cleanup := func() {
		for _, dir := range made {
			_ = os.RemoveAll(dir)
		}
	}
	for _, name := range e.Cfg.Gates.FreshCaches {
		dir, err := os.MkdirTemp("", "wk-gate-"+name+"-")
		if err != nil {
			return nil, cleanup, fmt.Errorf("fresh cache for %s: %w", name, err)
		}
		made, env[name] = append(made, dir), dir
	}
	return env, cleanup, nil
}

// gateList is [[gates.run]] as gate.Gate values, each routed through `wk run --slot` where the
// repository's slots file recognises its command (PD-10).
func (e *Env) gateList() []gateSpec {
	sf, found := e.slotsFile()
	list := make([]gateSpec, len(e.Cfg.Gates.Run))
	for i, r := range e.Cfg.Gates.Run {
		g := gate.Gate{Name: r.Name, Dir: r.Dir, Run: r.Run, NeedsEnv: r.NeedsEnv, Timeout: time.Duration(r.TimeoutSeconds) * time.Second}
		list[i] = gateSpec{g: g, argv0: r.Run[0]}
		if found && e.WKExe != "" {
			list[i].g, list[i].slotted = slotted(g, e.WKExe, sf)
			list[i].wrapper = e.WKExe
		}
	}
	return list
}

// slotsFile is ~/.wk/<repo>.slots.toml. A file that cannot be used is said and left out, so the
// gates run unslotted, as the hook fails open.
func (e *Env) slotsFile() (slot.SlotsFile, bool) {
	sf, found, err := slot.LoadSlotsFile(filepath.Join(e.home, e.Repo.Name+".slots.toml"))
	if err != nil {
		e.say("gates run without slots: %v", err)
		return slot.SlotsFile{}, false
	}
	return sf, found
}

// slotted gives g the command line slot.Rewrite makes of its argument vector, run by /bin/sh,
// when a spelling of sf matches it, and says so; otherwise g comes back as it is. The words are
// quoted for the shell, so the command it runs is the vector byte for byte.
func slotted(g gate.Gate, exe string, sf slot.SlotsFile) (gate.Gate, bool) {
	words := make([]string, len(g.Run))
	for i, w := range g.Run {
		q, err := syntax.Quote(w, syntax.LangBash)
		if err != nil {
			return g, false
		}
		words[i] = q
	}
	line, changed, _, err := slot.Rewrite(strings.Join(words, " "), exe, sf)
	if err != nil || !changed {
		return g, false
	}
	g.Run = []string{"/bin/sh", "-c", line}
	return g, true
}

// gateLog is ~/.wk/<repo>/gates.json, the worker's last gate run and its last move of the base
// branch, which wk status prints. Only the worker writes it, under landing.lock.
type gateLog struct {
	PID    int       `json:"pid,omitempty"`
	Batch  []string  `json:"batch,omitempty"`
	Tree   string    `json:"tree,omitempty"`
	State  string    `json:"state,omitempty"`
	Gates  []string  `json:"gates,omitempty"`
	Error  string    `json:"error,omitempty"`
	At     time.Time `json:"at"`
	Main   string    `json:"main,omitempty"`
	MainAt time.Time `json:"main_at"`
}

func (l *gateLog) record(v verdict, err error, at time.Time) {
	l.Gates, l.At = nil, at
	for _, r := range v.results {
		line := r.Name + " " + r.Status
		if r.Reason != "" {
			line += ", " + r.Reason
		}
		l.Gates = append(l.Gates, fmt.Sprintf("%s (%.1fs)", line, r.Seconds))
	}
	switch {
	case err != nil:
		l.State, l.Error = "error", err.Error()
	case v.green():
		l.State = "green"
	case v.red != nil:
		l.State = "red: " + v.red.Name
	default:
		l.State = "every gate skipped; nothing lands"
	}
}

func (e *Env) gateLogPath() string { return filepath.Join(e.State, "gates.json") }

func (e *Env) readGateLog() (gateLog, error) {
	var l gateLog
	data, err := os.ReadFile(e.gateLogPath())
	if errors.Is(err, fs.ErrNotExist) {
		return l, nil
	}
	if err != nil {
		return l, err
	}
	return l, json.Unmarshal(data, &l)
}

// noteGates changes the gate log by rename. It is a record for wk status only, so a failure to
// write it is said and the landing goes on.
func (e *Env) noteGates(change func(*gateLog)) {
	l, err := e.readGateLog()
	if err == nil {
		change(&l)
		err = writeJSON(e.gateLogPath(), l)
	}
	if err != nil {
		e.say("gates.json not written: %v", err)
	}
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
