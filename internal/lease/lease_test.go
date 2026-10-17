package lease

import (
	"encoding/json"
	"slices"
	"testing"
)

func TestOverlap(t *testing.T) {
	cases := []struct {
		name, a, b string
		want       bool
	}{
		{"equal files", "web/x.ts", "web/x.ts", true},
		{"equal directories", "web/", "web/", true},
		{"directory and a file in it", "web/", "web/x.ts", true},
		{"file in a directory, reversed", "web/x.ts", "web/", true},
		{"directory and a nested directory", "web/", "web/lib/", true},
		{"file name prefix is no overlap", "web/x.ts", "web/x.ts.bak", false},
		{"sibling files", "web/x.ts", "web/y.ts", false},
		{"sibling directories", "web/", "api/", false},
		{"shared name prefix", "web/", "webapp/x", false},
		{"file lease is no directory prefix", "web", "web/x.ts", false},
		{"file and directory of one name", "web", "web/", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := overlap(c.a, c.b); got != c.want {
				t.Errorf("overlap(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
			}
		})
	}
}

func TestClaimRefusesAnOverlapAndNamesTheHolder(t *testing.T) {
	var s Store
	if err := s.Claim("/r/a", "7", "s1", []string{"web/"}); err != nil {
		t.Fatal(err)
	}
	err := s.Claim("/r/b", "8", "s2", []string{"api/", "web/x.ts"})
	want := "lease web/x.ts overlaps web/, held by row 7 in /r/a"
	if err == nil || err.Error() != want {
		t.Fatalf("Claim error = %v, want %q", err, want)
	}
	if s.Claimed("/r/b") {
		t.Error("a refused claim was recorded")
	}
}

func TestClaimTwiceRefuses(t *testing.T) {
	var s Store
	mustClaim(t, &s, "/r/a", "7", "web/")
	err := s.Claim("/r/a/", "9", "s2", []string{"api/"})
	if err == nil || err.Error() != "/r/a has already claimed row 7; run `wk release` there first" {
		t.Errorf("second Claim error = %v", err)
	}
	if got := s.Workspaces["/r/a"]; got.Row != "7" || !slices.Equal(got.Leases, []string{"web/"}) {
		t.Errorf("a refused second claim changed the first: %+v", got)
	}
	s.Release("/r/a")
	mustClaim(t, &s, "/r/a", "9", "api/")
}

func TestClaimRefusesMalformedLeases(t *testing.T) {
	for _, l := range []string{"", "/abs", "../up", "a/../b", "./a", "a//b", ".", "..", "/"} {
		var s Store
		if err := s.Claim("/r/a", "7", "", []string{l}); err == nil {
			t.Errorf("Claim accepted lease %q", l)
		}
	}
}

func TestAdd(t *testing.T) {
	var s Store
	if err := s.Add("/r/a", []string{"x"}); err == nil || err.Error() != "/r/a has not claimed; run `wk claim <row>` there first" {
		t.Errorf("Add without a claim: %v", err)
	}
	mustClaim(t, &s, "/r/a", "7", "web/")
	mustClaim(t, &s, "/r/b", "8")
	err := s.Add("/r/b", []string{"web/x.ts"})
	if err == nil || err.Error() != "lease web/x.ts overlaps web/, held by row 7 in /r/a" {
		t.Errorf("Add over another's lease: %v", err)
	}
	if err := s.Add("/r/a", []string{"api/", "web/"}); err != nil {
		t.Fatal(err)
	}
	if got := s.Workspaces["/r/a"].Leases; !slices.Equal(got, []string{"api/", "web/"}) {
		t.Errorf("leases after Add = %q", got)
	}
}

func TestHolderReleaseClaimed(t *testing.T) {
	var s Store
	mustClaim(t, &s, "/r/a", "7", "web/", "go.mod")
	mustClaim(t, &s, "/r/b", "8")
	for _, c := range []struct {
		path, root, row string
		ok              bool
	}{
		{"web/x.ts", "/r/a", "7", true},
		{"go.mod", "/r/a", "7", true},
		{"web", "/r/a", "7", true},
		{".", "/r/a", "7", true},
		{"api/x.go", "", "", false},
		{"go.sum", "", "", false},
	} {
		root, row, ok := s.Holder(c.path)
		if root != c.root || row != c.row || ok != c.ok {
			t.Errorf("Holder(%q) = %q, %q, %v", c.path, root, row, ok)
		}
	}
	if !s.Claimed("/r/b") || s.Claimed("/r/c") {
		t.Error("Claimed: a --none claim must count and an absent root must not")
	}
	s.Release("/r/a")
	if _, _, ok := s.Holder("web/x.ts"); ok || s.Claimed("/r/a") {
		t.Error("Release left the claim behind")
	}
}

func TestStoreJSONRoundTrip(t *testing.T) {
	var s Store
	mustClaim(t, &s, "/r/a", "7", "web/", "go.mod")
	mustClaim(t, &s, "/r/b", "8")
	data, err := json.Marshal(&s)
	if err != nil {
		t.Fatal(err)
	}
	var back Store
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	for root, want := range s.Workspaces {
		got := back.Workspaces[root]
		if got.Row != want.Row || got.Session != want.Session || !slices.Equal(got.Leases, want.Leases) || !got.ClaimedAt.Equal(want.ClaimedAt) {
			t.Errorf("%s: got %+v, want %+v", root, got, want)
		}
	}
	if len(back.Workspaces) != 2 {
		t.Errorf("round trip has %d workspaces", len(back.Workspaces))
	}
}

func mustClaim(t *testing.T, s *Store, root, row string, leases ...string) {
	t.Helper()
	if err := s.Claim(root, row, "session-"+row, leases); err != nil {
		t.Fatal(err)
	}
}
