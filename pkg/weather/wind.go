package weather

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/louislef299/wave-report-agent/pkg/spot"
)

// The NWS gridded forecast returns 12-hour narrative periods ("Southwest wind
// 10 to 15 mph"), which cannot answer how long wind has held a bearing, and
// per the agent prompt it returns nothing at all for coordinates that fall in
// marine gridpoint zones — which is most lake spots. Open-Meteo's standard
// forecast endpoint serves hourly wind at the same coordinates as the marine
// endpoint, including observed history via past_days.
// https://open-meteo.com/en/docs

// WindResp holds the hourly wind series for a spot.
type WindResp struct {
	HourlyUnits WindHourlyUnits `json:"hourly_units"`
	Hourly      WindHourly      `json:"hourly"`
}

type WindHourlyUnits struct {
	Time             string `json:"time"`
	WindSpeed10m     string `json:"wind_speed_10m"`
	WindDirection10m string `json:"wind_direction_10m"`
	WindGusts10m     string `json:"wind_gusts_10m"`
}

// WindHourly mirrors Hourly's use of pointers so a reporting gap stays
// distinguishable from genuine calm.
type WindHourly struct {
	Time             []string   `json:"time"`
	WindSpeed10m     []*float64 `json:"wind_speed_10m"`
	WindDirection10m []*float64 `json:"wind_direction_10m"`
	WindGusts10m     []*float64 `json:"wind_gusts_10m"`
}

// GetWindForecast fetches the hourly wind series over the given window. Wind
// direction follows the meteorological convention of the bearing wind blows
// FROM, which is the same convention Spot.FetchMilesAt expects, so no
// conversion is needed between them.
func GetWindForecast(ctx context.Context, s *spot.Spot, w Window) (*WindResp, error) {
	body, err := get(ctx, windURL(s, w), nil)
	if err != nil {
		return nil, fmt.Errorf("fetching wind forecast for %s: %w", s.Name, err)
	}

	var resp WindResp
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parsing wind forecast for %s: %w", s.Name, err)
	}
	return &resp, nil
}

func windURL(s *spot.Spot, w Window) string {
	q := openMeteoQuery(s, w)
	q.Set("hourly", "wind_speed_10m,wind_direction_10m,wind_gusts_10m")
	q.Set("wind_speed_unit", "mph")
	return "https://api.open-meteo.com/v1/forecast?" + q.Encode()
}
