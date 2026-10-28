package compute

import (
	"log"
	"net/http"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners"
)

type cancel struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	ID          string `arg:"" name:"id" help:"id of the enqueued workload, printed when it was uploaded"`
}

func (t cancel) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, t.VcsURI)
	if err != nil {
		return err
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource)

	if err = runners.NewPushClient(chttp).Cancel(gctx.Context, t.ID); httpx.IsStatusError(err, http.StatusNotFound) != nil {
		log.Println("workload not found or already completed", t.ID)
		return nil
	} else if err != nil {
		return errorsx.Wrap(err, "unable to cancel workload")
	}

	log.Println("cancelled", t.ID)
	return nil
}
