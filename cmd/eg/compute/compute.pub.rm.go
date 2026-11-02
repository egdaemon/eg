package compute

import (
	"context"
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

type pubRemove struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	IDs         []string `arg:"" name:"id" help:"ids of the runners to remove"`
}

func (t pubRemove) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := cmdidentity.AccountID(ident.Resolve(tlsc.DefaultClient(), signer, t.VcsURI, false))
	if err != nil {
		return err
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource)

	return t.remove(gctx.Context, computeapi.NewPushClient(chttp))
}

// remove removes every requested runner, reporting those that couldn't be removed.
func (t pubRemove) remove(ctx context.Context, c *computeapi.PushClient) (err error) {
	for _, id := range t.IDs {
		if cause := c.RunnersDestroy(ctx, id); httpx.IsStatusError(cause, http.StatusNotFound) != nil {
			err = errorsx.Compact(err, errorsx.Errorf("runner not found: %s", id))
		} else {
			err = errorsx.Compact(err, errorsx.Wrapf(cause, "unable to remove runner: %s", id))
		}
	}

	return err
}
