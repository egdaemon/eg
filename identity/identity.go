// Package identity resolves which account an ssh key acts as. keys can be associated with many
// profiles, rather than requiring --account-id the account is detected from the repository
// being worked on and remembered. see Resolve.
package identity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	_ "github.com/duckdb/duckdb-go/v2" // registers the duckdb driver.
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/repositories"
	"github.com/mattn/go-isatty"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

// Source describes how an account was determined.
type Source string

const (
	SourceFlag       Source = "flag"       // explicitly provided, --account-id or EG_ACCOUNT.
	SourceCache      Source = "cached"     // remembered for the repository.
	SourceSingle     Source = "single"     // the ssh key is only associated with one profile.
	SourceRepository Source = "repository" // only one of the key's accounts has the repository.
	SourceManual     Source = "manual"     // chosen interactively.
)

// Chooser selects among profiles, see Choose.
type Chooser func([]*authn.Authn) (*authn.Authn, error)

// ID the identifier of the signer's key, matches the identity reported by `eg repo whoami`.
func ID(signer ssh.Signer) string {
	return md5x.String(ssh.FingerprintSHA256(signer.PublicKey()))
}

// Remembered returns the account remembered for the repository, blank when none.
func Remembered(ctx context.Context, db *sql.DB, signer ssh.Signer, vcsuri string) (account string, err error) {
	var h RepoAccount
	if err = RepoAccountFind(ctx, db, ID(signer), vcsuri).Scan(&h); errors.Is(err, sql.ErrNoRows) {
		return "", nil
	} else if err != nil {
		return "", errorsx.Wrap(err, "unable to lookup remembered account")
	}

	return h.AccountID, nil
}

// Remember the account for the repository.
func Remember(ctx context.Context, db *sql.DB, signer ssh.Signer, vcsuri string, account string) (err error) {
	h := RepoAccount{Identity: ID(signer), VcsURIMd5: md5x.String(vcsuri), VcsURI: vcsuri, AccountID: account}
	return errorsx.Wrap(RepoAccountUpsert(ctx, db, h).Scan(&h), "unable to remember account")
}

// Forget the account remembered for the repository.
func Forget(ctx context.Context, db *sql.DB, signer ssh.Signer, vcsuri string) (err error) {
	return errorsx.Wrap(discard(RepoAccountDelete(ctx, db, ID(signer), vcsuri)), "unable to forget account")
}

func discard(s RepoAccountScanner) (err error) {
	defer func() { err = errorsx.Compact(err, s.Close()) }()
	for s.Next() {
		var h RepoAccount
		if err = s.Scan(&h); err != nil {
			return err
		}
	}
	return s.Err()
}

// Resolution is the outcome of Resolve.
type Resolution struct {
	Account    string
	Source     Source
	Profile    *authn.Authn   // the profile when known.
	Candidates []*authn.Authn // the profiles to choose from when the account is ambiguous.
}

// ErrAmbiguous is returned when the account cannot be determined.
type ErrAmbiguous struct {
	Identity string
}

func (t ErrAmbiguous) Error() string {
	return fmt.Sprintf("multiple profiles are associated with this ssh key; specify one with --account-id: %s", t.Identity)
}

// IsInteractive true when prompting is possible.
func IsInteractive() bool {
	return isatty.IsTerminal(os.Stdin.Fd())
}

// Resolve determines the account to use. in order:
//  1. the explicitly provided account.
//  2. the account remembered for the repository.
//  3. the only profile associated with the ssh key.
//  4. the only account associated with the key that has the repository.
//  5. prompting, when choose is set and the session is interactive; otherwise ErrAmbiguous.
//
// db remembers accounts per repository, see Open, it may be nil. c is the base http client, it is not expected to be authenticated. vcsuri is the canonical uri of
// the repository and may be blank. choose may be nil to fail instead of prompting. refresh ignores
// the remembered account. detected and selected accounts are remembered.
func Resolve(ctx context.Context, db *sql.DB, c *http.Client, signer ssh.Signer, account string, vcsuri string, choose Chooser, refresh bool) (zero Resolution, err error) {
	if !stringsx.Blank(account) {
		return Resolution{Account: account, Source: SourceFlag}, nil
	}

	return resolve(ctx, db, c, signer, vcsuri, choose, refresh, func() (*authn.Authed, error) {
		return Profiles(ctx, c, signer)
	})
}

func resolve(ctx context.Context, db *sql.DB, c *http.Client, signer ssh.Signer, vcsuri string, choose Chooser, refresh bool, lookup func() (*authn.Authed, error)) (zero Resolution, err error) {
	vcsuri = strings.TrimSpace(vcsuri)

	if db != nil && !refresh && vcsuri != "" {
		if account, cause := Remembered(ctx, db, signer, vcsuri); cause != nil {
			errorsx.Log(cause)
		} else if account != "" {
			return Resolution{Account: account, Source: SourceCache}, nil
		}
	}

	profiles, err := lookup()
	if err != nil {
		return zero, err
	}

	remember := func(account string) {
		if db == nil || vcsuri == "" {
			return
		}
		errorsx.Log(Remember(ctx, db, signer, vcsuri, account))
	}

	switch len(profiles.Profiles) {
	case 0:
		return zero, fmt.Errorf("identity %s is not associated with any profiles", profiles.Identity.GetId())
	case 1:
		p := profiles.Profiles[0]
		return Resolution{Account: p.Profile.AccountId, Source: SourceSingle, Profile: p}, nil
	}

	candidates := profiles.Profiles
	if vcsuri != "" {
		if matched := WithRepository(ctx, c, candidates, vcsuri); len(matched) == 1 {
			p := matched[0]
			remember(p.Profile.AccountId)
			return Resolution{Account: p.Profile.AccountId, Source: SourceRepository, Profile: p}, nil
		} else if len(matched) > 1 {
			candidates = matched
		}
	}

	if choose == nil || !IsInteractive() {
		return Resolution{Candidates: candidates}, ErrAmbiguous{Identity: profiles.Identity.GetId()}
	}

	p, err := choose(candidates)
	if err != nil {
		return zero, err
	}
	remember(p.Profile.AccountId)
	return Resolution{Account: p.Profile.AccountId, Source: SourceManual, Profile: p}, nil
}

// Profiles lists every profile associated with the signer's key.
func Profiles(ctx context.Context, c *http.Client, signer ssh.Signer) (authed *authn.Authed, err error) {
	ctx = context.WithValue(ctx, oauth2.HTTPClient, c)

	token, err := authn.AutoRefreshToken(ctx, signer)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to generate refresh token")
	}

	cfg := authn.OAuth2SSHConfig(signer, "", authn.EndpointSSHAuth())
	return profiles(ctx, cfg.Client(ctx, token))
}

func profiles(ctx context.Context, oauthc *http.Client) (authed *authn.Authed, err error) {
	authed = &authn.Authed{}
	if err = authn.ExchangeAuthed(ctx, oauthc, fmt.Sprintf("%s/authn/ssh", eg.EnvAPIHostDefault()), nil, authed); err != nil {
		return nil, errorsx.Wrap(err, "unable to lookup profiles")
	}

	return authed, nil
}

// WithRepository filters the profiles to those whose account has the repository.
// failures are treated as no match.
func WithRepository(ctx context.Context, c *http.Client, profiles []*authn.Authn, vcsuri string) (matched []*authn.Authn) {
	for _, p := range profiles {
		found, err := hasRepository(ctx, c, p, vcsuri)
		if err != nil {
			debugx.Println("unable to lookup repository for profile", p.Profile.GetId(), err)
			continue
		}

		if found {
			matched = append(matched, p)
		}
	}

	return matched
}

func hasRepository(ctx context.Context, c *http.Client, p *authn.Authn, vcsuri string) (_ bool, err error) {
	var (
		resp repositories.SearchResponse
	)

	q := url.Values{"vcs_uri": []string{vcsuri}, "limit": []string{"1"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/repositories/?%s", eg.EnvAPIHostDefault(), q.Encode()), nil)
	if err != nil {
		return false, err
	}
	authn.BearerAuthorization(req, p.Token)

	r, err := httpx.AsError(c.Do(req))
	if err != nil {
		return false, err
	}
	defer httpx.AutoClose(r)

	if err = json.NewDecoder(r.Body).Decode(&resp); err != nil {
		return false, err
	}

	return len(resp.GetItems()) > 0, nil
}

// Choose presents an interactive picker for selecting among multiple profiles associated with an
// ssh key. requires a tty since huh renders an interactive prompt.
func Choose(profiles []*authn.Authn) (*authn.Authn, error) {
	if !IsInteractive() {
		return nil, errorsx.Notification(errors.New(
			"multiple profiles are associated with this ssh key; specify one with --account when not running interactively",
		))
	}

	options := make([]huh.Option[*authn.Authn], 0, len(profiles))
	for _, p := range profiles {
		label := fmt.Sprintf(
			"%s (account: %s)",
			stringsx.DefaultIfBlank(p.Profile.Display, p.Profile.Id),
			p.Profile.AccountId,
		)
		options = append(options, huh.NewOption(label, p))
	}

	var selected *authn.Authn
	prompt := huh.NewSelect[*authn.Authn]().
		Title("select the profile to login with").
		Options(options...).
		Value(&selected)

	if err := prompt.Run(); err != nil {
		return nil, errorsx.WithStack(err)
	}

	return selected, nil
}
