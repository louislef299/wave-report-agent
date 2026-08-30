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

## surfcheck: Deterministic Lake Scoring

The agent is the right shape for "give me a detailed overview" and the wrong
shape for "tell me when to drive to Duluth" — it is nondeterministic, cannot be
unit tested, costs money on the ~95% of runs when the lake is flat, and can rate
flat water as Good.

`cmd/surfcheck` is the deterministic half. Plain Go decides whether conditions
are surfable; the agent stays available for the narrative.

```bash
just dry              # score both lake spots, show reasoning, write nothing
just check            # score and record the run to the ledger
go run ./cmd/surfcheck -spot "Stoney Point" -v
```

Each forecast hour passes three gates:

1. **Size and period** — hard floors per board. Period is absolute: six feet at
   3s is chop and cannot rate above Poor at any height.
2. **Build** — was there a sustained blow above the wind threshold from a
   bearing with enough fetch, anywhere in the lookback window? This is why the
   forecast is fetched with `past_days=2`; duration is computed by looking
   backward, so forecast data alone cannot satisfy it.
3. **Groom** — how far the wind sits off the offshore bearing. Epic requires
   offshore, which is what makes the post-frontal NW veer at Stoney Point score
   above the peak of the NE blow that built the sea.

Fetch is per-spot data (`Spot.FetchArcs`), not a single `facing` cardinal —
a gale from a bearing backed by land builds nothing at any speed. Stoney Point's
productive window runs ENE–E down Superior's major axis; Empire Beach's runs
S–SSW down Lake Michigan. Both are configured, so a Superior-specific constant
fails the Empire tests.

Thresholds are seed values calibrated by eye, not measured constants. They are
expected to move.

### The Ledger

Every run writes to SQLite (`~/.local/state/wave-report/ledger.db`, pure-Go
driver, no cgo) so the thresholds can eventually be checked against history:

| Table | Holds |
|---|---|
| `runs` | one evaluation pass, stamped with `ruleset_version` |
| `observations` | the feature vector, one row per spot per hour |
| `decisions` | the verdict per board, **including Poor** |
| `payloads` | raw Open-Meteo responses |
| `outcomes` | hand-filled session results |

Three choices matter for using this as training data. Every run carries a
ruleset version, because thresholds will be tuned and rows produced by different
rules are otherwise not comparable. `observations.forecast_for` is separate from
the run time, so joining an early forecast against a later observation of the
same hour yields forecast error with no manual labelling (`Ledger.ForecastErrors`).
And Poor decisions are recorded too — a corpus of alerts alone has no negative
examples.

Raw payloads are kept because a forecast cannot be re-fetched once its window has
passed. About 30 KB per run, so roughly 90 MB/year at a three-hour cadence.

`outcomes` is the only source of truth for whether the surf was actually good:
both lake spots' nearest NDBC stations are C-MAN shore stations that report wind
and never waves.

## Project Structure

```
main.go                  # entry point, model selection, ADK launcher setup
cmd/
  surfcheck/             # deterministic scorer; prints and records, sends nothing
pkg/
  agent/
    agent.go             # llmagent definition and system prompt
    tools.go             # ADK adapters over the context.Context fetchers
    date.go              # date tool implementation
  spot/
    spot.go              # Spot type + GetSpotsOfInterest
    spots.go             # configured watch list
    fetch.go             # fetch arcs, bearing math, compass parsing
  surf/
    conditions.go        # normalized hourly domain + Merge
    rate.go              # board-specific scoring (lake spots)
  ledger/
    ledger.go            # SQLite persistence
    schema.go            # migrations
  weather/
    client.go            # shared HTTP client with timeouts
    marine.go            # Open-Meteo marine forecast
    wind.go              # Open-Meteo hourly wind (with history)
    nws.go               # NWS gridded weather
    buoy.go              # NOAA NDBC buoy observations
    tides.go             # NOAA CO-OPS tide predictions
    alerts.go            # NWS active alerts
```

The `weather` and `spot` packages take a `context.Context` rather than the ADK's
`tool.Context`, so they are callable from any binary; `pkg/agent/tools.go` holds
thin adapters that bridge the two for `functiontool.New`.

## Further Reading

- [ADK Go quickstart](https://google.github.io/adk-docs/get-started/quickstart/)
- [ADK tool/function calling](https://google.github.io/adk-docs/tools/)
- [Anthropic tool use](https://docs.anthropic.com/en/docs/build-with-claude/tool-use/overview)
