package compute

import (
	"database/sql"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/lan"
)

type lanRemove struct {
	Database string        `name:"database" help:"path of the database to operate on, defaults to the client's (the systems it can reach). the daemon's (who has reached it) is daemon.db within its cache directory"`
	PeerTTL  time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems are remembered after they were last seen" default:"720h" env:"EG_LAN_PEER_TTL"`
	PeerIDs  []string      `arg:"" name:"peer-id" help:"ids of the systems to forget"`
}

func (t lanRemove) Run(gctx *cmdopts.Global) (err error) {
	db, err := openlan(gctx.Context, t.Database, t.PeerTTL)
	if err != nil {
		return err
	}
	defer db.Close()

	return t.remove(gctx, db)
}

// remove forgets every requested system, reporting those that were not remembered.
func (t lanRemove) remove(gctx *cmdopts.Global, db *sql.DB) (err error) {
	for _, id := range t.PeerIDs {
		var p lan.Peer
		err = errorsx.Compact(err, errorsx.Wrapf(lan.PeerDeleteByID(gctx.Context, db, id).Scan(&p), "unable to remove system: %s", id))
	}

	return err
}
