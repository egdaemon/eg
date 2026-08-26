-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS eg_workloads_discovered (
	id UUID PRIMARY KEY,
	path VARCHAR NOT NULL,
	ts TIMESTAMPTZ NOT NULL
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS eg_workloads_discovered;
-- +goose StatementEnd
