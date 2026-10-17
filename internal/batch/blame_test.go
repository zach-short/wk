package batch_test

import (
	"testing"

	"github.com/zach-short/wk/internal/batch"
)

func TestBlame(t *testing.T) {
	// The P4 proof's batch: three changesets, one of which breaks the type-check gate.
	proof := map[string][]string{
		"aaa": {"web/src/list.tsx", "web/src/list.test.tsx"},
		"bbb": {"web/src/price.ts"},
		"ccc": {"internal/batch/batch.go"},
	}
	tests := []struct {
		name       string
		failed     []string
		diffs      map[string][]string
		wantID     string
		wantUnique bool
	}{
		{"one changeset touches the path the type-check names", []string{"web/src/price.ts"}, proof, "bbb", true},
		{"several paths, all in one changeset", []string{"web/src/list.tsx", "web/src/list.test.tsx"}, proof, "aaa", true},
		{"two changesets touch the path", []string{"web/src/price.ts"}, map[string][]string{
			"aaa": {"web/src/price.ts"}, "bbb": {"web/src/price.ts", "web/src/x.ts"}, "ccc": {"go.mod"},
		}, "", false},
		{"the paths name two changesets", []string{"web/src/price.ts", "internal/batch/batch.go"}, proof, "", false},
		{"a path-less failure blames nobody", nil, proof, "", false},
		{"a failed path no changeset touches", []string{"README.md"}, proof, "", false},
		{"a directory matches a file under it", []string{"internal/batch"}, map[string][]string{
			"aaa": {"internal/batch/blame.go"}, "bbb": {"internal/batchy/x.go"}, "ccc": {"internal"},
		}, "aaa", true},
		{"a directory written with ./ and a trailing slash", []string{"./web/src/"}, map[string][]string{
			"aaa": {"web/src/a.ts"}, "bbb": {"go/x.go"},
		}, "aaa", true},
		{"the repository root blames nobody", []string{".", "", "/"}, map[string][]string{"aaa": {"a.go"}}, "", false},
		{"a changeset with an empty diff is never blamed", []string{"a.go"}, map[string][]string{
			"aaa": nil, "bbb": {"a.go"},
		}, "bbb", true},
		{"no changesets", []string{"a.go"}, nil, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, unique := batch.Blame(tc.failed, tc.diffs)
			if id != tc.wantID || unique != tc.wantUnique {
				t.Errorf("Blame = %q, %v; want %q, %v", id, unique, tc.wantID, tc.wantUnique)
			}
		})
	}
}
