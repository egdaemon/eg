package cmdme

import (
	"errors"
	"fmt"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/identity"
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

	// the account being acted as. not being able to determine it is not fatal, the listing is still useful.
	var current string
	if r, err := ident.Resolve(c, signer, t.VcsURI, false); err == nil {
		current = r.Account
	} else if ambiguous := (identity.ErrAmbiguous{}); !errors.As(err, &ambiguous) {
		errorsx.Log(errorsx.Wrap(err, "unable to determine the current account"))
	}

	authed, err := identity.Profiles(gctx.Context, c, signer)
	if err != nil {
		return err
	}

	_, _ = lipgloss.Println(render(current, authed.Profiles...))

	return nil
}

// render the current profile followed by a table of every profile. the current profile is
// marked in the table as well as styled so it remains visible without color.
func render(current string, profiles ...*authn.Authn) string {
	var (
		bold    = lipgloss.NewStyle().Bold(true)
		marked  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("2"))
		rows    = make([][]string, 0, len(profiles))
		focused = -1
		summary = "current: unknown, select an account with --account-id"
	)

	for i, p := range profiles {
		mark := ""
		if p.GetProfile().GetAccountId() == current && current != "" {
			mark = "*"
			focused = i
			summary = fmt.Sprintf("current: %v", cmdidentity.TypographyAuthn(p))
		}

		rows = append(rows, []string{
			mark,
			fmt.Sprintf("%v", cmdidentity.TypographyAccountCompact(p.GetAccount())),
			fmt.Sprintf("%v", cmdidentity.TypographyProfileCompact(p.GetProfile())),
			p.GetProfile().GetEmail(),
		})
	}

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		Headers("", "ACCOUNT", "PROFILE", "EMAIL").
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

	return summary + "\n\n" + tbl.String()
}
