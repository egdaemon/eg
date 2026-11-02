package cmdaccounts

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net/http"

	"charm.land/huh/v2"
	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/huhx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type Signup struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Seed       string `name:"seed" help:"seed for generating determistic credentials, useful for ci/cd platforms" default:"${vars_ssh_key_seed}"`
	Endpoint   string `name:"endpoint" help:"specify the endpoint to connect to" default:"${vars_console_endpoint}" hidden:"true"`
	Account    string `name:"account" help:"optional account id or login domain of the account you want to register with, the login domain can be found at https://console.egdaemon.com/s/settings"`
	Name       string `name:"name" help:"display name for the profile, defaults to the git user.name then the current user" default:"${vars_user_name}"`
	Email      string `name:"email" help:"contact email for the profile, defaults to the git user.email then the current user at noreply.egdaemon.com" default:"${vars_user_email}"`
}

func (t Signup) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	var (
		signer       ssh.Signer
		state        string
		refreshtoken *oauth2.Token
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGenSeeded(t.Seed), t.SSHKeyPath); err != nil {
		return err
	}

	chttp := tlscfg.DefaultClient()
	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, "", authn.EndpointSSHAuth())

	if state, err = authn.AutoTokenStateIdentity(signer, t.Name, t.Email); err != nil {
		return err
	}

	if refreshtoken, err = authn.AutoRefreshTokenState(ctx, signer, state); err != nil {
		return errorsx.WithStack(err)
	}

	return t.run(ctx, gctx, cfg.Client(ctx, refreshtoken), chttp, signer, cmdidentity.Choose)
}

func (t Signup) run(ctx context.Context, gctx *cmdopts.Global, oauthc *http.Client, chttp *http.Client, signer ssh.Signer, choose func(*huh.Select[*authn.Authn], ...huh.Option[*authn.Authn]) (*authn.Authn, error)) (err error) {
	var (
		authed authn.Authed
	)

	// the account flag is an account id or the login domain of an account, the global account is always an id.
	options, err := authn.LoginOptionsFrom(t.Account, authn.LoginOptionAccountID(gctx.AccountID))
	if err != nil {
		return err
	}

	token, err := authn.LoginOptionsToken(ctx, oauthc, options...)
	if err != nil {
		return errorsx.Wrap(err, "unable to fetch login options")
	}

	if err = loginssh(ctx, oauthc, bytes.NewReader(token), &authed); err != nil {
		return err
	}

	opts, err := authn.DecodeLoginOptions(token)
	if err != nil {
		return err
	}

	create := func() error {
		// the server resolved no sso domain, signup for a new account.
		if stringsx.Blank(opts.ID) {
			return sshsession(ctx, chttp, signer, authn.EndpointSignup(), "")
		}

		// otherwise join the account, the issuer is the sso domain the server resolved.
		if err := sshsession(ctx, chttp, signer, authn.EndpointRegister(), opts.ID); err != nil {
			return err
		}

		fmt.Printf("requested access to %s, an administrator of the account needs to approve it before you can login. manage it at %s\n", t.Account, t.Endpoint)
		return nil
	}

	debugx.Fn(func() {
		log.Println("-------------------------------------------------------------------")
		log.Println("REGISTERING WITH", t.SSHKeyPath, identity.ID(signer.PublicKey()))
		log.Println("authed", spew.Sdump(&authed))
		log.Println("profiles", len(authed.Profiles))
		log.Println("-------------------------------------------------------------------")
	})

	switch len(authed.Profiles) {
	case 0:
		return create()
	case 1:
		// we only auto login if opts.AccountID is present it means the
		// user explicitly requested an account to register with.
		if stringsx.Present(opts.AccountID) {
			return session(ctx, chttp, authed.Profiles[0])
		}
	default:
	}

	var selected *authn.Authn
	choices := huhx.Default(huh.NewOption[*authn.Authn]("register a new account", nil), huhx.Options(cmdidentity.TypographyAuthn, authed.Profiles...))
	prompt := huh.NewSelect[*authn.Authn]().Title("select an account")

	if selected, err = choose(prompt, choices...); err != nil {
		return errorsx.WithStack(err)
	}

	if selected == nil {
		return create()
	}

	return session(ctx, chttp, selected)
}
