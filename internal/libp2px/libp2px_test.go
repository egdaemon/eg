package libp2px

import (
	"context"
	"crypto/rand"
	"fmt"
	"testing"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/net/swarm"
	relayv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

const echoProtocol = "/egdaemon/test/echo"

func newRelay(t *testing.T, priv crypto.PrivKey, addr multiaddr.Multiaddr) host.Host {
	t.Helper()
	h, err := libp2p.New(libp2p.Identity(priv), libp2p.ListenAddrs(addr), libp2p.DisableRelay())
	require.NoError(t, err)
	_, err = relayv2.New(h)
	require.NoError(t, err)
	return h
}

// reachable reports if the client can open a stream to the target through the
// relay, which requires the target to hold a reservation with it.
func reachable(ctx context.Context, client host.Host, relay peer.AddrInfo, target peer.ID) bool {
	ctx, done := context.WithTimeout(network.WithAllowLimitedConn(ctx, "test"), time.Second)
	defer done()

	if err := client.Connect(ctx, relay); err != nil {
		return false
	}

	// failed probes put the circuit address into dial backoff.
	client.Network().(*swarm.Swarm).Backoff().Clear(target)

	s, err := client.NewStream(ctx, target, echoProtocol)
	if err != nil {
		return false
	}
	defer s.Close()

	return true
}

func keeping(self peer.ID) (n int) {
	keepers.Range(func(k, _ any) bool {
		if k.(keeperKey).self == self {
			n++
		}
		return true
	})
	return n
}

func TestKeepReserved(t *testing.T) {
	ctx, done := context.WithTimeout(t.Context(), 30*time.Second)
	defer done()

	retry := backoff.Constant(100 * time.Millisecond)

	priv, _, err := crypto.GenerateEd25519Key(rand.Reader)
	require.NoError(t, err)

	relay := newRelay(t, priv, multiaddr.StringCast("/ip4/127.0.0.1/tcp/0"))
	relayinfo := peer.AddrInfo{ID: relay.ID(), Addrs: relay.Addrs()}

	runner, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.EnableRelay())
	require.NoError(t, err)
	defer runner.Close()
	runner.SetStreamHandler(echoProtocol, func(s network.Stream) { _ = s.Close() })

	client, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"), libp2p.EnableRelay())
	require.NoError(t, err)
	defer client.Close()

	circuit := relayinfo.Addrs[0].Encapsulate(multiaddr.StringCast(fmt.Sprintf("/p2p/%s/p2p-circuit", relay.ID())))
	client.Peerstore().AddAddr(runner.ID(), circuit, peerstore.PermanentAddrTTL)

	kctx, kdone := context.WithCancel(ctx)
	defer kdone()

	require.False(t, reachable(ctx, client, relayinfo, runner.ID()))

	keep(kctx, runner, retry, relayinfo)
	keep(kctx, runner, retry, relayinfo)
	require.Equal(t, 1, keeping(runner.ID()))

	require.Eventually(t, func() bool { return reachable(ctx, client, relayinfo, runner.ID()) }, 5*time.Second, 100*time.Millisecond)

	// a restarted relay has no reservations; the keeper must re-reserve on
	// disconnect rather than waiting for the reservation to expire.
	require.NoError(t, relay.Close())
	relay = newRelay(t, priv, relayinfo.Addrs[0])
	defer relay.Close()

	require.Eventually(t, func() bool { return reachable(ctx, client, relayinfo, runner.ID()) }, 10*time.Second, 100*time.Millisecond)

	kdone()
	require.Eventually(t, func() bool { return keeping(runner.ID()) == 0 }, 5*time.Second, 50*time.Millisecond)
}
