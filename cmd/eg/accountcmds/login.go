package accountcmds

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/charmbracelet/huh"
	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/mattn/go-isatty"
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

	chttp := httpx.BindRetryTransport(tlscfg.DefaultClient(), http.StatusTooManyRequests, http.StatusBadGateway)
	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, "", authn.EndpointSSHAuth())

	refreshtoken, err := authn.AutoRefreshToken(ctx, signer)
	if err != nil {
		return errorsx.WithStack(err)
	}

	return t.run(ctx, langx.FirstNonZero(t.Account, gctx.AccountID), cfg.Client(ctx, refreshtoken), chttp, signer, choosehuh)
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

// choosehuh presents an interactive picker for selecting among multiple
// profiles associated with an ssh key. requires a tty since huh renders
// an interactive prompt.
func choosehuh(profiles []*authn.Authn) (*authn.Authn, error) {
	if !isatty.IsTerminal(os.Stdin.Fd()) {
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
