package actl

import (
	"context"
	"log"
	"net/http"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runners/registration"
	"golang.org/x/crypto/ssh"
)

type AuthorizeSecret struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	SSHSeed    string `name:"sshkeyseed" help:"seed for generating the ssh key used to authenticate" default:"${vars_ssh_key_seed}" env:"${env_eg_ssh_key_seed}"`
	Seed       string `arg:"" name:"seed" placeholder:"00000000-0000-0000-0000-000000000000" required:"" env:"${env_eg_entropy_seed}"`
	Shared     bool   `name:"shared" help:"this setting is only useful for registering global runners and is a noop everywhere else" hidden:"true"`
}

// Signer derives this command's identity: its own explicit seed, run through
// the injected Entropy once, then through the injected keygen -- exported so
// it can be compared directly against daemon's equivalent in a cohesion test,
// with no network calls involved.
func (t AuthorizeSecret) Signer(entropy cmdopts.Entropy, keygen cmdopts.KeyGenSeeded) (ssh.Signer, error) {
	// entropy substitutes a random seed for a blank one, which would register an identity nobody can reproduce.
	if stringsx.Blank(t.Seed) {
		return nil, errorsx.String("a seed is required")
	}

	derived, err := entropy(t.Seed)
	if err != nil {
		return nil, err
	}
	return sshx.SignerFromGenerator(keygen(derived))
}

func (t AuthorizeSecret) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, entropy cmdopts.Entropy, keygen cmdopts.KeyGenSeeded) (err error) {
	var (
		asigner ssh.Signer
		dsigner ssh.Signer
	)

	if dsigner, err = t.Signer(entropy, keygen); err != nil {
		return err
	}

	if asigner, err = sshx.AutoCached(sshx.NewKeyGenSeeded(t.SSHSeed), t.SSHKeyPath); err != nil {
		return err
	}

	return t.run(gctx.Context, tlsc.DefaultClient(), gctx.AccountID, gctx.ProfileID, dsigner.PublicKey(), asigner)
}

func (t AuthorizeSecret) run(ctx context.Context, c *http.Client, account string, profile string, pub ssh.PublicKey, signer ssh.Signer) (err error) {
	httpc := authn.New(ctx, c, signer, account, profile, compute.NewAuthzTokenSource)

	rc := registration.NewRegistrationClient(httpc)
	req := &registration.RegistrationGrantRequest{Registration: &registration.Registration{Id: identity.ID(pub)}, Global: t.Shared}
	resp, err := rc.Grant(ctx, req)
	if err != nil {
		return err
	}

	log.Println("registration", TypographyRegistrationCompact(resp.GetRegistration()))

	return nil
}
