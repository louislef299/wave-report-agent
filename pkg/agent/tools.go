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

	return []tool.Tool{
		spotTool,
		nwsTool,
	}
}
