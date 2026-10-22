//go:build genieql.generate

package identity

import (
	"context"

	"github.com/egdaemon/eg/internal/sqlx"
	genieql "github.com/james-lawrence/genieql/ginterp"
)

func RepoAccount(gql genieql.Structure) {
	gql.From(gql.Table("repo_accounts"))
}

func RepoAccountScanner(gql genieql.Scanner, pattern func(p RepoAccount)) {}

// RepoAccountUpsert records the account to use for the repository, replacing any previous.
func RepoAccountUpsert(
	gql genieql.Insert,
	pattern func(ctx context.Context, q sqlx.Queryer, p RepoAccount) NewRepoAccountScannerStaticRow,
) {
	gql.Into("repo_accounts").Default("created_at", "updated_at").Conflict(
		"ON CONFLICT (identity, vcs_uri_md5) DO UPDATE SET vcs_uri = EXCLUDED.vcs_uri, account_id = EXCLUDED.account_id, updated_at = now()",
	)
}

func RepoAccountFind(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, identity string, vcsuri string) NewRepoAccountScannerStaticRow,
) {
	gql = gql.Query(`SELECT ` + RepoAccountScannerStaticColumns + ` FROM repo_accounts WHERE identity = {identity} AND vcs_uri_md5 = md5({vcsuri})::uuid`)
}

func RepoAccountDelete(
	gql genieql.Function,
	pattern func(ctx context.Context, q sqlx.Queryer, identity string, vcsuri string) NewRepoAccountScannerStatic,
) {
	gql = gql.Query(`DELETE FROM repo_accounts WHERE identity = {identity} AND vcs_uri_md5 = md5({vcsuri})::uuid RETURNING ` + RepoAccountScannerStaticColumns)
}
