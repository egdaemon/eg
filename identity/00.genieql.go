//go:build genieql.generate

package identity

import (
	"context"

	"github.com/egdaemon/eg/internal/sqlx"
	genieql "github.com/james-lawrence/genieql/ginterp"
)

func RepoIdentity(gql genieql.Structure) {
	gql.From(gql.Table("repo_identitys"))
}

func RepoIdentityScanner(gql genieql.Scanner, pattern func(p RepoIdentity)) {}

// RepoIdentityUpsert records the identity to use for the repository, replacing the previous record of the profile.
func RepoIdentityUpsert(
	gql genieql.Insert,
	pattern func(ctx context.Context, q sqlx.Queryer, p RepoIdentity) NewRepoIdentityScannerStaticRow,
) {
	gql.Into("repo_identitys").Default("created_at", "updated_at").Conflict(
		`ON CONFLICT (uid, vcs_uri_md5, profile_id) DO UPDATE SET id = EXCLUDED.id, uid_raw = EXCLUDED.uid_raw, vcs_uri = EXCLUDED.vcs_uri, account_id = EXCLUDED.account_id, "default" = EXCLUDED."default", updated_at = now()`,
	)
}

// RepoIdentityClearDefault removes the default mark of every identity of the repository except the profile.
func RepoIdentityClearDefault(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, uid string, vcsuri string, profile string) NewRepoIdentityScannerStatic,
) {
	gql = gql.Query(`UPDATE repo_identitys SET "default" = false, updated_at = now() WHERE uid = {uid} AND vcs_uri_md5 = md5({vcsuri})::uuid AND profile_id != {profile} AND "default" RETURNING ` + RepoIdentityScannerStaticColumns)
}

func RepoIdentityDelete(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, uid string, vcsuri string) NewRepoIdentityScannerStatic,
) {
	gql = gql.Query(`DELETE FROM repo_identitys WHERE uid = {uid} AND vcs_uri_md5 = md5({vcsuri})::uuid RETURNING ` + RepoIdentityScannerStaticColumns)
}
