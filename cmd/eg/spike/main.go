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
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/gofrs/uuid"
	"github.com/multiformats/go-multiaddr"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
)

func main() {
	log.SetFlags(log.Flags() | log.Lshortfile)
	ctx, done := context.WithCancelCause(context.Background())
	go cmdopts.Cleanup(ctx, done, &sync.WaitGroup{}, func() {
		log.Println("waiting for systems to shutdown")
	}, os.Kill, os.Interrupt)

	priv, public, err := crypto.GenerateEd25519Key(cryptox.NewPRNGSHA512([]byte("client")))
	if err != nil {
		log.Fatalln(err)
	}
	_ = public

	p2p := errorsx.Must(libp2p.New(
		libp2p.ListenAddrStrings("/ip6/::/tcp/0"),
		libp2p.Identity(priv),
	))

	log.Println("DERP DERP CLIENT", p2p.ID(), libp2px.Address(p2p))

	remote := errorsx.Must(multiaddr.NewMultiaddr("/ip6/::1/tcp/42879/p2p/12D3KooWSiCNhJkddYNSv6dretBir5coyaLmp943RYqE44jr4Kkv"))
	info := errorsx.Must(peer.AddrInfoFromP2pAddr(remote))
	p2p.Peerstore().AddAddrs(info.ID, info.Addrs, peerstore.TempAddrTTL)

	s := errorsx.Must(p2p.NewStream(context.Background(), info.ID, "/echo/1.0.0"))
	go func() {
		var buf [1024]byte
		if _, err = io.CopyBuffer(os.Stdout, s, buf[:]); err != nil {
			log.Println("copy failed", err)
		}
	}()

	bo := backoff.Constant(time.Second)
	p := backoff.Chan()
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
