package slot

import (
	"fmt"
	"strings"
	"text/tabwriter"
)

// The text shows the head of each ranking; --json lists every row.
const (
	commandsShown = 15
	holdersShown  = 10
	repeatsShown  = 10
	commandWidth  = 90
)

// Text is the report for a terminal. Durations read as seconds, minutes or hours by size.
func (s Stats) Text() string {
	var b strings.Builder
	fmt.Fprintf(&b, "runs.log in %s, %s: %s, %s\n", s.Dir, s.span(), plural(s.Runs, "run"), plural(s.Malformed, "unreadable line"))
	for _, cs := range s.Classes {
		cs.text(&b)
	}
	s.Repeats.text(&b)
	s.Missed.text(&b)
	s.Deadlines.text(&b)
	return b.String()
}

func (s Stats) span() string {
	switch {
	case s.Since == "" && s.Until == "":
		return "every date"
	case s.Until == "":
		return "from " + s.Since
	case s.Since == "":
		return "to " + s.Until
	}
	return s.Since + " to " + s.Until
}

func (cs ClassStats) text(b *strings.Builder) {
	fmt.Fprintf(b, "\n%s: %s, %s\n", cs.Name, plural(cs.Runs, "run"), plural(cs.Failed, "nonzero exit"))
	fmt.Fprintf(b, "  wait %s; run %s\n", cs.Wait.Text(), cs.Run.Text())
	o := cs.Overlap
	fmt.Fprintf(b, "  overlap: %s of waiting inside %s of wall clock, peak %d queued", dur(o.AgentSeconds), dur(o.WallSeconds), o.PeakQueued)
	if !o.PeakAt.IsZero() {
		fmt.Fprintf(b, " at %s", o.PeakAt.Format(logTimeLayout))
	}
	fmt.Fprintf(b, "\n  waits over %s: %d, %s in all\n", dur(longWait.Seconds()), cs.LongWaits.Count, dur(cs.LongWaits.TotalSeconds))
	cs.LongWaits.text(b)
	cs.commandsText(b)
}

func (lw LongWaits) text(b *strings.Builder) {
	if len(lw.Holders) == 0 {
		return
	}
	fmt.Fprintf(b, "  held the places during them:\n")
	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, h := range lw.Holders[:min(len(lw.Holders), holdersShown)] {
		_, _ = fmt.Fprintf(tw, "    %s\t%s\t%s\t%s\n", dur(h.HeldSeconds), plural(h.Runs, "run"), h.Subdir, clip(h.Command))
	}
	_ = tw.Flush()
}

func (cs ClassStats) commandsText(b *strings.Builder) {
	shown := cs.Commands[:min(len(cs.Commands), commandsShown)]
	fmt.Fprintf(b, "  commands by total wait, %d of %d:\n", len(shown), len(cs.Commands))
	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', tabwriter.AlignRight)
	_, _ = fmt.Fprintf(tw, "runs\tnonzero\twait\tmedian\tp90\trun\tmedian\t  subdir  command\n")
	for _, c := range shown {
		_, _ = fmt.Fprintf(tw, "%d\t%d\t%s\t%s\t%s\t%s\t%s\t  %s  %s\n", c.Runs, c.Failed,
			dur(c.Wait.Total), dur(c.Wait.Median), dur(c.Wait.P90), dur(c.Run.Total), dur(c.Run.Median), c.Subdir, clip(c.Command))
	}
	_ = tw.Flush()
}

func (rep Repeats) text(b *strings.Builder) {
	fmt.Fprintf(b, "\nrepeats (the same command in the same directory, issued within %s of the last one ending): %d, %d after a nonzero exit, %s of run time\n",
		dur(repeatWindow.Seconds()), rep.Count, rep.AfterFailure, dur(rep.RunSeconds))
	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, r := range rep.Commands[:min(len(rep.Commands), repeatsShown)] {
		_, _ = fmt.Fprintf(tw, "  %d\t%s\t%s\n", r.Count, r.Subdir, clip(r.Command))
	}
	_ = tw.Flush()
}

func (ms MissedStats) text(b *strings.Builder) {
	fmt.Fprintf(b, "\nmissed.log: %s\n", plural(ms.Lines, "line"))
	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, r := range ms.Reasons {
		_, _ = fmt.Fprintf(tw, "  %d\t%s\n", r.Count, r.Reason)
	}
	_ = tw.Flush()
}

func (ds DeadlineStats) text(b *strings.Builder) {
	fmt.Fprintf(b, "\nerrors.log: %d hook deadline misses", ds.Count)
	if ds.Count > 0 {
		fmt.Fprintf(b, " (deadline %d ms), latency %d to %d ms", ds.DeadlineMS, ds.MinMS, ds.MaxMS)
	}
	fmt.Fprintf(b, "; %s\n", plural(ds.Other, "other line"))
}

// Text is a distribution on one line.
func (d Distribution) Text() string {
	if d.Count == 0 {
		return "none"
	}
	return fmt.Sprintf("%s in %d, median %s, p90 %s", dur(d.Total), d.Count, dur(d.Median), dur(d.P90))
}

func dur(sec float64) string {
	switch {
	case sec < 60:
		return fmt.Sprintf("%.1fs", sec)
	case sec < 3600:
		return fmt.Sprintf("%dm%02ds", int(sec)/60, int(sec)%60)
	}
	return fmt.Sprintf("%.1fh", sec/3600)
}

func clip(s string) string {
	if len(s) <= commandWidth {
		return s
	}
	return s[:commandWidth-3] + "..."
}
