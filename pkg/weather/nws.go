package weather

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/louislef299/wave-report-agent/pkg/spot"
)

const (
	geoJSON    = "application/geo+json"
	nwsBaseUrl = "https://api.weather.gov"
)

type GridResp struct {
	Properties GridRespProperties `json:"properties"`
}

type GridRespProperties struct {
	Periods []GridRespPeriod `json:"periods"`
}

type GridRespPeriod struct {
	Name          string `json:"name"`
	Temperature   int32  `json:"temperature"`
	WindSpeed     string `json:"windSpeed"`
	WindDirection string `json:"windDirection"`
	Forecast      string `json:"detailedForecast"`
}

// GetNwsForecast gathers the 7-day forecast over 12 hour periods by calling the
// National Weather Service API and returning a GridResp.
// https://www.weather.gov/documentation/services-web-api
func GetNwsForecast(ctx context.Context, s *spot.Spot) (*GridResp, error) {
	var err error
	forecastURL, ok := s.Meta[spot.MetaNwsGridPoint]
	if !ok {
		forecastURL, err = GatherGridPoint(ctx, s)
		if err != nil {
			return nil, err
		}
	}

	f, ok := forecastURL.(string)
	if !ok {
		return nil, fmt.Errorf("didn't get expected metadata return type of string")
	}
	body, err := getNws(ctx, f)
	if err != nil {
		return nil, fmt.Errorf("fetching NWS forecast for %s: %w", s.Name, err)
	}

	var gr GridResp
	if err := json.Unmarshal(body, &gr); err != nil {
		return nil, fmt.Errorf("parsing NWS forecast for %s: %w", s.Name, err)
	}
	return &gr, nil
}

type PointsResp struct {
	Properties PointsRespProperties `json:"properties"`
}

type PointsRespProperties struct {
	Forecast string `json:"forecast"`
}

// GatherGridPoint uses the Latitude and Longitude provided by the Spot to
// gather the proper gridpoints(https://api.weather.gov/gridpoints) URL returned
// as a string. This allows for detailed forecast information in future calls.
func GatherGridPoint(ctx context.Context, s *spot.Spot) (string, error) {
	ll := fmt.Sprintf("%.2f,%.2f", s.Latitude, s.Longitude)
	u, err := url.JoinPath(nwsBaseUrl, "points", ll)
	if err != nil {
		return "", err
	}

	body, err := getNws(ctx, u)
	if err != nil {
		return "", fmt.Errorf("fetching NWS gridpoint for %s: %w", s.Name, err)
	}

	var weatherResp PointsResp
	if err := json.Unmarshal(body, &weatherResp); err != nil {
		return "", fmt.Errorf("parsing NWS gridpoint for %s: %w", s.Name, err)
	}
	return weatherResp.Properties.Forecast, nil
}
