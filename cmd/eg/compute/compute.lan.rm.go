package compute

import (
	"database/sql"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/lan"
)

type lanRemove struct {
	lanDatabase `embed:""`
	PeerIDs     []string `arg:"" name:"peer-id" help:"ids of the systems to forget"`
}

func (t lanRemove) Run(gctx *cmdopts.Global) (err error) {
	db, err := t.open(gctx)
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
