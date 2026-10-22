package daemons

import (
	"context"
	"io"
	"log"
	"net"

	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/libp2px/webrtcp2p"
	"github.com/egdaemon/eg/internal/netx"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"

	dht "github.com/libp2p/go-libp2p-kad-dht"

	rhost "github.com/libp2p/go-libp2p/p2p/host/routed"

	logging "github.com/ipfs/go-log/v2"

	ds "github.com/ipfs/go-datastore"
	dsync "github.com/ipfs/go-datastore/sync"
)

// P2PProxy creates the p2p host that proxies incoming streams to the local http listener.
func P2PProxy(ctx context.Context, seed []byte, httpl net.Listener) (zero host.Host, err error) {
	return P2PProxyGated(ctx, seed, httpl, passthrough{})
}

// P2PProxyGated is P2PProxy that consults the gate for every incoming proxy stream,
// the gate must not be nil.
func P2PProxyGated(ctx context.Context, seed []byte, httpl net.Listener, gate Gate) (zero host.Host, err error) {
	logging.SetAllLoggers(logging.LevelError)
	priv, _, err := crypto.GenerateEd25519Key(cryptox.NewChaCha8(seed))
	if err != nil {
		return zero, errorsx.Wrap(err, "unable to generate p2p credentials")
	}

	self := errorsx.Must(libp2p.New(
		libp2p.ListenAddrStrings(
			"/ip4/0.0.0.0/tcp/0",
			"/ip6/::/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic-v1",
			"/ip6/::/udp/0/quic-v1",
		),
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

	// go timex.Every(10*time.Minute, func() {
	// 	libp2px.SampledPeers(p2p)
	// })

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

	// proxy serves one stream, either from a libp2p stream or from a webrtc data channel.
	proxy := func(gate Gate, remote peer.ID, addr multiaddr.Multiaddr, s io.ReadWriteCloser) {
		defer s.Close()

		if !gate.Allow(ctx, remote, addr) {
			log.Println("rejecting stream from disabled peer", remote)
			if r, ok := s.(interface{ Reset() error }); ok {
				r.Reset()
			}
			return
		}

		dstaddr := httpl.Addr()
		dst, err := net.Dial(dstaddr.Network(), dstaddr.String())
		if err != nil {
			log.Println("unable to dial destination", dstaddr.Network(), dstaddr.String(), err)
			return
		}
		defer dst.Close()

		if err = netx.Proxy(ctx, s, dst); err != nil {
			log.Println("proxy ended", remote, err)
			return
		}
	}

	p2p.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
		log.Println("received new stream from", s.Conn().RemotePeer())
		proxy(gate, s.Conn().RemotePeer(), s.Conn().RemoteMultiaddr(), s)
	})

	// browsers cannot dial runners directly, they reach them through a relay and then
	// negotiate a webrtc connection, see webrtcp2p. these are not lan peers, so they bypass the
	// lan gate, the logs token is what authorizes them.
	p2p.SetStreamHandler(webrtcp2p.SignalProtocol, webrtcp2p.New(ctx, nil, map[protocol.ID]webrtcp2p.StreamHandler{
		"/egdaemon/proxy": func(remote peer.ID, addr multiaddr.Multiaddr, s io.ReadWriteCloser) {
			proxy(passthrough{}, remote, addr, s)
		},
	}).Handle)

	// go libp2px.DebugEvents(p2p)
	log.Println("p2p identity", p2p.ID(), libp2px.Address(p2p))

	return p2p, nil
}
