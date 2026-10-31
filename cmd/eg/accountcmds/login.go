package accountcmds

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type Login struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Seed       string `name:"seed" help:"seed for generating determistic credentials, useful for ci/cd platforms" default:"${vars_entropy_seed}"`
	Account    string `name:"account" help:"optional account id to login with, disambiguates when the ssh key is associated with multiple profiles" env:"EG_ACCOUNT"`
}

func (t Login) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
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

	return t.run(ctx, langx.FirstNonZero(t.Account, gctx.AccountID), cfg.Client(ctx, refreshtoken), chttp, signer, identity.Choose)
}

func (t Login) run(ctx context.Context, account string, oauthc *http.Client, chttp *http.Client, signer ssh.Signer, choose func([]*authn.Authn) (*authn.Authn, error)) (err error) {
	var (
		authed  authn.Authed
		options io.Reader
	)

	if !stringsx.Blank(account) {
		token, err := authn.LoginOptionsToken(ctx, oauthc, account)
		if err != nil {
			return errorsx.Wrap(err, "unable to fetch login options")
		}
		options = bytes.NewReader(token)
	}

	if err = loginssh(ctx, oauthc, options, &authed); err != nil {
		return err
	}

	debugx.Println("authed", spew.Sdump(&authed))

	switch len(authed.Profiles) {
	case 0:
		return errorsx.Notification(fmt.Errorf(
			"the ssh key (%s - %s) is not associated with any profiles, unable to login. associate the key with a profile or use the `%s register --help` command to setup a new account",
			t.SSHKeyPath,
			ssh.FingerprintSHA256(signer.PublicKey()),
			stringsx.First(os.Args...),
		))
	case 1:
		return session(ctx, chttp, authed.Profiles[0])
	default:
		profile, err := choose(authed.Profiles)
		if err != nil {
			return err
		}
		return session(ctx, chttp, profile)
	}
}
