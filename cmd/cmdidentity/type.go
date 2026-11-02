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

// New resolves accounts for the invocation, account is the explicit --account-id, profile is the explicit --profile-id
// and l selects among the profiles when the account is ambiguous.
func New(ctx context.Context, account string, profile string, l login) *T {
	return &T{ctx: ctx, account: account, profile: profile, store: migrations.Path(migrations.CLI), login: l}
}

// T resolves the identity to act as, see identity.Environment.
type T struct {
	ctx      context.Context
	account  string // explicit account from --account-id.
	profile  string // explicit profile from --profile-id.
	store    string // path of the account database file.
	login    login  // selects among the profiles when the account is ambiguous.
	env      identity.Environment
	resolved *identity.Resolution
}

// With returns T resolving for the environment, see identity.QueryEnvironment and identity.QueryVCSAuto.
func (t *T) With(env identity.Environment) *T {
	dup := *t
	dup.env = env
	dup.resolved = nil
	return &dup
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

// Resolve resolves the identity to act as for the environment, see With. without one the explicit --account-id and
// --profile-id are used. a repository is required unless an account or profile was explicitly provided, see
// cmdopts.WorkspaceVCSURI. the result is memoized for the lifetime of the process.
func (t *T) Resolve(c *http.Client, signer ssh.Signer) (_ *identity.Resolution, err error) {
	if t.resolved != nil {
		return t.resolved, nil
	}

	env := t.env
	if env == (identity.Environment{}) {
		env = identity.QueryEnvironment(t.account, t.profile, "")
	}

	// an explicit account or profile needs no repository.
	if stringsx.Blank(env.Account) && stringsx.Blank(env.Profile) && stringsx.Blank(env.VCSURI) {
		return nil, errorsx.New("unable to detect the account, a repository is required: run within a git repository with a remote or specify --account-id")
	}

	db, err := t.Store()
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to open the identity store")
	}
	defer func() { errorsx.Log(db.Close()) }()

	r, err := env.Resolve(t.ctx, db, c, signer, t.Choose)
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

	return stringsx.FirstNonBlank(r.Current.GetAccount().GetId(), r.Current.GetProfile().GetAccountId()), nil
}
