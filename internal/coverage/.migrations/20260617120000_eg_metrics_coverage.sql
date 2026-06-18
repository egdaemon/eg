-- +goose Up
-- +goose StatementBegin
CREATE TABLE 'eg.metrics.coverage' (
    id UUID PRIMARY KEY DEFAULT uuidv7(),
    path TEXT NOT NULL,
    statements FLOAT4 NOT NULL,
    branches FLOAT4 NOT NULL,
    fnname TEXT NOT NULL DEFAULT '',
    hits BIGINT NOT NULL DEFAULT 0
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS 'eg.metrics.coverage';
-- +goose StatementEnd
