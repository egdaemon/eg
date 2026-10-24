package daemons

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
)

// Gate decides whether a peer is allowed to open a stream to the daemon.
type Gate interface {
	Allow(ctx context.Context, remote peer.ID, addr multiaddr.Multiaddr) bool
}

// NewLANGate records the peers that reach the daemon and refuses the ones an operator disabled
// (see eg compute lan upd --database daemon.db). it is bookkeeping, not an authorization boundary: the
// control plane's token still decides whether a workload is accepted. any failure to read or
// write the record allows the stream.
//
// the database is shared with the rest of the daemon, see migrations.Open.
func NewLANGate(db *sql.DB, ttl time.Duration) Gate {
	return &langate{db: db, ttl: ttl}
}

type langate struct {
	db  *sql.DB
	ttl time.Duration
}

func (t *langate) Allow(ctx context.Context, remote peer.ID, addr multiaddr.Multiaddr) bool {
	// age out the stale peers.
	if err := lan.Purge(ctx, t.db, t.ttl); err != nil {
		log.Println(errorsx.Wrap(err, "unable to purge the lan peers"))
	}

	allowed, err := record(ctx, t.db, remote, addr)
	if err != nil {
		log.Println(errorsx.Wrap(err, "unable to record the lan peer, allowing the stream"))
		return true
	}

	return allowed
}

func record(ctx context.Context, db *sql.DB, remote peer.ID, addr multiaddr.Multiaddr) (allowed bool, err error) {
	var p lan.Peer

	switch err = lan.PeerFindByID(ctx, db, remote.String()).Scan(&p); {
	case err == nil:
		if p.Disabled {
			return false, nil
		}

		return true, lan.PeerTouch(ctx, db, p.PeerID).Scan(&p)
	case errors.Is(err, sql.ErrNoRows):
		p = lan.Peer{PeerID: remote.String(), Description: "connected to this runner"}
		if pub, cause := remote.ExtractPublicKey(); cause == nil {
			p.PublicKey, _ = pub.Raw()
		}

		if addr != nil {
			p.Addrs = lan.EncodeList([]string{addr.String()})
		}

		return true, lan.PeerUpsert(ctx, db, p).Scan(&p)
	default:
		return false, err
	}
}
