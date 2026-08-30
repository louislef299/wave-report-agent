package surf

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/weather"
)

func TestMergeFixtures(t *testing.T) {
	testCases := []struct {
		spot      string
		prefix    string
		wantHours int
		wantFirst Hour
	}{
		{
			spot:      "Stoney Point",
			prefix:    "stoney",
			wantHours: 48,
			wantFirst: Hour{
				Time:         time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
				WaveHeightFt: 0.197,
				WavePeriodS:  2.50,
				WindSpeedMph: 7.4,
				WindDirDeg:   20,
			},
		},
		{
			spot:      "Empire Beach",
			prefix:    "empire",
			wantHours: 48,
			wantFirst: Hour{
				Time:         time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC),
				WaveHeightFt: 0.591,
			},
		},
	}

	for _, tt := range testCases {
		t.Run(tt.spot, func(t *testing.T) {
			m, w := loadFixtures(t, tt.prefix)

			c, err := Merge(tt.spot, tt.wantFirst.Time, m, w)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(c.Hours) != tt.wantHours {
				t.Fatalf("got %d hours, want %d", len(c.Hours), tt.wantHours)
			}
			if c.Gaps != 0 {
				t.Errorf("got %d gaps in a complete fixture, want 0", c.Gaps)
			}

			got := c.Hours[0]
			if !got.Time.Equal(tt.wantFirst.Time) {
				t.Errorf("first hour time = %v, want %v", got.Time, tt.wantFirst.Time)
			}
			if got.WaveHeightFt != tt.wantFirst.WaveHeightFt {
				t.Errorf("first wave height = %v, want %v", got.WaveHeightFt, tt.wantFirst.WaveHeightFt)
			}
			if tt.wantFirst.WindSpeedMph != 0 && got.WindSpeedMph != tt.wantFirst.WindSpeedMph {
				t.Errorf("first wind speed = %v, want %v", got.WindSpeedMph, tt.wantFirst.WindSpeedMph)
			}

			// The scorer walks backward through this slice by timestamp, so
			// ascending order and hourly spacing are load-bearing.
			for i := 1; i < len(c.Hours); i++ {
				gap := c.Hours[i].Time.Sub(c.Hours[i-1].Time)
				if gap != time.Hour {
					t.Fatalf("hours %d->%d are %v apart, want 1h", i-1, i, gap)
				}
			}
		})
	}
}

// TestMergeSkipsIncompleteHours covers the reason the wire types decode into
// pointers: a null reading must drop the hour rather than score as flat calm.
func TestMergeSkipsIncompleteHours(t *testing.T) {
	marine := marineJSON(`["2026-01-01T00:00","2026-01-01T01:00","2026-01-01T02:00"]`,
		`[3.0, null, 4.0]`, `[5.0, 5.0, 5.0]`, `[90, 90, 90]`)
	wind := windJSON(`["2026-01-01T00:00","2026-01-01T01:00","2026-01-01T02:00"]`,
		`[20.0, 20.0, null]`, `[80, 80, 80]`, `[25.0, 25.0, 25.0]`)

	c, err := Merge("Test", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), marine, wind)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Hour 1 has a null wave height, hour 2 a null wind speed. Only hour 0 is
	// scoreable.
	if len(c.Hours) != 1 {
		t.Fatalf("got %d hours, want 1", len(c.Hours))
	}
	if c.Gaps != 2 {
		t.Errorf("got %d gaps, want 2", c.Gaps)
	}
	if c.Hours[0].WaveHeightFt != 3.0 {
		t.Errorf("wave height = %v, want 3.0", c.Hours[0].WaveHeightFt)
	}
}

// TestMergeJoinsOnTimestamp guards against joining by slice index. The two
// endpoints usually return identical time axes, but that is a convenience of
// the API rather than a guarantee, and an index join would silently pair wave
// data with the wrong hour's wind.
func TestMergeJoinsOnTimestamp(t *testing.T) {
	marine := marineJSON(`["2026-01-01T00:00","2026-01-01T01:00","2026-01-01T02:00"]`,
		`[3.0, 3.5, 4.0]`, `[5.0, 5.0, 5.0]`, `[90, 90, 90]`)
	// Wind starts an hour later and runs an hour longer.
	wind := windJSON(`["2026-01-01T01:00","2026-01-01T02:00","2026-01-01T03:00"]`,
		`[20.0, 21.0, 22.0]`, `[80, 80, 80]`, `[25.0, 25.0, 25.0]`)

	c, err := Merge("Test", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), marine, wind)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(c.Hours) != 2 {
		t.Fatalf("got %d hours, want 2 (the overlap)", len(c.Hours))
	}
	if got := c.Hours[0].Time.Hour(); got != 1 {
		t.Errorf("first merged hour = %02d:00, want 01:00", got)
	}
	// 01:00 pairs wave 3.5 with wind 20.0; an index join would give 3.0.
	if c.Hours[0].WaveHeightFt != 3.5 || c.Hours[0].WindSpeedMph != 20.0 {
		t.Errorf("first hour = %.1fft/%.1fmph, want 3.5ft/20.0mph",
			c.Hours[0].WaveHeightFt, c.Hours[0].WindSpeedMph)
	}
}

func TestMergeNoOverlap(t *testing.T) {
	marine := marineJSON(`["2026-01-01T00:00"]`, `[3.0]`, `[5.0]`, `[90]`)
	wind := windJSON(`["2026-06-01T00:00"]`, `[20.0]`, `[80]`, `[25.0]`)

	_, err := Merge("Test", time.Now(), marine, wind)
	if !errors.Is(err, ErrNoOverlap) {
		t.Fatalf("got %v, want ErrNoOverlap", err)
	}
}

func TestMergeRejectsRaggedSeries(t *testing.T) {
	// Three timestamps but only two wave heights. Indexing blindly would panic
	// or silently truncate.
	marine := marineJSON(`["2026-01-01T00:00","2026-01-01T01:00","2026-01-01T02:00"]`,
		`[3.0, 3.5]`, `[5.0, 5.0, 5.0]`, `[90, 90, 90]`)
	wind := windJSON(`["2026-01-01T00:00","2026-01-01T01:00","2026-01-01T02:00"]`,
		`[20.0, 20.0, 20.0]`, `[80, 80, 80]`, `[25.0, 25.0, 25.0]`)

	if _, err := Merge("Test", time.Now(), marine, wind); err == nil {
		t.Fatal("expected an error for a ragged series, got nil")
	}
}

func TestConditionsForecast(t *testing.T) {
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	c := Conditions{
		FetchedAt: base,
		Hours: []Hour{
			{Time: base.Add(-2 * time.Hour)},
			{Time: base.Add(-1 * time.Hour)},
			{Time: base},
			{Time: base.Add(time.Hour)},
		},
	}

	fc := c.Forecast()
	if len(fc) != 2 {
		t.Fatalf("got %d forecast hours, want 2", len(fc))
	}
	if !fc[0].Time.Equal(base) {
		t.Errorf("forecast starts at %v, want %v", fc[0].Time, base)
	}
}

func loadFixtures(t *testing.T, prefix string) (*weather.OpenMeteoResp, *weather.WindResp) {
	t.Helper()

	var m weather.OpenMeteoResp
	readJSON(t, filepath.Join("testdata", prefix+"_marine.json"), &m)

	var w weather.WindResp
	readJSON(t, filepath.Join("testdata", prefix+"_wind.json"), &w)

	return &m, &w
}

func readJSON(t *testing.T, path string, v any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	if err := json.Unmarshal(b, v); err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
}

func marineJSON(times, height, period, dir string) *weather.OpenMeteoResp {
	var r weather.OpenMeteoResp
	must(json.Unmarshal([]byte(`{"hourly":{
		"time":`+times+`,"wave_height":`+height+`,"wave_period":`+period+`,"wave_direction":`+dir+`}}`), &r))
	return &r
}

func windJSON(times, speed, dir, gust string) *weather.WindResp {
	var r weather.WindResp
	must(json.Unmarshal([]byte(`{"hourly":{
		"time":`+times+`,"wind_speed_10m":`+speed+`,"wind_direction_10m":`+dir+`,"wind_gusts_10m":`+gust+`}}`), &r))
	return &r
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}
