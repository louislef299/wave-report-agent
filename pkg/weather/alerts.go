package weather

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"google.golang.org/adk/tool"
)

// NwsAlert is a single active NWS weather alert for a location.
type NwsAlert struct {
	Event       string `json:"event" jsonschema_description:"Alert event name, e.g. 'Gale Warning', 'Small Craft Advisory', 'Storm Warning', 'High Surf Advisory'."`
	Headline    string `json:"headline" jsonschema_description:"Short one-line summary of the alert."`
	Description string `json:"description" jsonschema_description:"Full alert text including wind speeds, wave heights, and timing."`
	Severity    string `json:"severity" jsonschema_description:"NWS severity level: Extreme, Severe, Moderate, Minor, or Unknown."`
	Effective   string `json:"effective" jsonschema_description:"ISO8601 timestamp when the alert becomes effective."`
	Expires     string `json:"expires" jsonschema_description:"ISO8601 timestamp when the alert expires."`
}

// NwsAlertsResp holds all active NWS alerts for a location.
type NwsAlertsResp struct {
	Alerts []NwsAlert `json:"alerts" jsonschema_description:"Active alerts ordered as returned by NWS. Empty slice means no active alerts."`
}

// raw types for JSON decoding

type nwsAlertCollection struct {
	Features []nwsAlertFeature `json:"features"`
}

type nwsAlertFeature struct {
	Properties nwsAlertProperties `json:"properties"`
}

type nwsAlertProperties struct {
	Event       string `json:"event"`
	Headline    string `json:"headline"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Effective   string `json:"effective"`
	Expires     string `json:"expires"`
}

// GetNwsAlerts fetches active NWS weather alerts for the spot's coordinates.
// Returns an empty Alerts slice (not an error) when no alerts are active.
// Useful for all spot types but especially important for lake spots where
// Gale Warnings and Storm Warnings are the primary surf condition signal.
// https://www.weather.gov/documentation/services-web-api
func GetNwsAlerts(ctx tool.Context, s *spot.Spot) (*NwsAlertsResp, error) {
	url := fmt.Sprintf("%s/alerts/active?point=%.4f,%.4f", nwsBaseUrl, s.Latitude, s.Longitude)

	req, err := generateNwsReq(ctx, url)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching NWS alerts for %s: %w", s.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, ErrInvalidHttpResponse
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var raw nwsAlertCollection
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parsing NWS alerts: %w", err)
	}

	alerts := make([]NwsAlert, 0, len(raw.Features))
	for _, f := range raw.Features {
		p := f.Properties
		alerts = append(alerts, NwsAlert{
			Event:       p.Event,
			Headline:    p.Headline,
			Description: p.Description,
			Severity:    p.Severity,
			Effective:   p.Effective,
			Expires:     p.Expires,
		})
	}

	return &NwsAlertsResp{Alerts: alerts}, nil
}
