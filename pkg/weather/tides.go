package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"google.golang.org/adk/tool"
)

const tideTimeFormat = "20060102"

type TidePredictionArgs struct {
	Spot *spot.Spot

	// Represents the number of days to return in the result
	Days int
}

// TidePrediction is a single high or low tide event from the NOAA CO-OPS API.
type TidePrediction struct {
	Time     string  `json:"time"`
	HeightFt float64 `json:"height_ft"`
	// Type is "H" (high tide) or "L" (low tide).
	Type string `json:"type"`
}

// TidePredictionsResp holds today's and tomorrow's high/low tide predictions.
type TidePredictionsResp struct {
	StationID   string           `json:"station_id"`
	Predictions []TidePrediction `json:"predictions"`
}

// coopsPrediction matches the raw JSON shape returned by the CO-OPS API.
type coopsPrediction struct {
	T    string `json:"t"`
	V    string `json:"v"`
	Type string `json:"type"`
}

type coopsResp struct {
	Predictions []coopsPrediction `json:"predictions"`
}

// GetTidePredictions fetches today's and tomorrow's high/low tide predictions
// from the NOAA CO-OPS API for the spot's configured tide gauge station.
// Returns nil without error for lake spots (tides negligible) or spots with no
// station configured.
// https://api.tidesandcurrents.noaa.gov/api/prod
func GetTidePredictions(_ tool.Context, a *TidePredictionArgs) (*TidePredictionsResp, error) {
	if a.Spot.TideStationID == "" || a.Spot.TideStationID == "N/A" {
		return nil, nil
	}

	now := time.Now()
	begin := now.Format(tideTimeFormat)
	end := now.AddDate(0, 0, a.Days).Format(tideTimeFormat)

	url := fmt.Sprintf(
		"https://api.tidesandcurrents.noaa.gov/api/prod/datagetter"+
			"?station=%s&product=predictions&datum=MLLW"+
			"&time_zone=lst_ldt&interval=hilo&units=english&format=json"+
			"&begin_date=%s&end_date=%s",
		a.Spot.TideStationID, begin, end,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("fetching tide predictions for station %s: %w", a.Spot.TideStationID, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidHttpResponse
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw coopsResp
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing tide predictions: %w", err)
	}

	predictions := make([]TidePrediction, 0, len(raw.Predictions))
	for _, p := range raw.Predictions {
		h, err := strconv.ParseFloat(p.V, 64)
		if err != nil {
			continue
		}
		predictions = append(predictions, TidePrediction{
			Time:     p.T,
			HeightFt: h,
			Type:     p.Type,
		})
	}

	return &TidePredictionsResp{
		StationID:   a.Spot.TideStationID,
		Predictions: predictions,
	}, nil
}
