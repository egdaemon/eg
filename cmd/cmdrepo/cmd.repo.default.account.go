package cmdrepo

import (
	"fmt"
	"log"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
)

type DefaultAccount struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	ID          string `arg:"" optional:"" name:"id" help:"account id to use for this repository, detected when blank"`
}

func (t DefaultAccount) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	vcsuri := t.VcsURI
	if stringsx.Blank(vcsuri) {
		return errorsx.New("unable to detect the repository, run from within a git repository with a remote")
	}

	c := tlscfg.DefaultClient()

	db, err := gctx.Identity().Store()
	if err != nil {
		return err
	}
	defer db.Close()

	if !stringsx.Blank(t.ID) {
		authed, err := identity.Profiles(gctx.Context, c, signer)
		if err != nil {
			return err
		}

		valid := false
		for _, p := range authed.Profiles {
			valid = valid || p.Profile.AccountId == t.ID
		}
		if !valid {
			return fmt.Errorf("the ssh key %s is not associated with account %s", identity.ID(signer), t.ID)
		}

		if err = identity.Remember(gctx.Context, db, signer, vcsuri, t.ID); err != nil {
			return err
		}

		log.Println("account", t.ID, "will be used for", vcsuri)
		return nil
	}

	// detect, ignoring the remembered account. persists the result.
	r, err := identity.Resolve(gctx.Context, db, c, signer, "", vcsuri, identity.Choose, true)
	if err != nil {
		return err
	}

	log.Println("account", r.Account, "-", r.Source, "- will be used for", vcsuri)
	return nil
}
