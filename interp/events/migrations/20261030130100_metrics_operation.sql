-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS 'eg.metrics.operation' (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    name_md5 uuid GENERATED ALWAYS AS (md5(name)),
    ts TIMESTAMP NOT NULL,
    module TEXT NOT NULL,
    op TEXT NOT NULL,
    milliseconds INTERVAL NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "eg.metrics.operation";
-- +goose StatementEnd
