package weather

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// userAgent identifies this client to NOAA and NWS, which ask that automated
// callers declare a contact.
// https://www.weather.gov/documentation/services-web-api
const userAgent = "louislefebvre.net/wave-report-agent/1.0"

var ErrInvalidHttpResponse = errors.New("received an invalid HTTP response")

// httpClient bounds every outbound request. These fetchers are meant to run
// unattended on a schedule, where a hung NOAA connection would otherwise stall
// a run indefinitely.
var httpClient = &http.Client{Timeout: 15 * time.Second}

// get issues a GET bound to ctx and returns the response body. Extra headers
// are applied on top of the default User-Agent.
func get(ctx context.Context, url string, headers map[string]string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: GET %s returned %s", ErrInvalidHttpResponse, url, resp.Status)
	}
	return io.ReadAll(resp.Body)
}

// getNws issues a GET against the National Weather Service API, which serves
// GeoJSON.
// https://www.weather.gov/documentation/services-web-api
func getNws(ctx context.Context, url string) ([]byte, error) {
	return get(ctx, url, map[string]string{"Accept": geoJSON})
}
