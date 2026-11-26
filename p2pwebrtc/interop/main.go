// Command interop starts a relay and a runner behind it, both on loopback tcp, and prints the
// relay and the /p2p-circuit/webrtc address of the runner as a json line. the runner echoes
// /p2pwebrtc/test/echo streams. every line read from stdin is the /webrtc address of a remote echo
// server, the runner dials it with Transport.Connect, echoes a payload and prints a json result line.
// it serves until stdin closes. see run.sh.
package main

import (
	"bufio"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/egdaemon/p2pwebrtc"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	ma "github.com/multiformats/go-multiaddr"
)

func main() {
	if os.Getenv("P2PWEBRTC_DEBUG") != "" {
		p2pwebrtc.SetLogger(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})))
	}

	ctx := context.Background()

	r, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
	if err != nil {
		log.Fatalln(err)
	}
	defer r.Close()

	if _, err = relay.New(r); err != nil {
		log.Fatalln(err)
	}

	runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.EnableRelay())
	if err != nil {
		log.Fatalln(err)
	}
	defer runner.Close()

	t, err := p2pwebrtc.Add(runner)
	if err != nil {
		log.Fatalln(err)
	}

	runner.SetStreamHandler("/p2pwebrtc/test/echo", func(s network.Stream) {
		defer s.Close()
		log.Println("echo stream", s.Conn().ConnState().Transport, s.Conn().RemoteMultiaddr())
		io.Copy(s, s)
	})

	rinfo := peer.AddrInfo{ID: r.ID(), Addrs: r.Addrs()}
	if err = runner.Connect(ctx, rinfo); err != nil {
		log.Fatalln(err)
	}
	if _, err = client.Reserve(ctx, runner, rinfo); err != nil {
		log.Fatalln(err)
	}

	relayaddr := r.Addrs()[0].Encapsulate(ma.StringCast("/p2p/" + r.ID().String()))
	target := relayaddr.Encapsulate(ma.StringCast("/p2p-circuit/webrtc/p2p/" + runner.ID().String()))

	out := json.NewEncoder(os.Stdout)
	if err = out.Encode(map[string]string{"relay": relayaddr.String(), "target": target.String()}); err != nil {
		log.Fatalln(err)
	}

	lines := bufio.NewScanner(os.Stdin)
	for lines.Scan() {
		result := map[string]string{"remote": lines.Text()}
		if cause := dial(ctx, runner, t, lines.Text()); cause != nil {
			result["error"] = cause.Error()
		}
		if err = out.Encode(result); err != nil {
			log.Fatalln(err)
		}
	}
}

func dial(ctx context.Context, h host.Host, t *p2pwebrtc.Transport, remote string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	addr, err := ma.NewMultiaddr(remote)
	if err != nil {
		return err
	}
	pi, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return err
	}

	if err = t.Connect(ctx, peer.AddrInfo{ID: pi.ID, Addrs: []ma.Multiaddr{addr}}); err != nil {
		return err
	}

	s, err := h.NewStream(ctx, pi.ID, "/p2pwebrtc/test/echo")
	if err != nil {
		return err
	}
	defer s.Close()

	if tpt := s.Conn().ConnState().Transport; tpt != p2pwebrtc.TransportName {
		return fmt.Errorf("stream is on a %s connection", tpt)
	}

	payload := make([]byte, 1<<20)
	rand.Read(payload)
	go func() {
		s.Write(payload)
		s.CloseWrite()
	}()

	echoed, err := io.ReadAll(s)
	if err != nil {
		return err
	}
	if !bytes.Equal(payload, echoed) {
		return fmt.Errorf("echo mismatch %d != %d", len(payload), len(echoed))
	}
	return nil
}
