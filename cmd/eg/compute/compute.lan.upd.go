package compute

import (
	"database/sql"
	"math"
	"time"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/lan"
)

type lanUpdate struct {
	Database    string        `name:"database" help:"path of the database to operate on, defaults to the client's (the systems it can reach). the daemon's (who has reached it) is daemon.db within its cache directory"`
	PeerTTL     time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems are remembered after they were last seen" default:"720h" env:"EG_LAN_PEER_TTL"`
	PeerID      string        `arg:"" name:"peer-id" help:"id of the system to update"`
	Description *string       `name:"description" help:"describe the system"`
	Disable     bool          `name:"disable" help:"never use this system" xor:"state"`
	Enable      bool          `name:"enable" help:"allow using this system again" xor:"state"`
	TTL         time.Duration `name:"ttl" type:"durationinf" help:"how long this system is remembered after it was last seen. 'infinity' to remember it forever" xor:"ttl"`
	ResetTTL    bool          `name:"reset-ttl" help:"remember this system for the default duration" xor:"ttl"`
}

func (t lanUpdate) Run(gctx *cmdopts.Global) (err error) {
	db, err := openlan(gctx.Context, t.Database, t.PeerTTL)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = t.update(gctx, db)
	return err
}

func (t lanUpdate) update(gctx *cmdopts.Global, db *sql.DB) (p lan.Peer, err error) {
	if t.Description == nil && !t.Disable && !t.Enable && t.TTL == 0 && !t.ResetTTL {
		return p, errorsx.String("nothing to update, provide at least one of --description, --disable, --enable, --ttl, --reset-ttl")
	}

	if err = lan.PeerFindByID(gctx.Context, db, t.PeerID).Scan(&p); err != nil {
		return p, errorsx.Wrapf(err, "unable to find system: %s", t.PeerID)
	}

	if t.Description != nil {
		p.Description = *t.Description
	}

	switch {
	case t.Disable:
		p.Disabled = true
	case t.Enable:
		p.Disabled = false
	}

	switch {
	case t.ResetTTL:
		p.ExpiresAfter = 0
	case t.TTL == time.Duration(math.MaxInt):
		p.ExpiresAfter = -1 // never expires.
	case t.TTL != 0:
		p.ExpiresAfter = int64(t.TTL)
	}

	if err = lan.PeerUpdate(gctx.Context, db, p).Scan(&p); err != nil {
		return p, errorsx.Wrapf(err, "unable to update system: %s", t.PeerID)
	}

	return p, nil
}
