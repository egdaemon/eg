-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS 'eg.metrics.custom' (
    id UUID PRIMARY KEY,
    name TEXT NOT NULL,
    name_md5 uuid GENERATED ALWAYS AS (md5(name)),
    ts TIMESTAMP NOT NULL,
    metric JSON NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS "eg.metrics.custom";
-- +goose StatementEnd
