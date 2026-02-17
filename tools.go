package main

import (
	"log"

	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

func getTools() []tool.Tool {
	spotTool, err := functiontool.New(functiontool.Config{
		Name:        "get_spots_of_interest",
		Description: "Returns the spots of interest for the agent.",
	}, getSpotsOfInterest)
	if err != nil {
		log.Fatalf("Failed to create time tool: %v", err)
	}
	return []tool.Tool{spotTool}
}
