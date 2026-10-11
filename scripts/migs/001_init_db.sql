-- Baseline schema for the wave-report ledger: a record for training, not a
-- cache. Forecasts cannot be re-fetched once their window has passed, so
-- anything not written here is unrecoverable.
--
-- Runs outside a transaction because it creates the extension before calling
-- into it. A failed migration therefore leaves partial state, which is the
-- right trade for a local database that gets wiped with `just deepclean`.
-- +goose up

-- Values match the Go constants in pkg/surf/rate.go exactly. surf_rating is
-- declared worst-to-best so Postgres comparison follows ratingOrder for free:
-- rating >= 'Good' is the -min-rating filter with no lookup table.
CREATE TYPE surf_board  AS ENUM ('longboard', 'shortboard');
CREATE TYPE surf_rating AS ENUM ('Poor', 'Fair', 'Good', 'Epic');

-- One evaluation pass over every configured spot. Stays a plain table: it is
-- the foreign-key target for the hypertables below, and a hypertable cannot be
-- referenced by a foreign key.
CREATE TABLE runs (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    started_at      timestamptz NOT NULL,
    -- Thresholds will be tuned. Without knowing which rules produced a row,
    -- decisions from different eras are not comparable.
    ruleset_version text NOT NULL
);
CREATE INDEX idx_runs_started_at ON runs (started_at DESC);

-- The feature vector: one row per spot per hour per run.
--
-- Every run records the whole series, trailing observed hours included, so the
-- same forecast_for accumulates rows across runs -- which is why run_id is
-- part of the key rather than a conflict. Joining an early forecast against a
-- later observation of that hour yields forecast error with no labelling.
--
-- Every measurement is nullable on purpose. Open-Meteo reports gaps as JSON
-- null, and an absent wave height is not the same fact as a wave height of
-- zero (see pkg/weather/marine.go:54). Writing 0 for a gap would let a later
-- reader see missing data as calm water.
CREATE TABLE observations (
    spot           text        NOT NULL,
    forecast_for   timestamptz NOT NULL,
    run_id         bigint      NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    -- True when this hour was still in the future at fetch time.
    is_forecast    boolean     NOT NULL,
    wave_height_ft double precision,
    wave_period_s  double precision,
    wave_dir_deg   double precision,
    wind_speed_mph double precision,
    wind_dir_deg   double precision,
    wind_gust_mph  double precision,
    -- Miles of open water at this hour's wind bearing, derived from the spot's
    -- arcs at write time because those are configuration, not data: if an arc
    -- is later corrected, rows written under the old one must still say what
    -- the scorer actually saw.
    fetch_miles    double precision,
    -- Timescale requires the partitioning column in every unique index.
    -- Leading with spot suits the access pattern; forecast_for satisfies the
    -- rule and run_id keeps successive runs of the same hour distinct.
    PRIMARY KEY (spot, forecast_for, run_id)
);

SELECT create_hypertable('observations', by_range('forecast_for', INTERVAL '30 days'));

-- Serves the observed side of the forecast-error join.
CREATE INDEX idx_obs_observed ON observations (spot, forecast_for DESC)
    WHERE NOT is_forecast;

-- What the scorer concluded. Written on every run including Poor, so the
-- corpus contains correctly rejected conditions and not just alerts.
--
-- Deliberately NOT a hypertable: it is a handful of rows per run (spots x
-- boards), and MarkNotified updates a single row by surrogate id.
CREATE TABLE decisions (
    id              bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    run_id          bigint      NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    spot            text        NOT NULL,
    board           surf_board  NOT NULL,
    rating          surf_rating NOT NULL,
    -- The best window's bounds, null when nothing was surfable.
    window_start    timestamptz,
    window_end      timestamptz,
    peak_wave_ft    double precision,
    peak_period_s   double precision,
    sustained_hours integer,
    -- The human-readable explanations behind the rating. jsonb rather than
    -- text: this is structured data worth querying, unlike payloads.body.
    reasons         jsonb       NOT NULL,
    notified        boolean     NOT NULL DEFAULT false,
    UNIQUE (run_id, spot, board)
);
CREATE INDEX idx_decisions_spot     ON decisions (spot, id DESC);
CREATE INDEX idx_decisions_notified ON decisions (spot, board) WHERE notified;

-- Raw upstream responses, kept so features nobody has thought of yet can be
-- re-derived later.
--
-- body stays text, not jsonb: jsonb reorders keys and normalizes numbers, and
-- the whole reason pkg/weather returns raw bytes alongside the decoded value
-- is to keep exactly what upstream sent.
--
-- fetched_at exists because a hypertable needs a time column; set it to the
-- run's started_at so (run_id, spot, source) stays effectively unique.
CREATE TABLE payloads (
    run_id     bigint      NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
    spot       text        NOT NULL,
    source     text        NOT NULL,
    fetched_at timestamptz NOT NULL,
    body       text        NOT NULL,
    PRIMARY KEY (run_id, spot, source, fetched_at)
);

SELECT create_hypertable('payloads', by_range('fetched_at', INTERVAL '7 days'));

-- Ground truth, filled in by hand after a session. Sparse by nature, and the
-- only signal for whether the surf was actually any good: both lake spots'
-- nearest stations are shore stations (BSBM4, SLVM5) that report wind and
-- never waves. Plain table -- a few rows a season.
CREATE TABLE outcomes (
    id           bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    spot         text NOT NULL,
    session_date date NOT NULL,
    board        surf_board,
    went         boolean,
    -- Null means unrecorded, rather than overloading 0.
    quality      smallint CHECK (quality BETWEEN 1 AND 5),
    notes        text
);
CREATE INDEX idx_outcomes_spot ON outcomes (spot, session_date DESC);

-- Columnstore. A run only ever writes hours within +/-3 days of now
-- (weather.ScoringWindow), so a 30-day delay guarantees no chunk is compressed
-- while rows are still arriving in it.
ALTER TABLE observations SET (
    timescaledb.enable_columnstore = true,
    timescaledb.segmentby          = 'spot',
    timescaledb.orderby            = 'forecast_for DESC, run_id DESC'
);
CALL add_columnstore_policy('observations', after => INTERVAL '30 days');

-- Raw bodies are the bulkiest thing here by an order of magnitude and are
-- never written retroactively, so they can compress sooner. No retention
-- policy anywhere: historical forecasts are unrecoverable once dropped.
ALTER TABLE payloads SET (
    timescaledb.enable_columnstore = true,
    timescaledb.segmentby          = 'spot, source',
    timescaledb.orderby            = 'fetched_at DESC'
);
CALL add_columnstore_policy('payloads', after => INTERVAL '7 days');

-- The latest observed reading for each hour. Successive runs each re-record a
-- past hour, so joining observations to itself on (spot, forecast_for) without
-- deduplicating multiplies rows.
CREATE VIEW latest_observed AS
SELECT DISTINCT ON (spot, forecast_for)
       spot, forecast_for, run_id,
       wave_height_ft, wave_period_s, wave_dir_deg,
       wind_speed_mph, wind_dir_deg, wind_gust_mph, fetch_miles
FROM observations
WHERE NOT is_forecast
ORDER BY spot, forecast_for, run_id DESC;

-- Forecast skill: the model's own later view of an hour it previously
-- predicted. These are the labels that cost nothing to collect, as distinct
-- from the hand-filled outcomes table.
CREATE VIEW forecast_errors AS
SELECT f.spot,
       f.forecast_for,
       r.started_at AS predicted_at,
       EXTRACT(EPOCH FROM (f.forecast_for - r.started_at)) / 3600.0 AS lead_hours,
       f.wave_height_ft AS predicted_wave_ft,
       o.wave_height_ft AS observed_wave_ft,
       f.wave_height_ft - o.wave_height_ft AS error_ft
FROM observations f
JOIN runs r ON r.id = f.run_id
JOIN latest_observed o
  ON o.spot = f.spot AND o.forecast_for = f.forecast_for
WHERE f.is_forecast
  AND f.wave_height_ft IS NOT NULL
  AND o.wave_height_ft IS NOT NULL;

-- +goose down
DROP VIEW IF EXISTS forecast_errors;
DROP VIEW IF EXISTS latest_observed;
DROP TABLE IF EXISTS outcomes;
DROP TABLE IF EXISTS payloads;
DROP TABLE IF EXISTS decisions;
DROP TABLE IF EXISTS observations;
DROP TABLE IF EXISTS runs;
DROP TYPE IF EXISTS surf_rating;
DROP TYPE IF EXISTS surf_board;
