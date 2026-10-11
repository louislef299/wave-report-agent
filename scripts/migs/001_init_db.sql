-- Seeds the local TigerDB (switch to goose if grows too large)
-- +goose up
CREATE TABLE observations (
    id int PRIMARY KEY
);

-- +goose down
DROP TABLE observations;
