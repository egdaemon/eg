-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS lan_peers (
	peer_id VARCHAR PRIMARY KEY,
	public_key BLOB NOT NULL DEFAULT ''::BLOB,
	addrs VARCHAR NOT NULL DEFAULT '[]', -- json encoded list of multiaddrs.
	description VARCHAR NOT NULL DEFAULT '',
	os VARCHAR NOT NULL DEFAULT '',
	arch VARCHAR NOT NULL DEFAULT '',
	cores BIGINT NOT NULL DEFAULT 0,
	memory BIGINT NOT NULL DEFAULT 0, -- bytes
	disk BIGINT NOT NULL DEFAULT 0, -- bytes, 0 until the control plane reports it.
	vram BIGINT NOT NULL DEFAULT 0, -- bytes
	workload_ttl BIGINT NOT NULL DEFAULT 0, -- nanoseconds, maximum workload runtime. 0 until the control plane reports it.
	labels VARCHAR NOT NULL DEFAULT '[]', -- json encoded list of labels.
	uploadable BOOLEAN NOT NULL DEFAULT FALSE,
	first_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
	last_seen TIMESTAMPTZ NOT NULL DEFAULT now(),
	expires_after BIGINT NOT NULL DEFAULT 0, -- nanoseconds, per peer override of the cache ttl. 0 uses the default, negative never expires.
	disabled BOOLEAN NOT NULL DEFAULT FALSE
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS lan_peers;
-- +goose StatementEnd
