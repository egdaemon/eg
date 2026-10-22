-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS repo_accounts (
	identity UUID NOT NULL, -- md5 of the ssh key fingerprint, see identity.ID.
	vcs_uri_md5 UUID NOT NULL, -- md5(vcs_uri)::uuid keeps the primary key a fixed size.
	vcs_uri VARCHAR NOT NULL, -- canonical vcs uri of the repository.
	account_id VARCHAR NOT NULL,
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (identity, vcs_uri_md5)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS repo_accounts;
-- +goose StatementEnd
