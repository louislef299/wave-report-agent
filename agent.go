package main

import (
	"context"
	"fmt"
	"os"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

func newWaveAgent(ctx context.Context) (agent.Agent, error) {
	model, err := gemini.NewModel(ctx, "gemini-3-flash-preview", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		return nil, fmt.Errorf("create model: %w", err)
	}

	return llmagent.New(llmagent.Config{
		Name:        "wave_report_agent",
		Model:       model,
		Description: "Alerts users when wave conditions look perfect for surfing.",
		Instruction: "You are a diligent assistant that monitors weather conditions and alerts users when spots that are in the watch list look favorable for surfing. Use the get_spots_of_interest tool to gather the watch list that the user is interested in and get_spot_info to gather a prompt detailing the favorable conditions for that spot.",
	})
}
