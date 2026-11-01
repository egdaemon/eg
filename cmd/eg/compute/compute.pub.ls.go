package compute

import (
	"context"

	"charm.land/lipgloss/v2"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
)

type pubList struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Query       string `name:"query" help:"only runners whose description matches"`
	OS          string `name:"os" help:"only runners with this operating system"`
	Arch        string `name:"arch" help:"only runners with this architecture"`
	Cores       uint64 `name:"cores" help:"only runners with at least this many cores"`
	Memory      uint64 `name:"memory" help:"only runners with at least this much memory in bytes"`
	Vram        uint64 `name:"vram" help:"only runners with at least this much video memory in bytes"`
}

func (t pubList) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := cmdidentity.AccountID(ident.Resolve(tlsc.DefaultClient(), signer, t.VcsURI, false))
	if err != nil {
		return err
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, compute.NewAuthzTokenSource)

	runners, err := t.list(gctx.Context, computeapi.NewPushClient(chttp))
	if err != nil {
		return err
	}

	_, _ = lipgloss.Println(render(runners...))

	return nil
}

// list retrieves every runner of the account matching the filters, following the pages.
func (t pubList) list(ctx context.Context, c *computeapi.PushClient) (runners []*compute.Compute, err error) {
	var (
		req = &compute.SearchRequest{Query: t.Query, Os: t.OS, Arch: t.Arch, Cores: t.Cores, Memory: t.Memory, Vram: t.Vram, Limit: 100}
	)

	for {
		resp, err := c.RunnersSearch(ctx, req)
		if err != nil {
			return nil, errorsx.Wrap(err, "unable to retrieve runners")
		}

		runners = append(runners, resp.GetItems()...)

		if resp.GetNext().GetOffset() == 0 {
			break
		}

		req.Offset = resp.GetNext().GetOffset()
	}

	return runners, nil
}
