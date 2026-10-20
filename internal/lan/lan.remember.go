package lan

import (
	"context"
	"database/sql"
	"errors"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/libp2p/go-libp2p/core/peer"
)

// Remember records the systems seen on the network. systems already known keep their resources
// and operator settings, only their addresses and last seen change.
func Remember(ctx context.Context, db *sql.DB, found ...peer.AddrInfo) (err error) {
	for _, info := range found {
		var p Peer

		if err = PeerFindByID(ctx, db, info.ID.String()).Scan(&p); errors.Is(err, sql.ErrNoRows) {
			p = Peer{PeerID: info.ID.String(), Description: "discovered via mdns"}
			if pub, cause := info.ID.ExtractPublicKey(); cause == nil {
				p.PublicKey, _ = pub.Raw()
			}
		} else if err != nil {
			return errorsx.Wrap(err, "unable to lookup system")
		}

		addrs := make([]string, 0, len(info.Addrs))
		for _, a := range info.Addrs {
			addrs = append(addrs, a.String())
		}
		p.Addrs = EncodeList(addrs)

		if err = PeerUpsert(ctx, db, p).Scan(&p); err != nil {
			return errorsx.Wrap(err, "unable to remember system")
		}
	}

	return nil
}
