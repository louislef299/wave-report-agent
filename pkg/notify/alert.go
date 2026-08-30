package notify

import (
	"fmt"
	"strings"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/surf"
)

// FromVerdict renders a verdict as a notification in the given display
// timezone.
//
// The message has to survive being read on a lock screen at a glance, so the
// title carries the decision and the body carries only what would change
// whether you get in the car: when, how big, and how well built.
func FromVerdict(v surf.Verdict, city string, loc *time.Location) Notification {
	if loc == nil {
		loc = time.UTC
	}

	where := v.Spot
	if city != "" {
		where = fmt.Sprintf("%s, %s", v.Spot, city)
	}

	n := Notification{
		Title:    fmt.Sprintf("%s surf: %s (%s)", v.Rating, where, v.Board),
		Priority: priorityFor(v.Rating),
		Tags:     tagsFor(v.Rating),
	}

	var lines []string
	if w, ok := bestWindow(v); ok {
		lines = append(lines,
			formatWindow(w, loc),
			fmt.Sprintf("%.1fft at %.1fs · %dh of build", w.PeakWaveFt, w.PeakPeriodS, w.SustainedHours),
		)
		if extra := len(v.Windows) - 1; extra > 0 {
			lines = append(lines, fmt.Sprintf("%d more window(s) in range", extra))
		}
	}

	// The reasons are why it graded the way it did. Without them an alert is
	// an assertion you cannot check against the forecast you are about to go
	// look at anyway.
	lines = append(lines, v.Reasons...)
	n.Body = strings.Join(lines, "\n")

	return n
}

// formatWindow renders a session window, collapsing to one date when it does
// not cross midnight.
func formatWindow(w surf.Window, loc *time.Location) string {
	start, end := w.Start.In(loc), w.End.In(loc)
	sy, sm, sd := start.Date()
	ey, em, ed := end.Date()
	if sy == ey && sm == em && sd == ed {
		return fmt.Sprintf("%s %s–%s", start.Format("Mon Jan 2"), start.Format("15:04"), end.Format("15:04"))
	}
	return fmt.Sprintf("%s – %s", start.Format("Mon Jan 2 15:04"), end.Format("Mon Jan 2 15:04"))
}

func priorityFor(r surf.Rating) Priority {
	switch r {
	case surf.Epic:
		return PriorityUrgent
	case surf.Good:
		return PriorityHigh
	case surf.Fair:
		return PriorityDefault
	default:
		return PriorityLow
	}
}

// tagsFor picks ntfy emoji shortcodes. They are the only visual signal in a
// notification list, so they carry the rating.
func tagsFor(r surf.Rating) []string {
	switch r {
	case surf.Epic:
		return []string{"ocean", "fire"}
	case surf.Good:
		return []string{"ocean"}
	default:
		return []string{"droplet"}
	}
}

// bestWindow returns the window matching the verdict's overall rating, which
// is the one worth quoting.
func bestWindow(v surf.Verdict) (surf.Window, bool) {
	for _, w := range v.Windows {
		if w.Rating == v.Rating {
			return w, true
		}
	}
	if len(v.Windows) > 0 {
		return v.Windows[0], true
	}
	return surf.Window{}, false
}
