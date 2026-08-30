// Command surfcheck scores the configured lake spots and records what it
// decided.
//
// It prints and stores; it does not notify. Getting the judgement right and
// accumulating enough history to check it against comes first — a transport
// can be bolted on once the ratings are trustworthy.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/ledger"
	"github.com/louislef299/wave-report-agent/pkg/spot"
	"github.com/louislef299/wave-report-agent/pkg/surf"
	"github.com/louislef299/wave-report-agent/pkg/weather"
)

var boards = []surf.Board{surf.Longboard, surf.Shortboard}

func main() {
	var (
		dbPath   = flag.String("db", defaultDBPath(), "path to the ledger database")
		spotName = flag.String("spot", "all", "spot to evaluate, or 'all'")
		tzName   = flag.String("tz", "America/Chicago", "timezone for displayed times")
		verbose  = flag.Bool("v", false, "print the reasoning behind every verdict")
		dryRun   = flag.Bool("dry-run", false, "score and print without writing to the ledger")
	)
	flag.Parse()

	if err := run(context.Background(), *dbPath, *spotName, *tzName, *verbose, *dryRun); err != nil {
		fmt.Fprintln(os.Stderr, "surfcheck:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dbPath, spotName, tzName string, verbose, dryRun bool) error {
	loc, err := time.LoadLocation(tzName)
	if err != nil {
		return fmt.Errorf("loading timezone %q: %w", tzName, err)
	}

	res, err := spot.GetSpotsOfInterest(ctx, spot.SpotArgs{Name: spotName})
	if err != nil {
		return err
	}

	startedAt := time.Now().UTC()
	var runs []ledger.SpotRun

	for _, s := range res.Spots {
		// Rate encodes lake criteria: no groundswell, no tide, and wind that
		// is a prerequisite rather than a hazard. Ocean spots need different
		// rules and stay with the ADK agent for now.
		if s.SpotType != "lake" {
			if spotName != "all" {
				return fmt.Errorf("%s is an %s spot; surfcheck only scores lake spots", s.Name, s.SpotType)
			}
			continue
		}

		sr, err := evaluate(ctx, s, startedAt)
		if err != nil {
			// One unreachable endpoint should not lose the other spot's run.
			fmt.Fprintf(os.Stderr, "surfcheck: skipping %s: %v\n", s.Name, err)
			continue
		}
		runs = append(runs, sr)
		report(os.Stdout, sr, loc, verbose)
	}

	if len(runs) == 0 {
		return fmt.Errorf("no lake spots were successfully evaluated")
	}
	if dryRun {
		fmt.Println("dry run: nothing written to the ledger")
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return fmt.Errorf("creating ledger directory: %w", err)
	}
	l, err := ledger.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer l.Close()

	runID, err := l.RecordRun(ctx, ledger.RunInput{
		StartedAt:      startedAt,
		RulesetVersion: surf.RulesetVersion,
		Spots:          runs,
	})
	if err != nil {
		return err
	}

	fmt.Printf("recorded run %d (%s) to %s\n", runID, surf.RulesetVersion, dbPath)
	return nil
}

// evaluate fetches, merges and scores one spot.
func evaluate(ctx context.Context, s spot.Spot, startedAt time.Time) (ledger.SpotRun, error) {
	// Both endpoints are requested with the same window so their hourly axes
	// line up, and both raw bodies are kept for the ledger.
	marine, marineRaw, err := weather.GetMarineForecast(ctx, &s, weather.ScoringWindow)
	if err != nil {
		return ledger.SpotRun{}, err
	}
	wind, windRaw, err := weather.GetWindForecast(ctx, &s, weather.ScoringWindow)
	if err != nil {
		return ledger.SpotRun{}, err
	}

	conditions, err := surf.Merge(s.Name, startedAt, marine, wind)
	if err != nil {
		return ledger.SpotRun{}, err
	}

	verdicts := make([]surf.Verdict, 0, len(boards))
	for _, b := range boards {
		verdicts = append(verdicts, surf.Rate(conditions, s, b, surf.DefaultThresholds(b)))
	}

	return ledger.SpotRun{
		Spot:       s,
		Conditions: conditions,
		Verdicts:   verdicts,
		Payloads: map[string]string{
			"openmeteo_marine": string(marineRaw),
			"openmeteo_wind":   string(windRaw),
		},
	}, nil
}

func report(w *os.File, sr ledger.SpotRun, loc *time.Location, verbose bool) {
	fmt.Fprintf(w, "\n%s (%s, %s) — %d forecast hours",
		sr.Spot.Name, sr.Spot.City, sr.Spot.State, len(sr.Conditions.Forecast()))
	if sr.Conditions.Gaps > 0 {
		fmt.Fprintf(w, ", %d dropped", sr.Conditions.Gaps)
	}
	fmt.Fprintln(w)

	for _, v := range sr.Verdicts {
		fmt.Fprintf(w, "  %-11s %-5s %s\n", v.Board, v.Rating, summarize(v, loc))
		if verbose {
			for _, r := range v.Reasons {
				fmt.Fprintf(w, "%18s- %s\n", "", r)
			}
		}
	}
}

// summarize describes the best window, or says plainly that there isn't one.
func summarize(v surf.Verdict, loc *time.Location) string {
	var best *surf.Window
	for i := range v.Windows {
		if v.Windows[i].Rating == v.Rating {
			best = &v.Windows[i]
			break
		}
	}
	if best == nil {
		if len(v.Reasons) > 0 {
			return v.Reasons[0]
		}
		return "nothing surfable in range"
	}

	start := best.Start.In(loc)
	end := best.End.In(loc)
	when := fmt.Sprintf("%s %s–%s", start.Format("Mon"), start.Format("15:04"), end.Format("15:04"))
	if !sameDay(start, end) {
		when = fmt.Sprintf("%s – %s", start.Format("Mon 15:04"), end.Format("Mon 15:04"))
	}

	parts := []string{
		when,
		fmt.Sprintf("%.1fft @ %.1fs", best.PeakWaveFt, best.PeakPeriodS),
		fmt.Sprintf("%dh build", best.SustainedHours),
	}
	if n := len(v.Windows); n > 1 {
		parts = append(parts, fmt.Sprintf("+%d more window(s)", n-1))
	}
	return strings.Join(parts, " · ")
}

func sameDay(a, b time.Time) bool {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// defaultDBPath follows the XDG state convention, falling back to the working
// directory when the home directory is unavailable.
func defaultDBPath() string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, "wave-report", "ledger.db")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "ledger.db"
	}
	return filepath.Join(home, ".local", "state", "wave-report", "ledger.db")
}
