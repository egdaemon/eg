package p2pwebrtc_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"io"
	"testing"
	"time"

	"github.com/egdaemon/p2pwebrtc"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	"github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/relay"
	ma "github.com/multiformats/go-multiaddr"
	"github.com/pion/ice/v4"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

const echo = "/p2pwebrtc/test/echo"

// loopback keeps the ice candidates on the loopback interface so the test does not depend on the network.
func loopback(se *webrtc.SettingEngine) {
	se.SetIncludeLoopbackCandidate(true)
	se.SetICEMulticastDNSMode(ice.MulticastDNSModeDisabled)
	se.SetInterfaceFilter(func(name string) bool { return name == "lo" || name == "lo0" })
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4})
}

func newhost(t *testing.T, opts ...libp2p.Option) host.Host {
	t.Helper()
	h, err := libp2p.New(append([]libp2p.Option{libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0")}, opts...)...)
	require.NoError(t, err)
	t.Cleanup(func() { h.Close() })
	return h
}

// topology is a relay, a runner holding a reservation on it, and a dialer, all with the transport.
func topology(t *testing.T) (r, runner, dialer host.Host, rt, dt *p2pwebrtc.Transport) {
	t.Helper()
	ctx := t.Context()

	r = newhost(t, libp2p.DisableRelay())
	svc, err := relay.New(r)
	require.NoError(t, err)
	t.Cleanup(func() { svc.Close() })
	runner = newhost(t, libp2p.EnableRelay(), libp2p.AddrsFactory(p2pwebrtc.AddrsFactory(nil)))
	dialer = newhost(t, libp2p.EnableRelay())

	rt, err = p2pwebrtc.Add(runner, p2pwebrtc.WithSettingEngine(loopback))
	require.NoError(t, err)
	dt, err = p2pwebrtc.Add(dialer, p2pwebrtc.WithSettingEngine(loopback))
	require.NoError(t, err)

	rinfo := peer.AddrInfo{ID: r.ID(), Addrs: r.Addrs()}
	require.NoError(t, runner.Connect(ctx, rinfo))
	_, err = client.Reserve(ctx, runner, rinfo)
	require.NoError(t, err)

	runner.SetStreamHandler(echo, func(s network.Stream) {
		defer s.Close()
		io.Copy(s, s)
	})

	return r, runner, dialer, rt, dt
}

func circuit(t *testing.T, r host.Host, target peer.ID) ma.Multiaddr {
	t.Helper()
	return ma.StringCast("/ip4/127.0.0.1/tcp/" + port(t, r) + "/p2p/" + r.ID().String() + "/p2p-circuit/webrtc/p2p/" + target.String())
}

func port(t *testing.T, h host.Host) string {
	for _, a := range h.Addrs() {
		if v, err := a.ValueForProtocol(ma.P_TCP); err == nil {
			return v
		}
	}
	t.Fatal("no tcp address")
	return ""
}

func TestConnectAndEcho(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	r, runner, dialer, _, dt := topology(t)

	require.NoError(t, dt.Connect(ctx, peer.AddrInfo{ID: runner.ID(), Addrs: []ma.Multiaddr{circuit(t, r, runner.ID())}}))

	s, err := dialer.NewStream(ctx, runner.ID(), echo)
	require.NoError(t, err)
	require.Equal(t, p2pwebrtc.TransportName, s.Conn().ConnState().Transport)
	require.False(t, s.Conn().Stat().Limited)

	// larger than the relay's default 128KiB limit and a datachannel message.
	payload := make([]byte, 1<<20)
	_, err = rand.Read(payload)
	require.NoError(t, err)

	go func() {
		_, _ = s.Write(payload)
		_ = s.CloseWrite()
	}()

	received, err := io.ReadAll(s)
	require.NoError(t, err)
	require.True(t, bytes.Equal(payload, received), "echo mismatch: %d != %d", len(payload), len(received))
	require.NoError(t, s.Close())

	// the runner sees the connection with the dialer's peer id and an ip address.
	conns := runner.Network().ConnsToPeer(dialer.ID())
	var found network.Conn
	for _, c := range conns {
		if c.ConnState().Transport == p2pwebrtc.TransportName {
			found = c
		}
	}
	require.NotNil(t, found)
	_, err = found.RemoteMultiaddr().ValueForProtocol(ma.P_IP4)
	require.NoError(t, err, found.RemoteMultiaddr().String())
}

func TestManyStreams(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	r, runner, dialer, _, dt := topology(t)
	require.NoError(t, dt.Connect(ctx, peer.AddrInfo{ID: runner.ID(), Addrs: []ma.Multiaddr{circuit(t, r, runner.ID())}}))

	errs := make(chan error, 32)
	for i := range 32 {
		go func() {
			s, err := dialer.NewStream(ctx, runner.ID(), echo)
			if err != nil {
				errs <- err
				return
			}
			defer s.Close()
			msg := bytes.Repeat([]byte{byte(i)}, 1000+i)
			if _, err = s.Write(msg); err != nil {
				errs <- err
				return
			}
			s.CloseWrite()
			got, err := io.ReadAll(s)
			if err == nil && !bytes.Equal(got, msg) {
				err = io.ErrUnexpectedEOF
			}
			errs <- err
		}()
	}

	for range 32 {
		require.NoError(t, <-errs)
	}
}

func TestRunnerAdvertisesWebRTC(t *testing.T) {
	_, runner, _, _, _ := topology(t)
	for _, a := range runner.Addrs() {
		require.False(t, a.Equal(p2pwebrtc.ListenAddr), "bare /webrtc advertised")
	}
}

func TestSignalingRejectedAfterClose(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()

	r, runner, _, rt, dt := topology(t)
	require.NoError(t, rt.Close())

	require.Error(t, dt.Connect(ctx, peer.AddrInfo{ID: runner.ID(), Addrs: []ma.Multiaddr{circuit(t, r, runner.ID())}}))
}
