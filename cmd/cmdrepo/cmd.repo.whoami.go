package cmdrepo

import (
	"errors"
	"log"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
)

type Whoami struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t Whoami) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	log.Println("identity", cmdidentity.TypographySSHPublicKey(signer.PublicKey()))
	log.Println("base64", cmdidentity.TypographySSHPublicKeyB64(signer.PublicKey()))
	log.Println("repository", stringsx.DefaultIfBlank(t.VcsURI, "none detected"))

	c := tlscfg.DefaultClient()

	db, err := ident.Store()
	if err != nil {
		return err
	}
	defer db.Close()

	// prompts when ambiguous and interactive, the selection is remembered. otherwise the candidates are listed.
	r, err := identity.Resolve(gctx.Context, db, c, signer, gctx.AccountID, t.VcsURI, ident.Choose, false)
	if ambiguous := (identity.ErrAmbiguous{}); errors.As(err, &ambiguous) {
		log.Println("account ambiguous, candidates:")
		for _, p := range r.Candidates {
			log.Println("profile", cmdidentity.TypographyAuthn(p))
		}
		log.Println("select one with `eg repo default account <id>`")
		return nil
	} else if err != nil {
		return err
	}

	log.Println("account", r.Account, "-", r.Source)

	// resolve only reports the candidates when the account is ambiguous, list everything the key is associated with.
	authed, err := identity.Profiles(gctx.Context, c, signer)
	if err != nil {
		return err
	}

	r.Profile, _ = slicesx.Find(func(v *authn.Authn) bool { return v.Profile.GetAccountId() == r.Account }, authed.Profiles...)

	if r.Profile == nil {
		return nil
	}

	log.Println("current", cmdidentity.TypographyAuthn(r.Profile))

	return nil
}
