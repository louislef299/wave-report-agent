package surf

import (
	"strings"
	"testing"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/spot"
)

// Bearings used throughout. Stoney Point's productive window is ENE-E down
// Superior's major axis; Empire Beach's is S-SSW down Lake Michigan. Keeping
// them named makes the mirror-image tests readable.
const (
	stoneyBuildDeg = 70  // ENE, 300 mi of fetch
	stoneyGroomDeg = 330 // NW, offshore for an SSE-facing shore
	stoneyDeadDeg  = 225 // SW, across Minnesota

	empireBuildDeg = 200 // SSW, 210 mi of fetch
	empireGroomDeg = 90  // E, offshore for a W-facing shore
)

func TestRateFlatSummerLake(t *testing.T) {
	// Late August at Stoney: light NE breeze, sub-foot at 2.2s. This is the
	// common case, and anything but Poor here means the scorer is broken.
	c := conditions(t, "Stoney Point",
		block{n: 48, wave: 0.5, period: 2.2, wind: 8, dir: stoneyBuildDeg})

	for _, b := range []Board{Longboard, Shortboard} {
		t.Run(string(b), func(t *testing.T) {
			v := Rate(c, mustSpot(t, "Stoney Point"), b, DefaultThresholds(b))
			if v.Rating != Poor {
				t.Fatalf("rating = %s, want Poor (reasons: %v)", v.Rating, v.Reasons)
			}
			if len(v.Windows) != 0 {
				t.Errorf("got %d windows, want none", len(v.Windows))
			}
		})
	}
}

// TestRatePeriodGateDominatesHeight is the lesson from the North Shore
// research: a big number at 3s is chop, not surf. Height must never rescue a
// sub-threshold period.
func TestRatePeriodGateDominatesHeight(t *testing.T) {
	c := conditions(t, "Stoney Point",
		block{n: 24, wave: 6.0, period: 3.0, wind: 30, dir: stoneyBuildDeg, hist: true},
		block{n: 12, wave: 6.0, period: 3.0, wind: 30, dir: stoneyGroomDeg})

	for _, b := range []Board{Longboard, Shortboard} {
		t.Run(string(b), func(t *testing.T) {
			v := Rate(c, mustSpot(t, "Stoney Point"), b, DefaultThresholds(b))
			if v.Rating != Poor {
				t.Fatalf("6ft at 3.0s rated %s, want Poor", v.Rating)
			}
			if !hasReason(v.Reasons, "period") {
				t.Errorf("reasons %v should name the period gate", v.Reasons)
			}
		})
	}
}

// TestRateSuperiorSequence encodes the pattern the whole system exists to
// catch: a NE fetch builds the sea over a day, then the front passes and the
// wind veers NW, grooming what was already built. The groomed hours must rate
// above the peak of the blow even though the wave data is identical.
func TestRateSuperiorSequence(t *testing.T) {
	stoney := mustSpot(t, "Stoney Point")
	th := DefaultThresholds(Longboard)

	building := conditions(t, "Stoney Point",
		block{n: 24, wave: 5.0, period: 6.0, wind: 25, dir: stoneyBuildDeg, hist: true},
		block{n: 6, wave: 5.0, period: 6.0, wind: 25, dir: stoneyBuildDeg})

	groomed := conditions(t, "Stoney Point",
		block{n: 24, wave: 5.0, period: 6.0, wind: 25, dir: stoneyBuildDeg, hist: true},
		block{n: 6, wave: 5.0, period: 6.0, wind: 18, dir: stoneyGroomDeg})

	bv := Rate(building, stoney, Longboard, th)
	gv := Rate(groomed, stoney, Longboard, th)

	if bv.Rating != Good {
		t.Errorf("cross-shore peak of the blow rated %s, want Good (reasons: %v)", bv.Rating, bv.Reasons)
	}
	if gv.Rating != Epic {
		t.Errorf("post-frontal offshore groom rated %s, want Epic (reasons: %v)", gv.Rating, gv.Reasons)
	}
	if ratingOrder[gv.Rating] <= ratingOrder[bv.Rating] {
		t.Errorf("groomed (%s) should outrank building (%s) on identical wave data", gv.Rating, bv.Rating)
	}
}

// TestRateFetchGatesTheBuild is the point of storing fetch per bearing. Strong
// wind off a bearing backed by land leaves leftover swell unsupported, so it
// cannot grade as a developing swell.
func TestRateFetchGatesTheBuild(t *testing.T) {
	stoney := mustSpot(t, "Stoney Point")
	th := DefaultThresholds(Longboard)

	// Identical waves and wind speed. Only the bearing differs.
	productive := conditions(t, "Stoney Point",
		block{n: 30, wave: 3.0, period: 5.0, wind: 22, dir: stoneyBuildDeg})
	dead := conditions(t, "Stoney Point",
		block{n: 30, wave: 3.0, period: 5.0, wind: 22, dir: stoneyDeadDeg})

	pv := Rate(productive, stoney, Longboard, th)
	dv := Rate(dead, stoney, Longboard, th)

	if ratingOrder[dv.Rating] >= ratingOrder[pv.Rating] {
		t.Fatalf("SW wind rated %s vs ENE %s; a zero-fetch bearing must not grade as well",
			dv.Rating, pv.Rating)
	}
	if !hasReason(dv.Reasons, "fetch") && !hasReason(dv.Reasons, "sustained") {
		t.Errorf("reasons %v should explain the missing build", dv.Reasons)
	}
}

// TestRateEmpireMirrorsStoney is the guard against Superior-specific
// constants. Empire's productive bearing is roughly opposite Stoney's, so any
// hardcoded "NE is good" would fail here.
func TestRateEmpireMirrorsStoney(t *testing.T) {
	empire := mustSpot(t, "Empire Beach")
	th := DefaultThresholds(Longboard)

	groomed := conditions(t, "Empire Beach",
		block{n: 24, wave: 5.0, period: 6.0, wind: 25, dir: empireBuildDeg, hist: true},
		block{n: 6, wave: 5.0, period: 6.0, wind: 18, dir: empireGroomDeg})

	if v := Rate(groomed, empire, Longboard, th); v.Rating != Epic {
		t.Errorf("Empire post-frontal groom rated %s, want Epic (reasons: %v)", v.Rating, v.Reasons)
	}

	// Stoney's build bearing is Empire's offshore-ish quadrant and vice versa.
	// Feeding Empire's data to Stoney must not produce a build.
	stoneyOnEmpireWind := conditions(t, "Stoney Point",
		block{n: 30, wave: 3.0, period: 5.0, wind: 25, dir: empireBuildDeg})
	v := Rate(stoneyOnEmpireWind, mustSpot(t, "Stoney Point"), Longboard, th)
	if v.Rating == Epic || v.Rating == Good {
		t.Errorf("SSW wind at Stoney rated %s; that bearing has no fetch there", v.Rating)
	}
}

// TestRateBoardFloorsDiffer covers the ask that started this: a day that is
// worth a longboard is often not worth a shortboard. Lake waves are gutless,
// so the shortboard floor sits meaningfully higher.
func TestRateBoardFloorsDiffer(t *testing.T) {
	stoney := mustSpot(t, "Stoney Point")

	// Small but clean and well built: a longboard day.
	c := conditions(t, "Stoney Point",
		block{n: 30, wave: 2.6, period: 4.0, wind: 20, dir: stoneyBuildDeg})

	lv := Rate(c, stoney, Longboard, DefaultThresholds(Longboard))
	sv := Rate(c, stoney, Shortboard, DefaultThresholds(Shortboard))

	if lv.Rating == Poor {
		t.Errorf("2.6ft at 4.0s rated %s for a longboard, expected something rideable", lv.Rating)
	}
	if sv.Rating != Poor {
		t.Errorf("2.6ft at 4.0s rated %s for a shortboard, want Poor", sv.Rating)
	}
}

// TestRateWindowsCoverPassingHours checks that surfable hours are collapsed
// into contiguous windows with the peak values carried through.
func TestRateWindowsCoverPassingHours(t *testing.T) {
	c := conditions(t, "Stoney Point",
		block{n: 24, wave: 4.0, period: 6.0, wind: 25, dir: stoneyBuildDeg, hist: true}, // history
		block{n: 4, wave: 4.5, period: 6.0, wind: 20, dir: stoneyGroomDeg},              // surfable
		block{n: 4, wave: 0.4, period: 2.0, wind: 5, dir: stoneyGroomDeg},               // dies off
		block{n: 3, wave: 4.8, period: 6.2, wind: 18, dir: stoneyGroomDeg},              // brief return
	)

	v := Rate(c, mustSpot(t, "Stoney Point"), Longboard, DefaultThresholds(Longboard))
	if len(v.Windows) != 2 {
		t.Fatalf("got %d windows, want 2 separated by the flat spell", len(v.Windows))
	}
	if got := v.Windows[0].End.Sub(v.Windows[0].Start); got != 3*time.Hour {
		t.Errorf("first window spans %v, want 3h across 4 hourly samples", got)
	}
	if v.Windows[1].PeakWaveFt != 4.8 {
		t.Errorf("second window peak = %v ft, want 4.8", v.Windows[1].PeakWaveFt)
	}
	if v.Rating != Epic {
		t.Errorf("overall rating = %s, want the best window's Epic", v.Rating)
	}
}

// TestRateIgnoresHistoryWhenRating makes sure past hours only feed the
// duration calculation and never produce a window of their own — you cannot
// surf yesterday.
func TestRateIgnoresHistoryWhenRating(t *testing.T) {
	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	c := Conditions{Spot: "Stoney Point", FetchedAt: base.Add(30 * time.Hour)}
	for i := range 30 {
		c.Hours = append(c.Hours, Hour{
			Time: base.Add(time.Duration(i) * time.Hour),
			// Excellent conditions, but entirely in the past.
			WaveHeightFt: 6, WavePeriodS: 7, WindSpeedMph: 25, WindDirDeg: stoneyGroomDeg,
		})
	}

	v := Rate(c, mustSpot(t, "Stoney Point"), Longboard, DefaultThresholds(Longboard))
	if v.Rating != Poor || len(v.Windows) != 0 {
		t.Fatalf("rating = %s with %d windows; history must not produce windows", v.Rating, len(v.Windows))
	}
}

func TestRateEmptyConditions(t *testing.T) {
	v := Rate(Conditions{Spot: "Stoney Point"}, mustSpot(t, "Stoney Point"),
		Longboard, DefaultThresholds(Longboard))
	if v.Rating != Poor {
		t.Errorf("rating = %s on empty conditions, want Poor", v.Rating)
	}
	if len(v.Reasons) == 0 {
		t.Error("expected a reason explaining the absence of data")
	}
}

// block describes a run of identical hours. Blocks marked hist stand in for
// what past_days supplies in production: observed wind that feeds the duration
// calculation but is never itself rated.
type block struct {
	n      int
	wave   float64
	period float64
	wind   float64
	dir    float64
	hist   bool
}

func conditions(t *testing.T, spotName string, blocks ...block) Conditions {
	t.Helper()

	var history int
	for _, b := range blocks {
		if b.hist {
			history += b.n
		}
	}

	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	c := Conditions{Spot: spotName, FetchedAt: base.Add(time.Duration(history) * time.Hour)}

	var i int
	for _, b := range blocks {
		for range b.n {
			c.Hours = append(c.Hours, Hour{
				Time:         base.Add(time.Duration(i) * time.Hour),
				WaveHeightFt: b.wave,
				WavePeriodS:  b.period,
				WaveDirDeg:   b.dir,
				WindSpeedMph: b.wind,
				WindDirDeg:   b.dir,
				WindGustMph:  b.wind * 1.3,
			})
			i++
		}
	}
	return c
}

func mustSpot(t *testing.T, name string) spot.Spot {
	t.Helper()
	res, err := spot.GetSpotsOfInterest(t.Context(), spot.SpotArgs{Name: name})
	if err != nil {
		t.Fatalf("looking up spot %q: %v", name, err)
	}
	return res.Spots[0]
}

func hasReason(reasons []string, substr string) bool {
	for _, r := range reasons {
		if strings.Contains(strings.ToLower(r), substr) {
			return true
		}
	}
	return false
}
