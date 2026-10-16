// Package hive is the terminal view of the worktree hive: the same verdicts, colors and
// honeycomb layout as the page wk viz serves, redrawn in place. It only reads.
package hive

import (
	"math"
	"regexp"
	"sort"
	"strconv"

	"github.com/charmbracelet/lipgloss"
	"github.com/zach-short/landing-layer/internal/wk"
)

// Key names one cell style. The page's styleOf in internal/viz/index.html makes the same choice;
// change both together.
type Key string

const (
	Working   Key = "working"
	Idle      Key = "idle"
	Land      Key = "land"
	Removable Key = "removable"
	Skip      Key = "skip"
	Prunable  Key = "prunable"
)

// Keys is the legend order, the same as the page's chips.
var Keys = []Key{Working, Idle, Land, Removable, Skip, Prunable}

var labels = map[Key]string{
	Working: "working", Idle: "kept", Land: "run /land-worktrees",
	Removable: "removable", Skip: "skipped", Prunable: "prunable",
}

// Label is the text the legend and the detail use.
func (k Key) Label() string { return labels[k] }

// Short is the text a cell carries under its name, which must fit the cell; the page uses the
// same words.
func (k Key) Short() string {
	if k == Land {
		return "to land"
	}
	return labels[k]
}

var busy = regexp.MustCompile(`changed \d+ minutes? ago|process is working`)

// StyleOf maps a verdict to its style. A kept worktree with a live session reads as working.
func StyleOf(it wk.ReportItem) Key {
	switch it.Verdict {
	case wk.Removable:
		return Removable
	case wk.KeptForLanding:
		return Land
	case wk.Skipped:
		return Skip
	case wk.Prunable:
		return Prunable
	}
	for _, r := range it.Reasons {
		if busy.MatchString(r) {
			return Working
		}
	}
	return Idle
}

// palette is the page's default Bioluminescent direction: its light and dark verdict colors.
var palette = map[Key]lipgloss.AdaptiveColor{
	Working:   {Light: "#12a37a", Dark: "#54f0b0"},
	Idle:      {Light: "#c98a12", Dark: "#ffc65c"},
	Land:      {Light: "#d64570", Dark: "#ff6f91"},
	Removable: {Light: "#8aa3a8", Dark: "#6d8a92"},
	Skip:      {Light: "#2f7fd1", Dark: "#6ab8ff"},
	Prunable:  {Light: "#b6cccf", Dark: "#34525a"},
}

var (
	hubFill = lipgloss.AdaptiveColor{Light: "#0b3036", Dark: "#0b2b33"}
	hubInk  = lipgloss.AdaptiveColor{Light: "#8ff5e8", Dark: "#5ef2e0"}
	muted   = lipgloss.AdaptiveColor{Light: "#4d7a80", Dark: "#6fa5ad"}
)

// cellStyle fills a cell with its verdict color and picks a readable ink for each background.
func cellStyle(k Key) lipgloss.Style {
	c := palette[k]
	return lipgloss.NewStyle().
		Background(c).
		Foreground(lipgloss.AdaptiveColor{Light: inkFor(c.Light), Dark: inkFor(c.Dark)}).
		Width(cellWidth).
		Align(lipgloss.Center)
}

// inkFor returns whichever of near-black and near-white has the higher contrast on the fill.
func inkFor(hex string) string {
	const dark, light = "#1d1408", "#fffaf0"
	fill := luminance(hex)
	onDark := (fill + 0.05) / (luminance(dark) + 0.05)
	onLight := (luminance(light) + 0.05) / (fill + 0.05)
	if onDark >= onLight {
		return dark
	}
	return light
}

func luminance(hex string) float64 {
	if len(hex) != 7 || hex[0] != '#' {
		return 0
	}
	ch := func(s string) float64 {
		v, _ := strconv.ParseUint(s, 16, 8)
		c := float64(v) / 255
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*ch(hex[1:3]) + 0.7152*ch(hex[3:5]) + 0.0722*ch(hex[5:7])
}

// Slot is one cell position in axial hex coordinates; D is its ring, counted from the hub.
type Slot struct{ Q, R, D int }

// Slots returns the first n positions around the hub, ring by ring and by angle within a ring:
// the same order as ringCoords on the page, so a worktree sits in the same place in both views.
func Slots(n int) []Slot {
	type slot struct {
		Slot
		a float64
	}
	var all []slot
	for q := -8; q <= 8; q++ {
		for r := -8; r <= 8; r++ {
			d := (abs(q) + abs(r) + abs(q+r)) / 2
			if d > 0 {
				all = append(all, slot{Slot{q, r, d}, math.Atan2(float64(r)+float64(q)/2, float64(q)+0.0001)})
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].D != all[j].D {
			return all[i].D < all[j].D
		}
		return all[i].a < all[j].a
	})
	if n > len(all) {
		n = len(all)
	}
	out := make([]Slot, n)
	for i := range out {
		out[i] = all[i].Slot
	}
	return out
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
