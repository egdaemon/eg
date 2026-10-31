package accountcmds

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"charm.land/huh/v2"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type Signup struct {
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Seed        string `name:"seed" help:"seed for generating determistic credentials, useful for ci/cd platforms" default:"${vars_entropy_seed}"`
	Endpoint    string `name:"endpoint" help:"specify the endpoint to connect to" default:"${vars_console_endpoint}" hidden:"true"`
	Account     string `name:"account" help:"optional name of the account you want to register with can be found at https://console.egdaemon.com/s/settings"`
	Name        string `name:"name" help:"display name for the profile, defaults to the git user.name then the current user" default:"${vars_user_name}"`
	Email       string `name:"email" help:"contact email for the profile, defaults to the git user.email then the current user at noreply.egdaemon.com" default:"${vars_user_email}"`
	AutoBrowser bool   `name:"browser" help:"automatically open browser if possible" default:"true"`
}

func (t Signup) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	var (
		signer       ssh.Signer
		state        string
		authed       authn.Authed
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

	oauthc := cfg.Client(ctx, refreshtoken)

	if err = loginssh(ctx, oauthc, nil, &authed); err != nil {
		return err
	}

	create := func() error {
		if stringsx.Blank(t.Account) {
			return sshsession(ctx, chttp, signer, authn.EndpointSignup(), "")
		}

		// when joining an existing account the sso domain id is the md5 of the lowercased account name.
		if err := sshsession(ctx, chttp, signer, authn.EndpointRegister(), md5x.String(strings.ToLower(t.Account))); err != nil {
			return err
		}

		fmt.Printf("requested access to %s, an administrator of the account needs to approve it before you can login. manage it at %s\n", t.Account, t.Endpoint)
		if t.AutoBrowser {
			errorsx.Log(errorsx.Wrap(exec.CommandContext(gctx.Context, "xdg-open", t.Endpoint).Run(), "unable to automatically open browser"))
		}
		return nil
	}

	switch len(authed.Profiles) {
	case 0:
		return create()
	default:
		if !identity.IsInteractive() {
			if len(authed.Profiles) == 1 {
				return session(ctx, chttp, authed.Profiles[0])
			}
			profile, err := identity.Choose(authed.Profiles)
			if err != nil {
				return err
			}
			return session(ctx, chttp, profile)
		}

		var selected *authn.Authn
		choices := make([]huh.Option[*authn.Authn], 0, len(authed.Profiles)+1)
		choices = append(choices, huh.NewOption[*authn.Authn]("register a new account", nil))
		for _, p := range authed.Profiles {
			choices = append(choices, huh.NewOption(
				fmt.Sprintf("%s (account: %s)", stringsx.DefaultIfBlank(p.Profile.Display, p.Profile.Id), p.Profile.AccountId),
				p,
			))
		}

		if err = huh.NewSelect[*authn.Authn]().Title("select an account").Options(choices...).Value(&selected).Run(); err != nil {
			return errorsx.WithStack(err)
		}

		if selected == nil {
			return create()
		}

		return session(ctx, chttp, selected)
	}
}
