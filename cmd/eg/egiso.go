package main

import (
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/egiso"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/opencontainers/go-digest"
)

type Egiso struct {
	Users egisoCmdUsers `cmd:"" name:"users" help:"list the users and groups of an egiso, for mapping its owners to virtual users"`
}

// egisoCmdUsers lists every id an egiso defines or that owns its files, and the virtual user the
// default owner map exposes it as.
type egisoCmdUsers struct {
	Location string `arg:"" help:"path or https URL of the egiso"`
	TOC      string `name:"toc" help:"digest of the egiso's table of contents" required:""`
}

func (t egisoCmdUsers) Run(gctx *cmdopts.Global) (err error) {
	var (
		toc     digest.Digest
		archive *egiso.Archive
		users   []egiso.Account
		groups  []egiso.Account
	)

	if toc, err = digest.Parse(t.TOC); err != nil {
		return errorsx.Wrap(err, "invalid --toc")
	}

	if archive, err = egiso.Open(gctx.Context, t.Location, toc); err != nil {
		return err
	}
	defer archive.Close()

	if users, groups, err = archive.Accounts(); err != nil {
		return err
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	accounts(w, "UID", users)
	fmt.Fprintln(w)
	accounts(w, "GID", groups)
	return w.Flush()
}

// accounts prints one row per id, with the virtual user the default owner map exposes it as.
func accounts(w io.Writer, column string, ids []egiso.Account) {
	owners := egiso.NewOwners()
	fmt.Fprintf(w, "%s\tNAME\tFILES\tEXPOSED AS\n", column)
	for _, a := range ids {
		fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", a.ID, a.Name, a.Files, owners.Lookup(a.ID))
	}
}
