package libp2px

import (
	"context"

	"github.com/egdaemon/eg/internal/errorsx"
	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"
)

// NewClient creates an ephemeral, dial only, p2p host that routes to peers via
// the dht seeded by the given bootstrap peers.
func NewClient(ctx context.Context, bootstrap ...peer.AddrInfo) (_ host.Host, err error) {
	self, err := libp2p.New(
		libp2p.NoListenAddrs,
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create p2p host")
	}

	ldht, err := dht.New(
		ctx,
		self,
		dht.Datastore(dsync.MutexWrap(ds.NewMapDatastore())),
		dht.Mode(dht.ModeClient),
		dht.BootstrapPeers(bootstrap...),
	)
	if err != nil {
		errorsx.Log(self.Close())
		return nil, errorsx.Wrap(err, "unable to setup dht")
	}

	p2p := rhost.Wrap(self, ldht)

	if err = Connect(ctx, p2p, bootstrap...); err != nil {
		errorsx.Log(p2p.Close())
		return nil, err
	}

	if err = ldht.Bootstrap(ctx); err != nil {
		errorsx.Log(p2p.Close())
		return nil, errorsx.Wrap(err, "unable to bootstrap dht")
	}

	return p2p, nil
}
