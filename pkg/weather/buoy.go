package weather

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"google.golang.org/adk/tool"
)

// BuoyObservation holds the most recent real-time observation from a NOAA NDBC
// buoy. Wave height is in feet, wind speed in mph, directions in degrees true.
// A value of -1 indicates the measurement was unavailable (reported as "MM" by
// NDBC).
type BuoyObservation struct {
	StationID        string  `json:"station_id"`
	WindDirectionDeg float64 `json:"wind_direction_deg" jsonschema_description:"Wind direction in degrees true (where wind is coming FROM). -1 if unavailable."`
	WindSpeedMph     float64 `json:"wind_speed_mph" jsonschema_description:"Wind speed in mph. -1 if unavailable."`
	GustSpeedMph     float64 `json:"gust_speed_mph" jsonschema_description:"Gust speed in mph. -1 if unavailable."`
	WaveHeightFt     float64 `json:"wave_height_ft" jsonschema_description:"Significant wave height in feet. -1 if unavailable."`
	DominantPeriodS  float64 `json:"dominant_period_s" jsonschema_description:"Dominant wave period in seconds. -1 if unavailable."`
	MeanWaveDirDeg   float64 `json:"mean_wave_dir_deg" jsonschema_description:"Mean wave direction in degrees true (where waves are coming FROM). -1 if unavailable."`
	WaterTempC       float64 `json:"water_temp_c" jsonschema_description:"Water temperature in Celsius. -1 if unavailable."`
	ObservationTime  string  `json:"observation_time" jsonschema_description:"UTC time of this observation in format YYYY-MM-DD HH:mm."`
}

// GetBuoyObservations fetches the latest real-time observation from the NOAA
// NDBC buoy nearest to the given surf spot. Returns the most recent reading
// that has wave data.
func GetBuoyObservations(_ tool.Context, s *spot.Spot) (*BuoyObservation, error) {
	if s.NearestBuoyID == "" {
		return nil, fmt.Errorf("no buoy ID configured for spot %q", s.Name)
	}

	url := fmt.Sprintf("https://www.ndbc.noaa.gov/data/realtime2/%s.txt", s.NearestBuoyID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching buoy %s: %w", s.NearestBuoyID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidHttpResponse
	}

	return parseBuoyData(resp.Body, s.NearestBuoyID)
}

// parseBuoyData parses the NDBC standard meteorological text format.
// Format: two header rows (prefixed with #), then space-separated data rows
// newest-first.
// Columns: YY MM DD hh mm WDIR WSPD GST WVHT DPD APD MWD PRES ATMP WTMP DEWP
// VIS PTDY TIDE
func parseBuoyData(r io.Reader, stationID string) (*BuoyObservation, error) {
	scanner := bufio.NewScanner(r)

	// Skip the two header rows
	for range 2 {
		if !scanner.Scan() {
			return nil, fmt.Errorf("unexpected end of buoy data before headers")
		}
	}

	// Read data rows until we find one with wave data (WVHT != MM)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)
		// Expected columns: YY MM DD hh mm WDIR WSPD GST WVHT DPD APD MWD PRES
		// ATMP WTMP ...
		if len(fields) < 15 {
			continue
		}

		obs := &BuoyObservation{
			StationID:       stationID,
			ObservationTime: fmt.Sprintf("%s-%s-%s %s:%s", fields[0], fields[1], fields[2], fields[3], fields[4]),

			WindDirectionDeg: parseNdbcFloat(fields[5]),
			WindSpeedMph:     metersPerSecToMph(parseNdbcFloat(fields[6])),
			GustSpeedMph:     metersPerSecToMph(parseNdbcFloat(fields[7])),
			WaveHeightFt:     metersToFeet(parseNdbcFloat(fields[8])),
			DominantPeriodS:  parseNdbcFloat(fields[9]),
			MeanWaveDirDeg:   parseNdbcFloat(fields[11]),
			WaterTempC:       parseNdbcFloat(fields[14]),
		}

		// Prefer rows that have wave height data, but return first row if we've
		// tried several and still have no wave data (some buoys don't report it).
		if obs.WaveHeightFt > 0 || obs.DominantPeriodS > 0 {
			return obs, nil
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading buoy data: %w", err)
	}
	return nil, fmt.Errorf("no usable observations found for buoy %s", stationID)
}

// parseNdbcFloat converts an NDBC field to float64. Returns -1 for "MM"
// (missing).
func parseNdbcFloat(s string) float64 {
	if s == "MM" {
		return -1
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return -1
	}
	return v
}

func metersToFeet(m float64) float64 {
	if m < 0 {
		return m
	}
	return math.Round(m*3.28084*10) / 10
}

func metersPerSecToMph(mps float64) float64 {
	if mps < 0 {
		return mps
	}
	return math.Round(mps*2.23694*10) / 10
}
