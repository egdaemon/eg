package cmdopts

import (
	"context"
	"database/sql"
	"net/http"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/go-git/go-git/v6"
	"golang.org/x/crypto/ssh"
)

// VCS the repository a command acts on, embedded by the commands that detect the account from it.
type VCS struct {
	VcsURI string `name:"vcs-uri" help:"canonical uri of the repository, used to detect the account" default:"${vars_vcs_uri}"`
}

// Identity resolves the account to act as, see identity.Resolve.
type Identity struct {
	ctx      context.Context
	account  string // explicit account from --account-id.
	store    string // path of the account database file.
	resolved *identity.Resolution
}

// Store opens the local database, shared with the lan peers, the caller closes it.
func (t *Identity) Store() (*sql.DB, error) {
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

// Account resolves the account to act as. the explicit --account-id wins, otherwise the account
// is detected from the repository, a blank vcsuri is an error, see WorkspaceVCSURI.
// the result is memoized for the lifetime of the process.
func (t *Identity) Account(c *http.Client, signer ssh.Signer, vcsuri string) (string, error) {
	r, err := t.Resolve(c, signer, vcsuri, false)
	if err != nil {
		return "", err
	}

	return r.Account, nil
}

// Resolve is Account with the full details of how the account was determined. refresh ignores the
// remembered account.
func (t *Identity) Resolve(c *http.Client, signer ssh.Signer, vcsuri string, refresh bool) (_ *identity.Resolution, err error) {
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

	r, err := identity.Resolve(t.ctx, db, c, signer, t.account, vcsuri, identity.Choose, refresh)
	if err != nil {
		return &r, err
	}

	t.resolved = &r
	return t.resolved, nil
}

// WorkspaceVCSURI the canonical vcs uri of the repository containing the working directory,
// blank when there is none.
func WorkspaceVCSURI() string {
	repo, err := git.PlainOpen(gitx.DetectRoot())
	if err != nil {
		return ""
	}

	// a repository without the default remote is an expected state, not an error.
	uri, err := gitx.CanonicalURI(repo, git.DefaultRemoteName)
	if err != nil {
		return ""
	}

	return uri
}
