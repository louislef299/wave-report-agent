package surf

import (
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/spot"
)

// Rate targets lake spots, where the weather is the swell and there is no
// groundswell to arrive independently of local wind. Ocean spots need
// different criteria entirely (tide, swell period bands, groundswell vs
// windswell) and are still handled by the ADK agent.

// Board is what you plan to ride. It selects a threshold set rather than
// changing the algorithm: lake waves are steep and gutless, so a shortboard
// needs meaningfully more size and period than a longboard to make a section.
type Board string

const (
	Longboard  Board = "longboard"
	Shortboard Board = "shortboard"
)

// Rating uses the same vocabulary as the agent's prompt so the deterministic
// scorer and the narrative report cannot disagree about what a word means.
type Rating string

const (
	Poor Rating = "Poor"
	Fair Rating = "Fair"
	Good Rating = "Good"
	Epic Rating = "Epic"
)

var ratingOrder = map[Rating]int{Poor: 0, Fair: 1, Good: 2, Epic: 3}

// AtLeast reports whether r is at least as good as floor. Alerting uses it to
// decide what is worth interrupting someone for.
func (r Rating) AtLeast(floor Rating) bool {
	return ratingOrder[r] >= ratingOrder[floor]
}

// ParseRating converts a rating name, case-insensitively.
func ParseRating(s string) (Rating, error) {
	for _, r := range []Rating{Poor, Fair, Good, Epic} {
		if strings.EqualFold(string(r), s) {
			return r, nil
		}
	}
	return "", fmt.Errorf("%q is not a rating (Poor, Fair, Good, Epic)", s)
}

// RulesetVersion identifies the scoring rules in force. Bump it whenever
// DefaultThresholds or the gate logic changes: the ledger stamps every run
// with it, and without that stamp decisions made under different rules cannot
// be told apart when the stored history is later used for training.
const RulesetVersion = "v1"

// Thresholds are the tunable floors for one board. They are passed in rather
// than read from a global so that the ledger can record which values produced
// a decision and tuning them does not invalidate stored history.
type Thresholds struct {
	// MinWaveFt and MinPeriodS are hard floors; below either, nothing is
	// surfable. Multiples of these floors set where Fair becomes Good and Good
	// becomes Epic.
	MinWaveFt  float64
	MinPeriodS float64

	// MinWindMph and MinFetchMiles define a "productive" hour of wind: fast
	// enough to build, from a bearing with enough open water to build across.
	MinWindMph    float64
	MinFetchMiles float64

	// MinSustainedHours is how long productive wind must have held for the sea
	// to count as properly developed rather than a passing squall.
	MinSustainedHours int
}

// DefaultThresholds returns the seed values for a board. These are starting
// points calibrated against Lake Superior, not measured constants; they are
// expected to move once the ledger has enough labelled history to check them
// against.
func DefaultThresholds(b Board) Thresholds {
	switch b {
	case Shortboard:
		return Thresholds{
			MinWaveFt:         3.5,
			MinPeriodS:        4.5,
			MinWindMph:        18,
			MinFetchMiles:     150,
			MinSustainedHours: 12,
		}
	default:
		return Thresholds{
			MinWaveFt:         2.0,
			MinPeriodS:        3.5,
			MinWindMph:        15,
			MinFetchMiles:     80,
			MinSustainedHours: 8,
		}
	}
}

// Window is a contiguous run of surfable forecast hours.
//
// The measurements describe the window's single best hour rather than the
// maximum of each field taken independently, which could otherwise report a
// height from one hour beside a period from another and describe an hour that
// never existed.
type Window struct {
	Start, End     time.Time
	PeakWaveFt     float64
	PeakPeriodS    float64
	SustainedHours int
	Rating         Rating
}

// Verdict is the scorer's full output for one spot and board.
type Verdict struct {
	Spot    string
	Board   Board
	Rating  Rating
	Windows []Window

	// Reasons explains the outcome in plain language, on both a pass and a
	// fail. It is what you read when a day looked good and no alert fired.
	Reasons []string
}

// Rate scores the forecast portion of a series. Past hours are never rated —
// they exist only to supply the wind history that duration is computed from.
func Rate(c Conditions, s spot.Spot, b Board, th Thresholds) Verdict {
	v := Verdict{Spot: c.Spot, Board: b, Rating: Poor}

	offshore, err := s.OffshoreDeg()
	if err != nil {
		v.Reasons = append(v.Reasons, fmt.Sprintf("cannot determine offshore bearing: %v", err))
		return v
	}

	forecast := c.Forecast()
	if len(forecast) == 0 {
		v.Reasons = append(v.Reasons, "no forecast hours available")
		return v
	}

	// Score every forecast hour, then collapse the surfable ones into windows.
	type scored struct {
		hour      Hour
		rating    Rating
		sustained int
		reasons   []string
	}
	scores := make([]scored, len(forecast))
	offset := len(c.Hours) - len(forecast)

	for i, h := range forecast {
		sustained := buildRun(c.Hours, offset+i, s, th)
		rating, reasons := rateHour(h, sustained, offshore, th)
		scores[i] = scored{hour: h, rating: rating, sustained: sustained, reasons: reasons}
	}

	// Collect contiguous runs of surfable hours, remembering which hour in
	// each run scored best so the window can be described by that one hour.
	better := func(i, j int) bool {
		if ratingOrder[scores[i].rating] != ratingOrder[scores[j].rating] {
			return ratingOrder[scores[i].rating] > ratingOrder[scores[j].rating]
		}
		return scores[i].hour.WaveHeightFt > scores[j].hour.WaveHeightFt
	}

	type windowAcc struct {
		win     Window
		bestIdx int
	}
	var accs []windowAcc
	curIdx := -1

	for i, sc := range scores {
		if sc.rating == Poor {
			curIdx = -1
			continue
		}

		if curIdx < 0 {
			accs = append(accs, windowAcc{
				win:     Window{Start: sc.hour.Time, Rating: sc.rating},
				bestIdx: i,
			})
			curIdx = len(accs) - 1
		}

		acc := &accs[curIdx]
		acc.win.End = sc.hour.Time
		if ratingOrder[sc.rating] > ratingOrder[acc.win.Rating] {
			acc.win.Rating = sc.rating
		}
		if better(i, acc.bestIdx) {
			acc.bestIdx = i
		}
	}

	bestWindow := -1
	for i, acc := range accs {
		best := scores[acc.bestIdx]
		acc.win.PeakWaveFt = best.hour.WaveHeightFt
		acc.win.PeakPeriodS = best.hour.WavePeriodS
		acc.win.SustainedHours = best.sustained
		v.Windows = append(v.Windows, acc.win)

		if ratingOrder[acc.win.Rating] > ratingOrder[v.Rating] {
			v.Rating = acc.win.Rating
			bestWindow = i
		}
	}

	// Explain the best hour on a pass. On a total washout, explain the hour
	// that came closest, which is more useful than the first hour in the list.
	explain := 0
	if bestWindow >= 0 {
		explain = accs[bestWindow].bestIdx
	} else {
		for i, sc := range scores {
			if sc.hour.WaveHeightFt > scores[explain].hour.WaveHeightFt {
				explain = i
			}
		}
	}
	v.Reasons = scores[explain].reasons

	if c.Gaps > 0 {
		v.Reasons = append(v.Reasons,
			fmt.Sprintf("%d hours dropped for incomplete readings; model coverage here may be thin", c.Gaps))
	}
	return v
}

// rateHour grades a single hour by starting at Epic and applying caps. Caps
// read more clearly than a weighted score and each one carries its own
// explanation, so a verdict can always say what held it back.
func rateHour(h Hour, sustained int, offshoreDeg float64, th Thresholds) (Rating, []string) {
	var reasons []string
	rating := Epic

	capAt := func(limit Rating, format string, args ...any) {
		if ratingOrder[limit] < ratingOrder[rating] {
			rating = limit
		}
		reasons = append(reasons, fmt.Sprintf(format, args...))
	}

	// Period is the quality gate. On a lake, height without period is chop:
	// the wave stands up fast, carries little water, and does not push.
	switch {
	case h.WavePeriodS < th.MinPeriodS:
		capAt(Poor, "period %.1fs is below the %.1fs floor — texture, not surf", h.WavePeriodS, th.MinPeriodS)
	case h.WavePeriodS < th.MinPeriodS+1:
		capAt(Fair, "period %.1fs is only just above the floor", h.WavePeriodS)
	case h.WavePeriodS < th.MinPeriodS+2:
		capAt(Good, "period %.1fs is workable but short of well-organized", h.WavePeriodS)
	}

	switch {
	case h.WaveHeightFt < th.MinWaveFt:
		capAt(Poor, "%.1fft is below the %.1fft floor for a %s", h.WaveHeightFt, th.MinWaveFt, boardWord(th))
	case h.WaveHeightFt < th.MinWaveFt*1.5:
		capAt(Fair, "%.1fft is rideable but small", h.WaveHeightFt)
	case h.WaveHeightFt < th.MinWaveFt*2:
		capAt(Good, "%.1fft is a solid size without being a standout", h.WaveHeightFt)
	}

	// Duration separates a developed sea from a passing squall. The marine
	// model already accounts for fetch in the height it reports, so a short
	// build caps quality rather than vetoing the waves outright — leftover
	// swell is still surfable, just not a signal worth driving for.
	switch {
	case sustained == 0:
		capAt(Fair, "no sustained wind above %.0f mph from a bearing with %.0f+ mi of fetch in the lookback window",
			th.MinWindMph, th.MinFetchMiles)
	case sustained < th.MinSustainedHours:
		capAt(Good, "only %dh of productive wind against a %dh target — sea is still building",
			sustained, th.MinSustainedHours)
	}

	// Offshore wind holds a face up; onshore crumbles it from behind. On
	// Superior this is the post-frontal veer, and it is what separates a
	// groomed session from the messy peak of the blow.
	off := angleDiff(h.WindDirDeg, offshoreDeg)
	switch {
	case off <= 60:
		// Offshore or near enough. No cap; this is what Epic requires.
	case off <= 120:
		capAt(Good, "cross-shore wind at %.0f° is %0.f° off offshore", h.WindDirDeg, off)
	case h.WindSpeedMph >= 20:
		capAt(Fair, "%.0f mph onshore wind will have it blown out", h.WindSpeedMph)
	default:
		capAt(Good, "light onshore wind at %.0f mph", h.WindSpeedMph)
	}

	if rating == Epic {
		reasons = append(reasons, fmt.Sprintf(
			"%.1fft at %.1fs with %dh of build and offshore wind — everything lines up",
			h.WaveHeightFt, h.WavePeriodS, sustained))
	}
	return rating, reasons
}

// buildRun returns the longest unbroken run of productive wind hours within
// the lookback window ending at hours[idx].
//
// It deliberately looks for a run *somewhere* in the window rather than one
// still in progress. The best sessions come after the wind has veered
// offshore, which is by definition a bearing with no fetch — requiring the
// blow to be ongoing would score exactly the hours worth surfing as unbuilt.
func buildRun(hours []Hour, idx int, s spot.Spot, th Thresholds) int {
	if idx < 0 || idx >= len(hours) {
		return 0
	}

	lookback := time.Duration(th.MinSustainedHours*3) * time.Hour
	cutoff := hours[idx].Time.Add(-lookback)

	var best, run int
	var prev time.Time
	for _, h := range hours {
		if h.Time.Before(cutoff) || h.Time.After(hours[idx].Time) {
			continue
		}

		// A gap in the series breaks the run: hours are dropped when a source
		// reports nothing, and counting across the hole would overstate how
		// long the wind actually held.
		if !prev.IsZero() && h.Time.Sub(prev) != time.Hour {
			run = 0
		}
		prev = h.Time

		if h.WindSpeedMph >= th.MinWindMph && s.FetchMilesAt(h.WindDirDeg) >= th.MinFetchMiles {
			run++
			best = max(best, run)
		} else {
			run = 0
		}
	}
	return best
}

// angleDiff returns the smallest angle between two bearings, in [0, 180].
func angleDiff(a, b float64) float64 {
	d := math.Abs(spot.NormalizeDeg(a) - spot.NormalizeDeg(b))
	if d > 180 {
		d = 360 - d
	}
	return d
}

func boardWord(th Thresholds) string {
	if th.MinWaveFt >= DefaultThresholds(Shortboard).MinWaveFt {
		return "shortboard"
	}
	return "longboard"
}
