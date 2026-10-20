package libp2px

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/zeroconf/v2"
	"github.com/multiformats/go-multiaddr"
)

// MDNSService is the service name advertised/queried for eg runners on the LAN.
const MDNSService = "_eg._udp"

// MDNSLookup performs an mdns scan yielding the peers found.
// injectable so tests can supply canned peers instead of hitting the network.
type MDNSLookup func(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo]

// dnsaddrprefix is how libp2p's mdns service publishes a peer's addresses within TXT records.
const dnsaddrprefix = "dnsaddr="

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

// DefaultMDNSLookup scans the LAN for advertised eg runners for up to timeout, yielding each one the first time
// it is seen. unlike Advertise it only queries; it doesn't announce the caller and needs no libp2p host.
// stopping early ends the scan.
func DefaultMDNSLookup(ctx context.Context, timeout time.Duration) iterx.Seq[peer.AddrInfo] {
	return iterx.New(func(ctx context.Context, yield func(peer.AddrInfo) bool) error {
		ctx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		var (
			seen    = make(map[peer.ID]struct{})
			entries = make(chan *zeroconf.ServiceEntry, 128)
			browsed = make(chan error, 1)
		)

		// browse blocks until the context ends, ending is how we stop scanning, not a failure.
		// it owns the entries channel and closes it when it finishes cleanly, but not when it fails to start.
		go func() { browsed <- zeroconf.Browse(ctx, MDNSService, "local", entries) }()

		// returns false when the consumer is done.
		deliver := func(entry *zeroconf.ServiceEntry) bool {
			for _, info := range addrinfos(entry) {
				if _, found := seen[info.ID]; found {
					continue
				}
				seen[info.ID] = struct{}{}

				if !yield(info) {
					return false
				}
			}

			return true
		}

		for {
			select {
			case entry, ok := <-entries:
				if !ok {
					entries = nil // finished, the result is waiting on browsed.
					continue
				}

				if !deliver(entry) {
					cancel()
					for {
						select { // discard so browsing is never blocked on a consumer that left.
						case <-entries:
						case <-browsed:
							return nil
						}
					}
				}
			case err := <-browsed:
				// take whatever was left buffered.
				for drained := false; entries != nil && !drained; {
					select {
					case entry, ok := <-entries:
						drained = !ok || !deliver(entry)
					default:
						drained = true
					}
				}

				if err != nil && ctx.Err() == nil {
					return errorsx.Wrap(err, "unable to scan for mdns services")
				}

				return nil
			}
		}
	})
}

// addrinfos extracts the peers a service entry advertises, libp2p publishes their addresses within its TXT records.
func addrinfos(entry *zeroconf.ServiceEntry) []peer.AddrInfo {
	addrs := make([]multiaddr.Multiaddr, 0, len(entry.Text))
	for _, txt := range entry.Text {
		raw, ok := strings.CutPrefix(txt, dnsaddrprefix)
		if !ok {
			continue
		}

		addr, err := multiaddr.NewMultiaddr(raw)
		if err != nil {
			continue
		}

		addrs = append(addrs, addr)
	}

	infos, err := peer.AddrInfosFromP2pAddrs(addrs...)
	if err != nil {
		return nil
	}

	return infos
}
