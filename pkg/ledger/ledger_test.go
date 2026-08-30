package ledger

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"github.com/louislef299/wave-report-agent/pkg/surf"
)

const testRuleset = "v1-test"

func TestOpenMigratesIdempotently(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")

	l := openAt(t, path)
	if err := l.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}

	// Reopening an already-migrated file must be a no-op rather than an error.
	l2 := openAt(t, path)
	defer l2.Close()

	var version int
	if err := l2.db.QueryRowContext(t.Context(), "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("reading user_version: %v", err)
	}
	if version != schemaVersion {
		t.Errorf("user_version = %d, want %d", version, schemaVersion)
	}
}

func TestRecordRunRoundTrip(t *testing.T) {
	l := openTemp(t)
	ctx := t.Context()

	started := time.Date(2026, 11, 5, 12, 0, 0, 0, time.UTC)
	runID, err := l.RecordRun(ctx, RunInput{
		StartedAt:      started,
		RulesetVersion: testRuleset,
		Spots: []SpotRun{
			spotRun(t, "Stoney Point", started, surf.Good),
			spotRun(t, "Empire Beach", started, surf.Poor),
		},
	})
	if err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
	if runID == 0 {
		t.Fatal("expected a non-zero run id")
	}

	// Both spots, both boards.
	if n := count(t, l, "SELECT count(*) FROM decisions WHERE run_id = ?", runID); n != 4 {
		t.Errorf("got %d decisions, want 4 (2 spots x 2 boards)", n)
	}
	if n := count(t, l, "SELECT count(*) FROM observations WHERE run_id = ?", runID); n == 0 {
		t.Error("expected observations to be recorded")
	}
	if n := count(t, l, "SELECT count(*) FROM payloads WHERE run_id = ?", runID); n != 4 {
		t.Errorf("got %d payloads, want 4 (2 sources x 2 spots)", n)
	}

	// Observations must carry the fetch available at each hour's wind bearing;
	// that is the feature the scorer keys on and it cannot be recovered later
	// without the spot's arcs.
	var fetch float64
	err = l.db.QueryRowContext(ctx,
		`SELECT fetch_miles FROM observations WHERE run_id = ? AND spot = ? LIMIT 1`,
		runID, "Stoney Point").Scan(&fetch)
	if err != nil {
		t.Fatalf("reading fetch_miles: %v", err)
	}
	if fetch != 300 {
		t.Errorf("fetch_miles = %v, want 300 for an ENE wind at Stoney", fetch)
	}
}

// TestRecordRunKeepsNegatives guards the training set. Logging only alerts
// would leave a corpus with no examples of conditions that were correctly
// rejected.
func TestRecordRunKeepsNegatives(t *testing.T) {
	l := openTemp(t)
	started := time.Date(2026, 8, 30, 12, 0, 0, 0, time.UTC)

	if _, err := l.RecordRun(t.Context(), RunInput{
		StartedAt:      started,
		RulesetVersion: testRuleset,
		Spots:          []SpotRun{spotRun(t, "Stoney Point", started, surf.Poor)},
	}); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	if n := count(t, l, "SELECT count(*) FROM decisions WHERE rating = 'Poor'"); n != 2 {
		t.Errorf("got %d Poor decisions, want 2", n)
	}
}

// TestRecordRunIsAtomic checks that a rejected run leaves nothing behind. A
// half-written run would show up later as a spot that mysteriously stopped
// being evaluated.
func TestRecordRunIsAtomic(t *testing.T) {
	l := openTemp(t)
	started := time.Now().UTC()

	good := spotRun(t, "Stoney Point", started, surf.Good)
	bad := spotRun(t, "Empire Beach", started, surf.Good)
	bad.Spot.Name = "" // rejected during validation

	if _, err := l.RecordRun(t.Context(), RunInput{
		StartedAt:      started,
		RulesetVersion: testRuleset,
		Spots:          []SpotRun{good, bad},
	}); err == nil {
		t.Fatal("expected an error for a spot with no name")
	}

	for _, table := range []string{"runs", "observations", "decisions", "payloads"} {
		if n := count(t, l, "SELECT count(*) FROM "+table); n != 0 {
			t.Errorf("%s has %d rows after a failed run, want 0", table, n)
		}
	}
}

func TestRecordRunRequiresRulesetVersion(t *testing.T) {
	l := openTemp(t)
	_, err := l.RecordRun(t.Context(), RunInput{
		StartedAt: time.Now().UTC(),
		Spots:     []SpotRun{spotRun(t, "Stoney Point", time.Now().UTC(), surf.Poor)},
	})
	if err == nil {
		t.Fatal("expected an error when the ruleset version is empty")
	}
}

func TestRecentDecisions(t *testing.T) {
	l := openTemp(t)
	ctx := t.Context()

	base := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	for i := range 3 {
		at := base.Add(time.Duration(i) * time.Hour)
		if _, err := l.RecordRun(ctx, RunInput{
			StartedAt:      at,
			RulesetVersion: testRuleset,
			Spots: []SpotRun{
				spotRun(t, "Stoney Point", at, surf.Good),
				spotRun(t, "Empire Beach", at, surf.Fair),
			},
		}); err != nil {
			t.Fatalf("RecordRun: %v", err)
		}
	}

	got, err := l.RecentDecisions(ctx, "Stoney Point", 4)
	if err != nil {
		t.Fatalf("RecentDecisions: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d decisions, want the 4 most recent", len(got))
	}
	for _, d := range got {
		if d.Spot != "Stoney Point" {
			t.Fatalf("got a decision for %q, want only Stoney Point", d.Spot)
		}
	}
	// Newest first.
	for i := 1; i < len(got); i++ {
		if got[i].StartedAt.After(got[i-1].StartedAt) {
			t.Fatalf("decisions are not newest-first: %v before %v", got[i-1].StartedAt, got[i].StartedAt)
		}
	}
	if len(got[0].Reasons) == 0 {
		t.Error("expected reasons to survive the round trip")
	}
	if got[0].Notified {
		t.Error("a fresh decision should not be marked notified")
	}
}

func TestMarkNotified(t *testing.T) {
	l := openTemp(t)
	ctx := t.Context()
	at := time.Now().UTC()

	if _, err := l.RecordRun(ctx, RunInput{
		StartedAt:      at,
		RulesetVersion: testRuleset,
		Spots:          []SpotRun{spotRun(t, "Stoney Point", at, surf.Epic)},
	}); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}

	before, err := l.RecentDecisions(ctx, "Stoney Point", 1)
	if err != nil {
		t.Fatalf("RecentDecisions: %v", err)
	}
	if err := l.MarkNotified(ctx, before[0].ID); err != nil {
		t.Fatalf("MarkNotified: %v", err)
	}

	after, err := l.RecentDecisions(ctx, "Stoney Point", 1)
	if err != nil {
		t.Fatalf("RecentDecisions: %v", err)
	}
	if !after[0].Notified {
		t.Error("decision should be marked notified")
	}

	if err := l.MarkNotified(ctx, 99999); err == nil {
		t.Error("expected an error marking a decision that does not exist")
	}
}

// TestForecastErrors covers the automatic half of the labelling strategy: an
// hour predicted by an earlier run, compared against the same hour once it has
// been observed. No manual effort produces these rows.
func TestForecastErrors(t *testing.T) {
	l := openTemp(t)
	ctx := t.Context()

	target := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)

	// A run six hours earlier, for which target is still in the future.
	early := target.Add(-6 * time.Hour)
	predicted := spotRun(t, "Stoney Point", early, surf.Good)
	predicted.Conditions = seriesAt(early, target, 4.0)
	mustRecord(t, l, early, predicted)

	// A later run, for which target is now history.
	late := target.Add(2 * time.Hour)
	observed := spotRun(t, "Stoney Point", late, surf.Good)
	observed.Conditions = seriesAt(late, target, 3.0)
	mustRecord(t, l, late, observed)

	errs, err := l.ForecastErrors(ctx, "Stoney Point")
	if err != nil {
		t.Fatalf("ForecastErrors: %v", err)
	}
	if len(errs) == 0 {
		t.Fatal("expected at least one forecast/observation pair")
	}

	var found bool
	for _, e := range errs {
		if e.ForecastFor.Equal(target) {
			found = true
			if e.PredictedWaveFt != 4.0 || e.ObservedWaveFt != 3.0 {
				t.Errorf("got predicted %.1f / observed %.1f, want 4.0 / 3.0",
					e.PredictedWaveFt, e.ObservedWaveFt)
			}
			if e.ErrorFt != 1.0 {
				t.Errorf("error = %.1f ft, want 1.0", e.ErrorFt)
			}
		}
	}
	if !found {
		t.Errorf("no pair for the target hour %v", target)
	}
}

func TestOutcomes(t *testing.T) {
	l := openTemp(t)
	ctx := t.Context()

	want := Outcome{
		Spot:        "Stoney Point",
		SessionDate: time.Date(2026, 11, 2, 0, 0, 0, 0, time.UTC),
		Board:       string(surf.Longboard),
		Went:        true,
		Quality:     4,
		Notes:       "head high on the sets, NW groom held for two hours",
	}
	if err := l.RecordOutcome(ctx, want); err != nil {
		t.Fatalf("RecordOutcome: %v", err)
	}

	got, err := l.Outcomes(ctx, "Stoney Point")
	if err != nil {
		t.Fatalf("Outcomes: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d outcomes, want 1", len(got))
	}
	if got[0].Quality != want.Quality || got[0].Notes != want.Notes || !got[0].Went {
		t.Errorf("round trip mismatch: got %+v, want %+v", got[0], want)
	}
	if !got[0].SessionDate.Equal(want.SessionDate) {
		t.Errorf("session date = %v, want %v", got[0].SessionDate, want.SessionDate)
	}
}

// helpers

func openTemp(t *testing.T) *Ledger {
	t.Helper()
	return openAt(t, filepath.Join(t.TempDir(), "ledger.db"))
}

// openAt uses a real file rather than :memory:. With database/sql pooling,
// each connection to :memory: gets its own private database, so a migration
// run on one connection is invisible to the next.
func openAt(t *testing.T, path string) *Ledger {
	t.Helper()
	l, err := Open(t.Context(), path)
	if err != nil {
		t.Fatalf("Open(%s): %v", path, err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func mustRecord(t *testing.T, l *Ledger, at time.Time, runs ...SpotRun) {
	t.Helper()
	if _, err := l.RecordRun(t.Context(), RunInput{
		StartedAt:      at,
		RulesetVersion: testRuleset,
		Spots:          runs,
	}); err != nil {
		t.Fatalf("RecordRun: %v", err)
	}
}

func spotRun(t *testing.T, spotName string, at time.Time, rating surf.Rating) SpotRun {
	t.Helper()

	res, err := spot.GetSpotsOfInterest(t.Context(), spot.SpotArgs{Name: spotName})
	if err != nil {
		t.Fatalf("looking up %q: %v", spotName, err)
	}
	s := res.Spots[0]

	c := seriesAt(at, at, 3.5)
	c.Spot = spotName

	mk := func(b surf.Board) surf.Verdict {
		v := surf.Verdict{Spot: spotName, Board: b, Rating: rating, Reasons: []string{"synthetic test verdict"}}
		if rating != surf.Poor {
			v.Windows = []surf.Window{{
				Start: at, End: at.Add(2 * time.Hour),
				PeakWaveFt: 4.5, PeakPeriodS: 6.0, SustainedHours: 14, Rating: rating,
			}}
		}
		return v
	}

	return SpotRun{
		Spot:       s,
		Conditions: c,
		Verdicts:   []surf.Verdict{mk(surf.Longboard), mk(surf.Shortboard)},
		Payloads: map[string]string{
			"openmeteo_marine": `{"hourly":{"time":[]}}`,
			"openmeteo_wind":   `{"hourly":{"time":[]}}`,
		},
	}
}

// seriesAt builds a short series centred on target, fetched at fetchedAt. The
// wind sits at 70 degrees, which is Stoney Point's long-fetch bearing.
func seriesAt(fetchedAt, target time.Time, waveFt float64) surf.Conditions {
	c := surf.Conditions{Spot: "Stoney Point", FetchedAt: fetchedAt}
	for i := -2; i <= 2; i++ {
		c.Hours = append(c.Hours, surf.Hour{
			Time:         target.Add(time.Duration(i) * time.Hour),
			WaveHeightFt: waveFt,
			WavePeriodS:  6.0,
			WaveDirDeg:   70,
			WindSpeedMph: 25,
			WindDirDeg:   70,
			WindGustMph:  32,
		})
	}
	return c
}

func count(t *testing.T, l *Ledger, query string, args ...any) int {
	t.Helper()
	var n int
	if err := l.db.QueryRowContext(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("counting with %q: %v", query, err)
	}
	return n
}
