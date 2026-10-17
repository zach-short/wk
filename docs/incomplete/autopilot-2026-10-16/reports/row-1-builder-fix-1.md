# Row 1, builder fix round 1 (Opus 5.5, agent a62bd52ebf20cb308), arrived 2026-10-16 21:00 EDT

HEAD on `ap/row-1` is `51f1282` (commits `f06b396` "correct CLAUDE.md cites and timings, move the git line out of never do this" and `51f1282` "remove a double colon in CLAUDE.md"; both name CLAUDE.md only, no trailer). Not merged, not pushed.
Fixes: rules intro rewritten (three rules named with their tests, four with no test checked on 2026-10-16); cites fixed to main.go:145-146 and :89-90; the git line moved to a new "## Git" section with the no-trailer rule; timings now "45 to 50 s" and "40 to 46 s"; hook.go cite 18-20; go run refusal cite hooks.go:25 and main.go:196; two bullets reworded to remove a double colon.
Gate at 51f1282, clean tree: exit 0, internal/wk 44.4 s, 0 issues.
Diffstat main...HEAD: CLAUDE.md 185, passoff/1 12; 2 files, +179/-18.
Paste-block cites to correct (already applied to reports/row-1-builder.md by this session at 20:59 EDT): main.go:145-146, :89-90, hooks.go:25 + main.go:196.
