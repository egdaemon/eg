package compute

import (
	"database/sql"
	"io"
	"time"

	"github.com/alecthomas/kong"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/libp2px"
)

type lanList struct {
	lanDatabase `embed:""`
	Scan        bool          `name:"scan" help:"scan the network for systems before listing"`
	ScanTimeout time.Duration `name:"scan-timeout" help:"how long to scan for" default:"3s"`
}

func (t lanList) Run(kctx *kong.Context, gctx *cmdopts.Global, find libp2px.MDNSLookup) (err error) {
	db, err := t.open(gctx)
	if err != nil {
		return err
	}
	defer db.Close()

	return t.list(gctx, db, find, kctx.Stdout)
}

// list the remembered systems, scanning for new ones first when requested.
func (t lanList) list(gctx *cmdopts.Global, db *sql.DB, find libp2px.MDNSLookup, out io.Writer) (err error) {
	if t.Scan {
		if err = lan.Discover(gctx.Context, db, find, t.ScanTimeout); err != nil {
			return err
		}
	}

	return lanTable(gctx.Context, db, t.PeerTTL, out)
}
