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
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
)

type historyCmd struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Query       string `name:"query" help:"only workloads whose description matches"`
	Limit       uint64 `name:"limit" help:"maximum number of workloads to retrieve, the server caps this at 100" default:"8"`
}

func (t historyCmd) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
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

	_, _ = lipgloss.Println(renderHistory(items...))

	return nil
}

// list retrieves a single page of the most recently completed workloads of the account matching the filters.
func (t historyCmd) list(ctx context.Context, c *computeapi.WorkloadClient) (items []*compute.History, err error) {
	resp, err := c.HistorySearch(ctx, &compute.SearchHistoryRequest{Query: t.Query, Limit: t.Limit})
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve history")
	}

	return resp.GetItems(), nil
}

// renderHistory renders the completed workloads as a table.
func renderHistory(items ...*compute.History) string {
	var (
		bold = lipgloss.NewStyle().Bold(true)
		rows = make([][]string, 0, len(items))
	)

	for _, h := range items {
		rows = append(rows, []string{
			fmt.Sprintf("%v", TypographyHistoryResult(h)),
			fmt.Sprintf("%v", TypographyDuration(time.Duration(h.GetDuration()))),
			fmt.Sprintf("%.64v", TypographyHistoryCompact(h)),
			h.GetVcsUri(),
			fmt.Sprintf("%.8v", h.GetVcsCommit()),
			fmt.Sprintf("%v", TypographyTimestamp(h.GetInitiatedAt())),
			fmt.Sprintf("%v", TypographyTimestamp(h.GetCompletedAt())),
		})
	}

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("RESULT", "DURATION", "WORKLOAD", "VCS", "COMMIT", "INITIATED", "COMPLETED").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return bold.Padding(0, 1)
			}
			return lipgloss.NewStyle().Padding(0, 1)
		})

	return fmt.Sprintf("workloads: %d\n\n%s", len(items), tbl.String())
}
