package spot

import (
	"fmt"
	"testing"
)

func TestGetSpotsOfInterest(t *testing.T) {
	testCases := []struct {
		name      string
		expected  error
		returnLen int
	}{
		{
			name:      "all",
			expected:  nil,
			returnLen: len(spots),
		},
		{
			name:      "Mordor",
			expected:  ErrInvalidName,
			returnLen: 0,
		},
	}

	for _, tt := range testCases {
		t.Run(fmt.Sprintf("Spot %s", tt.name), func(t *testing.T) {
			result, err := GetSpotsOfInterest(nil, SpotArgs{Name: tt.name})
			if err != tt.expected {
				t.Fatalf("Returned error did not match expected error:\n\tReturned: %v\n\tExpected: %v", err, tt.expected)
			}

			if sl := len(result.Spots); sl != tt.returnLen {
				t.Fatalf("Expected %d returned spots, go %d", tt.returnLen, sl)
			}
		})
	}
}
