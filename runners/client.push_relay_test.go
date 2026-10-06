package runners_test

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/libp2p/go-libp2p/p2p/protocol/holepunch"
	"github.com/libp2p/go-libp2p/p2p/transport/quicreuse"
	"github.com/marcopolo/simnet"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

type sourceIPSelector struct {
	ip atomic.Pointer[net.IP]
}

func (m *sourceIPSelector) PreferredSourceIPForDestination(_ *net.UDPAddr) (net.IP, error) {
	return *m.ip.Load(), nil
}

// quicSimnet routes the host's quic transport over a simulated network so
// hosts get public addresses (required for hole punching). hosts that aren't
// public sit behind a firewall that only admits packets from addresses they
// have already sent to.
func quicSimnet(public bool, router *simnet.SimpleFirewallRouter) libp2p.Option {
	m := &sourceIPSelector{}
	return libp2p.QUICReuse(
		quicreuse.NewConnManager,
		quicreuse.OverrideSourceIPSelector(func() (quicreuse.SourceIPSelector, error) {
			return m, nil
		}),
		quicreuse.OverrideListenUDP(func(_ string, address *net.UDPAddr) (net.PacketConn, error) {
			m.ip.Store(&address.IP)
			if public {
				router.SetAddrPubliclyReachable(address)
			}
			c := simnet.NewSimConn(address)
			c.SetUpPacketReceiver(router)
			router.AddNode(address, c)
			return c, nil
		}),
	)
}

func newSimHost(t *testing.T, opts ...libp2p.Option) host.Host {
	t.Helper()
	h, err := libp2p.New(append(opts, libp2p.ResourceManager(&network.NullResourceManager{}))...)
	require.NoError(t, err)
	t.Cleanup(func() { _ = h.Close() })
	return h
}

func waitForHolePunching(t *testing.T, hosts ...host.Host) {
	t.Helper()
	for _, h := range hosts {
		require.Eventually(t, func() bool {
			for _, p := range h.Mux().Protocols() {
				if p == holepunch.Protocol {
					return true
				}
			}
			return false
		}, 5*time.Second, 50*time.Millisecond)
	}
}

func TestUploadViaRelay(t *testing.T) {
	ctx, done := context.WithTimeout(t.Context(), 30*time.Second)
	defer done()

	router := &simnet.SimpleFirewallRouter{}

	relay := newSimHost(t,
		quicSimnet(true, router),
		libp2p.ListenAddrs(ma.StringCast("/ip4/1.2.0.1/udp/8000/quic-v1")),
		libp2p.DisableRelay(),
	)
	_, err := relayv2.New(relay)
	require.NoError(t, err)
	relayinfo := peer.AddrInfo{ID: relay.ID(), Addrs: relay.Addrs()}

	var received atomic.Int64
	runner := newSimHost(t,
		quicSimnet(false, router),
		libp2p.ListenAddrs(ma.StringCast("/ip4/2.2.0.2/udp/8001/quic-v1")),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(holepunch.DirectDialTimeout(100*time.Millisecond)),
		libp2p.ForceReachabilityPrivate(),
	)
	runner.SetStreamHandler("/egdaemon/proxy", func(s network.Stream) {
		defer s.Close()
		req, err := http.ReadRequest(bufio.NewReader(s))
		if err != nil {
			return
		}

		rec := httptest.NewRecorder()
		if f, _, err := req.FormFile("kernel"); err == nil {
			n, _ := io.Copy(io.Discard, f)
			received.Store(n)
			rec.WriteHeader(http.StatusAccepted)
		} else {
			rec.WriteHeader(http.StatusBadRequest)
		}
		_ = rec.Result().Write(s)
	})

	client := newSimHost(t,
		quicSimnet(false, router),
		libp2p.ListenAddrs(ma.StringCast("/ip4/2.2.0.1/udp/8000/quic-v1")),
		libp2p.EnableRelay(),
		libp2p.EnableHolePunching(holepunch.DirectDialTimeout(100*time.Millisecond)),
		libp2p.ForceReachabilityPrivate(),
	)

	require.NoError(t, libp2px.Connect(ctx, runner, relayinfo))
	require.NoError(t, libp2px.Reserve(ctx, runner, relayinfo))
	require.NoError(t, libp2px.Connect(ctx, client, relayinfo))

	waitForHolePunching(t, client, runner)

	// the client only knows the runner's id.
	require.Empty(t, client.Peerstore().Addrs(runner.ID()))

	// larger than the relay's default per circuit data limit (128KB), the
	// upload must go over the hole punched connection.
	kernel := bytes.Repeat([]byte("k"), 1<<20)

	req := &runners.EnqueuedDequeueResponse{
		Enqueued: &runners.Enqueued{Id: uuid.Must(uuid.NewV7()).String()},
	}

	accepted, err := runners.Upload(ctx, client, &compute.Compute{Id: "relayed", P2Pid: runner.ID().String()}, req, bytes.NewReader(kernel), strings.NewReader(""))
	require.NoError(t, err)
	require.True(t, accepted)
	require.Equal(t, int64(len(kernel)), received.Load())

	direct := false
	for _, c := range client.Network().ConnsToPeer(runner.ID()) {
		if _, err := c.RemoteMultiaddr().ValueForProtocol(ma.P_CIRCUIT); err != nil {
			direct = true
		}
	}
	require.True(t, direct, "expected a hole punched connection to the runner")
}
