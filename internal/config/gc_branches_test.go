package config

import (
	"slices"
	"strings"
	"testing"
)

func TestBranchPatternsDefaultToNone(t *testing.T) {
	if got := Defaults().GC.BranchPatterns; got == nil || len(got) != 0 {
		t.Fatalf("gc.branch_patterns defaults to %q, want an empty list", got)
	}
	cfg, _, err := Load(write(t, "[gc]\nbranch_patterns = [\"row-*\"]\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(cfg.GC.BranchPatterns, []string{"row-*"}) {
		t.Fatalf("gc.branch_patterns = %q", cfg.GC.BranchPatterns)
	}
}

func TestBranchPatternRefusals(t *testing.T) {
	for name, body := range map[string]string{
		"empty":     "[gc]\nbranch_patterns = [\"\"]\n",
		"malformed": "[gc]\nbranch_patterns = [\"row-*\", \"row-[\"]\n",
	} {
		t.Run(name, func(t *testing.T) {
			_, _, err := Load(write(t, body))
			if err == nil || !strings.Contains(err.Error(), "gc.branch_patterns[") {
				t.Fatalf("err = %v, want a gc.branch_patterns refusal", err)
			}
		})
	}
}
