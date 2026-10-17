package batch_test

import (
	"slices"
	"testing"
	"time"

	"github.com/zach-short/wk/internal/batch"
	"github.com/zach-short/wk/internal/changeset"
)

var t0 = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func rec(id, state string, at time.Time) changeset.Record {
	return changeset.Record{ID: id, State: state, At: at}
}

func ids(rs []changeset.Record) []string {
	var out []string
	for _, r := range rs {
		out = append(out, r.ID)
	}
	return out
}

func TestPick(t *testing.T) {
	idle := 120 * time.Second
	sub := changeset.Submitted
	tests := []struct {
		name    string
		records []changeset.Record
		max     int
		now     time.Time
		want    []string
	}{
		{"quiet batch forms", []changeset.Record{
			rec("a", sub, t0), rec("b", sub, t0.Add(10*time.Second)), rec("c", sub, t0.Add(20*time.Second)),
		}, 6, t0.Add(20*time.Second + idle), []string{"a", "b", "c"}},
		{"a burst still arriving waits", []changeset.Record{
			rec("a", sub, t0), rec("b", sub, t0.Add(90*time.Second)),
		}, 6, t0.Add(120 * time.Second), nil},
		{"quiet exactly idle long forms", []changeset.Record{
			rec("a", sub, t0),
		}, 6, t0.Add(idle), []string{"a"}},
		{"a full batch forms at once", []changeset.Record{
			rec("a", sub, t0), rec("b", sub, t0), rec("c", sub, t0),
		}, 2, t0, []string{"a", "b"}},
		{"refused, landed, gated and failed are never picked", []changeset.Record{
			rec("a", changeset.Landed, t0), rec("b", changeset.Refused, t0), rec("c", sub, t0),
			rec("d", changeset.Failed, t0), rec("e", changeset.Gated, t0), rec("f", sub, t0),
		}, 6, t0.Add(idle), []string{"c", "f"}},
		{"records that are not submitted do not count toward max", []changeset.Record{
			rec("a", changeset.Landed, t0), rec("b", sub, t0), rec("c", changeset.Refused, t0), rec("d", sub, t0),
		}, 2, t0, []string{"b", "d"}},
		{"queue order is kept, not time order", []changeset.Record{
			rec("late", sub, t0.Add(30*time.Second)), rec("early", sub, t0),
		}, 6, t0.Add(30*time.Second + idle), []string{"late", "early"}},
		{"an empty queue picks nothing", nil, 6, t0, nil},
		{"nothing submitted picks nothing", []changeset.Record{rec("a", changeset.Landed, t0)}, 6, t0.Add(idle), nil},
		{"a max under one picks nothing", []changeset.Record{rec("a", sub, t0)}, 0, t0.Add(idle), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := ids(batch.Pick(tc.records, tc.max, idle, tc.now)); !slices.Equal(got, tc.want) {
				t.Errorf("Pick = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHalves(t *testing.T) {
	sub := changeset.Submitted
	tests := []struct {
		name        string
		batch       []changeset.Record
		first, rest []string
		wantOK      bool
	}{
		{"four splits two and two", []changeset.Record{rec("a", sub, t0), rec("b", sub, t0), rec("c", sub, t0), rec("d", sub, t0)}, []string{"a", "b"}, []string{"c", "d"}, true},
		{"three puts the extra one first", []changeset.Record{rec("a", sub, t0), rec("b", sub, t0), rec("c", sub, t0)}, []string{"a", "b"}, []string{"c"}, true},
		{"two splits one and one", []changeset.Record{rec("a", sub, t0), rec("b", sub, t0)}, []string{"a"}, []string{"b"}, true},
		{"one cannot split", []changeset.Record{rec("a", sub, t0)}, []string{"a"}, nil, false},
		{"none cannot split", nil, nil, nil, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			first, rest, ok := batch.Halves(tc.batch)
			if !slices.Equal(ids(first), tc.first) || !slices.Equal(ids(rest), tc.rest) || ok != tc.wantOK {
				t.Errorf("Halves = %v, %v, %v; want %v, %v, %v", ids(first), ids(rest), ok, tc.first, tc.rest, tc.wantOK)
			}
		})
	}
}

func TestHalvesFirstDoesNotAliasSecond(t *testing.T) {
	b := []changeset.Record{rec("a", changeset.Submitted, t0), rec("b", changeset.Submitted, t0)}
	first, _, _ := batch.Halves(b)
	_ = append(first, rec("x", changeset.Submitted, t0))
	if b[1].ID != "b" {
		t.Errorf("appending to first overwrote the second half: batch[1] = %q", b[1].ID)
	}
}
