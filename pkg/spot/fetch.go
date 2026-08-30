package spot

import (
	"fmt"
	"math"
	"strings"
)

// FetchArc records how many miles of open water lie upwind of a spot for wind
// arriving from a range of bearings.
//
// Fetch is the single strongest predictor of lake surf. Wave growth is capped
// by whichever of fetch, wind speed, or duration runs out first, and on an
// enclosed lake it is almost always fetch. A gale from a bearing backed by
// land builds nothing no matter how hard it blows, which is why a spot's
// "facing" direction alone cannot decide whether wind is productive.
//
// Bearings are degrees true and follow the meteorological convention of the
// direction wind blows FROM, matching both NDBC and Open-Meteo. Endpoints are
// inclusive, and an arc may wrap through 0 (e.g. FromDeg 340, ToDeg 20).
type FetchArc struct {
	FromDeg int     `json:"from_deg" jsonschema_description:"Start of the bearing range in degrees true, inclusive."`
	ToDeg   int     `json:"to_deg" jsonschema_description:"End of the bearing range in degrees true, inclusive. May be less than from_deg, meaning the arc wraps through 0."`
	Miles   float64 `json:"miles" jsonschema_description:"Miles of open water upwind across this bearing range."`
}

// Contains reports whether a wind bearing falls inside the arc. The bearing is
// normalized first, so callers may pass raw forecast values.
func (a FetchArc) Contains(bearingDeg float64) bool {
	b := NormalizeDeg(bearingDeg)
	from, to := float64(a.FromDeg), float64(a.ToDeg)

	if from <= to {
		return b >= from && b <= to
	}
	// The arc wraps through 0, so it covers two spans: from..360 and 0..to.
	return b >= from || b <= to
}

// FetchMilesAt returns the miles of open water upwind at the given bearing, or
// 0 when no arc covers it (the bearing points at land, or the spot has no arcs
// configured — ocean spots do not use them).
//
// Arcs are a coarse discretization of a continuous quantity and may share
// boundaries, so an overlap resolves to the longest matching fetch.
func (s Spot) FetchMilesAt(bearingDeg float64) float64 {
	var miles float64
	for _, a := range s.FetchArcs {
		if a.Contains(bearingDeg) {
			miles = max(miles, a.Miles)
		}
	}
	return miles
}

// OffshoreDeg returns the bearing that offshore wind arrives from: the reverse
// of the direction the beach faces. Offshore wind holds up a wave face rather
// than crumbling it from behind, which on a lake is what separates a groomed
// swell from a blown-out one.
func (s Spot) OffshoreDeg() (float64, error) {
	facing, err := Cardinal(s.Facing)
	if err != nil {
		return 0, fmt.Errorf("spot %s: %w", s.Name, err)
	}
	return NormalizeDeg(facing + 180), nil
}

// NormalizeDeg maps any angle onto [0, 360).
func NormalizeDeg(deg float64) float64 {
	d := math.Mod(deg, 360)
	if d < 0 {
		d += 360
	}
	return d
}

// cardinals maps the 16-point compass to degrees true.
var cardinals = map[string]float64{
	"N": 0, "NNE": 22.5, "NE": 45, "ENE": 67.5,
	"E": 90, "ESE": 112.5, "SE": 135, "SSE": 157.5,
	"S": 180, "SSW": 202.5, "SW": 225, "WSW": 247.5,
	"W": 270, "WNW": 292.5, "NW": 315, "NNW": 337.5,
}

// Cardinal converts a 16-point compass abbreviation to degrees true. Deriving
// the angle from Spot.Facing on demand keeps a single source of truth, rather
// than storing a second numeric field that can drift out of agreement with it.
func Cardinal(s string) (float64, error) {
	deg, ok := cardinals[strings.ToUpper(strings.TrimSpace(s))]
	if !ok {
		return 0, fmt.Errorf("%q is not a 16-point compass direction", s)
	}
	return deg, nil
}
