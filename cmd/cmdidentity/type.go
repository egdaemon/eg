package cmdidentity

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
)

// New resolves accounts for the invocation, account is the explicit --account-id and l selects among the profiles
// when the account is ambiguous.
func New(ctx context.Context, account string, l login) *T {
	return &T{ctx: ctx, account: account, store: migrations.Path(migrations.CLI), login: l}
}

// T resolves the account to act as, see identity.Resolve.
type T struct {
	ctx      context.Context
	account  string // explicit account from --account-id.
	store    string // path of the account database file.
	login    login  // selects among the profiles when the account is ambiguous.
	resolved *identity.Resolution
}

// Choose selects among the profiles associated with the ssh key, see login.
func (t *T) Choose(profiles []*authn.Authn) (*authn.Authn, error) {
	return t.login(profiles)
}

// Store opens the local database, shared with the lan peers, the caller closes it.
func (t *T) Store() (*sql.DB, error) {
	db, err := migrations.OpenLocked(t.ctx, t.store)
	if err != nil {
		return nil, err
	}

	if err = lan.Purge(t.ctx, db, lan.DefaultTTL); err != nil {
		errorsx.Log(db.Close())
		return nil, err
	}

	return db, nil
}

// Resolve resolves the account to act as. the explicit --account-id wins, otherwise the account is detected from
// the repository, a blank vcsuri is an error, see cmdopts.WorkspaceVCSURI. the result is memoized for the lifetime of
// the process. refresh ignores the remembered account.
func (t *T) Resolve(c *http.Client, signer ssh.Signer, vcsuri string, refresh bool) (_ *identity.Resolution, err error) {
	if t.resolved != nil && !refresh {
		return t.resolved, nil
	}

	// an explicit account needs no repository.
	if stringsx.Blank(t.account) && stringsx.Blank(vcsuri) {
		return nil, errorsx.New("unable to detect the account, a repository is required: run within a git repository with a remote or specify --account-id")
	}

	// the cache is an optimization, never fail because of it.
	db, err := t.Store()
	if err != nil {
		errorsx.Log(errorsx.Wrap(err, "account cache unavailable"))
		db = nil
	}
	if db != nil {
		defer func() { errorsx.Log(db.Close()) }()
	}

	r, err := identity.Resolve(t.ctx, db, c, signer, t.account, vcsuri, t.Choose, refresh)
	if err != nil {
		return &r, err
	}

	t.resolved = &r
	return t.resolved, nil
}

// AccountID the account of a resolution, accepts the results of T.Resolve directly.
func AccountID(r *identity.Resolution, err error) (string, error) {
	if err != nil {
		return "", err
	}

	return r.Current.Account.Id, nil
}
