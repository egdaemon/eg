package actl

import (
	"context"
	"net/http"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners/registration"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type AuthorizeManual struct {
	ID         string `arg:"" name:"id" help:"grant authorization to compute" required:""`
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Shared     bool   `name:"shared" help:"this setting is only useful for registering global runners and is a noop everywhere else" hidden:"true"`
}

func (t AuthorizeManual) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		signer ssh.Signer
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	return t.run(gctx.Context, tlsc.DefaultClient(), gctx.AccountID, signer)
}

func (t AuthorizeManual) run(ctx context.Context, c *http.Client, account string, signer ssh.Signer) (err error) {
	tokensrc := compute.NewAuthzTokenSource(c, signer, authn.EndpointCompute(), account)
	httpc := oauth2.NewClient(context.WithValue(ctx, oauth2.HTTPClient, c), tokensrc)

	rc := registration.NewRegistrationClient(httpc)
	if _, err = rc.Grant(ctx, &registration.RegistrationGrantRequest{Registration: &registration.Registration{Id: t.ID}, Global: t.Shared}); err != nil {
		return err
	}

	return nil
}
