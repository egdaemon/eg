package cmdaccounts

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/browserx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type Login struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Seed       string `name:"seed" help:"seed for generating determistic credentials, useful for ci/cd platforms" default:"${vars_ssh_key_seed}"`
	Account    string `name:"account" help:"optional account id or login domain to login with, disambiguates when the ssh key is associated with multiple profiles" env:"EG_ACCOUNT"`
	Browser    bool   `name:"browser" help:"open the console in the default browser once logged in"`
}

func (t Login) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	var (
		signer ssh.Signer
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGenSeeded(t.Seed), t.SSHKeyPath); err != nil {
		return err
	}

	chttp := tlscfg.DefaultClient()
	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, "", authn.EndpointSSHAuth())

	refreshtoken, err := authn.AutoRefreshToken(ctx, signer)
	if err != nil {
		return errorsx.WithStack(err)
	}

	return t.run(ctx, gctx.AccountID, cfg.Client(ctx, refreshtoken), chttp, signer, ident.Choose)
}

func (t Login) run(ctx context.Context, account string, oauthc *http.Client, chttp *http.Client, signer ssh.Signer, choose func([]*authn.Authn) (*authn.Authn, error)) (err error) {
	var (
		authed authn.Authed
	)

	// the account flag is an account id or the login domain of an account, the global account (EG_ACCOUNT) is always an id and always sent.
	options, err := authn.LoginOptionsFrom(t.Account, authn.LoginOptionAccountID(account))
	if err != nil {
		return err
	}

	token, err := authn.LoginOptionsToken(ctx, oauthc, options...)
	if err != nil {
		return errorsx.Wrap(err, "unable to fetch login options")
	}

	if !stringsx.Blank(t.Account) {
		opts, err := authn.DecodeLoginOptions(token)
		if err != nil {
			return err
		}

		// the server ignores unknown login domains, which would otherwise login without the requested account.
		if stringsx.Blank(opts.AccountID) {
			return errorsx.Notification(fmt.Errorf("you requested the account '%s'. None was found.", t.Account))
		}
	}

	if err = loginssh(ctx, oauthc, bytes.NewReader(token), &authed); err != nil {
		return err
	}

	debugx.Println("authed", spew.Sdump(&authed))

	var profile *authn.Authn

	switch len(authed.Profiles) {
	case 0:
		return errorsx.Notification(fmt.Errorf(
			"the ssh key (%s - %s) is not associated with any profiles, unable to login. associate the key with a profile or use the `%s register --help` command to setup a new account",
			t.SSHKeyPath,
			ssh.FingerprintSHA256(signer.PublicKey()),
			stringsx.First(os.Args...),
		))
	case 1:
		profile = authed.Profiles[0]
	default:
		if profile, err = choose(authed.Profiles); err != nil {
			return err
		}
	}

	if t.Browser {
		return browserx.Open(ctx, fmt.Sprintf("%s?lt=%s", eg.EnvConsoleHostDefault(), profile.Token))
	}

	if err = session(ctx, chttp, profile); err != nil {
		return err
	}

	return nil
}
