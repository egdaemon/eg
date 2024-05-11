package actl

import (
	"context"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners/registration"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type AuthorizeAgent struct {
	Seed AuthorizeSecret `cmd:"" help:"register a signing secret"`
	ID   AuthorizeManual `cmd:"" help:"register using the id provided by the daemon without knowing the secret"`
}

type AuthorizeSecret struct {
	Seed       string `arg:"" name:"seed" placeholder:"00000000-0000-0000-0000-000000000000"`
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Shared     bool   `name:"shared" help:"this setting is only useful for registering global runners and is a noop everywhere else" hidden:"true"`
}

func (t AuthorizeSecret) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		signer ssh.Signer
	)

	if signer, err = sshx.SignerFromGenerator(sshx.NewKeyGenSeeded(t.Seed)); err != nil {
		return err
	}

	regid := md5x.String(ssh.FingerprintSHA256(signer.PublicKey()))

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	tokensrc := compute.NewAuthzTokenSource(tlsc.DefaultClient(), signer, authn.EndpointCompute())

	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, tlsc.DefaultClient())
	httpc := oauth2.NewClient(ctx, tokensrc)

	rc := registration.NewRegistrationClient(httpc)
	if _, err = rc.Grant(ctx, &registration.RegistrationGrantRequest{Registration: &registration.Registration{Id: regid}, Global: t.Shared}); err != nil {
		return err
	}

	return nil
}

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

	tokensrc := compute.NewAuthzTokenSource(tlsc.DefaultClient(), signer, authn.EndpointCompute())

	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, tlsc.DefaultClient())
	httpc := oauth2.NewClient(ctx, tokensrc)

	rc := registration.NewRegistrationClient(httpc)
	if _, err = rc.Grant(ctx, &registration.RegistrationGrantRequest{Registration: &registration.Registration{Id: t.ID}, Global: t.Shared}); err != nil {
		return err
	}

	return nil
}
