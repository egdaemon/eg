// Package identity resolves which account an ssh key acts as. keys can be associated with many
// profiles, rather than requiring --account-id the account is detected from the repository
// being worked on and remembered. see Resolve.
package identity

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Masterminds/squirrel"
	_ "github.com/duckdb/duckdb-go/v2" // registers the duckdb driver.
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/internal/squirrelx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/termx"
	"github.com/gofrs/uuid/v5"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

// Source describes how an account was determined.
type Source string

const (
	SourceFlag   Source = "flag"   // explicitly provided, --account-id or EG_ACCOUNT.
	SourceCache  Source = "cached" // remembered for the repository.
	SourceManual Source = "manual" // chosen interactively.
)

// Chooser selects among profiles, see cmdidentity.Choose.
type Chooser func([]*authn.Authn) (*authn.Authn, error)

// ID the identifier of the ssh key, matches the identity reported by `eg repo whoami`
// and the id a runner is authorized and registered under.
func ID(k ssh.PublicKey) string {
	return md5x.String(ssh.FingerprintSHA256(k))
}

// Filter narrows the identities remembered for an ssh key, filters given together all apply.
type Filter = squirrel.Sqlizer

// Remembered selects the identities remembered for the ssh key, across every repository. add filters to narrow them,
// see RepoIdentitySearch to run it.
func Remembered(ctx context.Context, signer ssh.Signer) squirrel.SelectBuilder {
	return squirrelx.PSQL.Select(sqlx.Columns(RepoIdentityScannerStaticColumns)...).From("repo_identitys").Where(
		squirrel.Eq{"uid": ID(signer.PublicKey())},
	)
}

// RepoIdentitySearch runs the query and returns a scanner for the results.
func RepoIdentitySearch(ctx context.Context, q sqlx.Queryer, b squirrel.SelectBuilder) RepoIdentityScanner {
	return NewRepoIdentityScannerStatic(b.RunWith(q).QueryContext(ctx))
}

// QueryVCSURI matches the identities remembered for the repository, blank matches every repository.
func QueryVCSURI(vcsuri string) Filter {
	if stringsx.Blank(vcsuri) {
		return squirrelx.Noop{}
	}

	return squirrel.Expr("vcs_uri_md5 = md5(?)::uuid", strings.TrimSpace(vcsuri))
}

// QueryAccountID matches the identities of the account, blank matches every account.
func QueryAccountID(id string) Filter {
	if stringsx.Blank(id) {
		return squirrelx.Noop{}
	}

	return squirrel.Eq{"account_id": id}
}

// QueryProfileID matches the identities of the profile, blank matches every profile.
func QueryProfileID(id string) Filter {
	if stringsx.Blank(id) {
		return squirrelx.Noop{}
	}

	return squirrel.Eq{"profile_id": id}
}

// QueryDefault matches the default identity of each repository.
func QueryDefault() Filter {
	return squirrel.Expr(`"default" = true`)
}

// Environment is what is known about the identity to act as: the explicit account and profile (--account-id,
// --profile-id) and the repository being worked on. it is the query over the remembered identities, and also what
// is needed to detect the identity when none is remembered.
type Environment struct {
	Account string
	Profile string
	VCSURI  string
	Default bool   // only the default identity of the repository.
	Source  Source // how a resolved identity is reported.
}

// ToSql the filter of the remembered identities, blank values match everything.
func (t Environment) ToSql() (string, []interface{}, error) {
	filters := squirrel.And{QueryAccountID(t.Account), QueryProfileID(t.Profile), QueryVCSURI(t.VCSURI)}
	if t.Default {
		filters = append(filters, QueryDefault())
	}

	return filters.ToSql()
}

// QueryEnvironment selects the identities remembered for the account, profile and repository, blank values match
// everything.
func QueryEnvironment(account, profile, vcsuri string) Environment {
	return Environment{Account: strings.TrimSpace(account), Profile: strings.TrimSpace(profile), VCSURI: strings.TrimSpace(vcsuri), Source: SourceFlag}
}

// QueryVCSAuto selects the identity to use for the repository, the environment narrowed to the default identity.
func QueryVCSAuto(account, profile, vcsuri string) Environment {
	e := QueryEnvironment(account, profile, vcsuri)
	e.Default = true
	e.Source = SourceCache
	return e
}

// remembered the most recently remembered identity matching the filter, nil when there is none.
func remembered(ctx context.Context, db *sql.DB, signer ssh.Signer, filter Filter) (_ *RepoIdentity, err error) {
	h, err := sqlx.ScanOne(RepoIdentitySearch(ctx, db, Remembered(ctx, signer).Where(filter).OrderBy("updated_at DESC").Limit(1)))
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	} else if err != nil {
		return nil, errorsx.Wrap(err, "unable to lookup remembered identity")
	}

	return &h, nil
}

// Remember the identity for the repository, as the default identity of the repository when it is the default.
func Remember(ctx context.Context, db *sql.DB, signer ssh.Signer, vcsuri string, identity *authn.Identity, current *authn.Authn, isdefault bool) (err error) {
	// identities are remembered per repository.
	if vcsuri = strings.TrimSpace(vcsuri); vcsuri == "" {
		return nil
	}

	h := RepoIdentity{
		UID:       ID(signer.PublicKey()),
		UIDRaw:    identity.GetUidraw(),
		ID:        stringsx.FirstNonBlank(identity.GetId(), uuid.Nil.String()),
		VcsURIMd5: md5x.String(vcsuri),
		VcsURI:    vcsuri,
		AccountID: stringsx.FirstNonBlank(current.GetProfile().GetAccountId(), current.GetAccount().GetId()),
		ProfileID: current.GetProfile().GetId(),
		Default:   isdefault,
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return errorsx.Wrap(err, "unable to remember identity")
	}
	defer func() {
		if err != nil {
			err = errorsx.Compact(err, tx.Rollback())
		}
	}()

	// exactly one default identity per repository.
	if isdefault {
		if err = sqlx.Discard(sqlx.Scan(RepoIdentityClearDefault(ctx, tx, h.UID, vcsuri, h.ProfileID))); err != nil {
			return errorsx.Wrap(err, "unable to remember identity")
		}
	}

	if err = RepoIdentityUpsert(ctx, tx, h).Scan(&h); err != nil {
		return errorsx.Wrap(err, "unable to remember identity")
	}

	return errorsx.Wrap(tx.Commit(), "unable to remember identity")
}

// Forget the identities remembered for the repository.
func Forget(ctx context.Context, db *sql.DB, signer ssh.Signer, vcsuri string) (err error) {
	return errorsx.Wrap(sqlx.Discard(sqlx.Scan(RepoIdentityDelete(ctx, db, ID(signer.PublicKey()), vcsuri))), "unable to forget identity")
}

// Resolution is the outcome of Resolve.
type Resolution struct {
	Source     Source
	Identity   *authn.Identity
	Current    *authn.Authn   // the resolved profile, only the account is populated when it was not looked up.
	Candidates []*authn.Authn // the profiles to choose from when the account is ambiguous.
}

// ErrAmbiguous signals that multiple profiles are associated with an ssh key and none was chosen.
const ErrAmbiguous = errorsx.String("multiple profiles are associated with this ssh key")

// Resolve determines the identity to use. in order:
//  1. the identity remembered for the environment, see QueryEnvironment and QueryVCSAuto.
//  2. the only profile associated with the ssh key, matching the explicitly provided account and profile.
//  3. prompting, when choose is set and the session is interactive; otherwise ErrAmbiguous.
//
// db remembers identities per repository, see Open. c is the base http client, it is not expected to be
// authenticated. choose may be nil to fail instead of prompting. looked up and selected identities are remembered.
func (t Environment) Resolve(ctx context.Context, db *sql.DB, c *http.Client, signer ssh.Signer, choose Chooser) (zero Resolution, err error) {
	return t.resolve(ctx, db, signer, choose, func() (*authn.Authed, error) {
		return Profiles(ctx, c, signer, t.Account, t.Profile)
	})
}

func (t Environment) resolve(ctx context.Context, db *sql.DB, signer ssh.Signer, choose Chooser, lookup func() (*authn.Authed, error)) (zero Resolution, err error) {
	vcsuri := strings.TrimSpace(t.VCSURI)

	if h, cause := remembered(ctx, db, signer, t); cause != nil {
		errorsx.Log(cause)
	} else if h != nil {
		return Resolution{
			Identity: &authn.Identity{Id: h.ID, Uid: h.UID, Uidraw: h.UIDRaw},
			Current:  &authn.Authn{Account: &authn.Account{Id: h.AccountID}, Profile: &authn.Profile{Id: h.ProfileID, AccountId: h.AccountID}},
			Source:   t.Source,
		}, nil
	}

	profiles, err := lookup()
	if err != nil {
		return zero, err
	}

	switch len(profiles.Profiles) {
	case 0:
		return zero, fmt.Errorf("identity %s is not associated with any profiles", profiles.Identity.GetId())
	case 1:
		p := profiles.Profiles[0]
		errorsx.Log(Remember(ctx, db, signer, vcsuri, profiles.Identity, p, t.Default))
		return Resolution{Identity: profiles.Identity, Current: p, Source: t.Source}, nil
	}

	if choose == nil || !termx.IsInteractive() {
		return Resolution{Identity: profiles.Identity, Candidates: profiles.Profiles}, errorsx.Wrapf(ErrAmbiguous, "identity %s has %d candidate profiles, specify one with --account-id or EG_ACCOUNT", profiles.Identity.GetId(), len(profiles.Profiles))
	}

	p, err := choose(profiles.Profiles)
	if err != nil {
		return zero, err
	}
	errorsx.Log(Remember(ctx, db, signer, vcsuri, profiles.Identity, p, t.Default))
	return Resolution{Identity: profiles.Identity, Current: p, Source: SourceManual}, nil
}

// Profiles lists every profile associated with the signer's key, narrowed by the server to the account and profile
// when they are provided.
func Profiles(ctx context.Context, c *http.Client, signer ssh.Signer, account string, profile string) (authed *authn.Authed, err error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c)

	token, err := authn.AutoRefreshToken(ctx, signer)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to generate refresh token")
	}

	cfg := authn.OAuth2SSHConfig(signer, "", authn.EndpointSSHAuth())
	return profiles(ctx, cfg.Client(ctx, token), account, profile)
}

func profiles(ctx context.Context, oauthc *http.Client, account string, profile string) (authed *authn.Authed, err error) {
	var options io.Reader
	if !stringsx.Blank(account) || !stringsx.Blank(profile) {
		token, err := authn.LoginOptionsToken(ctx, oauthc, authn.LoginOptionAccountID(account), authn.LoginOptionProfileID(profile))
		if err != nil {
			return nil, errorsx.Wrap(err, "unable to fetch login options")
		}
		options = bytes.NewReader(token)
	}

	authed = &authn.Authed{}
	if err = authn.ExchangeAuthed(ctx, oauthc, fmt.Sprintf("%s/authn/ssh", eg.EnvAPIHostDefault()), options, authed); err != nil {
		return nil, errorsx.Wrap(err, "unable to lookup profiles")
	}

	return authed, nil
}
