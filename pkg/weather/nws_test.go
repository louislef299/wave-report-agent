package weather

import (
	"fmt"
	"strings"
	"testing"

	"github.com/louislef299/wave-report-agent/pkg/spot"
)

func TestGatherGridPoint(t *testing.T) {
	testCases := []struct {
		spot        *spot.Spot
		expected    string
		errExpected error
	}{
		{
			spot: &spot.Spot{
				Name:      "Morro Bay",
				Latitude:  32.7487318,
				Longitude: -117.2583427,
			},
			expected:    "https://api.weather.gov/gridpoints/SGX/53,16/forecast",
			errExpected: nil,
		},
	}

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("Grid Point %s", tt.spot.Name), func(t *testing.T) {
			forecast, err := GatherGridPoint(t.Context(), tt.spot)
			if err != tt.errExpected {
				t.Fatalf("the expected error(%v) didn't match the received error: %v", tt.errExpected, err)
			}

			if !strings.EqualFold(forecast, tt.expected) {
				t.Fatalf("the expected grid point(%s) didn't match the received grid point: %s", tt.expected, forecast)
			}
		})
	}
}
