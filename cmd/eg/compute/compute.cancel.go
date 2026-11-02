package compute

import (
	"github.com/egdaemon/eg/identity"
	"log"
	"net/http"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/sshx"
)

type cancel struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	ID          string `arg:"" name:"id" help:"id of the enqueued workload, printed when it was uploaded"`
}

func (t cancel) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := cmdidentity.AccountID(ident.With(identity.QueryVCSAuto(gctx.AccountID, gctx.ProfileID, t.VcsURI)).Resolve(tlsc.DefaultClient(), signer))
	if err != nil {
		return err
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource)

	if err = computeapi.NewPushClient(chttp).Cancel(gctx.Context, t.ID); httpx.IsStatusError(err, http.StatusNotFound) != nil {
		log.Println("workload not found or already completed", t.ID)
		return nil
	} else if err != nil {
		return errorsx.Wrap(err, "unable to cancel workload")
	}

	log.Println("cancelled", t.ID)
	return nil
}
