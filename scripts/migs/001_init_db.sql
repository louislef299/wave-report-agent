-- Seeds the local TigerDB (switch to goose if grows too large)
-- +goose no transaction
-- +goose up
CREATE TABLE IF NOT EXISTS observations (
    id int PRIMARY KEY
);

-- +goose down
DROP TABLE observations;
