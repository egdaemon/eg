package cmdrepo

import (
	"fmt"
	"log"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
)

type DefaultAccount struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	ID          string `arg:"" optional:"" name:"id" help:"account id to use for this repository, detected when blank"`
}

func (t DefaultAccount) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	vcsuri := t.VcsURI
	if stringsx.Blank(vcsuri) {
		return errorsx.New("unable to detect the repository, run from within a git repository with a remote")
	}

	c := tlscfg.DefaultClient()

	db, err := ident.Store()
	if err != nil {
		return err
	}
	defer db.Close()

	if !stringsx.Blank(t.ID) {
		authed, err := identity.Profiles(gctx.Context, c, signer, "", "")
		if err != nil {
			return err
		}

		selected, valid := slicesx.Find(func(v *authn.Authn) bool { return v.GetProfile().GetAccountId() == t.ID }, authed.Profiles...)
		if !valid {
			return fmt.Errorf("the ssh key %s is not associated with account %s", identity.ID(signer.PublicKey()), t.ID)
		}

		if err = identity.Remember(gctx.Context, db, signer, vcsuri, authed.Identity, selected, true); err != nil {
			return err
		}

		log.Println("account", t.ID, "will be used for", vcsuri)
		return nil
	}

	// detect again, the remembered identities are forgotten first. persists the result.
	if err = identity.Forget(gctx.Context, db, signer, vcsuri); err != nil {
		return err
	}

	r, err := identity.QueryVCSAuto("", "", vcsuri).Resolve(gctx.Context, db, c, signer, ident.Choose)
	if err != nil {
		return err
	}

	log.Println("account", r.Current.Account.Id, "-", r.Source, "- will be used for", vcsuri)
	return nil
}
