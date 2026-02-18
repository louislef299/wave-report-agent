package weather

import (
	"fmt"
	"testing"

	"github.com/louislef299/wave-report-agent/pkg/spot"
)

func TestGetBuoyObservations(t *testing.T) {
	testCases := []struct {
		spot *spot.Spot
		// expectWaveData is true for offshore ocean buoys that report WVHT/DPD.
		// C-MAN shore stations (used for lake spots) only report wind data.
		expectWaveData bool
	}{
		{
			spot: &spot.Spot{
				Name:          "Ocean Beach",
				SpotType:      "ocean",
				NearestBuoyID: "46086",
			},
			expectWaveData: true,
		},
		{
			spot: &spot.Spot{
				Name:          "Rincon Point",
				SpotType:      "ocean",
				NearestBuoyID: "46053",
			},
			expectWaveData: true,
		},
		{
			spot: &spot.Spot{
				Name:          "Empire Beach",
				SpotType:      "lake",
				NearestBuoyID: "BSBM4",
			},
			expectWaveData: false,
		},
		{
			spot: &spot.Spot{
				Name:          "Stoney Point",
				SpotType:      "lake",
				NearestBuoyID: "SLVM5",
			},
			expectWaveData: false,
		},
	}

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("%s (%s)", tt.spot.Name, tt.spot.NearestBuoyID), func(t *testing.T) {
			obs, err := GetBuoyObservations(nil, tt.spot)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if obs == nil {
				t.Fatal("expected observation, got nil")
			}

			// All station types should report at least wind speed.
			// Wind direction may be MM on some shore stations (e.g. pressure-only sensors).
			if obs.WindSpeedMph < 0 {
				t.Errorf("expected valid wind speed, got %v (missing)", obs.WindSpeedMph)
			}

			// Offshore ocean buoys additionally report wave height.
			if tt.expectWaveData {
				if obs.WaveHeightFt <= 0 {
					t.Errorf("ocean buoy: expected wave height > 0, got %v", obs.WaveHeightFt)
				}
			}
		})
	}
}
