-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS compute_history (
	id VARCHAR PRIMARY KEY,
	description VARCHAR NOT NULL DEFAULT '',
	vcs_uri VARCHAR NOT NULL DEFAULT '',
	vcs_commit VARCHAR NOT NULL DEFAULT '',
	uploaded_by VARCHAR NOT NULL DEFAULT '',
	result INTEGER NOT NULL DEFAULT 0, -- 0 unknown, 1 failure, 2 completed, 3 cancelled. matches the control plane.
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	initiated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	completed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	duration BIGINT NOT NULL DEFAULT 0, -- nanoseconds
	logs_path VARCHAR NOT NULL DEFAULT '',
	expires_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS compute_history;
-- +goose StatementEnd
