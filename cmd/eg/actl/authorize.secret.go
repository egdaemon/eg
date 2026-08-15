package actl

import (
	"context"
	"net/http"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners/registration"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type AuthorizeSecret struct {
	Seed       string `arg:"" name:"seed" placeholder:"00000000-0000-0000-0000-000000000000"`
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Shared     bool   `name:"shared" help:"this setting is only useful for registering global runners and is a noop everywhere else" hidden:"true"`
}

// Signer derives this command's identity: its own explicit seed, run through
// the injected Entropy once, then through the injected keygen -- exported so
// it can be compared directly against daemon's equivalent in a cohesion test,
// with no network calls involved.
func (t AuthorizeSecret) Signer(entropy cmdopts.Entropy, keygen cmdopts.KeyGenSeeded) (ssh.Signer, error) {
	derived, err := entropy(t.Seed)
	if err != nil {
		return nil, err
	}
	return sshx.SignerFromGenerator(keygen(derived))
}

func (t AuthorizeSecret) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, entropy cmdopts.Entropy, keygen cmdopts.KeyGenSeeded) (err error) {
	var (
		signer ssh.Signer
	)

	if signer, err = t.Signer(entropy, keygen); err != nil {
		return err
	}

	regid := md5x.String(ssh.FingerprintSHA256(signer.PublicKey()))

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	return t.run(gctx.Context, tlsc.DefaultClient(), gctx.AccountID, regid, signer)
}

func (t AuthorizeSecret) run(ctx context.Context, c *http.Client, account, regid string, signer ssh.Signer) (err error) {
	tokensrc := compute.NewAuthzTokenSource(c, signer, authn.EndpointCompute(), account)
	httpc := oauth2.NewClient(context.WithValue(ctx, oauth2.HTTPClient, c), tokensrc)

	rc := registration.NewRegistrationClient(httpc)
	if _, err = rc.Grant(ctx, &registration.RegistrationGrantRequest{Registration: &registration.Registration{Id: regid}, Global: t.Shared}); err != nil {
		return err
	}

	return nil
}
