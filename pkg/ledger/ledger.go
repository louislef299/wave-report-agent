// Package ledger persists every scoring decision and the conditions behind it
// to SQLite.
//
// It is a record for training, not a cache. The scorer's thresholds are seed
// values rather than measured constants, and the only way to check them is
// against history — so each run stores the features it read, the verdict it
// reached, the ruleset that produced it, and the raw upstream payloads it came
// from. Forecasts cannot be re-fetched once their window has passed; anything
// not written here is unrecoverable.
package ledger

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/louislef299/wave-report-agent/pkg/spot"
	"github.com/louislef299/wave-report-agent/pkg/surf"

	_ "modernc.org/sqlite" // pure-Go driver: no cgo, so this cross-compiles
)

// stamp is the storage format for every timestamp. SQLite has no time type,
// and RFC3339 in UTC sorts lexically in chronological order.
const stamp = time.RFC3339

type Ledger struct {
	db *sql.DB
}

// Open connects to the database at path, creating and migrating it as needed.
func Open(ctx context.Context, path string) (*Ledger, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("opening ledger at %s: %w", path, err)
	}

	// SQLite permits exactly one writer. The default connection pool will
	// happily open several and then fail them with "database is locked", which
	// serialising here avoids outright.
	db.SetMaxOpenConns(1)

	pragmas := []string{
		"PRAGMA journal_mode = WAL",
		"PRAGMA busy_timeout = 5000",
		"PRAGMA foreign_keys = ON",
	}
	for _, p := range pragmas {
		if _, err := db.ExecContext(ctx, p); err != nil {
			db.Close()
			return nil, fmt.Errorf("applying %q: %w", p, err)
		}
	}

	l := &Ledger{db: db}
	if err := l.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return l, nil
}

func (l *Ledger) Close() error { return l.db.Close() }

// migrate applies any migrations the database has not seen. Tracking progress
// in PRAGMA user_version means there is no bootstrap table to create first.
func (l *Ledger) migrate(ctx context.Context) error {
	var version int
	if err := l.db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		return fmt.Errorf("reading schema version: %w", err)
	}

	for i := version; i < len(migrations); i++ {
		tx, err := l.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("starting migration %d: %w", i+1, err)
		}
		if _, err := tx.ExecContext(ctx, migrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("applying migration %d: %w", i+1, err)
		}
		// user_version does not accept a bound parameter.
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("PRAGMA user_version = %d", i+1)); err != nil {
			tx.Rollback()
			return fmt.Errorf("recording migration %d: %w", i+1, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", i+1, err)
		}
	}
	return nil
}

// SpotRun is everything one spot produced during a run.
type SpotRun struct {
	Spot       spot.Spot
	Conditions surf.Conditions

	// Verdicts holds one entry per board evaluated.
	Verdicts []surf.Verdict

	// Payloads maps a source name (e.g. "openmeteo_marine") to the raw
	// response body it returned.
	Payloads map[string]string
}

// RunInput is a complete evaluation pass, written atomically.
type RunInput struct {
	StartedAt      time.Time
	RulesetVersion string
	Spots          []SpotRun
}

// RecordRun writes an entire pass in one transaction and returns the run id.
// A partially written run would later read as a spot that quietly stopped
// being evaluated, so it is all or nothing.
func (l *Ledger) RecordRun(ctx context.Context, in RunInput) (int64, error) {
	if in.RulesetVersion == "" {
		return 0, errors.New("ruleset version is required: decisions are not comparable without it")
	}
	for i, sr := range in.Spots {
		if sr.Spot.Name == "" {
			return 0, fmt.Errorf("spot %d has no name", i)
		}
	}

	tx, err := l.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("starting run: %w", err)
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx,
		`INSERT INTO runs (started_at, ruleset_version) VALUES (?, ?)`,
		in.StartedAt.UTC().Format(stamp), in.RulesetVersion)
	if err != nil {
		return 0, fmt.Errorf("inserting run: %w", err)
	}
	runID, err := res.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("reading run id: %w", err)
	}

	for _, sr := range in.Spots {
		if err := insertObservations(ctx, tx, runID, sr); err != nil {
			return 0, err
		}
		if err := insertDecisions(ctx, tx, runID, sr); err != nil {
			return 0, err
		}
		if err := insertPayloads(ctx, tx, runID, sr); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing run: %w", err)
	}
	return runID, nil
}

func insertObservations(ctx context.Context, tx *sql.Tx, runID int64, sr SpotRun) error {
	stmt, err := tx.PrepareContext(ctx, `
        INSERT INTO observations (
            run_id, spot, forecast_for, is_forecast,
            wave_height_ft, wave_period_s, wave_dir_deg,
            wind_speed_mph, wind_dir_deg, wind_gust_mph, fetch_miles
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return fmt.Errorf("preparing observation insert: %w", err)
	}
	defer stmt.Close()

	for _, h := range sr.Conditions.Hours {
		isForecast := 0
		if !h.Time.Before(sr.Conditions.FetchedAt) {
			isForecast = 1
		}
		_, err := stmt.ExecContext(ctx,
			runID, sr.Spot.Name, h.Time.UTC().Format(stamp), isForecast,
			h.WaveHeightFt, h.WavePeriodS, h.WaveDirDeg,
			h.WindSpeedMph, h.WindDirDeg, h.WindGustMph,
			sr.Spot.FetchMilesAt(h.WindDirDeg))
		if err != nil {
			return fmt.Errorf("inserting observation for %s at %v: %w", sr.Spot.Name, h.Time, err)
		}
	}
	return nil
}

func insertDecisions(ctx context.Context, tx *sql.Tx, runID int64, sr SpotRun) error {
	for _, v := range sr.Verdicts {
		reasons, err := json.Marshal(v.Reasons)
		if err != nil {
			return fmt.Errorf("encoding reasons for %s/%s: %w", sr.Spot.Name, v.Board, err)
		}

		// A verdict may hold several windows; the one matching the overall
		// rating is what a notification would quote. The rest stay
		// recoverable from the stored payloads.
		var start, end any
		var peakWave, peakPeriod any
		var sustained any
		if w, ok := bestWindow(v); ok {
			start = w.Start.UTC().Format(stamp)
			end = w.End.UTC().Format(stamp)
			peakWave, peakPeriod, sustained = w.PeakWaveFt, w.PeakPeriodS, w.SustainedHours
		}

		_, err = tx.ExecContext(ctx, `
            INSERT INTO decisions (
                run_id, spot, board, rating, window_start, window_end,
                peak_wave_ft, peak_period_s, sustained_hours, reasons
            ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			runID, sr.Spot.Name, string(v.Board), string(v.Rating),
			start, end, peakWave, peakPeriod, sustained, string(reasons))
		if err != nil {
			return fmt.Errorf("inserting decision for %s/%s: %w", sr.Spot.Name, v.Board, err)
		}
	}
	return nil
}

func insertPayloads(ctx context.Context, tx *sql.Tx, runID int64, sr SpotRun) error {
	for source, body := range sr.Payloads {
		_, err := tx.ExecContext(ctx,
			`INSERT INTO payloads (run_id, spot, source, body) VALUES (?, ?, ?, ?)`,
			runID, sr.Spot.Name, source, body)
		if err != nil {
			return fmt.Errorf("inserting %s payload for %s: %w", source, sr.Spot.Name, err)
		}
	}
	return nil
}

func bestWindow(v surf.Verdict) (surf.Window, bool) {
	var best surf.Window
	var found bool
	for _, w := range v.Windows {
		if w.Rating == v.Rating {
			return w, true
		}
		if !found {
			best, found = w, true
		}
	}
	return best, found
}

// Decision is a stored verdict joined to the run that produced it.
type Decision struct {
	ID             int64
	RunID          int64
	StartedAt      time.Time
	Spot           string
	Board          string
	Rating         string
	WindowStart    *time.Time
	WindowEnd      *time.Time
	PeakWaveFt     float64
	PeakPeriodS    float64
	SustainedHours int
	Reasons        []string
	Notified       bool
}

// RecentDecisions returns the newest decisions for a spot, most recent first.
func (l *Ledger) RecentDecisions(ctx context.Context, spotName string, limit int) ([]Decision, error) {
	rows, err := l.db.QueryContext(ctx, `
        SELECT d.id, d.run_id, r.started_at, d.spot, d.board, d.rating,
               d.window_start, d.window_end, d.peak_wave_ft, d.peak_period_s,
               d.sustained_hours, d.reasons, d.notified
        FROM decisions d
        JOIN runs r ON r.id = d.run_id
        WHERE d.spot = ?
        ORDER BY r.started_at DESC, d.id DESC
        LIMIT ?`, spotName, limit)
	if err != nil {
		return nil, fmt.Errorf("querying decisions for %s: %w", spotName, err)
	}
	defer rows.Close()

	var out []Decision
	for rows.Next() {
		var (
			d          Decision
			startedAt  string
			start, end sql.NullString
			wave, per  sql.NullFloat64
			sustained  sql.NullInt64
			reasons    string
		)
		if err := rows.Scan(&d.ID, &d.RunID, &startedAt, &d.Spot, &d.Board, &d.Rating,
			&start, &end, &wave, &per, &sustained, &reasons, &d.Notified); err != nil {
			return nil, fmt.Errorf("scanning decision: %w", err)
		}

		if d.StartedAt, err = time.Parse(stamp, startedAt); err != nil {
			return nil, fmt.Errorf("parsing run timestamp %q: %w", startedAt, err)
		}
		if d.WindowStart, err = parseNullTime(start); err != nil {
			return nil, err
		}
		if d.WindowEnd, err = parseNullTime(end); err != nil {
			return nil, err
		}
		d.PeakWaveFt, d.PeakPeriodS = wave.Float64, per.Float64
		d.SustainedHours = int(sustained.Int64)
		if err := json.Unmarshal([]byte(reasons), &d.Reasons); err != nil {
			return nil, fmt.Errorf("decoding reasons for decision %d: %w", d.ID, err)
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

// MarkNotified records that a decision was sent, so a later run does not send
// it again.
func (l *Ledger) MarkNotified(ctx context.Context, decisionID int64) error {
	res, err := l.db.ExecContext(ctx,
		`UPDATE decisions SET notified = 1 WHERE id = ?`, decisionID)
	if err != nil {
		return fmt.Errorf("marking decision %d notified: %w", decisionID, err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("no decision with id %d", decisionID)
	}
	return nil
}

// ForecastError pairs a predicted hour with the same hour once observed.
type ForecastError struct {
	Spot            string
	ForecastFor     time.Time
	PredictedAt     time.Time
	PredictedWaveFt float64
	ObservedWaveFt  float64
	ErrorFt         float64
	LeadHours       float64
}

// ForecastErrors returns every forecast/observation pair for a spot. These are
// the labels that cost nothing to collect: the model's own later view of an
// hour it previously predicted. They measure forecast skill rather than
// whether the surf was good, which is what the hand-filled outcomes table is
// for.
func (l *Ledger) ForecastErrors(ctx context.Context, spotName string) ([]ForecastError, error) {
	rows, err := l.db.QueryContext(ctx, `
        SELECT f.spot, f.forecast_for, fr.started_at, f.wave_height_ft, o.wave_height_ft
        FROM observations f
        JOIN runs fr ON fr.id = f.run_id
        JOIN observations o
          ON o.spot = f.spot AND o.forecast_for = f.forecast_for AND o.is_forecast = 0
        WHERE f.spot = ? AND f.is_forecast = 1
          AND f.wave_height_ft IS NOT NULL AND o.wave_height_ft IS NOT NULL
        ORDER BY f.forecast_for DESC`, spotName)
	if err != nil {
		return nil, fmt.Errorf("querying forecast errors for %s: %w", spotName, err)
	}
	defer rows.Close()

	var out []ForecastError
	for rows.Next() {
		var (
			fe                  ForecastError
			forecastFor, predAt string
		)
		if err := rows.Scan(&fe.Spot, &forecastFor, &predAt,
			&fe.PredictedWaveFt, &fe.ObservedWaveFt); err != nil {
			return nil, fmt.Errorf("scanning forecast error: %w", err)
		}
		if fe.ForecastFor, err = time.Parse(stamp, forecastFor); err != nil {
			return nil, fmt.Errorf("parsing forecast_for %q: %w", forecastFor, err)
		}
		if fe.PredictedAt, err = time.Parse(stamp, predAt); err != nil {
			return nil, fmt.Errorf("parsing run timestamp %q: %w", predAt, err)
		}
		fe.ErrorFt = fe.PredictedWaveFt - fe.ObservedWaveFt
		fe.LeadHours = fe.ForecastFor.Sub(fe.PredictedAt).Hours()
		out = append(out, fe)
	}
	return out, rows.Err()
}

// Outcome is a hand-recorded session result, the only source of truth for
// whether conditions were actually any good.
type Outcome struct {
	ID          int64
	Spot        string
	SessionDate time.Time
	Board       string
	Went        bool
	Quality     int // 1-5, 0 when unrecorded
	Notes       string
}

func (l *Ledger) RecordOutcome(ctx context.Context, o Outcome) error {
	if o.Spot == "" {
		return errors.New("outcome requires a spot")
	}
	_, err := l.db.ExecContext(ctx, `
        INSERT INTO outcomes (spot, session_date, board, went, quality, notes)
        VALUES (?, ?, ?, ?, ?, ?)`,
		o.Spot, o.SessionDate.UTC().Format(stamp), o.Board, o.Went, o.Quality, o.Notes)
	if err != nil {
		return fmt.Errorf("recording outcome for %s: %w", o.Spot, err)
	}
	return nil
}

func (l *Ledger) Outcomes(ctx context.Context, spotName string) ([]Outcome, error) {
	rows, err := l.db.QueryContext(ctx, `
        SELECT id, spot, session_date, board, went, quality, notes
        FROM outcomes WHERE spot = ? ORDER BY session_date DESC`, spotName)
	if err != nil {
		return nil, fmt.Errorf("querying outcomes for %s: %w", spotName, err)
	}
	defer rows.Close()

	var out []Outcome
	for rows.Next() {
		var (
			o           Outcome
			sessionDate string
			board, note sql.NullString
			quality     sql.NullInt64
		)
		if err := rows.Scan(&o.ID, &o.Spot, &sessionDate, &board, &o.Went, &quality, &note); err != nil {
			return nil, fmt.Errorf("scanning outcome: %w", err)
		}
		if o.SessionDate, err = time.Parse(stamp, sessionDate); err != nil {
			return nil, fmt.Errorf("parsing session_date %q: %w", sessionDate, err)
		}
		o.Board, o.Notes, o.Quality = board.String, note.String, int(quality.Int64)
		out = append(out, o)
	}
	return out, rows.Err()
}

func parseNullTime(s sql.NullString) (*time.Time, error) {
	if !s.Valid {
		return nil, nil
	}
	t, err := time.Parse(stamp, s.String)
	if err != nil {
		return nil, fmt.Errorf("parsing timestamp %q: %w", s.String, err)
	}
	return &t, nil
}
