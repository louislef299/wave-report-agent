package spot

import (
	"fmt"
	"math"
	"testing"
)

func TestFetchArcContains(t *testing.T) {
	testCases := []struct {
		name    string
		arc     FetchArc
		bearing float64
		want    bool
	}{
		{"inside", FetchArc{FromDeg: 60, ToDeg: 100}, 80, true},
		{"lower bound inclusive", FetchArc{FromDeg: 60, ToDeg: 100}, 60, true},
		{"upper bound inclusive", FetchArc{FromDeg: 60, ToDeg: 100}, 100, true},
		{"below", FetchArc{FromDeg: 60, ToDeg: 100}, 59.9, false},
		{"above", FetchArc{FromDeg: 60, ToDeg: 100}, 100.1, false},

		// An arc spanning north wraps through 0. No configured spot uses one
		// today, so without these cases the wrap path would be untested and
		// would fail silently on the next spot added.
		{"wrapping, above from", FetchArc{FromDeg: 340, ToDeg: 20}, 350, true},
		{"wrapping, below to", FetchArc{FromDeg: 340, ToDeg: 20}, 10, true},
		{"wrapping, at 0", FetchArc{FromDeg: 340, ToDeg: 20}, 0, true},
		{"wrapping, outside", FetchArc{FromDeg: 340, ToDeg: 20}, 30, false},
		{"wrapping, far outside", FetchArc{FromDeg: 340, ToDeg: 20}, 180, false},

		// Bearings arrive from forecast data and are not guaranteed normalized.
		{"negative normalizes", FetchArc{FromDeg: 340, ToDeg: 20}, -10, true},
		{"over 360 normalizes", FetchArc{FromDeg: 340, ToDeg: 20}, 370, true},
		{"exactly 360 is 0", FetchArc{FromDeg: 340, ToDeg: 20}, 360, true},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.arc.Contains(tt.bearing); got != tt.want {
				t.Fatalf("FetchArc{%d,%d}.Contains(%v) = %v, want %v",
					tt.arc.FromDeg, tt.arc.ToDeg, tt.bearing, got, tt.want)
			}
		})
	}
}

func TestFetchMilesAt(t *testing.T) {
	stoney := mustSpot(t, "Stoney Point")
	empire := mustSpot(t, "Empire Beach")
	ocean := mustSpot(t, "Ocean Beach")

	testCases := []struct {
		name    string
		spot    Spot
		bearing float64
		want    float64
	}{
		// Superior's long axis lies E/ENE of Stoney Point. Wind from the
		// opposite quadrant crosses land and can build nothing at any speed.
		{"stoney down the long axis", stoney, 80, 300},
		{"stoney NE", stoney, 45, 150},
		{"stoney ESE", stoney, 115, 180},
		{"stoney SW is land", stoney, 225, 0},
		{"stoney W is land", stoney, 270, 0},
		{"stoney N is land", stoney, 0, 0},

		// Empire's productive window is the mirror image: S/SSW down the
		// length of Lake Michigan. Anything Superior-specific in the scoring
		// shows up here.
		{"empire down the long axis", empire, 200, 210},
		{"empire straight across", empire, 280, 70},
		{"empire NE is land", empire, 45, 0},
		{"empire E is land", empire, 90, 0},

		// Overlapping arcs meet at a shared boundary; fetch is continuous, so
		// the more generous arc is the physically correct answer.
		{"stoney at shared boundary", stoney, 60, 300},

		// Ocean spots have no arcs configured and must not report fetch.
		{"ocean spot has no arcs", ocean, 270, 0},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.spot.FetchMilesAt(tt.bearing); got != tt.want {
				t.Fatalf("%s.FetchMilesAt(%v) = %v, want %v", tt.spot.Name, tt.bearing, got, tt.want)
			}
		})
	}
}

func TestCardinal(t *testing.T) {
	testCases := []struct {
		in      string
		want    float64
		wantErr bool
	}{
		{in: "N", want: 0},
		{in: "NE", want: 45},
		{in: "E", want: 90},
		{in: "SSE", want: 157.5},
		{in: "S", want: 180},
		{in: "WSW", want: 247.5},
		{in: "W", want: 270},
		{in: "NNW", want: 337.5},
		{in: "sse", want: 157.5},   // case-insensitive
		{in: " SSE ", want: 157.5}, // tolerant of stray whitespace
		{in: "XX", wantErr: true},
		{in: "", wantErr: true},
	}

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("Cardinal(%q)", tt.in), func(t *testing.T) {
			got, err := Cardinal(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("Cardinal(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestOffshoreDeg(t *testing.T) {
	testCases := []struct {
		spotName string
		want     float64
	}{
		// Offshore wind blows from land out over the water, so it arrives from
		// the bearing opposite the one the beach faces.
		{spotName: "Stoney Point", want: 337.5}, // faces SSE
		{spotName: "Empire Beach", want: 90},    // faces W
		{spotName: "Ocean Beach", want: 67.5},   // faces WSW
	}

	for _, tt := range testCases {
		t.Run(tt.spotName, func(t *testing.T) {
			s := mustSpot(t, tt.spotName)
			got, err := s.OffshoreDeg()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("%s.OffshoreDeg() = %v, want %v", tt.spotName, got, tt.want)
			}
		})
	}
}

// TestLakeSpotsHaveFetchArcs guards the invariant that makes lake scoring
// possible at all: without arcs, every bearing reports zero fetch and no lake
// spot can ever rate above Poor.
func TestLakeSpotsHaveFetchArcs(t *testing.T) {
	for _, s := range spots {
		if s.SpotType != "lake" {
			continue
		}
		t.Run(s.Name, func(t *testing.T) {
			if len(s.FetchArcs) == 0 {
				t.Fatalf("lake spot %s has no fetch arcs configured", s.Name)
			}
			var best float64
			for _, a := range s.FetchArcs {
				if a.FromDeg < 0 || a.FromDeg > 359 || a.ToDeg < 0 || a.ToDeg > 359 {
					t.Errorf("arc %+v has a bearing outside [0,359]", a)
				}
				if a.Miles <= 0 {
					t.Errorf("arc %+v has non-positive fetch", a)
				}
				best = max(best, a.Miles)
			}
			// Superior and Michigan both run well over 100 miles along their
			// major axes; a lake spot whose best arc is short is misconfigured.
			if best < 100 {
				t.Errorf("best fetch for %s is only %v mi, expected a long-axis arc", s.Name, best)
			}
		})
	}
}

func mustSpot(t *testing.T, name string) Spot {
	t.Helper()
	res, err := GetSpotsOfInterest(t.Context(), SpotArgs{Name: name})
	if err != nil {
		t.Fatalf("looking up spot %q: %v", name, err)
	}
	return res.Spots[0]
}
