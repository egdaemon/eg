package actl

import (
	"context"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners/registration"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type AuthorizeAgent struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	ID         string `name:"id" help:"grant authorization to compute" required:""`
	Shared     bool   `name:"shared" help:"this setting is only useful for registering global runners and is a noop everywhere else" hidden:"true"`
}

func (t AuthorizeAgent) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		signer ssh.Signer
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	tokensrc := compute.NewAuthzTokenSource(tlsc.DefaultClient(), signer, authn.EndpointCompute())

	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, tlsc.DefaultClient())
	httpc := oauth2.NewClient(ctx, tokensrc)

	rc := registration.NewRegistrationClient(httpc)
	if _, err = rc.Grant(ctx, &registration.RegistrationGrantRequest{Registration: &registration.Registration{Id: t.ID}, Global: t.Shared}); err != nil {
		return err
	}

	return nil
}
