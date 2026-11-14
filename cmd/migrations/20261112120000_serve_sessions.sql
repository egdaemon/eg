-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS serve_sessions (
	id VARCHAR PRIMARY KEY, -- the uuid of the serve invocation, the container is named eg-<id>.
	module VARCHAR NOT NULL DEFAULT '',
	directory VARCHAR NOT NULL DEFAULT '', -- repository root on the host.
	pid BIGINT NOT NULL DEFAULT 0, -- the serving cli process, used to detect sessions that ended without being stopped.
	ssh_requested BIGINT NOT NULL DEFAULT -1, -- the --ssh flag: -1 disabled, 0 automatic port, otherwise the fixed port. used to restart the session.
	ssh_port BIGINT NOT NULL DEFAULT 0, -- host port published to the container's sshd while running, 0 otherwise.
	ssh_user VARCHAR NOT NULL DEFAULT '',
	ssh_key_path VARCHAR NOT NULL DEFAULT '', -- private key of the identity authorized within the container.
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	stopped_at TIMESTAMPTZ -- NULL while running. stopped sessions are kept as history so they can be restarted.
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS serve_sessions;
-- +goose StatementEnd
