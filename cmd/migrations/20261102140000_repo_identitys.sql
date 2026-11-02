-- +goose Up
-- +goose StatementBegin
-- the remembered account per repository is replaced by the remembered identities per repository, so the
-- profile is remembered as well and a repository can have several identities, one of which is the default.
DROP TABLE IF EXISTS repo_accounts;
CREATE TABLE IF NOT EXISTS repo_identitys (
	uid UUID NOT NULL, -- md5 of the ssh key fingerprint, see identity.ID.
	uid_raw VARCHAR NOT NULL, -- the ssh key fingerprint.
	id UUID NOT NULL, -- the identity id the server reported for the ssh key.
	vcs_uri_md5 UUID NOT NULL, -- md5(vcs_uri)::uuid keeps the primary key a fixed size.
	vcs_uri VARCHAR NOT NULL, -- canonical vcs uri of the repository.
	account_id VARCHAR NOT NULL,
	profile_id VARCHAR NOT NULL,
	"default" BOOLEAN NOT NULL DEFAULT false, -- the identity to use for the repository when no hint selects one.
	created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
	PRIMARY KEY (uid, vcs_uri_md5, profile_id)
);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS repo_identitys;
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
