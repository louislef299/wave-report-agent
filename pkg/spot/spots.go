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
		Spec:          "Beach break with shifting sandbars. Mornings traditionally better than afternoons. Highly exposed spot — conditions are frequently rougher than forecasts suggest. Strong rip currents are common, especially with wind > 15 mph or during large swell. Exercise caution in strong wind regardless of direction. Best swell directions: NW to W; SW also works. Holds up to ~8ft — above that most sets close out across the entire beach. Very low or negative tides accelerate closeout tendency. Best season: November through February (NW groundswell season).",
		Meta:          map[string]any{},
	},
	{
		Name:          "Rincon Point",
		City:          "Carpinteria",
		State:         "California",
		Latitude:      34.3728477,
		Longitude:     -119.4984414,
		SpotType:      "ocean",
		BreakType:     "point break",
		Facing:        "SW",
		NearestBuoyID: "46053",
		TideStationID: "9411340",
		TidalRange:    ">2ft",
		Spec:          "Classic California point break, known as the 'Queen of the Coast.' Optimal swell: WNW to W (250–280°), 4–8ft, 13s+ period. Key nuance: Rincon breaks significantly smaller than nearby spots when NW swell period is very long (>16s) — the swell wraps around the point and loses energy; conditions improve when period drops below 16s or swell shifts more WSW. All three sections (The Point, Rivermouth, Indicator) improve as the tide drops. Best at low to mid falling tide. Offshore wind: NE. Best season: October through March (west/northwest groundswell season).",
		Meta:          map[string]any{},
	},
	{
		Name:          "Morro Rock",
		City:          "Morro Bay",
		State:         "California",
		Latitude:      35.3706,
		Longitude:     -120.8657,
		SpotType:      "ocean",
		BreakType:     "beach break",
		Facing:        "W",
		NearestBuoyID: "46215",
		TideStationID: "9412110",
		TidalRange:    ">2ft",
		Spec:          "West-facing beach break on the exposed north side of Morro Rock, on California's Central Coast north of Point Conception. Catches nearly every swell — W, NW, and SW all reach the sandbars, forming feathering A-frames on the right sand setup. Working size 2–6ft: below 2ft it goes flat, above 6ft the bars tend to close out; waves grow bigger and beefier the farther north up Morro Strand you go. Optimal: W swell (260–280°) with an E to ESE offshore wind. Low to mid tide shapes the best form. Being north of Point Conception, the spot is fully exposed to NW windswell — summers are typically small and frequently blown out by afternoon NW onshore winds, so dawn sessions are essential. Comes alive in fall (September–November) when swell angles and winds align; January is often the most consistent for clean, larger surf. Best season: fall through winter.",
		Meta:          map[string]any{},
	},
	{
		Name:          "Empire Beach",
		City:          "Empire",
		State:         "Michigan",
		Latitude:      44.8120363,
		Longitude:     -86.1093288,
		SpotType:      "lake",
		BreakType:     "beach break",
		Facing:        "W",
		NearestBuoyID: "BSBM4",
		TideStationID: "N/A",
		TidalRange:    "N/A",
		Spec:          "W/NW winds produce ~60 miles of fetch — small to moderate waves. S/SW winds produce 250+ miles of fetch across the full length of Lake Michigan — best swell quality with longer periods and larger wave heights. Best conditions come from sustained S/SW winds at 15+ mph for 2+ days. Summer surfing is generally inconsistent; fall through early spring is the prime season.",
		// Empire sits on Lake Michigan's east shore, so open water spans S
		// through W to NNW and everything from N through E is Michigan. The
		// long axis runs south down the length of the lake.
		FetchArcs: []FetchArc{
			{FromDeg: 170, ToDeg: 220, Miles: 210}, // S–SSW, down the lake
			{FromDeg: 220, ToDeg: 260, Miles: 90},  // SW–WSW, oblique
			{FromDeg: 260, ToDeg: 300, Miles: 70},  // W–WNW, straight across
			{FromDeg: 300, ToDeg: 340, Miles: 80},  // NW–NNW, up past the Manitous
		},
		Meta: map[string]any{},
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
		NearestBuoyID: "SLVM5",
		TideStationID: "N/A",
		TidalRange:    "N/A",
		// Superior's major axis runs ENE from the Duluth end toward Whitefish
		// Bay, so the longest fetch sits near 80–95°, not at true NE. Wind
		// from S through NW crosses Minnesota and builds nothing.
		FetchArcs: []FetchArc{
			{FromDeg: 40, ToDeg: 60, Miles: 150},   // NE, to the Ontario shore
			{FromDeg: 60, ToDeg: 100, Miles: 300},  // ENE–E, the long axis
			{FromDeg: 100, ToDeg: 130, Miles: 180}, // ESE, toward the south shore
			{FromDeg: 130, ToDeg: 180, Miles: 25},  // SE–S, across to Wisconsin
		},
		Spec: "Rocky point break on the MN North Shore of Lake Superior. Lake surf depends entirely on wind-generated swell — there is no groundswell. Requires 2-3 days of sustained NE or NW winds at 15+ mph to build surfable waves. Classic pattern: NE/N winds (onshore) build waves across the lake, then a shift to NW (offshore) cleans up the faces. Gale warnings (34-47 knots) issued for western Lake Superior are a strong positive signal — prime surf conditions. Storm warnings (48+ knots) can produce 6-8ft+ waves but may be dangerous even for experienced surfers. 4-6ft waves are ideal. No tidal influence. Best season: late fall and winter when low-pressure systems produce frequent gales.",
		Meta: map[string]any{},
	},
}
