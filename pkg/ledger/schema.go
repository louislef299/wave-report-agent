package ledger

// schemaVersion is the highest migration index applied by Open. It is tracked
// in SQLite's own PRAGMA user_version, which avoids needing a bootstrap table
// or a migration library for what is a handful of statements.
var schemaVersion = len(migrations)

// migrations run in order. Each entry moves user_version forward by one, so
// entries are append-only: editing one that has already shipped would leave
// existing databases silently inconsistent with new ones.
var migrations = []string{
	`
-- One evaluation pass over every configured spot.
CREATE TABLE runs (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at      TEXT NOT NULL,
    -- Thresholds will be tuned. Without knowing which rules produced a row,
    -- decisions from different eras are not comparable and the ledger cannot
    -- be used as training data.
    ruleset_version TEXT NOT NULL
);
CREATE INDEX idx_runs_started_at ON runs(started_at);

-- The feature vector: one row per spot per hour per run.
--
-- Every run records the whole series, trailing observed hours included, so the
-- same hour accumulates predictions from successive runs. Joining an early
-- forecast against a later observation of that hour yields forecast error with
-- no manual labelling.
CREATE TABLE observations (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id         INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    spot           TEXT NOT NULL,
    forecast_for   TEXT NOT NULL,
    -- 1 when this hour was still in the future at fetch time.
    is_forecast    INTEGER NOT NULL,
    wave_height_ft REAL,
    wave_period_s  REAL,
    wave_dir_deg   REAL,
    wind_speed_mph REAL,
    wind_dir_deg   REAL,
    wind_gust_mph  REAL,
    -- Miles of open water at this hour's wind bearing. Derived from the spot's
    -- arcs at write time because those are configuration, not data: if the
    -- arcs are later corrected, rows written under the old ones must still say
    -- what the scorer actually saw.
    fetch_miles    REAL,
    UNIQUE(run_id, spot, forecast_for)
);
CREATE INDEX idx_obs_lookup ON observations(spot, forecast_for, is_forecast);

-- What the scorer concluded. Written on every run including Poor, so the
-- corpus contains correctly rejected conditions and not just alerts.
CREATE TABLE decisions (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id          INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    spot            TEXT NOT NULL,
    board           TEXT NOT NULL,
    rating          TEXT NOT NULL,
    -- The best window's bounds, null when nothing was surfable.
    window_start    TEXT,
    window_end      TEXT,
    peak_wave_ft    REAL,
    peak_period_s   REAL,
    sustained_hours INTEGER,
    -- JSON array of the human-readable explanations behind the rating.
    reasons         TEXT NOT NULL,
    notified        INTEGER NOT NULL DEFAULT 0,
    UNIQUE(run_id, spot, board)
);
CREATE INDEX idx_decisions_spot ON decisions(spot, id);

-- Raw upstream responses, kept so features nobody has thought of yet can be
-- re-derived later. Historical forecasts cannot be re-fetched once the window
-- has passed, so anything not stored here is gone for good.
CREATE TABLE payloads (
    run_id INTEGER NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    spot   TEXT NOT NULL,
    source TEXT NOT NULL,
    body   TEXT NOT NULL,
    PRIMARY KEY (run_id, spot, source)
);

-- Ground truth, filled in by hand after a session. Sparse by nature, and the
-- only signal available for whether the surf was actually any good: both lake
-- spots' nearest stations are shore stations that report wind and never waves.
CREATE TABLE outcomes (
    id           INTEGER PRIMARY KEY AUTOINCREMENT,
    spot         TEXT NOT NULL,
    session_date TEXT NOT NULL,
    board        TEXT,
    went         INTEGER,
    quality      INTEGER,
    notes        TEXT
);
CREATE INDEX idx_outcomes_spot ON outcomes(spot, session_date);
`,
}
