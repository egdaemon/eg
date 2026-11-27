package daemons_test

import (
	"context"
	"io"
	"net"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/p2pwebrtc"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

// the console's path to a runner: through the runner's relay reservation, upgraded to webrtc.
func TestP2PProxyWebRTC(t *testing.T) {
	ctx, done := context.WithTimeout(t.Context(), time.Minute)
	defer done()

	httpl, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer httpl.Close()

	go func() {
		for {
			c, err := httpl.Accept()
			if err != nil {
				return
			}

			go func() {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}()
		}
	}()

	r, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.DisableRelay())
	require.NoError(t, err)
	defer r.Close()
	_, err = relay.New(r)
	require.NoError(t, err)

	db := testx.MustT(migrations.Open(t.Context(), sqltestx.Path(t)))(t)
	defer db.Close()
	runner, err := daemons.P2PProxyGated(ctx, []byte("p2p-proxy-webrtc-test"), httpl, daemons.NewLANGate(db, lan.DefaultTTL))
	require.NoError(t, err)
	defer runner.Close()

	rinfo := peer.AddrInfo{ID: r.ID(), Addrs: r.Addrs()}
	require.NoError(t, libp2px.Connect(ctx, runner, rinfo))
	require.NoError(t, libp2px.Reserve(ctx, runner, rinfo))

	client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.EnableRelay())
	require.NoError(t, err)
	defer client.Close()
	dialer, err := p2pwebrtc.Add(client)
	require.NoError(t, err)

	target := r.Addrs()[0].Encapsulate(ma.StringCast("/p2p/" + r.ID().String() + "/p2p-circuit/webrtc/p2p/" + runner.ID().String()))
	require.NoError(t, dialer.Connect(ctx, peer.AddrInfo{ID: runner.ID(), Addrs: []ma.Multiaddr{target}}))

	s, err := client.NewStream(ctx, runner.ID(), "/egdaemon/proxy")
	require.NoError(t, err)
	defer s.Close()
	require.Equal(t, p2pwebrtc.TransportName, s.Conn().ConnState().Transport)

	require.NoError(t, s.SetDeadline(time.Now().Add(5*time.Second)))
	_, err = s.Write([]byte("ping"))
	require.NoError(t, err)

	buf := make([]byte, 4)
	_, err = io.ReadFull(s, buf)
	require.NoError(t, err)
	require.Equal(t, "ping", string(buf))

	// the gate recorded the peer with the address of the webrtc connection.
	var p lan.Peer
	require.NoError(t, lan.PeerFindByID(ctx, db, client.ID().String()).Scan(&p))
	require.Contains(t, p.Addrs, "/udp/")
	require.Contains(t, p.Addrs, "/webrtc")
}
