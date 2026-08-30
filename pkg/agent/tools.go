package agent

import (
	"log"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"github.com/louislef299/wave-report-agent/pkg/weather"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// The ADK requires tool handlers to take a tool.Context specifically
// (functiontool.Func is func(tool.Context, TArgs) (TResults, error)), which
// would otherwise weld the weather and spot packages to the ADK and make them
// uncallable from a plain binary. The fetchers therefore take a
// context.Context, and the adapters below bridge the two. tool.Context embeds
// context.Context, so each adapter is a straight pass-through.

func spotsHandler(ctx tool.Context, args spot.SpotArgs) (spot.SpotsResult, error) {
	return spot.GetSpotsOfInterest(ctx, args)
}

func nwsHandler(ctx tool.Context, s *spot.Spot) (*weather.GridResp, error) {
	return weather.GetNwsForecast(ctx, s)
}

func marineHandler(ctx tool.Context, s *spot.Spot) (*weather.OpenMeteoResp, error) {
	return weather.GetHourlyMarineForecast(ctx, s)
}

func buoyHandler(ctx tool.Context, s *spot.Spot) (*weather.BuoyObservation, error) {
	return weather.GetBuoyObservations(ctx, s)
}

func tidesHandler(ctx tool.Context, a *weather.TidePredictionArgs) (*weather.TidePredictionsResp, error) {
	return weather.GetTidePredictions(ctx, a)
}

func alertsHandler(ctx tool.Context, s *spot.Spot) (*weather.NwsAlertsResp, error) {
	return weather.GetNwsAlerts(ctx, s)
}

func getTools() []tool.Tool {
	spotTool, err := functiontool.New(functiontool.Config{
		Name:        "get_spots_of_interest",
		Description: "Returns the spots of interest for the agent. Use name='all' to return all configured surf spots.",
	}, spotsHandler)
	if err != nil {
		log.Fatal("Failed to create time tool:", err)
	}

	nwsTool, err := functiontool.New(functiontool.Config{
		Name:        "get_spot_weather",
		Description: "Returns the temperature, wind speed, forecast, and direction of a provided Spot.",
	}, nwsHandler)
	if err != nil {
		log.Fatal("Failed to create National Weather Service tool:", err)
	}

	openMetroTool, err := functiontool.New(functiontool.Config{
		Name:        "get_spot_marine_forecast",
		Description: "Returns hourly marine forecast information of a provided Spot. Used with all SpotTypes.",
	}, marineHandler)
	if err != nil {
		log.Fatal("Failed to create Open Metro tool:", err)
	}

	currentDateTool, err := functiontool.New(functiontool.Config{
		Name:        "get_current_date",
		Description: "Returns the current date in RFC3339 format so agent can gather bearings. Only required if the current date is required & unknown.",
	}, getDate)
	if err != nil {
		log.Fatal("Failed to create current date tool:", err)
	}

	buoyTool, err := functiontool.New(functiontool.Config{
		Name:        "get_buoy_observations",
		Description: "Returns the latest real-time buoy observations (wave height, dominant period, mean wave direction, wind speed, wind direction) from the nearest NOAA NDBC buoy to the spot. Use this to validate forecast data against actual conditions and identify discrepancies.",
	}, buoyHandler)
	if err != nil {
		log.Fatal("Failed to create buoy tool:", err)
	}

	tidesTool, err := functiontool.New(functiontool.Config{
		Name:        "get_tide_predictions",
		Description: "Returns today's and tomorrow's high and low tide predictions (local time, height in feet relative to MLLW) from the nearest NOAA CO-OPS tide gauge station. Returns nil for lake spots where tides are negligible. Use this to identify the best low-to-mid tide session window.",
	}, tidesHandler)
	if err != nil {
		log.Fatal("Failed to create tides tool:", err)
	}

	alertsTool, err := functiontool.New(functiontool.Config{
		Name:        "get_nws_alerts",
		Description: "Returns active NWS weather alerts (Gale Warnings, Storm Warnings, Small Craft Advisories, High Surf Advisories, etc.) for the spot's coordinates. Call for all spot types. Especially important for lake spots where Gale Warnings and Storm Warnings are the primary surf condition signal. Returns an empty list when no alerts are active.",
	}, alertsHandler)
	if err != nil {
		log.Fatal("Failed to create alerts tool:", err)
	}

	return []tool.Tool{
		spotTool,
		nwsTool,
		openMetroTool,
		currentDateTool,
		buoyTool,
		tidesTool,
		alertsTool,
	}
}
