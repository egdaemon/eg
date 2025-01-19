package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"sync"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/gofrs/uuid"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"

	dht "github.com/libp2p/go-libp2p-kad-dht"

	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"

	logging "github.com/ipfs/go-log/v2"
)

// https://github.com/libp2p/go-libp2p/tree/master/examples/routed-echo
// DERP0="client1" DERP1="/ip6/::/tcp/10000" DERP2="/ip6/::1/tcp/10001/p2p/12D3KooWBYpVUpJzA93DV4tG1fWKoZcPHd39KaySA2hNc5r1jN1u" go run ./cmd/eg/spike/...
// DERP0="client2" DERP1="/ip6/::/tcp/10001" DERP2="/ip6/::1/tcp/10000/p2p/12D3KooWRFWGYNCzvE3sv5cQ7Vuw7hXc78TR19UuGVb2BVKr9T7A" go run ./cmd/eg/spike/...

func main() {
	const rendezvous = "00000000-0000-0000-0000-000000000001"
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithCancelCause(context.Background())
	go cmdopts.Cleanup(ctx, done, &sync.WaitGroup{}, func() {
		log.Println("waiting for systems to shutdown")
	}, os.Kill, os.Interrupt)

	priv, _, err := crypto.GenerateEd25519Key(cryptox.NewPRNGSHA512([]byte(envx.String("", "DERP0"))))
	if err != nil {
		log.Fatalln(err)
	}

	bootstrap1 := errorsx.Must(peer.AddrInfoFromString("/ip6/::1/tcp/8090/p2p/12D3KooWGLug1kTX1EdzM2tMoyFdipe6nSqvxFYxNTZ99FxBwREK"))
	bootstrap2 := errorsx.Must(peer.AddrInfoFromString(envx.String("", "DERP2")))

	self := errorsx.Must(libp2p.New(
		libp2p.NoListenAddrs,
		// libp2p.ListenAddrStrings(envx.String("/ip6/::/tcp/0", "DERP1")),
		libp2p.Identity(priv),
		libp2p.DefaultTransports,
		libp2p.DefaultMuxers,
		libp2p.DefaultSecurity,
		libp2p.NATPortMap(),
		libp2p.EnableNATService(),
		libp2p.EnableAutoNATv2(),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(),
	))

	logging.SetAllLoggers(logging.LevelInfo)

	// Construct a datastore (needed by the DHT). This is just a simple, in-memory thread-safe datastore.
	dstore := dsync.MutexWrap(ds.NewLogDatastore(ds.NewMapDatastore(), "dhtstore"))

	// Make the DHT
	ldht := dht.NewDHT(ctx, self, dstore)

	// Make the routed host
	p2p := rhost.Wrap(self, ldht)

	if err = libp2px.Connect(ctx, p2p, *bootstrap1, *bootstrap2); err != nil {
		log.Fatalln(err)
	}

	// agent:  12D3KooWSiCNhJkddYNSv6dretBir5coyaLmp943RYqE44jr4Kkv
	// test1:  12D3KooWRFWGYNCzvE3sv5cQ7Vuw7hXc78TR19UuGVb2BVKr9T7A
	// test2:  12D3KooWRFWGYNCzvE3sv5cQ7Vuw7hXc78TR19UuGVb2BVKr9T7A
	// daemon: 12D3KooWGLug1kTX1EdzM2tMoyFdipe6nSqvxFYxNTZ99FxBwREK
	// rendez: 00000000-0000-0000-0000-000000000001
	log.Println("DERP DERP CLIENT", p2p.ID(), libp2px.Address(p2p), rendezvous)

	// disco := drouting.NewRoutingDiscovery(ldht)
	// dutil.Advertise(ctx, disco, rendezvous)
	info := errorsx.Must(peer.AddrInfoFromString("/p2p-circuit/p2p/12D3KooWSiCNhJkddYNSv6dretBir5coyaLmp943RYqE44jr4Kkv"))

	go func() {
		for {
			pi, err := ldht.FindPeer(ctx, info.ID)
			if err != nil {
				log.Println("unable to find peer", err)
				time.Sleep(30 * time.Second)
				continue
			}

			log.Println("DERP DERP", info.Addrs, "=>", pi.Addrs)
			time.Sleep(30 * time.Second)
		}
	}()

	go timex.Every(10*time.Second, func() {
		log.Println("peers", len(p2p.Peerstore().Peers()))
		if err = ldht.Bootstrap(ctx); err != nil {
			log.Fatalln(err)
		}
	})

	bo := backoff.Constant(3 * time.Second)
	p := backoff.Chan()

	for {
		if err := ldht.Ping(ctx, info.ID); err == nil {
			break
		} else {
			log.Println("unable to ping", err)
		}

		if _, err := ldht.FindPeer(ctx, info.ID); err == nil {
			break
		} else {
			log.Println("unable to find peer", info.ID, err)
		}

		if _, err := ldht.FindPeer(ctx, bootstrap1.ID); err != nil {
			log.Println("unable to find bootstrap", info.ID, err)
		}

		log.Println("DERP DERP PING FAILED")
		select {
		case <-p.Await(bo):
		case <-ctx.Done():
			return
		}
	}

	s := errorsx.Must(p2p.NewStream(ctx, info.ID, "/echo/1.0.0"))
	go func() {
		var buf [1024]byte
		if _, err = io.CopyBuffer(os.Stdout, s, buf[:]); err != nil {
			log.Println("copy failed", err)
		}
	}()

	for {
		select {
		case <-p.Await(bo):
			msg := fmt.Sprintf("ping %s\n", errorsx.Must(uuid.NewV7()))
			_, _ = s.Write([]byte(msg))
		case <-ctx.Done():
			return
		}
	}
}
