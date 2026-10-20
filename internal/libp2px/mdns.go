package libp2px

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
)

// MDNSService is the service name advertised/queried for eg runners on the LAN.
const MDNSService = "_eg._udp"

// MDNSLookup performs an mdns scan using the given host, returning the peers found.
// injectable so tests can supply canned peers instead of hitting the network.
type MDNSLookup func(ctx context.Context, p2p host.Host, timeout time.Duration) ([]peer.AddrInfo, error)

type notifee func(peer.AddrInfo)

func (t notifee) HandlePeerFound(p peer.AddrInfo) { t(p) }

// Advertise announces the host's peer id and listen addresses on the LAN. the peer id is the
// host's public key so nothing further needs to be published for the peer to be authenticated
// once dialed. closing the returned closer stops the announcement.
func Advertise(p2p host.Host) (io.Closer, error) {
	svc := mdns.NewMdnsService(p2p, MDNSService, notifee(func(peer.AddrInfo) {}))
	if err := svc.Start(); err != nil {
		return nil, errorsx.Wrap(err, "unable to start mdns service")
	}

	return svc, nil
}

// DefaultMDNSLookup scans the LAN for eg runners for up to timeout, returning everyone found.
func DefaultMDNSLookup(ctx context.Context, p2p host.Host, timeout time.Duration) ([]peer.AddrInfo, error) {
	var (
		m     sync.Mutex
		found = make(map[peer.ID]peer.AddrInfo)
	)

	svc := mdns.NewMdnsService(p2p, MDNSService, notifee(func(p peer.AddrInfo) {
		m.Lock()
		defer m.Unlock()
		found[p.ID] = p
	}))

	if err := svc.Start(); err != nil {
		return nil, errorsx.Wrap(err, "unable to start mdns scan")
	}
	defer func() { errorsx.Log(svc.Close()) }()

	select {
	case <-time.After(timeout):
	case <-ctx.Done():
	}

	m.Lock()
	defer m.Unlock()

	results := make([]peer.AddrInfo, 0, len(found))
	for _, p := range found {
		results = append(results, p)
	}

	return results, nil
}
