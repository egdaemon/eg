package daemons

import (
	"context"
	"io"
	"log"
	"net"
	"time"

	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/netx"
	"github.com/egdaemon/eg/internal/timex"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"

	dht "github.com/libp2p/go-libp2p-kad-dht"

	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"

	logging "github.com/ipfs/go-log/v2"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
)

func P2PProxy(ctx context.Context, rendezvous string, seed []byte, httpl net.Listener) (zero host.Host, err error) {
	logging.SetAllLoggers(logging.LevelInfo)
	priv, _, err := crypto.GenerateEd25519Key(cryptox.NewPRNGSHA512(seed))
	if err != nil {
		return zero, errorsx.Wrap(err, "unable to generate p2p credentials")
	}

	self := errorsx.Must(libp2p.New(
		libp2p.ListenAddrStrings("/ip6/::/tcp/0"),
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

	// Construct a datastore (needed by the DHT). This is just a simple, in-memory thread-safe datastore.
	dstore := dsync.MutexWrap(ds.NewMapDatastore())

	// Make the DHT
	ldht, err := dht.New(ctx, self, dht.Datastore(dstore), dht.Mode(dht.ModeClient))
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to setup dht")
	}

	// Make the routed host
	p2p := rhost.Wrap(self, ldht)

	if err = ldht.Bootstrap(ctx); err != nil {
		log.Fatalln(err)
	}

	go timex.Every(10*time.Minute, func() {
		libp2px.SampledPeers(p2p)
	})

	// Set a stream handler on host A. /echo/1.0.0 is
	// a user-defined protocol name.
	p2p.SetStreamHandler("/echo/1.0.0", func(s network.Stream) {
		log.Println("listener received new stream from", s.Conn().RemotePeer())
		defer s.Close()
		var buf [1024]byte
		if _, err = io.CopyBuffer(s, s, buf[:]); err != nil {
			s.Reset()
			return
		}
	})

	p2p.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
		log.Println("received new stream from", s.Conn().RemotePeer())
		defer s.Close()

		dstaddr := httpl.Addr()
		dst, err := net.Dial(dstaddr.Network(), dstaddr.String())
		if err != nil {
			log.Println("unable to dial destination", dstaddr.Network(), dstaddr.String(), err)
			return
		}
		defer dst.Close()

		if err = netx.Proxy(ctx, s, dst); err != nil {
			log.Println("proxy ended", s.Conn().RemotePeer(), err)
			return
		}
	})

	// go libp2px.DebugEvents(p2p)
	log.Println("p2p identity", p2p.ID(), libp2px.Address(p2p), rendezvous)

	return p2p, nil
}
