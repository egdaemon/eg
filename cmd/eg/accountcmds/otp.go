package accountcmds

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type OTP struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t OTP) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	type reqstate struct {
		PublicKey []byte `json:"pkey"`
		Email     string `json:"email"`
		Display   string `json:"display"`
	}

	var (
		signer ssh.Signer
		authed authn.Authed
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	chttp := tlscfg.DefaultClient()
	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, chttp)
	cfg := authn.OAuth2SSHConfig(signer, "", authn.EndpointSSHAuth())

	refreshtoken, err := authn.AutoRefreshToken(ctx, signer)
	if err != nil {
		return errorsx.WithStack(err)
	}
	if err = loginssh(ctx, cfg.Client(ctx, refreshtoken), &authed); err != nil {
		return err
	}

	debugx.Println("authed", spew.Sdump(&authed))

	switch len(authed.Profiles) {
	case 0:
		return errorsx.Notification(fmt.Errorf(
			"the ssh key (%s) is not associated with any profiles, unable to login. associate the key with a profile or use the `%s register --help` command to setup a new account",
			t.SSHKeyPath,
			stringsx.First(os.Args...),
		))
	case 1:
		return otp(ctx, chttp, authed.Profiles[0])
	default:
		return errorsx.Notification(errors.New("you've already registered an account; multiple account support will be implemented in the future"))
	}
}
