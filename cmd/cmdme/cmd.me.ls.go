package cmdme

import (
	"errors"
	"fmt"
	"slices"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
	"github.com/egdaemon/eg/internal/cmpx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
)

type Ls struct {
	cmdopts.VCS `embed:""`
	SSHKeyPath  string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t Ls) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, ident *cmdidentity.T) (err error) {
	signer, err := sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath)
	if err != nil {
		return err
	}

	c := tlsc.DefaultClient()

	r, err := ident.Resolve(c, signer, t.VcsURI, false)
	if !errors.Is(err, identity.ErrAmbiguous) {
		errorsx.Log(errorsx.Wrap(err, "unable to determine the current account"))
	}

	authed, err := identity.Profiles(gctx.Context, c, signer)
	if err != nil {
		return err
	}

	_, _ = lipgloss.Println(render(authed.Identity, r.Current.GetAccount(), authed.Profiles...))

	return nil
}

// render the current profile followed by a table of every profile. the current profile is
// marked in the table as well as styled so it remains visible without color.
func render(ident *authn.Identity, current *authn.Account, idents ...*authn.Authn) string {
	var (
		bold    = lipgloss.NewStyle().Bold(true)
		marked  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
		rows    = make([][]string, 0, len(idents))
		focused = -1
		session = cmdidentity.UXSessionFull(ident, nil)
	)

	// the current account first, then by profile id.
	iscurrent := func(v *authn.Authn) bool {
		return current.GetId() != "" && v.GetProfile().GetAccountId() == current.GetId()
	}

	order := cmpx.For[*authn.Authn]().Bool(iscurrent).String(func(v *authn.Authn) string { return v.GetProfile().GetId() })
	idents = slices.SortedStableFunc(slices.Values(idents), order.Compare)

	for i, id := range idents {
		mark := ""
		if id.GetProfile().GetAccountId() == current.GetId() && current.GetId() != "" {
			mark = "*"
			focused = i
			session = cmdidentity.UXSessionFull(ident, id)
		}

		rows = append(rows, []string{
			mark,
			id.GetProfile().GetId(),
			fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(id.GetAccount())),
			fmt.Sprintf("%v", cmdidentity.TypographyProfileCompact(id.GetProfile())),
			id.GetProfile().GetEmail(),
		})
	}

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("", "ID", "ACCOUNT", "NAME", "EMAIL").
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return bold.Padding(0, 1)
			case row == focused:
				return marked.Padding(0, 1)
			default:
				return lipgloss.NewStyle().Padding(0, 1)
			}
		})

	return fmt.Sprintf("%v", session) + "\n\n" + tbl.String()
}
