package daemons

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"sync"
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
// (see eg compute lan upd --runner). it is bookkeeping, not an authorization boundary: the
// control plane's token still decides whether a workload is accepted. any failure to read or
// write the record allows the stream.
//
// the database is opened for each stream and closed again, duckdb only allows a single process
// to have a database open, holding it for the life of the daemon would lock out the eg compute lan
// commands.
func NewLANGate(path string, ttl time.Duration) Gate {
	return &langate{path: path, ttl: ttl}
}

type langate struct {
	m    sync.Mutex
	path string
	ttl  time.Duration
}

func (t *langate) Allow(ctx context.Context, remote peer.ID, addr multiaddr.Multiaddr) bool {
	t.m.Lock()
	defer t.m.Unlock()

	db, err := lan.Open(ctx, t.path, t.ttl)
	if err != nil {
		log.Println(errorsx.Wrap(err, "unable to open the lan peers database, allowing the stream"))
		return true
	}
	defer db.Close()

	allowed, err := record(ctx, db, remote, addr)
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
