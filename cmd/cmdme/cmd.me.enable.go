package cmdme

import (
	"github.com/egdaemon/eg/identity"
	"log"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/me"
)

type Enable struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Profile     string `arg:"" name:"profile-id" help:"id of the profile to enable, see eg me ls"`
}

func (t Enable) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := cmdidentity.AccountID(ident.With(identity.QueryEnvironment(gctx.AccountID, gctx.ProfileID, t.VcsURI)).Resolve(tlsc.DefaultClient(), signer))
	if err != nil {
		return err
	}

	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	p, err := me.EnableProfile(gctx.Context, c, t.Profile)
	if err != nil {
		if me.Forbidden(err) {
			return errorsx.Wrap(err, "the current profile cannot be changed, select a different one with --account-id")
		}

		if me.NotFound(err) {
			return errorsx.Wrap(err, "no profile with that id is associated with your ssh key")
		}

		return err
	}

	log.Println("enabled", cmdidentity.UXProfileFull(p))

	return nil
}
