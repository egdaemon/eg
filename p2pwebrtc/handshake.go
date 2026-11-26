package p2pwebrtc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"time"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/p2p/transport/webrtc/pb"
	"github.com/libp2p/go-msgio"
	"google.golang.org/protobuf/proto"

	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr/net"
	"github.com/pion/datachannel"
	"github.com/pion/webrtc/v4"
)

// initChannelLabel is the label of the datachannel the initiator creates so the offer has an
// application section to gather candidates for. it carries no data and the initiator closes it
// once it opens. see js-libp2p's private-to-private/initiate-connection.ts.
const initChannelLabel = "init"

// initChannelLinger bounds how long the recipient keeps the init channel when the initiator does not
// close it.
const initChannelLinger = 30 * time.Second

// maxAcceptQueueLen bounds the datachannels the remote can open before they are accepted.
const maxAcceptQueueLen = 256

// peerconn is a peer connection under negotiation.
type peerconn struct {
	pc       *webrtc.PeerConnection
	incoming chan dataChannel
	closed   chan struct{}

	connected chan struct{}
	failed    chan struct{}
	once      sync.Once
	fonce     sync.Once
}

func (t *Transport) newPeerConn(scope network.ConnManagementScope) (*peerconn, error) {
	se := webrtc.SettingEngine{LoggerFactory: pionfactory{}}
	se.DetachDataChannels()
	se.SetICETimeouts(t.disconnected, t.failed, t.keepalive)
	se.SetSCTPMaxReceiveBufferSize(sctpReceiveBufferSize)
	for _, fn := range t.settings {
		fn(&se)
	}

	if err := scope.ReserveMemory(sctpReceiveBufferSize, network.ReservationPriorityMedium); err != nil {
		return nil, err
	}

	pc, err := webrtc.NewAPI(webrtc.WithSettingEngine(se)).NewPeerConnection(t.config)
	if err != nil {
		return nil, fmt.Errorf("failed to create peer connection: %w", err)
	}

	p := &peerconn{
		pc:        pc,
		incoming:  make(chan dataChannel, maxAcceptQueueLen),
		closed:    make(chan struct{}, 1),
		connected: make(chan struct{}),
		failed:    make(chan struct{}),
	}

	// the handler is installed before negotiation, channels the remote opens early are queued.
	pc.OnDataChannel(func(dc *webrtc.DataChannel) {
		dc.OnOpen(func() {
			rwc, err := dc.Detach()
			if err != nil {
				log.Warn("could not detach datachannel", "label", dc.Label(), "error", err)
				return
			}

			if dc.Label() == initChannelLabel {
				go drainInitChannel(rwc)
				return
			}

			select {
			case p.incoming <- dataChannel{rwc, dc}:
			default:
				log.Warn("connection busy, rejecting stream")
				b, _ := proto.Marshal(&pb.Message{Flag: pb.Message_RESET.Enum()})
				w := msgio.NewWriter(rwc)
				w.WriteMsg(b)
				rwc.Close()
			}
		})
	})

	pc.OnConnectionStateChange(p.onstate)

	pc.SCTP().OnClose(func(error) {
		// pion can invoke the callback more than once.
		select {
		case p.closed <- struct{}{}:
		default:
		}
	})

	return p, nil
}

func (p *peerconn) onstate(s webrtc.PeerConnectionState) {
	switch s {
	case webrtc.PeerConnectionStateConnected:
		p.once.Do(func() { close(p.connected) })
	case webrtc.PeerConnectionStateFailed, webrtc.PeerConnectionStateClosed:
		p.fonce.Do(func() { close(p.failed) })
	}
}

func (p *peerconn) isconnected() bool {
	select {
	case <-p.connected:
		return true
	default:
		return false
	}
}

// trickle sends the local candidates to the remote until the peer connection connects.
func (p *peerconn) trickle(sig *signaler) {
	p.pc.OnICECandidate(func(c *webrtc.ICECandidate) {
		// nil is the end of gathering, js-libp2p does not send it either.
		if c == nil || p.isconnected() {
			return
		}

		encoded, err := encodeCandidate(c)
		if err != nil {
			log.Debug("unable to encode ice candidate", "error", err)
			return
		}

		if err := sig.write(message{Type: messageICECandidate, Data: encoded}); err != nil {
			log.Debug("unable to send ice candidate", "error", err)
		}
	})
}

// await reads the remote's candidates until the peer connection connects. the signaling stream
// ending early is not an error, the remote may have sent everything it has.
func (p *peerconn) await(ctx context.Context, sig *signaler) error {
	errc := make(chan error, 1)
	go func() {
		for {
			m, err := sig.read()
			if err != nil {
				if !errors.Is(err, io.EOF) {
					log.Debug("signaling stream ended", "error", err)
				}
				return
			}

			if m.Type != messageICECandidate {
				errc <- fmt.Errorf("expected %s, received %s", messageICECandidate, m.Type)
				return
			}

			c, err := decodeCandidate(m.Data)
			if err != nil {
				errc <- err
				return
			}

			if c == nil {
				continue
			}

			if err := p.pc.AddICECandidate(*c); err != nil {
				log.Debug("bad ice candidate", "candidate", c.Candidate, "error", err)
			}
		}
	}()

	select {
	case <-p.connected:
		return nil
	case <-p.failed:
		return fmt.Errorf("peer connection %s", p.pc.ConnectionState())
	case err := <-errc:
		if p.isconnected() {
			return nil
		}
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *peerconn) close() {
	p.pc.OnICECandidate(nil)
	p.pc.Close()
}

// drainInitChannel waits for the initiator to close the init channel and closes our side, the
// initiator waits for the channel to close before it closes the signaling stream.
func drainInitChannel(rwc datachannel.ReadWriteCloser) {
	defer rwc.Close()
	if dc, ok := rwc.(*datachannel.DataChannel); ok {
		dc.SetReadDeadline(time.Now().Add(initChannelLinger))
	}
	var buf [1024]byte
	for {
		if _, err := rwc.Read(buf[:]); err != nil {
			return
		}
	}
}

// bindstream ties the signaling stream to the context, it is reset when the context ends first.
func bindstream(ctx context.Context, s network.Stream) (stop func() bool) {
	if dl, ok := ctx.Deadline(); ok {
		s.SetDeadline(dl)
	}
	return context.AfterFunc(ctx, func() { s.Reset() })
}

// remotekey is the remote's public key as authenticated by the connection carrying the signaling.
func remotekey(s network.Stream) (ic.PubKey, error) {
	if k := s.Conn().RemotePublicKey(); k != nil {
		return k, nil
	}
	return s.Conn().RemotePeer().ExtractPublicKey()
}

// handleSignaling is the recipient: the remote dialed us through the relay and offers a peer
// connection. see js-libp2p's private-to-private/signaling-stream-handler.ts.
func (t *Transport) handleSignaling(s network.Stream) {
	remote := s.Conn().RemotePeer()

	if !t.acquire() {
		log.Debug("too many webrtc handshakes in flight, rejecting", "peer", remote)
		s.Reset()
		return
	}
	defer t.release()

	ctx, cancel := context.WithTimeout(t.ctx, t.handshakeTimeout)
	defer cancel()

	c, err := t.accept(ctx, s)
	if err != nil {
		log.Debug("inbound webrtc handshake failed", "peer", remote, "error", err)
		s.Reset()
		return
	}

	if err = t.deliver(ctx, c); err != nil {
		log.Debug("unable to hand the webrtc connection to the swarm", "peer", remote, "error", err)
		c.Close()
		return
	}

	log.Debug("inbound webrtc connection", "peer", remote, "addr", c.remoteMultiaddr)
}

func (t *Transport) accept(ctx context.Context, s network.Stream) (_ *connection, err error) {
	if t.listener.Load() == nil {
		return nil, errors.New("not listening on " + ListenAddr.String())
	}

	remote := s.Conn().RemotePeer()
	key, err := remotekey(s)
	if err != nil {
		return nil, fmt.Errorf("unable to determine the remote's public key: %w", err)
	}

	stop := bindstream(ctx, s)
	defer stop()

	scope, err := t.rcmgr.OpenConnection(network.DirInbound, false, ListenAddr)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			scope.Done()
		}
	}()

	if err = scope.SetPeer(remote); err != nil {
		return nil, err
	}

	p, err := t.newPeerConn(scope)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			p.close()
		}
	}()

	sig := newSignaler(s)
	p.trickle(sig)

	offer, err := sig.readType(messageSDPOffer)
	if err != nil {
		return nil, fmt.Errorf("unable to read the offer: %w", err)
	}

	if err = p.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeOffer, SDP: offer.Data}); err != nil {
		return nil, fmt.Errorf("unable to set the remote description: %w", err)
	}

	answer, err := p.pc.CreateAnswer(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create the answer: %w", err)
	}

	// the answer is written before the local description is set, so it precedes the candidates.
	if err = sig.write(message{Type: messageSDPAnswer, Data: answer.SDP}); err != nil {
		return nil, fmt.Errorf("unable to send the answer: %w", err)
	}

	if err = p.pc.SetLocalDescription(answer); err != nil {
		return nil, fmt.Errorf("unable to set the local description: %w", err)
	}

	if err = p.await(ctx, sig); err != nil {
		return nil, err
	}

	p.pc.OnICECandidate(nil)
	// the signaling is done once connected, the remote does not wait for us to close.
	s.Close()

	c, err := t.establish(network.DirInbound, p, scope, remote, key)
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Connect establishes a webrtc connection to the peer and adds it to the host's swarm. pi.Addrs are
// relayed addresses of the peer, with or without the trailing /webrtc; without addresses an existing
// connection to the peer, or the peerstore, is used to reach it.
// see js-libp2p's private-to-private/initiate-connection.ts.
func (t *Transport) Connect(ctx context.Context, pi peer.AddrInfo) (err error) {
	if err = t.ctx.Err(); err != nil {
		return err
	}

	if !t.acquire() {
		return errors.New("too many webrtc handshakes in flight")
	}
	defer t.release()

	ctx, cancel := context.WithTimeout(ctx, t.handshakeTimeout)
	defer cancel()

	if len(pi.Addrs) > 0 {
		t.host.Peerstore().AddAddrs(pi.ID, SignalingAddrs(pi.Addrs...), peerstore.TempAddrTTL)
	}

	s, err := t.host.NewStream(network.WithAllowLimitedConn(ctx, string(SignalingProtocol)), pi.ID, SignalingProtocol)
	if err != nil {
		return fmt.Errorf("unable to open the signaling stream: %w", err)
	}

	c, err := t.initiate(ctx, s, pi.ID)
	if err != nil {
		s.Reset()
		return err
	}

	if err = t.deliver(ctx, c); err != nil {
		c.Close()
		return err
	}

	// the swarm adds the connection asynchronously, wait so the caller's next NewStream uses it.
	return waitconn(ctx, t.host.Network(), c)
}

func waitconn(ctx context.Context, n network.Network, c *connection) error {
	tick := time.NewTicker(10 * time.Millisecond)
	defer tick.Stop()
	for {
		for _, conn := range n.ConnsToPeer(c.remotePeer) {
			if conn.ConnState().Transport == TransportName && conn.RemoteMultiaddr().Equal(c.remoteMultiaddr) {
				return nil
			}
		}

		if c.IsClosed() {
			return c.closeErr
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-tick.C:
		}
	}
}

func (t *Transport) initiate(ctx context.Context, s network.Stream, remote peer.ID) (_ *connection, err error) {
	key, err := remotekey(s)
	if err != nil {
		return nil, fmt.Errorf("unable to determine the remote's public key: %w", err)
	}

	stop := bindstream(ctx, s)
	defer stop()

	scope, err := t.rcmgr.OpenConnection(network.DirOutbound, false, ListenAddr)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			scope.Done()
		}
	}()

	if err = scope.SetPeer(remote); err != nil {
		return nil, err
	}

	p, err := t.newPeerConn(scope)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err != nil {
			p.close()
		}
	}()

	initdc, err := p.pc.CreateDataChannel(initChannelLabel, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create the init channel: %w", err)
	}
	initopen := make(chan datachannel.ReadWriteCloser, 1)
	initdc.OnOpen(func() {
		rwc, err := initdc.Detach()
		if err != nil {
			log.Debug("unable to detach the init channel", "error", err)
			return
		}
		initopen <- rwc
	})

	sig := newSignaler(s)
	p.trickle(sig)

	offer, err := p.pc.CreateOffer(nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create the offer: %w", err)
	}

	if err = sig.write(message{Type: messageSDPOffer, Data: offer.SDP}); err != nil {
		return nil, fmt.Errorf("unable to send the offer: %w", err)
	}

	if err = p.pc.SetLocalDescription(offer); err != nil {
		return nil, fmt.Errorf("unable to set the local description: %w", err)
	}

	answer, err := sig.readType(messageSDPAnswer)
	if err != nil {
		return nil, fmt.Errorf("unable to read the answer: %w", err)
	}

	if err = p.pc.SetRemoteDescription(webrtc.SessionDescription{Type: webrtc.SDPTypeAnswer, SDP: answer.Data}); err != nil {
		return nil, fmt.Errorf("unable to set the remote description: %w", err)
	}

	if err = p.await(ctx, sig); err != nil {
		return nil, err
	}
	p.pc.OnICECandidate(nil)

	select {
	case rwc := <-initopen:
		rwc.Close()
	case <-p.failed:
		return nil, fmt.Errorf("peer connection %s", p.pc.ConnectionState())
	case <-ctx.Done():
		return nil, fmt.Errorf("init channel did not open: %w", ctx.Err())
	}

	s.Close()

	return t.establish(network.DirOutbound, p, scope, remote, key)
}

// establish wraps the connected peer connection into a libp2p connection.
func (t *Transport) establish(dir network.Direction, p *peerconn, scope network.ConnManagementScope, remote peer.ID, key ic.PubKey) (*connection, error) {
	local, raddr := selectedaddrs(p.pc)
	c, err := newConnection(p.pc, t, scope, t.host.ID(), local, remote, key, raddr, p.incoming, p.closed)
	if err != nil {
		return nil, err
	}

	if err = t.gate(dir, c); err != nil {
		c.Close()
		return nil, err
	}

	return c, nil
}

// selectedaddrs are the local and remote addresses of the selected ICE candidate pair as
// /ip4/<ip>/udp/<port>/webrtc, or /webrtc when they are unknown (for example an mDNS candidate).
func selectedaddrs(pc *webrtc.PeerConnection) (local, remote ma.Multiaddr) {
	local, remote = ListenAddr, ListenAddr

	pair, err := pc.SCTP().Transport().ICETransport().GetSelectedCandidatePair()
	if err != nil || pair == nil {
		return local, remote
	}

	return candidateaddr(pair.Local), candidateaddr(pair.Remote)
}

func candidateaddr(c *webrtc.ICECandidate) ma.Multiaddr {
	if c == nil {
		return ListenAddr
	}

	ip := net.ParseIP(c.Address)
	if ip == nil {
		return ListenAddr
	}

	a, err := manet.FromNetAddr(&net.UDPAddr{IP: ip, Port: int(c.Port)})
	if err != nil {
		return ListenAddr
	}

	return a.AppendComponent(&webrtcComponent)
}
