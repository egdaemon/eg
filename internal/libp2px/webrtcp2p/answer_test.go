package webrtcp2p

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	multistream "github.com/multiformats/go-multistream"
	"github.com/pion/webrtc/v4"
	"github.com/stretchr/testify/require"
)

const echoProtocol protocol.ID = "/test/echo"

func newHost(t *testing.T) host.Host {
	h, err := libp2p.New(libp2p.ListenAddrStrings("/ip4/127.0.0.1/tcp/0"))
	require.NoError(t, err)
	t.Cleanup(func() { h.Close() })
	return h
}

// dial plays the browser: it opens the signaling stream, offers, and returns the established peer connection.
func dial(t *testing.T, ctx context.Context, from host.Host, to peer.AddrInfo) *webrtc.PeerConnection {
	require.NoError(t, from.Connect(ctx, to))
	s, err := from.NewStream(ctx, to.ID, SignalProtocol)
	require.NoError(t, err)

	se := webrtc.SettingEngine{}
	se.DetachDataChannels()
	pc, err := webrtc.NewAPI(webrtc.WithSettingEngine(se)).NewPeerConnection(webrtc.Configuration{})
	require.NoError(t, err)
	t.Cleanup(func() { pc.Close() })

	var wmu sync.Mutex
	send := func(m signal) {
		wmu.Lock()
		defer wmu.Unlock()
		require.NoError(t, writeSignal(s, m))
	}

	connected := make(chan struct{})
	var once sync.Once
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		if state == webrtc.PeerConnectionStateConnected {
			once.Do(func() { close(connected) })
		}
	})
	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}
		encoded, err := json.Marshal(c.ToJSON())
		require.NoError(t, err)
		send(signal{kind: signalICECandidate, data: string(encoded)})
	})

	_, err = pc.CreateDataChannel(initChannel, nil)
	require.NoError(t, err)

	offer, err := pc.CreateOffer(nil)
	require.NoError(t, err)

	// the answerer must see the offer before candidates, as a browser would send it.
	send(signal{kind: signalSDPOffer, data: offer.SDP})
	require.NoError(t, pc.SetLocalDescription(offer))

	go func() {
		r := bufio.NewReader(s)
		for {
			m, err := readSignal(r)
			if err != nil {
				return
			}

			switch m.kind {
			case signalSDPAnswer:
				if err := pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: m.data}); err != nil {
					t.Error(err)
					return
				}
			case signalICECandidate:
				var c webrtc.ICECandidateInit
				if err := json.Unmarshal([]byte(m.data), &c); err != nil {
					t.Error(err)
					return
				}
				if c.Candidate != "" {
					_ = pc.AddICECandidate(c)
				}
			}
		}
	}()

	select {
	case <-connected:
	case <-ctx.Done():
		t.Fatal("peer connection was not established", ctx.Err())
	}

	return pc
}

// open a stream for the protocol the way a browser does, a data channel plus multistream-select.
func open(t *testing.T, pc *webrtc.PeerConnection, proto protocol.ID) *stream {
	dc, err := pc.CreateDataChannel("", nil)
	require.NoError(t, err)

	opened := make(chan *stream, 1)
	dc.OnOpen(func() {
		raw, err := dc.Detach()
		require.NoError(t, err)
		opened <- newStream(dc, raw)
	})

	select {
	case s := <-opened:
		require.NoError(t, multistream.SelectProtoOrFail(proto, s))
		return s
	case <-time.After(10 * time.Second):
		t.Fatal("data channel did not open")
		return nil
	}
}

func newAnswerer(ctx context.Context, remotes chan<- peer.ID) *Answerer {
	a := New(ctx, nil, map[protocol.ID]StreamHandler{
		echoProtocol: func(remote peer.ID, _ multiaddr.Multiaddr, s io.ReadWriteCloser) {
			defer s.Close()
			if remotes != nil {
				remotes <- remote
			}
			_, _ = io.Copy(s, s)
		},
	})
	// no stun in tests, host candidates are enough on loopback.
	a.stun = nil
	return a
}

func TestAnswerEchoes(t *testing.T) {
	ctx, done := context.WithTimeout(context.Background(), 60*time.Second)
	defer done()

	runner, browser := newHost(t), newHost(t)
	remotes := make(chan peer.ID, 1)
	runner.SetStreamHandler(SignalProtocol, newAnswerer(ctx, remotes).Handle)

	pc := dial(t, ctx, browser, peer.AddrInfo{ID: runner.ID(), Addrs: runner.Addrs()})
	s := open(t, pc, echoProtocol)

	// larger than a single message to exercise the chunking.
	payload := make([]byte, 100_000)
	for i := range payload {
		payload[i] = byte(i)
	}

	go func() {
		_, err := s.Write(payload)
		require.NoError(t, err)
		require.NoError(t, s.CloseWrite())
	}()

	got := make([]byte, len(payload))
	_, err := io.ReadFull(s, got)
	require.NoError(t, err)
	require.Equal(t, payload, got)
	require.Equal(t, browser.ID(), <-remotes)
}

func TestAnswerRejectsUnknownProtocol(t *testing.T) {
	ctx, done := context.WithTimeout(context.Background(), 60*time.Second)
	defer done()

	runner, browser := newHost(t), newHost(t)
	runner.SetStreamHandler(SignalProtocol, newAnswerer(ctx, nil).Handle)

	pc := dial(t, ctx, browser, peer.AddrInfo{ID: runner.ID(), Addrs: runner.Addrs()})

	dc, err := pc.CreateDataChannel("", nil)
	require.NoError(t, err)
	opened := make(chan *stream, 1)
	dc.OnOpen(func() {
		raw, err := dc.Detach()
		require.NoError(t, err)
		opened <- newStream(dc, raw)
	})

	s := <-opened
	require.ErrorContains(t, multistream.SelectProtoOrFail("/nope", s), "not supported")
}
