-- +goose Up
-- +goose StatementBegin
-- sessions are recreated rather than altered, they are only a record of the runs of this machine.
DROP TABLE IF EXISTS serve_sessions;
CREATE TABLE serve_sessions (
	id VARCHAR PRIMARY KEY, -- the uuid of the serve invocation, the container is named eg-<id>.
	module VARCHAR NOT NULL DEFAULT '',
	directory VARCHAR NOT NULL DEFAULT '', -- repository root on the host.
	branch VARCHAR NOT NULL DEFAULT '', -- the branch the session's changes are committed to, restored when the session is resumed. blank until one is chosen.
	pid BIGINT NOT NULL DEFAULT 0, -- the serving cli process, used to detect sessions that ended without being stopped.
	pty_socket VARCHAR NOT NULL DEFAULT '', -- unix socket the session's terminal can be attached to while it runs, blank when it has none.
	ssh_requested BIGINT NOT NULL DEFAULT -1, -- the --ssh flag: -1 disabled, 0 automatic port, otherwise the fixed port. used to restart the session.
	ssh_port BIGINT NOT NULL DEFAULT 0, -- host port published to the container's sshd while running, 0 otherwise.
	ssh_user VARCHAR NOT NULL DEFAULT '',
	ssh_key_path VARCHAR NOT NULL DEFAULT '', -- private key of the identity authorized within the container.
	cap_ssh BOOLEAN NOT NULL DEFAULT false, -- reachable over ssh.
	cap_agent BOOLEAN NOT NULL DEFAULT false, -- an agent can run inside it.
	cap_patch BOOLEAN NOT NULL DEFAULT false, -- its changes are patched back to the worktree.
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	stopped_at TIMESTAMPTZ NOT NULL DEFAULT 'infinity' -- infinity while running. stopped sessions are kept as history so they can be restarted.
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS serve_sessions;
CREATE TABLE serve_sessions (
	id VARCHAR PRIMARY KEY,
	module VARCHAR NOT NULL DEFAULT '',
	directory VARCHAR NOT NULL DEFAULT '',
	branch VARCHAR NOT NULL DEFAULT '',
	pid BIGINT NOT NULL DEFAULT 0,
	ssh_requested BIGINT NOT NULL DEFAULT -1,
	ssh_port BIGINT NOT NULL DEFAULT 0,
	ssh_user VARCHAR NOT NULL DEFAULT '',
	ssh_key_path VARCHAR NOT NULL DEFAULT '',
	cap_ssh BOOLEAN NOT NULL DEFAULT false,
	cap_agent BOOLEAN NOT NULL DEFAULT false,
	cap_patch BOOLEAN NOT NULL DEFAULT false,
	started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	stopped_at TIMESTAMPTZ
);
-- +goose StatementEnd
