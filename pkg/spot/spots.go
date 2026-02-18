package spot

var spots = []Spot{
	{
		Name:          "Ocean Beach",
		City:          "San Diego",
		State:         "California",
		Latitude:      32.7487318,
		Longitude:     -117.2583427,
		SpotType:      "ocean",
		BreakType:     "beach break",
		Facing:        "WSW",
		NearestBuoyID: "46086",
		TideStationID: "9410170",
		TidalRange:    ">2ft",
		Spec:          "Beach break with shifting sandbars. Mornings traditionally better than afternoons. Highly exposed spot — conditions are frequently rougher than forecasts suggest. Strong rip currents are common, especially with wind > 15 mph or during large swell. Exercise caution in strong wind regardless of direction.",
		Meta:          map[string]any{},
	},
	{
		Name:          "Stoney Point",
		City:          "Duluth",
		State:         "Minnesota",
		Latitude:      46.9666696,
		Longitude:     -91.6359906,
		SpotType:      "lake",
		BreakType:     "point break",
		Facing:        "SSE",
		NearestBuoyID: "N/A",
		TidalRange:    "N/A",
		Spec:          "Rocky point break on the MN North Shore of Lake Superior. Lake surf depends entirely on wind-generated swell — there is no groundswell. Requires 2-3 days of sustained NE or NW winds at 15+ mph to build surfable waves. Classic pattern: NE/N winds (onshore) build waves across the lake, then a shift to NW (offshore) cleans up the faces. Gale warnings (34-47 knots) issued for western Lake Superior are a strong positive signal — prime surf conditions. Storm warnings (48+ knots) can produce 6-8ft+ waves but may be dangerous even for experienced surfers. 4-6ft waves are ideal. No tidal influence. Best season: late fall and winter when low-pressure systems produce frequent gales.",
		Meta:          map[string]any{},
	},
}
