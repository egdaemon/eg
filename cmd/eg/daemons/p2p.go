package daemons

import (
	"context"
	"io"
	"log"
	"net"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/libp2px"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
)

func P2PProxy(ctx context.Context, seed []byte, httpl net.Listener) (zero peer.ID, err error) {
	priv, _, err := crypto.GenerateEd25519Key(cryptox.NewPRNGSHA512(seed))
	if err != nil {
		return zero, errorsx.Wrap(err, "unable to generate p2p credentials")
	}

	// disable p2p proxy connection until we decide to use it.
	if envx.Boolean(true, eg.EnvP2PProxyDisabled) {
		return peer.IDFromPrivateKey(priv)
	}

	p2p := errorsx.Must(libp2p.New(
		libp2p.ListenAddrStrings("/ip6/::/tcp/0"),
		libp2p.Identity(priv),
	))

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

	log.Println("p2p identity", p2p.ID(), libp2px.Address(p2p))

	return p2p.ID(), nil
}
