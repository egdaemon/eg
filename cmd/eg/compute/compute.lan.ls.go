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
	Database    string        `name:"database" help:"path of the database to operate on, defaults to the client's (the systems it can reach). the daemon's (who has reached it) is daemon.db within its cache directory"`
	PeerTTL     time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems are remembered after they were last seen" default:"720h" env:"EG_LAN_PEER_TTL"`
	Scan        bool          `name:"scan" help:"scan the network for systems before listing"`
	ScanTimeout time.Duration `name:"scan-timeout" help:"how long to scan for" default:"3s"`
}

func (t lanList) Run(kctx *kong.Context, gctx *cmdopts.Global, find libp2px.MDNSLookup) (err error) {
	db, err := openlan(gctx.Context, t.Database, t.PeerTTL)
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
