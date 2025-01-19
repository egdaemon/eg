package libp2px

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/numericx"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/libp2p/go-libp2p/core/event"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
)

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
			defer log.Println("bootstrapDial", p2p.ID(), p.ID)
			log.Printf("%s bootstrapping to %s", p2p.ID(), p.ID)

			p2p.Peerstore().AddAddrs(p.ID, p.Addrs, peerstore.PermanentAddrTTL)
			if err := p2p.Connect(ctx, p); err != nil {
				log.Println("bootstrapDialFailed", p.ID)
				log.Printf("failed to bootstrap with %v: %s", p.ID, err)
				errs <- err
				return
			}

			log.Println(ctx, "bootstrapDialSuccess", p.ID)
			log.Printf("bootstrapped with %v", p.ID)
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
