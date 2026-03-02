# wave-report-agent

A surf condition analyst built with the [Google Agent Development Kit (ADK)](https://google.github.io/adk-docs/) for Go. Given a list of configured surf spots, the agent fetches real-time weather, buoy, tide, and marine forecast data, then rates conditions and recommends the best session window.

## What It Does

The agent orchestrates several free public APIs to produce a structured surf report:

| Tool | Source |
|---|---|
| Marine forecast | [Open-Meteo](https://open-meteo.com/en/docs/marine-weather-api) |
| NWS weather grid | [National Weather Service API](https://www.weather.gov/documentation/services-web-api) |
| Buoy observations | [NOAA NDBC](https://www.ndbc.noaa.gov/) |
| Tide predictions | [NOAA CO-OPS](https://tidesandcurrents.noaa.gov/) |
| Weather alerts | [NWS Alerts API](https://www.weather.gov/documentation/services-web-api#/default/alerts_query) |

It handles both ocean and lake spots (Great Lakes surf is real) with distinct evaluation criteria for each. See also [GLERL GLCFS](https://www.glerl.noaa.gov/res/glcfs/) for Great Lakes coastal forecasting context.

## Prerequisites

- Go 1.22+
- An Anthropic API key (or a Google API key if you switch to Gemini)
- The local `claude-go-adk` sibling repo (see `go.mod` replace directive)

```bash
# Expected directory layout
parent/
├── claude-go-adk/
└── wave-report-agent/
```

## Running

```bash
export ANTHROPIC_API_KEY=<your-key>
go run . web  # starts the ADK dev UI at localhost:8080
```

The `launcher` package from the ADK provides the CLI and web interfaces out of the box. Run `go run . --help` for all subcommands.

## How It Works

The ADK follows a standard [agent loop](https://google.github.io/adk-docs/get-started/core-concepts/): the model receives a prompt, decides which tools to call, receives the results, and continues until it has enough information to respond.

**Agent definition** (`pkg/agent/agent.go`):

```go
llmagent.New(llmagent.Config{
    Name:        "wave_report_agent",
    Model:       model,
    Instruction: prompt,   // system prompt encoding all evaluation criteria
    Tools:       getTools(),
})
```

**Tool registration** (`pkg/agent/tools.go`): Each tool is a plain Go function wrapped with `functiontool.New`. The ADK uses struct field tags (`jsonschema_description`) to generate the JSON schema the model sees when deciding which tool to call — no separate schema definition needed.

**Spots** (`pkg/spot/spots.go`): The watch list is a hardcoded slice of `Spot` structs. To add a spot, append to that slice with the appropriate lat/lon, NDBC buoy ID, and CO-OPS tide station ID.

## Swapping Models

`main.go` defines two model constructors — one for Claude, one for Gemini. Swap the argument passed to `NewWaveAgent`:

```go
// Claude (default)
wagent.NewWaveAgent(ctx, getClaudeModel())

// Gemini
wagent.NewWaveAgent(ctx, getGeminiModel(ctx))
```

Claude requires `ANTHROPIC_API_KEY`; Gemini requires `GOOGLE_API_KEY`.

## Project Structure

```
main.go                  # entry point, model selection, ADK launcher setup
pkg/
  agent/
    agent.go             # llmagent definition and system prompt
    tools.go             # tool wiring (functiontool.New calls)
    date.go              # date tool implementation
  spot/
    spot.go              # Spot type + GetSpotsOfInterest tool func
    spots.go             # configured watch list
  weather/
    marine.go            # Open-Meteo marine forecast
    nws.go               # NWS gridded weather
    buoy.go              # NOAA NDBC buoy observations
    tides.go             # NOAA CO-OPS tide predictions
    alerts.go            # NWS active alerts
```

## Further Reading

- [ADK Go quickstart](https://google.github.io/adk-docs/get-started/quickstart/)
- [ADK tool/function calling](https://google.github.io/adk-docs/tools/)
- [Anthropic tool use](https://docs.anthropic.com/en/docs/build-with-claude/tool-use/overview)
