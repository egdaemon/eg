package cmdrepo

import (
	"encoding/base64"
	"errors"
	"log"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/stringsx"
	"golang.org/x/crypto/ssh"
)

type Whoami struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t Whoami) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	log.Println("identity", identity.ID(signer))
	log.Println("fingerprint", ssh.FingerprintSHA256(signer.PublicKey()))
	log.Println("base64", base64.URLEncoding.EncodeToString(signer.PublicKey().Marshal()))

	log.Println("repository", stringsx.DefaultIfBlank(t.VcsURI, "none detected"))

	c := tlscfg.DefaultClient()

	db, err := gctx.Identity().Store()
	if err != nil {
		return err
	}
	defer db.Close()

	// prompts when ambiguous and interactive, the selection is remembered. otherwise the candidates are listed.
	r, err := identity.Resolve(gctx.Context, db, c, signer, gctx.AccountID, t.VcsURI, identity.Choose, false)
	if ambiguous := (identity.ErrAmbiguous{}); errors.As(err, &ambiguous) {
		log.Println("account ambiguous, candidates:")
		for _, p := range r.Candidates {
			logprofile(p.Profile)
		}
		log.Println("select one with `eg repo default account <id>`")
		return nil
	} else if err != nil {
		return err
	}

	log.Println("account", r.Account, "-", r.Source)
	if r.Profile == nil {
		authed, err := identity.Profiles(gctx.Context, c, signer)
		if err != nil {
			return err
		}

		for _, p := range authed.Profiles {
			if p.Profile.AccountId == r.Account {
				r.Profile = p
				break
			}
		}
	}

	if r.Profile != nil {
		logprofile(r.Profile.Profile)
	}

	return nil
}

func logprofile(p *authn.Profile) {
	log.Println("profile", p.Id, "-", stringsx.DefaultIfBlank(p.Display, p.Email), "- account", p.AccountId)
}
