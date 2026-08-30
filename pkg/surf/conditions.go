// Package surf turns raw weather feeds into surf condition judgements. The
// domain types here are deliberately free of any wire format so that scoring
// is a pure function over plain numbers, testable without a network.
package surf

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/weather"
)

// openMeteoTime is the naive hourly stamp Open-Meteo returns. Requests ask for
// UTC, so these parse as UTC and stay that way through the domain; converting
// to local time is a presentation concern.
const openMeteoTime = "2006-01-02T15:04"

var ErrNoOverlap = errors.New("marine and wind series do not overlap")

// Hour is one hour of conditions at a spot, normalized to imperial units.
// Directions are degrees true and follow the meteorological convention of the
// bearing the wind or swell arrives FROM.
type Hour struct {
	Time         time.Time
	WaveHeightFt float64
	WavePeriodS  float64
	WaveDirDeg   float64
	WindSpeedMph float64
	WindDirDeg   float64
	WindGustMph  float64
}

// Conditions is a contiguous hourly series for a single spot, spanning both
// observed history and forecast.
type Conditions struct {
	Spot string

	// FetchedAt separates history from forecast. Hours before it were observed
	// (or reanalyzed); hours at or after it are predictions.
	FetchedAt time.Time

	// Hours is ascending by time. Entries may be missing where a source
	// reported a gap, so consumers that care about elapsed time must compare
	// timestamps rather than counting slice positions.
	Hours []Hour

	// Gaps counts source hours dropped for incomplete readings. A large value
	// means the model has poor coverage at these coordinates and any verdict
	// built from it deserves suspicion.
	Gaps int
}

// Forecast returns the hours at or after FetchedAt.
func (c Conditions) Forecast() []Hour {
	for i, h := range c.Hours {
		if !h.Time.Before(c.FetchedAt) {
			return c.Hours[i:]
		}
	}
	return nil
}

// Merge joins a marine series and a wind series into one hourly view.
//
// The join is on timestamp rather than slice index. In practice Open-Meteo
// returns identical time axes for both endpoints when they are requested with
// the same window, but that is a convenience of the API and not a contract;
// pairing by index would silently attach one hour's waves to another hour's
// wind if either series ever shifted.
//
// Hours missing any value the scorer needs are dropped and counted in Gaps.
func Merge(spotName string, fetchedAt time.Time, m *weather.OpenMeteoResp, w *weather.WindResp) (Conditions, error) {
	c := Conditions{Spot: spotName, FetchedAt: fetchedAt}

	if m == nil || w == nil {
		return c, fmt.Errorf("%s: marine and wind responses are both required", spotName)
	}

	winds, err := indexWind(spotName, w)
	if err != nil {
		return c, err
	}

	mh := m.Hourly
	if err := checkLen(spotName, "marine", len(mh.Time),
		len(mh.WaveHeight), len(mh.WavePeriod), len(mh.WaveDirection)); err != nil {
		return c, err
	}

	var matched int
	for i, ts := range mh.Time {
		t, err := time.Parse(openMeteoTime, ts)
		if err != nil {
			return c, fmt.Errorf("%s: parsing marine timestamp %q: %w", spotName, ts, err)
		}

		wind, ok := winds[t]
		if !ok {
			// The wind series does not cover this hour at all, which is a
			// window mismatch rather than a reporting gap.
			continue
		}
		matched++

		height, period, dir := mh.WaveHeight[i], mh.WavePeriod[i], mh.WaveDirection[i]
		if !wind.complete || height == nil || period == nil || dir == nil {
			c.Gaps++
			continue
		}

		c.Hours = append(c.Hours, Hour{
			Time:         t,
			WaveHeightFt: *height,
			WavePeriodS:  *period,
			WaveDirDeg:   *dir,
			WindSpeedMph: wind.speed,
			WindDirDeg:   wind.dir,
			WindGustMph:  wind.gust,
		})
	}

	if matched == 0 {
		return c, fmt.Errorf("%s: %w", spotName, ErrNoOverlap)
	}

	sort.Slice(c.Hours, func(i, j int) bool { return c.Hours[i].Time.Before(c.Hours[j].Time) })
	return c, nil
}

type windReading struct {
	speed, dir, gust float64

	// complete distinguishes an hour the wind series reported with null
	// readings from one it never covered. Merge counts the former as a gap and
	// the latter as a window mismatch.
	complete bool
}

// indexWind keys wind readings by timestamp, retaining incomplete ones so that
// Merge can tell a reporting gap apart from an uncovered hour.
func indexWind(spotName string, w *weather.WindResp) (map[time.Time]windReading, error) {
	wh := w.Hourly
	if err := checkLen(spotName, "wind", len(wh.Time),
		len(wh.WindSpeed10m), len(wh.WindDirection10m)); err != nil {
		return nil, err
	}

	out := make(map[time.Time]windReading, len(wh.Time))
	for i, ts := range wh.Time {
		t, err := time.Parse(openMeteoTime, ts)
		if err != nil {
			return nil, fmt.Errorf("%s: parsing wind timestamp %q: %w", spotName, ts, err)
		}

		speed, dir := wh.WindSpeed10m[i], wh.WindDirection10m[i]
		if speed == nil || dir == nil {
			out[t] = windReading{}
			continue
		}

		// Gusts are informational, so a missing one does not disqualify the
		// hour the way a missing sustained speed does.
		var gust float64
		if i < len(wh.WindGusts10m) && wh.WindGusts10m[i] != nil {
			gust = *wh.WindGusts10m[i]
		}

		out[t] = windReading{speed: *speed, dir: *dir, gust: gust, complete: true}
	}
	return out, nil
}

// checkLen rejects a ragged response before anything indexes into it.
func checkLen(spotName, series string, want int, got ...int) error {
	for _, n := range got {
		if n != want {
			return fmt.Errorf("%s: ragged %s series: %d timestamps but %d values",
				spotName, series, want, n)
		}
	}
	return nil
}
