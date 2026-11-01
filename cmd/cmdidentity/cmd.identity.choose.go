package cmdidentity

import (
	"charm.land/huh/v2"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/huhx"
)

// Choose presents an interactive picker for selecting among multiple profiles associated with an
// ssh key. requires a tty since huh renders an interactive prompt.
func Choose(prompt *huh.Select[*authn.Authn], profiles ...huh.Option[*authn.Authn]) (*authn.Authn, error) {
	var selected *authn.Authn
	prompt.Options(profiles...).Value(&selected)

	if err := huhx.Prompter().NoTTY(errorsx.Wrapf(identity.ErrAmbiguous, "specify one with --account when not running interactively")).Run(prompt); err != nil {
		return nil, err
	}

	return selected, nil
}

// login chooses the profile to login with when an ssh key is associated with several. held by T, see Login for the
// interactive implementation.
type login func(profiles []*authn.Authn) (*authn.Authn, error)

// Login is the interactive login.
func Login(profiles []*authn.Authn) (*authn.Authn, error) {
	prompt := huh.NewSelect[*authn.Authn]().Title("select the profile to login with")
	return Choose(prompt, huhx.Options(TypographyAuthn, profiles...)...)
}
