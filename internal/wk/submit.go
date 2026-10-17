package wk

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/zach-short/wk/internal/changeset"
	"github.com/zach-short/wk/internal/gitx"
)

// Submit builds the working tree of the workspace holding dir into a changeset, appends it to
// the queue as submitted and prints its id, the only line on stdout (option O). The row comes
// from the workspace's P2 claim unless row is given; with neither the message has no prefix.
// review is the reference landing.require_review asks for. The caller starts the worker.
func (e *Env) Submit(dir, message, row, review string, out io.Writer) error {
	if message == "" {
		return errors.New("wk submit needs -m with a message")
	}
	root, err := e.workspaceOnly(dir)
	if err != nil {
		return err
	}
	if !e.Cfg.Landing.Enabled {
		return fmt.Errorf("landing.enabled is false in %s; nothing lands in this repository", e.configPath())
	}
	tip, err := e.landingTip()
	if err != nil {
		return err
	}
	base, err := changeset.Base(root, tip)
	if err != nil {
		return err
	}
	if err := e.sayIncluded(root); err != nil {
		return err
	}
	tree, err := changeset.Tree(root)
	if err != nil {
		return err
	}
	if row == "" {
		row = e.claimedRow(root)
	}
	now := e.clock()
	rec := changeset.Record{
		ID: changeset.NewID(tree, base, now), State: changeset.Submitted, Tree: tree, Base: base,
		Message: subject(row, message), Row: row, Workspace: root, Review: review, At: now,
	}
	if err := e.queue().Append(rec); err != nil {
		return err
	}
	_, _ = fmt.Fprintln(out, rec.ID)
	return nil
}

// subject is the landing commit's message: `row <n>: <message>` in lowercase, no trailer
// (option Q), and the message alone when the workspace has no row.
func subject(row, message string) string {
	if row == "" {
		return strings.ToLower(message)
	}
	return strings.ToLower("row " + row + ": " + message)
}

// sayIncluded lists on the log what `add -A` puts in the tree, every path that differs from
// HEAD (owner amendment, 2026-10-16), so a stray file is seen before it lands.
func (e *Env) sayIncluded(root string) error {
	paths, err := gitx.DirtyPaths(root)
	if err != nil {
		return err
	}
	for _, p := range paths {
		e.say("includes %s", p)
	}
	if len(paths) == 0 {
		e.say("the working tree equals HEAD; the changeset carries no change of its own")
	}
	return nil
}

// claimedRow is the row the workspace claimed under P2, or "" when it claimed none, claimed
// --none, or the store cannot be read, which is said on the log and costs only the prefix.
func (e *Env) claimedRow(root string) string {
	f, err := loadLeases(e.State)
	if err != nil {
		e.say("no row prefix: %v", err)
		return ""
	}
	if row := f.Workspaces[root].Row; row != noneRow {
		return row
	}
	return ""
}

// workspaceOnly is workspaceRoot with D15: the main checkout is reserved for landing, so a
// submit or a sync from it is refused.
func (e *Env) workspaceOnly(dir string) (string, error) {
	root, err := workspaceRoot(dir)
	if err != nil {
		return "", err
	}
	if root == canonical(e.Repo.Primary) {
		return "", fmt.Errorf("%s is the main checkout, which is reserved for landing; work in a wk new workspace", root)
	}
	return root, nil
}

func (e *Env) configPath() string {
	return filepath.Join(e.home, e.Repo.Name+".toml")
}
