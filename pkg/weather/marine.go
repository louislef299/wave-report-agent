package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/louislef299/wave-report-agent/pkg/spot"
)

// https://open-meteo.com/en/docs/marine-weather-api#data_sources

// Window describes the time span of an Open-Meteo request. Both the marine and
// forecast endpoints accept the same parameters and return the same hourly time
// axis, so requesting them with an identical Window keeps their series aligned.
type Window struct {
	PastDays     int
	ForecastDays int
}

var (
	// AgentWindow is what the ADK tool requests. The model reads the series as
	// text, so past hours would be paid for in tokens without being used.
	AgentWindow = Window{PastDays: 0, ForecastDays: 7}

	// ScoringWindow is what the scorer requests. The trailing days are not
	// optional: wave growth depends on how long wind has already been blowing,
	// which is computed by looking backward from a candidate hour. With
	// forecast data alone the earliest hours can never satisfy a sustained
	// wind gate.
	ScoringWindow = Window{PastDays: 2, ForecastDays: 3}
)

type OpenMeteoResp struct {
	HourlyUnits HourlyUnits `json:"hourly_units"`
	Hourly      Hourly      `json:"hourly"`
}

type HourlyUnits struct {
	Time               string `json:"time"`
	WaveHeight         string `json:"wave_height"`
	WaveDirection      string `json:"wave_direction"`
	WavePeriod         string `json:"wave_period"`
	WindWaveHeight     string `json:"wind_wave_height"`
	WindWaveDirection  string `json:"wind_wave_direction"`
	WindWavePeriod     string `json:"wind_wave_period"`
	SwellWaveHeight    string `json:"swell_wave_height"`
	SwellWaveDirection string `json:"swell_wave_direction"`
	SwellWavePeriod    string `json:"swell_wave_period"`
	SeaLevelHeightMsl  string `json:"sea_level_height_msl"`
}

// Hourly holds the marine series. Every measurement is a pointer because
// Open-Meteo reports gaps as JSON null, and an absent wave height is not the
// same fact as a wave height of zero — one means "no model coverage here", the
// other means "flat". Decoding into a plain float64 would silently conflate
// them and let the scorer read missing data as calm water.
type Hourly struct {
	Time               []string   `json:"time"`
	WaveHeight         []*float64 `json:"wave_height"`
	WaveDirection      []*float64 `json:"wave_direction"`
	WavePeriod         []*float64 `json:"wave_period"`
	WindWaveHeight     []*float64 `json:"wind_wave_height"`
	WindWaveDirection  []*float64 `json:"wind_wave_direction"`
	WindWavePeriod     []*float64 `json:"wind_wave_period"`
	SwellWaveHeight    []*float64 `json:"swell_wave_height"`
	SwellWaveDirection []*float64 `json:"swell_wave_direction"`
	SwellWavePeriod    []*float64 `json:"swell_wave_period"`
	SeaLevelHeightMsl  []*float64 `json:"sea_level_height_msl"`
}

// GetHourlyMarineForecast returns the forecast-only marine series used by the
// ADK agent tool.
func GetHourlyMarineForecast(ctx context.Context, s *spot.Spot) (*OpenMeteoResp, error) {
	resp, _, err := GetMarineForecast(ctx, s, AgentWindow)
	return resp, err
}

// GetMarineForecast fetches the hourly marine series over the given window and
// returns the raw body alongside the decoded value. The ledger stores the raw
// bytes so that fields this struct does not model today can still be recovered
// later; re-encoding the decoded value would silently drop them.
func GetMarineForecast(ctx context.Context, s *spot.Spot, w Window) (*OpenMeteoResp, []byte, error) {
	body, err := get(ctx, marineURL(s, w), nil)
	if err != nil {
		return nil, nil, fmt.Errorf("fetching marine forecast for %s: %w", s.Name, err)
	}

	var openResp OpenMeteoResp
	if err := json.Unmarshal(body, &openResp); err != nil {
		return nil, nil, fmt.Errorf("parsing marine forecast for %s: %w", s.Name, err)
	}
	return &openResp, body, nil
}

func marineURL(s *spot.Spot, w Window) string {
	q := openMeteoQuery(s, w)
	q.Set("hourly", "wave_height,wave_direction,wave_period,"+
		"wind_wave_height,wind_wave_direction,wind_wave_period,"+
		"swell_wave_height,swell_wave_direction,swell_wave_period,sea_level_height_msl")
	q.Set("length_unit", "imperial")
	q.Set("wind_speed_unit", "kn")
	return "https://marine-api.open-meteo.com/v1/marine?" + q.Encode()
}

// openMeteoQuery builds the parameters shared by the marine and forecast
// endpoints. Times are requested in UTC so the domain layer never has to
// reason about DST; conversion to local time is a presentation concern.
func openMeteoQuery(s *spot.Spot, w Window) url.Values {
	q := url.Values{}
	q.Set("latitude", fmt.Sprintf("%.4f", s.Latitude))
	q.Set("longitude", fmt.Sprintf("%.4f", s.Longitude))
	q.Set("timezone", "UTC")
	if w.PastDays > 0 {
		q.Set("past_days", fmt.Sprint(w.PastDays))
	}
	if w.ForecastDays > 0 {
		q.Set("forecast_days", fmt.Sprint(w.ForecastDays))
	}
	return q
}
