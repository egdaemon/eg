package compute

import (
	"context"
	"fmt"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/cmdtypography"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
)

type lsCmd struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Query       string `name:"query" help:"only workloads whose description matches"`
	OS          string `name:"os" help:"only workloads for this operating system"`
	Arch        string `name:"arch" help:"only workloads for this architecture"`
	Limit       uint64 `name:"limit" help:"page size used when retrieving workloads" default:"100" hidden:"true"`
}

func (t lsCmd) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	// '-' is the escape hatch for removing the default detected repository from the filter.
	if t.VcsURI == "-" {
		t.VcsURI = ""
	}

	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	aid, err := cmdidentity.AccountID(ident.With(identity.QueryVCSAuto(gctx.AccountID, gctx.ProfileID, t.VcsURI)).Resolve(tlsc.DefaultClient(), signer))
	if err != nil {
		return err
	}

	chttp := authn.New(gctx.Context, tlsc.DefaultClient(), signer, aid, gctx.ProfileID, compute.NewAuthzTokenSource)

	items, err := t.list(gctx.Context, computeapi.NewWorkloadClient(chttp))
	if err != nil {
		return err
	}

	_, _ = lipgloss.Println(renderWorkloads(items...))

	return nil
}

// list retrieves every active workload of the account matching the filters, following the pages.
func (t lsCmd) list(ctx context.Context, c *computeapi.WorkloadClient) (items []*computeapi.Enqueued, err error) {
	req := &computeapi.EnqueuedSearchRequest{Query: t.Query, Os: t.OS, Arch: t.Arch, VcsUri: t.VcsURI, Limit: t.Limit}

	for {
		resp, err := c.Search(ctx, req)
		if err != nil {
			return nil, errorsx.Wrap(err, "unable to retrieve workloads")
		}

		items = append(items, resp.GetItems()...)

		if resp.GetNext().GetOffset() == 0 {
			return items, nil
		}

		req.Offset = resp.GetNext().GetOffset()
	}
}

// renderWorkloads renders the workloads as a table.
func renderWorkloads(items ...*computeapi.Enqueued) string {
	var (
		bold = lipgloss.NewStyle().Bold(true)
		rows = make([][]string, 0, len(items))
	)

	for _, w := range items {
		rows = append(rows, []string{
			fmt.Sprintf("%v", TypographyWorkloadState(w)),
			fmt.Sprintf("%v", TypographyWorkloadQueued(w)),
			fmt.Sprintf("%v", TypographyWorkloadRunning(w)),
			fmt.Sprintf("%.64v", TypographyWorkloadCompact(w)),
			fmt.Sprintf("%v", TypographyWorkloadPlatform(w)),
			w.GetVcsUri(),
			fmt.Sprintf("%.8v", w.GetVcsCommit()),
			fmt.Sprintf("%v", cmdtypography.TimestampRFC3339(w.GetCreatedAt()).Layout(time.DateTime).Zero("").Infinity("", "")),
			fmt.Sprintf("%v", cmdtypography.TimestampRFC3339(w.GetInitiatedAt()).Layout(time.DateTime).Zero("").Infinity("", "")),
		})
	}

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("STATE", "QUEUED", "RUNNING", "WORKLOAD", "OS/ARCH", "VCS", "COMMIT", "CREATED", "STARTED").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return bold.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	return fmt.Sprintf("workloads: %d\n\n%s", len(items), tbl.String())
}
