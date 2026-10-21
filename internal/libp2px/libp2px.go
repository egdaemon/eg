package libp2px

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/multiformats/go-multiaddr"
)

const (
	reserveTimeout = 30 * time.Second
	// spreads relay reconnects across nodes so a restarted relay isn't stampeded.
	reserveJitterWindow = 30 * time.Second
	// how often the keeper checks for changes to the relays it is given.
	reserveReconcile = 30 * time.Second
)

// tracks the relays a keeper is currently maintaining reservations with.
type keeper struct {
	mu     sync.Mutex
	active map[peer.ID]struct{}
}

// begin reports whether the relay wasn't already being maintained.
func (t *keeper) begin(relay peer.ID) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.active[relay]; ok {
		return false
	}

	if t.active == nil {
		t.active = make(map[peer.ID]struct{})
	}
	t.active[relay] = struct{}{}

	return true
}

func (t *keeper) end(relay peer.ID) {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.active, relay)
}

func (t *keeper) count() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.active)
}

func newRetry(p2p host.Host) backoff.Strategy {
	return backoff.New(
		backoff.Exponential(time.Second),
		backoff.Maximum(time.Minute),
		backoff.JitterRandom(backoff.DynamicHashDuration(reserveJitterWindow, p2p.ID().String())),
	)
}

// KeepReservedFrom maintains relay reservations for the lifetime of ctx, renewing
// ahead of expiry and re-reserving as soon as the connection to a relay is lost.
// it is intended to be called once per host. reservations are kept with each
// relay stored in the returned list, follow a relay's addresses when they
// change, and are dropped once the relay leaves the list.
func KeepReservedFrom(ctx context.Context, p2p host.Host) *atomic.Pointer[[]peer.AddrInfo] {
	relays := new(atomic.Pointer[[]peer.AddrInfo])
	go keepfrom(ctx, p2p, new(keeper), newRetry(p2p), relays, reserveReconcile)
	return relays
}

func keepfrom(ctx context.Context, p2p host.Host, t *keeper, retry backoff.Strategy, relays *atomic.Pointer[[]peer.AddrInfo], every time.Duration) {
	for {
		if current := relays.Load(); current != nil {
			for _, r := range *current {
				t.keepsource(ctx, p2p, retry, r.ID, every, func() (peer.AddrInfo, bool) {
					if current := relays.Load(); current != nil {
						if i := slices.IndexFunc(*current, func(c peer.AddrInfo) bool { return c.ID == r.ID }); i >= 0 {
							return (*current)[i], true
						}
					}
					return peer.AddrInfo{}, false
				})
			}
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(every):
		}
	}
}

// keepsource starts maintaining a reservation with the relay unless one is
// already being maintained. source reports the relay's current addresses, and
// false once the relay should no longer be kept.
func (t *keeper) keepsource(ctx context.Context, p2p host.Host, retry backoff.Strategy, relay peer.ID, poll time.Duration, source func() (peer.AddrInfo, bool)) {
	if !t.begin(relay) {
		return
	}

	go func() {
		defer t.end(relay)
		keepReserved(ctx, p2p, retry, poll, source)
	}()
}

func sameAddrs(a, b peer.AddrInfo) bool {
	return a.ID == b.ID && len(a.Addrs) == len(b.Addrs) && !slices.ContainsFunc(a.Addrs, func(x multiaddr.Multiaddr) bool {
		return !slices.ContainsFunc(b.Addrs, x.Equal)
	})
}

func keepReserved(ctx context.Context, p2p host.Host, retry backoff.Strategy, poll time.Duration, source func() (peer.AddrInfo, bool)) {
	relay, ok := source()
	if !ok {
		return
	}

	sub, err := p2p.EventBus().Subscribe(new(event.EvtPeerConnectednessChanged))
	if err != nil {
		log.Println("unable to watch relay connectivity", relay.ID, err)
		return
	}
	defer sub.Close()

	ticker := time.NewTicker(poll)
	defer ticker.Stop()

	attempt := int64(0)
	wait := time.After(0)

	for {
		select {
		case <-wait:
		case <-ticker.C:
			current, ok := source()
			if !ok {
				debugx.Println("relay no longer known, releasing", relay.ID)
				return
			}

			// pick up new addresses without waiting out the backoff or renewal.
			if !sameAddrs(relay, current) {
				debugx.Println("relay addresses changed, re-reserving", relay.ID)
				relay = current
				attempt = 0
				wait = time.After(0)
			}
			continue
		case evt := <-sub.Out():
			if e, ok := evt.(event.EvtPeerConnectednessChanged); ok && e.Peer == relay.ID && e.Connectedness == network.NotConnected {
				d := retry.Backoff(attempt)
				debugx.Println("relay disconnected, re-reserving", relay.ID, "in", d)
				wait = time.After(d)
			}
			continue
		case <-ctx.Done():
			return
		}

		if current, ok := source(); ok {
			relay = current
		} else {
			debugx.Println("relay no longer known, releasing", relay.ID)
			return
		}

		rctx, done := context.WithTimeout(ctx, reserveTimeout)
		rsvp, err := client.Reserve(rctx, p2p, relay)
		done()
		if err != nil {
			log.Println("unable to reserve relay", relay.ID, err)
			wait = time.After(retry.Backoff(attempt))
			attempt++
			continue
		}

		debugx.Println("reserved relay with", relay.ID, "until", rsvp.Expiration)
		attempt = 0
		wait = time.After(time.Until(rsvp.Expiration) / 2)
	}
}

// Reserve requests a relay reservation from each of the given peers so that
// others can reach this host via circuit addresses through them (see
// CircuitAddrs). reservations expire (1h by default); see KeepReservedFrom
// for keeping them alive.
func Reserve(ctx context.Context, p2p host.Host, relays ...peer.AddrInfo) error {
	if len(relays) < 1 {
		return errors.New("no relay peers")
	}

	errs := make(chan error, len(relays))
	var wg sync.WaitGroup
	for _, r := range relays {
		wg.Go(func(r peer.AddrInfo) func() {
			return func() {
				rsvp, err := client.Reserve(ctx, p2p, r)
				if err != nil {
					debugx.Printf("failed to reserve relay with %v: %s", r.ID, err)
					errs <- err
					return
				}
				debugx.Printf("reserved relay with %v until %s", r.ID, rsvp.Expiration)
			}
		}(r))
	}
	wg.Wait()
	close(errs)

	count := 0
	var err error
	for e := range errs {
		count++
		err = e
	}

	if count == len(relays) {
		return fmt.Errorf("failed to reserve any relay. %s", err)
	}

	return nil
}

// CircuitAddrs returns circuit addresses for reaching a peer through each relay
// this host is directly connected to.
func CircuitAddrs(p2p host.Host) (addrs []multiaddr.Multiaddr) {
	for _, c := range p2p.Network().Conns() {
		if c.Stat().Limited {
			continue
		}

		circuit, err := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s/p2p-circuit", c.RemotePeer()))
		if err != nil {
			continue
		}

		addrs = append(addrs, c.RemoteMultiaddr().Encapsulate(circuit))
	}

	return addrs
}

// RelayCircuitAddrs returns circuit addresses for reaching a peer through the
// given relay addresses (each including the relay's /p2p/ id).
func RelayCircuitAddrs(relays ...string) (addrs []multiaddr.Multiaddr) {
	circuit := multiaddr.StringCast("/p2p-circuit")
	for _, r := range relays {
		a, err := multiaddr.NewMultiaddr(r)
		if err != nil {
			continue
		}

		if _, err := a.ValueForProtocol(multiaddr.P_P2P); err != nil {
			continue
		}

		addrs = append(addrs, a.Encapsulate(circuit))
	}

	return addrs
}

func Address(p2p host.Host) string {
	// Build host multiaddress
	host, _ := multiaddr.NewMultiaddr(fmt.Sprintf("/p2p/%s", p2p.ID()))
	return host.String()
}

func StringsToPeers(addrs ...string) []peer.AddrInfo {
	return slicesx.Filter(func(p peer.AddrInfo) bool {
		return stringsx.Present(p.ID.String())
	}, slicesx.MapTransform(func(s string) peer.AddrInfo {
		return langx.Autoderef(errorsx.Zero(peer.AddrInfoFromString(s)))
	}, addrs...)...)
}

func Connect(ctx context.Context, p2p host.Host, peers ...peer.AddrInfo) error {
	if len(peers) < 1 {
		return errors.New("not enough bootstrap peers")
	}

	errs := make(chan error, len(peers))
	var wg sync.WaitGroup
	for _, p := range peers {
		// performed asynchronously because when performed synchronously, if
		// one `Connect` call hangs, subsequent calls are more likely to
		// fail/abort due to an expiring context.
		// Also, performed asynchronously for dial speed.
		wg.Add(1)
		go func(p peer.AddrInfo) {
			defer wg.Done()
			debugx.Printf("%s bootstrapping with %s", p2p.ID(), p.ID)

			if err := p2p.Connect(ctx, p); err != nil {
				debugx.Printf("failed to bootstrap with %v: %s", p.ID, err)
				errs <- err
				return
			}

			debugx.Printf("bootstrapped with %v", p.ID)
		}(p)
	}
	wg.Wait()

	// our failure condition is when no connection attempt succeeded.
	// So drain the errs channel, counting the results.
	close(errs)
	count := 0
	var err error
	for err = range errs {
		if err != nil {
			count++
		}
	}
	if count == len(peers) {
		return fmt.Errorf("failed to bootstrap. %s", err)
	}
	return nil
}

func DebugEvents(p2p host.Host) {
	sub := errorsx.Must(p2p.EventBus().Subscribe(event.WildcardSubscription))
	defer sub.Close()
	for evt := range sub.Out() {
		log.Printf("p2p event %v\n", evt)
	}
}

func SampledPeers(p2p host.Host) {
	peers := p2p.Peerstore().Peers()
	rand.Shuffle(len(peers), func(i, j int) {
		peers[i], peers[j] = peers[j], peers[i]
	})

	log.Println("peers", len(peers))
	peers = peers[:numericx.Min(4, len(peers))]
	for _, id := range peers {
		log.Println("peer", id, p2p.Peerstore().Addrs(id))
	}
}
