package cmdme

import (
	"charm.land/huh/v2"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/huhx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/me"
	"log"
)

type Edit struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Display     string `name:"display" help:"new display name, prompted for when neither --display nor --email is provided"`
	Email       string `name:"email" help:"new email address"`
}

func (t Edit) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := cmdidentity.AccountID(ident.Resolve(tlsc.DefaultClient(), signer, t.VcsURI, false))
	if err != nil {
		return err
	}

	// the api replaces the display and email with what is sent, anything not specified keeps its current value.
	authed, err := identity.Profiles(gctx.Context, tlsc.DefaultClient(), signer)
	if err != nil {
		return err
	}

	current := &authn.Profile{}
	if p, ok := slicesx.Find(func(v *authn.Authn) bool { return v.GetProfile().GetAccountId() == aid }, authed.Profiles...); ok {
		current = p.GetProfile()
	}

	display, email := stringsx.First(t.Display, current.GetDisplay()), stringsx.First(t.Email, current.GetEmail())

	// nothing specified, prompt with the current values.
	if stringsx.Blank(t.Display) && stringsx.Blank(t.Email) {
		form := huh.NewForm(huh.NewGroup(
			huh.NewInput().Title("display name").Value(&display),
			huh.NewInput().Title("email").Value(&email),
		))

		if err = huhx.Prompter().NoTTY(errorsx.New("nothing to edit, specify --display or --email when not running interactively")).Run(form); err != nil {
			return err
		}
	}

	c := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource, authn.OptionEndpoint(authn.EndpointMetaAuthz()))

	p, err := me.UpdateProfile(gctx.Context, c, display, email)
	if err != nil {
		return err
	}

	log.Println("updated", cmdidentity.TypographyProfileCompact(p), p.GetEmail())

	return nil
}
