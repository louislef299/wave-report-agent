package main

import (
	"context"
	"fmt"
	"log"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"github.com/louislef299/wave-report-agent/pkg/weather"
)

var stoney = spot.Spot{
	Name:          "Stoney Point",
	City:          "Duluth",
	State:         "Minnesota",
	Latitude:      46.9666696,
	Longitude:     -91.6359906,
	SpotType:      "lake",
	BreakType:     "point break",
	Facing:        "SSE",
	NearestBuoyID: "SLVM5",
	TideStationID: "N/A",
	TidalRange:    "N/A",
	// Superior's major axis runs ENE from the Duluth end toward Whitefish
	// Bay, so the longest fetch sits near 80–95°, not at true NE. Wind
	// from S through NW crosses Minnesota and builds nothing.
	FetchArcs: []spot.FetchArc{
		{FromDeg: 40, ToDeg: 60, Miles: 150},   // NE, to the Ontario shore
		{FromDeg: 60, ToDeg: 100, Miles: 300},  // ENE–E, the long axis
		{FromDeg: 100, ToDeg: 130, Miles: 180}, // ESE, toward the south shore
		{FromDeg: 130, ToDeg: 180, Miles: 25},  // SE–S, across to Wisconsin
	},
	Spec: "Rocky point break on the MN North Shore of Lake Superior. Lake surf depends entirely on wind-generated swell — there is no groundswell. Requires 2-3 days of sustained NE or NW winds at 15+ mph to build surfable waves. Classic pattern: NE/N winds (onshore) build waves across the lake, then a shift to NW (offshore) cleans up the faces. Gale warnings (34-47 knots) issued for western Lake Superior are a strong positive signal — prime surf conditions. Storm warnings (48+ knots) can produce 6-8ft+ waves but may be dangerous even for experienced surfers. 4-6ft waves are ideal. No tidal influence. Best season: late fall and winter when low-pressure systems produce frequent gales.",
	Meta: map[string]any{},
}

func main() {
	resp, _, err := weather.GetWindForecast(context.TODO(), &stoney, weather.Window{
		PastDays:     0,
		ForecastDays: 1,
	})
	if err != nil {
		log.Fatal(err)
	}

	// pretty, err := json.MarshalIndent(resp.Hourly, "", "    ")
	// if err != nil {
	// 	log.Fatal(err)
	// }
	fmt.Printf("%+v\n", resp.Hourly)
}

// A late-night update: after actually programming a bit and understanding the
// problem better, I concluded that this data would probably better fit in a TSDB.
// After a little research, I landed on https://github.com/timescale/timescaledb and
// am planning on just adding an extension onto a postgres db to add the
// observations there. Should be easier than introducing a whole new database
// technology.
//
// Step 1, update the compose system, check
// Step 2, need to integrate with the Go app: https://www.tigerdata.com/docs/get-started/quickstart/connect-your-app#tab=go
