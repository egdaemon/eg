package webrtcp2p

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	multistream "github.com/multiformats/go-multistream"
	"github.com/pion/webrtc/v4"
)

const (
	// how long the peer connection has to reach connected after the offer arrives.
	connectTimeout = 45 * time.Second
	// how long a new data channel has to negotiate a protocol.
	negotiateTimeout = 30 * time.Second
	// how long a disconnected peer connection has to recover before it is closed.
	disconnectGrace = 10 * time.Second
	// the data channel label the dialer uses to force ice information into the offer, ignored.
	initChannel = "init"
	// bounds the peer connections (udp sockets, goroutines) one runner will hold for browsers.
	maxConnections = 32
)

// DefaultSTUN are the servers used to discover the runner's public address, the same ones
// js-libp2p defaults to. they only see the ice binding requests, no libp2p traffic.
var DefaultSTUN = []string{
	"stun:stun.l.google.com:19302",
	"stun:global.stun.twilio.com:3478",
}

// StreamHandler serves one negotiated protocol stream. it owns the stream and must close it.
// remote is the authenticated peer of the relayed connection the offer arrived on.
type StreamHandler func(remote peer.ID, addr multiaddr.Multiaddr, s io.ReadWriteCloser)

// Answerer answers webrtc signaling requests.
type Answerer struct {
	ctx      context.Context
	api      *webrtc.API
	stun     []string
	handlers map[protocol.ID]StreamHandler
	slots    chan struct{}
}

// New answerer serving the given protocols. an empty stun uses DefaultSTUN.
func New(ctx context.Context, stun []string, handlers map[protocol.ID]StreamHandler) *Answerer {
	if len(stun) == 0 {
		stun = DefaultSTUN
	}

	se := webrtc.SettingEngine{}
	// detached channels expose plain read/write, which the message framing is layered on.
	se.DetachDataChannels()
	se.SetNetworkTypes([]webrtc.NetworkType{webrtc.NetworkTypeUDP4, webrtc.NetworkTypeUDP6})

	return &Answerer{
		ctx:      ctx,
		api:      webrtc.NewAPI(webrtc.WithSettingEngine(se)),
		stun:     stun,
		handlers: handlers,
		slots:    make(chan struct{}, maxConnections),
	}
}

// Handle is the network.StreamHandler for SignalProtocol.
func (t *Answerer) Handle(s network.Stream) {
	select {
	case t.slots <- struct{}{}:
		defer func() { <-t.slots }()
	default:
		log.Println("webrtc: too many connections, rejecting", s.Conn().RemotePeer())
		_ = s.Reset()
		return
	}

	remote, addr := s.Conn().RemotePeer(), s.Conn().RemoteMultiaddr()
	if err := t.answer(s, remote, addr); err != nil {
		log.Println("webrtc: signaling failed", remote, err)
		_ = s.Reset()
		return
	}

	_ = s.Close()
}

// answer performs the signaling exchange and returns once the peer connection is established
// (the connection then lives on until it fails or the remote closes it) or has failed.
func (t *Answerer) answer(s io.ReadWriter, remote peer.ID, addr multiaddr.Multiaddr) (err error) {
	ctx, cancel := context.WithTimeout(t.ctx, connectTimeout)
	defer cancel()

	var (
		wmu       sync.Mutex // guards writes to the signaling stream and the candidate queue
		answered  bool
		queued    []signal
		connected = make(chan struct{})
		failed    = make(chan error, 1)
		once      sync.Once
	)

	fail := func(err error) {
		select {
		case failed <- err:
		default:
		}
	}

	// failures of the signaling stream only matter until the exchange is over, after that the
	// stream is closed on purpose and the peer connection carries on without it.
	sigdone := make(chan struct{})
	defer close(sigdone)
	sigfail := func(err error) {
		select {
		case <-sigdone:
		default:
			fail(err)
		}
	}

	pc, err := t.api.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{{URLs: t.stun}},
	})
	if err != nil {
		return errorsx.Wrap(err, "unable to create peer connection")
	}

	// the peer connection outlives the signaling exchange, it is closed when it ends.
	established := false
	defer func() {
		if !established {
			_ = pc.Close()
		}
	}()

	// ice reports disconnected on a network blip and can recover on its own, give it a moment
	// before giving up. failed and closed are final.
	var (
		gmu   sync.Mutex
		grace *time.Timer
	)
	pc.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		gmu.Lock()
		defer gmu.Unlock()

		if grace != nil {
			grace.Stop()
			grace = nil
		}

		switch state {
		case webrtc.PeerConnectionStateConnected:
			once.Do(func() { close(connected) })
		case webrtc.PeerConnectionStateDisconnected:
			grace = time.AfterFunc(disconnectGrace, func() {
				fail(errors.New("peer connection disconnected"))
				_ = pc.Close()
			})
		case webrtc.PeerConnectionStateFailed:
			fail(errors.New("peer connection failed"))
			_ = pc.Close()
		case webrtc.PeerConnectionStateClosed:
			fail(errors.New("peer connection closed"))
			_ = pc.Close()
		}
	})

	pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		if c == nil {
			return
		}

		encoded, err := json.Marshal(c.ToJSON())
		if err != nil {
			log.Println("webrtc: unable to encode candidate", err)
			return
		}

		m := signal{kind: signalICECandidate, data: string(encoded)}

		// candidates must not overtake the answer, the dialer cannot use them until it has the answer.
		wmu.Lock()
		defer wmu.Unlock()
		if !answered {
			queued = append(queued, m)
			return
		}

		if err := writeSignal(s, m); err != nil {
			sigfail(err)
		}
	})

	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		if dc.Label() == initChannel {
			return
		}

		dc.OnOpen(func() {
			raw, err := dc.Detach()
			if err != nil {
				log.Println("webrtc: unable to detach data channel", remote, err)
				return
			}

			go t.serve(newStream(dc, raw), remote, addr)
		})
	})

	// reads run on their own goroutine because remote candidates keep arriving after the answer.
	// it ends with the signaling stream, which the dialer (or the deferred close) closes.
	reader := bufio.NewReader(s)
	remoteSet := make(chan struct{}) // closed once the offer is applied, candidates can't be added before.
	offers := make(chan signal, 1)
	go func() {
		defer close(offers)
		for {
			m, err := readSignal(reader)
			if err != nil {
				if !errors.Is(err, io.EOF) {
					sigfail(err)
				}
				return
			}

			if m.kind == signalSDPOffer {
				offers <- m
				continue
			}

			if m.kind != signalICECandidate {
				continue
			}

			var c webrtc.ICECandidateInit
			if err := json.Unmarshal([]byte(m.data), &c); err != nil {
				sigfail(errorsx.Wrap(err, "unable to decode candidate"))
				return
			}

			// the end of candidates marker is an empty candidate.
			if c.Candidate == "" {
				continue
			}

			select {
			case <-remoteSet:
			case <-ctx.Done():
				return
			}

			if err := pc.AddICECandidate(c); err != nil {
				log.Println("webrtc: unable to add candidate", remote, err)
			}
		}
	}()

	var offer signal
	select {
	case m, ok := <-offers:
		if !ok {
			return errors.New("signaling ended before an offer arrived")
		}
		offer = m
	case err = <-failed:
		return err
	case <-ctx.Done():
		return errorsx.Wrap(ctx.Err(), "timed out waiting for the offer")
	}

	if err = pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer.data}); err != nil {
		return errorsx.Wrap(err, "unable to set the offer")
	}
	close(remoteSet)

	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		return errorsx.Wrap(err, "unable to create the answer")
	}

	if err = pc.SetLocalDescription(answer); err != nil {
		return errorsx.Wrap(err, "unable to set the answer")
	}

	wmu.Lock()
	err = writeSignal(s, signal{kind: signalSDPAnswer, data: answer.SDP})
	if err == nil {
		answered = true
		for _, m := range queued {
			if err = writeSignal(s, m); err != nil {
				break
			}
		}
		queued = nil
	}
	wmu.Unlock()
	if err != nil {
		return errorsx.Wrap(err, "unable to send the answer")
	}

	select {
	case <-connected:
		established = true
		go t.supervise(pc, failed)
		return nil
	case err = <-failed:
		return err
	case <-ctx.Done():
		return errorsx.Wrap(ctx.Err(), "timed out establishing the peer connection")
	}
}

// supervise closes the peer connection when it is lost or the daemon is shutting down.
func (t *Answerer) supervise(pc *webrtc.PeerConnection, failed <-chan error) {
	defer pc.Close()

	select {
	case <-failed:
	case <-t.ctx.Done():
	}
}

// serve negotiates the protocol of a new stream and hands it to the handler.
func (t *Answerer) serve(s *stream, remote peer.ID, addr multiaddr.Multiaddr) {
	mux := multistream.NewMultistreamMuxer[protocol.ID]()
	for proto := range t.handlers {
		mux.AddHandler(proto, nil)
	}

	timer := time.AfterFunc(negotiateTimeout, func() { _ = s.Reset() })
	proto, _, err := mux.Negotiate(s)
	if !timer.Stop() || err != nil {
		if err != nil {
			log.Println("webrtc: unable to negotiate a protocol", remote, err)
		}
		_ = s.Reset()
		return
	}

	t.handlers[proto](remote, addr, s)
}
