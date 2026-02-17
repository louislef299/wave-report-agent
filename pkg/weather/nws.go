package weather

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"google.golang.org/adk/tool"
)

const (
	geoJSON    = "application/geo+json"
	nwsBaseUrl = "https://api.weather.gov"
)

var ErrInvalidHttpResponse = errors.New("received an invalid HTTP response")

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
func GetNwsForecast(ctx tool.Context, s *spot.Spot) (*GridResp, error) {
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
	resp, err := http.Get(f)
	if err != nil {
		return nil, err
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var gr GridResp
	err = json.Unmarshal(resBody, &gr)
	if err != nil {
		return nil, err
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

	req, err := generateNwsReq(ctx, u)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", ErrInvalidHttpResponse
	}

	resBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var weatherResp PointsResp
	err = json.Unmarshal(resBody, &weatherResp)
	if err != nil {
		return "", err
	}
	return weatherResp.Properties.Forecast, nil
}

// generateNwsReq generates a request to send to the National Weather Service
// API and tries to follow best practices.
// https://www.weather.gov/documentation/services-web-api
func generateNwsReq(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", geoJSON)
	req.Header.Set("User-Agent", "louislefebvre.net/wave-report-agent/1.0")

	return req, nil
}
