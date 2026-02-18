package agent

import (
	"log"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"github.com/louislef299/wave-report-agent/pkg/weather"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

func getTools() []tool.Tool {
	spotTool, err := functiontool.New(functiontool.Config{
		Name:        "get_spots_of_interest",
		Description: "Returns the spots of interest for the agent. Use name='all' to return all configured surf spots.",
	}, spot.GetSpotsOfInterest)
	if err != nil {
		log.Fatal("Failed to create time tool:", err)
	}

	nwsTool, err := functiontool.New(functiontool.Config{
		Name:        "get_spot_weather",
		Description: "Returns the temperature, wind speed, forecast, and direction of a provided Spot.",
	}, weather.GetNwsForecast)
	if err != nil {
		log.Fatal("Failed to create National Weather Service tool:", err)
	}

	openMetroTool, err := functiontool.New(functiontool.Config{
		Name:        "get_spot_marine_forecast",
		Description: "Returns hourly marine forecast information of a provided Spot. Used with all SpotTypes.",
	}, weather.GetHourlyMarineForecast)
	if err != nil {
		log.Fatal("Failed to create Open Metro tool:", err)
	}

	currentDateTool, err := functiontool.New(functiontool.Config{
		Name:        "get_current_date",
		Description: "Returns the current date in RFC3339 format so agent can gather bearings. Only required if the current date is required & unknown.",
	}, getDate)

	buoyTool, err := functiontool.New(functiontool.Config{
		Name:        "get_buoy_observations",
		Description: "Returns the latest real-time buoy observations (wave height, dominant period, mean wave direction, wind speed, wind direction) from the nearest NOAA NDBC buoy to the spot. Use this to validate forecast data against actual conditions and identify discrepancies.",
	}, weather.GetBuoyObservations)
	if err != nil {
		log.Fatal("Failed to create buoy tool:", err)
	}

	tidesTool, err := functiontool.New(functiontool.Config{
		Name:        "get_tide_predictions",
		Description: "Returns today's and tomorrow's high and low tide predictions (local time, height in feet relative to MLLW) from the nearest NOAA CO-OPS tide gauge station. Returns nil for lake spots where tides are negligible. Use this to identify the best low-to-mid tide session window.",
	}, weather.GetTidePredictions)
	if err != nil {
		log.Fatal("Failed to create tides tool:", err)
	}

	return []tool.Tool{
		spotTool,
		nwsTool,
		openMetroTool,
		currentDateTool,
		buoyTool,
		tidesTool,
	}
}
