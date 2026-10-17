package board

import (
	"slices"
	"strings"
	"testing"
)

// wkBoard is the wk board's header and rows 1 to 5, copied from
// ~/Projects/wk/PASSOFF.md on 2026-10-16.
var wkBoard = strings.Join([]string{
	"# PASSOFF: wk (the board)",
	"",
	"| # | Task | Status | Model | Lane | Waits on | Files it owns |",
	"|---|------|--------|-------|------|----------|---------------|",
	"| 1 | Run Part 0 for wk: fill CLAUDE.md and the ledger's environment | `DONE - 2026-10-16` | Opus 5.5 | B | nothing | `CLAUDE.md`; `HANDOFF.md` |",
	"| 2 | Ratify the P1 amendments and write the plan for P2 to P4 | `DONE - 2026-10-16` | Fable 5.1 | A | nothing | `docs/incomplete/landing-layer/`; `passoff/3-*.md` to `passoff/5-*.md` |",
	"| 3 | Build P2, path leases: integrate the daemon, the lease store, the board parser and the edit hook | `OPEN` | Opus 5.5 | A | 2; 6, 7, 8; `/Users/zachshort/Projects/ezhomesteading/PASSOFF.md` item 200 | `cmd/wk/main.go`; `internal/config/config.go`; `internal/settings/settings.go` (sequential overlap with ezhomesteading item 200); `internal/wk/lease.go`, `internal/wk/leasehook.go`, `internal/wk/daemon.go`, `internal/wk/hooks.go`; `README.md`; `~/.wk/wk.toml`; `~/.wk/ezhomesteading.toml`; `docs/incomplete/landing-layer/PLAN-P2-P4.md` (As built, P2) |",
	"| 4 | Build P3, the landing queue: integrate `wk submit`, the landing loop, `wk sync` and `wk land` | `OPEN` | Fable 5.1 | A | 3; 9, 10 | `cmd/wk/main.go`; `internal/config/config.go`; `internal/wk/landing.go`, `internal/wk/submit.go`, `internal/wk/status.go`; `README.md`; `~/.wk/wk.toml`; `~/.wk/ezhomesteading.toml`; `docs/incomplete/landing-layer/PLAN-P2-P4.md` (As built, P3) |",
	"| 5 | Build P4, one gate run per batch: integrate the trial, the batch loop, attribution and the fast-forward of `main` | `OPEN` | Opus 5.5 | A | 4; 11, 12 | `cmd/wk/main.go`; `internal/config/config.go`; `internal/wk/trial.go`, `internal/wk/gatekeeper.go`, `internal/wk/landing.go`, `internal/wk/status.go`; `README.md`; `~/.wk/wk.toml`; `~/.wk/ezhomesteading.toml`; `docs/incomplete/landing-layer/PLAN-P2-P4.md` (As built, P4) |",
	"",
	"Each item's prompt is `passoff/<n>-<slug>.md`.",
}, "\n")

// ezhCells are "Files it owns" cells from ~/Projects/ezhomesteading on
// 2026-10-16: rows 76, 77 and 200 of PASSOFF.md, and row 200 as its item file
// passoff/200-*.md states it, which the board shortens to "(more in the file)".
var ezhCells = map[string]string{
	"76":        "A: `backend/services/visibilityService.go`, `backend/repository/location_postgres.go`, `backend/services/checkoutPlanService.go`, `backend/services/offerService.go`, `backend/controllers/resource/checkoutPlanControllers.go`, one new `backend/migrations/` file. B: `backend/repository/order.go`, `backend/repository/order_postgres.go` (the inbox and list reads only), `backend/controllers/resource/orderResourceControllers.go`, `backend/routes/routes.go` (one route), `packages/core/src/orders/order-api.ts`, `packages/core/src/orders/chat/orders-inbox.ts`, `web/features/orders/`, `native/features/orders/`, `native/services/order-services.ts`, new Past orders routes in `web/app/` and `native/app/`. C: `backend/services/orderChatService.go` (proposal path and `GetChat` only), `backend/controllers/resource/orderChatControllers.go`, the chat composer in `packages/core/src/orders/chat/`, `web/features/orders/` chat components, `native/features/orders/` chat components",
	"77":        "per item in the audit",
	"200":       "`~/Projects/wk`; `~/.wk/machine.toml`, `~/.wk/ezhomesteading.toml`; `.claude/settings.local.json`; `SLOTS-SCOPE.md` (more in the file)",
	"200, item": "`~/Projects/wk` (`cmd/wk/main.go`, a new `internal/slot`, `internal/config`, `internal/settings`, `README.md`); `~/.wk/machine.toml`; `~/.wk/ezhomesteading.toml`; `.claude/settings.local.json` in the main checkout and the template; `docs/incomplete/landing-layer/SLOTS-SCOPE.md` and the slots plan beside it",
}

func TestCell(t *testing.T) {
	cell, err := Cell(wkBoard, "1")
	if err != nil || cell != "`CLAUDE.md`; `HANDOFF.md`" {
		t.Fatalf("Cell(1) = %q, %v", cell, err)
	}
	for _, row := range []string{"9", "", "Files it owns"} {
		if _, err := Cell(wkBoard, row); err == nil || !strings.Contains(err.Error(), `"`+row+`"`) {
			t.Errorf("Cell(%q) error = %v, want one naming the row", row, err)
		}
	}
}

func TestCellMatchesTheWholeFirstCell(t *testing.T) {
	board := "| 10 | ten | `ten.go` |\n| 1 | one | `one.go` |\n| 1 | again | `again.go` |"
	cell, err := Cell(board, "1")
	if err != nil || cell != "`one.go`" {
		t.Fatalf("Cell(1) = %q, %v; want the first row 1, not row 10", cell, err)
	}
}

func TestCellRefusesALineWithOneCell(t *testing.T) {
	if _, err := Cell("| 8 |\n| 8 | `late.go` |", "8"); err == nil || !strings.Contains(err.Error(), `"8"`) {
		t.Fatalf("Cell error = %v, want one naming row 8", err)
	}
}

func TestParseLiveCells(t *testing.T) {
	cases := []struct {
		name          string
		cell          func(t *testing.T) string
		paths, unread []string
	}{
		{"wk 1", wkCell("1"), []string{"CLAUDE.md", "HANDOFF.md"}, nil},
		{
			"wk 2", wkCell("2"),
			[]string{"docs/incomplete/landing-layer/", "passoff/3-*.md", "passoff/5-*.md"},
			[]string{"to"},
		},
		{
			"wk 3", wkCell("3"),
			[]string{"cmd/wk/main.go", "internal/config/config.go", "internal/settings/settings.go", "internal/wk/lease.go", "internal/wk/leasehook.go", "internal/wk/daemon.go", "internal/wk/hooks.go", "README.md", "docs/incomplete/landing-layer/PLAN-P2-P4.md"},
			[]string{"(sequential overlap with ezhomesteading item 200);", "~/.wk/wk.toml", "~/.wk/ezhomesteading.toml", "(As built, P2)"},
		},
		{
			"wk 4", wkCell("4"),
			[]string{"cmd/wk/main.go", "internal/config/config.go", "internal/wk/landing.go", "internal/wk/submit.go", "internal/wk/status.go", "README.md", "docs/incomplete/landing-layer/PLAN-P2-P4.md"},
			[]string{"~/.wk/wk.toml", "~/.wk/ezhomesteading.toml", "(As built, P3)"},
		},
		{
			"wk 5", wkCell("5"),
			[]string{"cmd/wk/main.go", "internal/config/config.go", "internal/wk/trial.go", "internal/wk/gatekeeper.go", "internal/wk/landing.go", "internal/wk/status.go", "README.md", "docs/incomplete/landing-layer/PLAN-P2-P4.md"},
			[]string{"~/.wk/wk.toml", "~/.wk/ezhomesteading.toml", "(As built, P4)"},
		},
		{
			"ezhomesteading 76", ezhCell("76"),
			[]string{
				"backend/services/visibilityService.go", "backend/repository/location_postgres.go", "backend/services/checkoutPlanService.go", "backend/services/offerService.go", "backend/controllers/resource/checkoutPlanControllers.go", "backend/migrations/",
				"backend/repository/order.go", "backend/repository/order_postgres.go", "backend/controllers/resource/orderResourceControllers.go", "backend/routes/routes.go", "packages/core/src/orders/order-api.ts", "packages/core/src/orders/chat/orders-inbox.ts", "web/features/orders/", "native/features/orders/", "native/services/order-services.ts", "web/app/", "native/app/",
				"backend/services/orderChatService.go", "GetChat", "backend/controllers/resource/orderChatControllers.go", "packages/core/src/orders/chat/",
			},
			[]string{"A:", ", one new", "file. B:", "(the inbox and list reads only),", "(one route),", ", new Past orders routes in", "and", ". C:", "(proposal path and", "only),", ", the chat composer in", "chat components,", "chat components"},
		},
		{"ezhomesteading 77", ezhCell("77"), nil, []string{"per item in the audit"}},
		{
			"ezhomesteading 200", ezhCell("200"),
			[]string{".claude/settings.local.json", "SLOTS-SCOPE.md"},
			[]string{"~/Projects/wk", "~/.wk/machine.toml", "~/.wk/ezhomesteading.toml", "(more in the file)"},
		},
		{
			"ezhomesteading 200, item file", ezhCell("200, item"),
			[]string{"cmd/wk/main.go", "internal/slot", "internal/config", "internal/settings", "README.md", ".claude/settings.local.json", "docs/incomplete/landing-layer/SLOTS-SCOPE.md"},
			[]string{"~/Projects/wk", ", a new", "~/.wk/machine.toml", "~/.wk/ezhomesteading.toml", "in the main checkout and the template;", "and the slots plan beside it"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			checkParse(t, c.cell(t), c.paths, c.unread)
		})
	}
}

func TestParseTokenForms(t *testing.T) {
	cases := []struct {
		cell          string
		paths, unread []string
	}{
		{"`a/`; `b/**`; `c/**/*`; `d//`; `e/**/`; `f/**/**`", []string{"a/", "b/", "c/", "d/", "e/", "f/"}, nil},
		{"`./src/*.ts`, `src//*.ts`, `./x?.go`", []string{"src/*.ts", "x?.go"}, nil},
		{"`./e.go`, `f/./g.go`, `a/` again `a/`", []string{"e.go", "f/g.go", "a/"}, []string{"again"}},
		{"`src/**/*.ts`, `x?.go`, `[ab].go`", []string{"src/**/*.ts", "x?.go", "[ab].go"}, nil},
		{"`web/*/`, `web/*/**`, `web/*/**/*`, `./web/*/**/`", []string{"web/*/**/*"}, nil},
		{"`/etc/hosts` `~/x` `a/../b` `..` `go test ./...` `./` `.`", nil, []string{"/etc/hosts", "~/x", "a/../b", "..", "go test ./...", "./", "."}},
		{"`a.go` → `b.go`", []string{"a.go", "b.go"}, []string{"→"}},
		{"`a.go`; `b.go", []string{"a.go"}, []string{"`b.go"}},
		{"``; , . ( ) ;", nil, nil},
		{"", nil, nil},
	}
	for _, c := range cases {
		t.Run(c.cell, func(t *testing.T) {
			checkParse(t, c.cell, c.paths, c.unread)
		})
	}
}

func TestExpand(t *testing.T) {
	files := []string{
		"README.md",
		"docs/incomplete/landing-layer/DESIGN.md",
		"docs/incomplete/landing-layer/notes/a.md",
		"docs/incomplete/landing-layerx/b.md",
		"passoff/3-build-p2.md",
		"passoff/30-other.md",
		"src/a.ts",
		"src/lib/b.ts",
		"src/lib/deep/c.ts",
		"src/lib/d.tsx",
		"lib/e.ts",
	}
	patterns := []string{"passoff/3-*.md", "docs/incomplete/landing-layer/", "src/**/*.ts", "missing/**/*.go", "README.md", "GetChat", "[bad"}
	matched, unmatched := Expand(patterns, files)
	want := []string{
		"README.md",
		"docs/incomplete/landing-layer/DESIGN.md",
		"docs/incomplete/landing-layer/notes/a.md",
		"passoff/3-build-p2.md",
		"src/a.ts",
		"src/lib/b.ts",
		"src/lib/deep/c.ts",
	}
	if !slices.Equal(matched, want) {
		t.Errorf("matched = %q\nwant      %q", matched, want)
	}
	if want := []string{"missing/**/*.go", "GetChat", "[bad"}; !slices.Equal(unmatched, want) {
		t.Errorf("unmatched = %q, want %q", unmatched, want)
	}
}

func TestExpandCleanedGlobs(t *testing.T) {
	globs, _ := Parse("`./src/*.ts`; `./nowhere/*.go`")
	matched, unmatched := Expand(globs, []string{"src/a.ts", "src/lib/b.ts"})
	if want := []string{"src/a.ts"}; !slices.Equal(matched, want) {
		t.Errorf("matched = %q, want %q", matched, want)
	}
	if want := []string{"nowhere/*.go"}; !slices.Equal(unmatched, want) {
		t.Errorf("unmatched = %q, want %q", unmatched, want)
	}
	if matched, _ := Expand([]string{"./src/*.ts"}, []string{"src/a.ts"}); len(matched) != 1 {
		t.Errorf("Expand of an uncleaned glob matched %q, want src/a.ts", matched)
	}
}

func wkCell(row string) func(t *testing.T) string {
	return func(t *testing.T) string {
		t.Helper()
		cell, err := Cell(wkBoard, row)
		if err != nil {
			t.Fatal(err)
		}
		return cell
	}
}

func ezhCell(row string) func(t *testing.T) string {
	return func(*testing.T) string { return ezhCells[row] }
}

func checkParse(t *testing.T, cell string, wantPaths, wantUnread []string) {
	t.Helper()
	paths, unread := Parse(cell)
	if !slices.Equal(paths, wantPaths) {
		t.Errorf("paths = %q\nwant    %q", paths, wantPaths)
	}
	if !slices.Equal(unread, wantUnread) {
		t.Errorf("unread = %q\nwant     %q", unread, wantUnread)
	}
}

func TestColumnFindsTheCellByHeader(t *testing.T) {
	text := "intro\n\n| # | Task | Files it owns | Model |\n|---|---|---|---|\n| 3 | build | `a.go`; `b/` | Opus |\n| 4 | next | `c.go` | Fable |\n\n| # | Other |\n|---|---|\n| 9 | x |\n"
	for _, tc := range []struct{ row, want, err string }{
		{"3", "`a.go`; `b/`", ""},
		{"4", "`c.go`", ""},
		{"9", "", `board row "9" has no "Files it owns" column`},
		{"5", "", `board has no table row "5"`},
	} {
		got, err := Column(text, tc.row, "Files it owns")
		if got != tc.want || (err == nil) != (tc.err == "") || err != nil && err.Error() != tc.err {
			t.Errorf("Column(%s) = %q, %v; want %q, %q", tc.row, got, err, tc.want, tc.err)
		}
	}
}
