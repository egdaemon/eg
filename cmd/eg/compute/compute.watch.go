package compute

import (
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/sshx"
)

type watch struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	ID          string `arg:"" name:"id" help:"id of the enqueued workload, printed when it was uploaded"`
}

func (t watch) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := gctx.Identity().Account(tlsc.DefaultClient(), signer, t.VcsURI)
	if err != nil {
		return err
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource)

	return followenqueued(gctx.Context, chttp, t.ID)
}
