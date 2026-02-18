package spot

import (
	"errors"
	"strings"

	"google.golang.org/adk/tool"
)

const MetaNwsGridPoint = "nws_grid_point"

var ErrInvalidName = errors.New("could not find a spot with the provided name")

type Spot struct {
	Name  string `json:"name" jsonschema_description:"The human-reable name of the spot."`
	City  string `json:"city" jsonschema_description:"The city the Spot is located in."`
	State string `json:"state" jsonschema_description:"The state the Spot is located in."`

	Longitude float32 `json:"longitude" jsonschema_description:"The longitudinal point to find the spot."`
	Latitude  float32 `json:"latitude" jsonschema_description:"The latitudinal point to find the spot."`

	SpotType      string `json:"spot_type" jsonschema_description:"The type of surf spot: 'ocean' or 'lake'. Lake spots depend entirely on locally generated wind swell; ocean spots prefer distant groundswell. Evaluation criteria differ significantly between the two."`
	BreakType     string `json:"break_type" jsonschema_description:"The type of wave break: beach break, reef break, or point break."`
	Facing        string `json:"facing" jsonschema_description:"Cardinal direction the beach faces (e.g. WSW). Used to determine whether wind is offshore or onshore."`
	NearestBuoyID string `json:"nearest_buoy_id" jsonschema_description:"NOAA NDBC station ID of the nearest offshore buoy for real-time wave observations."`

	TidalRange string         `json:"tidal_range" jsonschema_description:"The ideal tidal range for the spot(ex:6ft-4ft)."`
	Spec       string         `json:"spec" jsonschema_description:"Additional specification information to look for at this spot."`
	Meta       map[string]any `json:"meta" jsonschema_description:"Optional metadata to tie to the spot."`
}

type SpotArgs struct {
	Name string `json:"name" jsonschema_description:"The name of the spot to gather information for. Sending a name of 'all' will return all spots of interest."`
}

type SpotsResult struct {
	Spots []Spot `json:"spots"`
}

func GetSpotsOfInterest(_ tool.Context, args SpotArgs) (SpotsResult, error) {
	if strings.EqualFold(args.Name, "all") {
		return SpotsResult{Spots: spots}, nil
	}

	for _, s := range spots {
		if strings.EqualFold(s.Name, args.Name) {
			return SpotsResult{Spots: []Spot{s}}, nil
		}
	}
	return SpotsResult{}, ErrInvalidName
}
