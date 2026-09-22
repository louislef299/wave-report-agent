package main

import (
	"context"
	"log"
	"os"

	"github.com/louislef299/claude-go-adk"
	wagent "github.com/louislef299/wave-report-agent/pkg/agent"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

func main() {
	ctx := context.Background()
	waveAgent, err := wagent.NewWaveAgent(ctx, getClaudeModel())
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(waveAgent),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}

func getGeminiModel(ctx context.Context) model.LLM {
	model, err := gemini.NewModel(ctx, "gemini-3-flash-preview", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		panic(err)
	}
	return model
}

func getClaudeModel() model.LLM {
	return claude.NewModel("claude-sonnet-4-5-20250929", claude.WithDebug())
}
