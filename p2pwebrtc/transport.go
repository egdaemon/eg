// Package p2pwebrtc implements the listener and dialer of libp2p's private to private webrtc
// (the /webrtc multiaddr, https://github.com/libp2p/specs/blob/master/webrtc/webrtc.md) for go-libp2p.
//
// a peer behind a NAT holds a relay reservation; the remote dials it through the relay, the two
// exchange SDP and ICE candidates over a /webrtc-signaling/0.0.1 stream on the relayed connection,
// and continue on a direct webrtc connection. the relay only carries the signaling.
//
// go-libp2p only ships webrtc-direct. this package plugs into an existing host:
//
//	t, err := p2pwebrtc.Add(h, p2pwebrtc.WithSTUN(p2pwebrtc.DefaultSTUNServers...))
//
// connections accepted or dialed by the transport are handed to the host's swarm, so stream handlers,
// identify and h.NewStream work over them like over any other connection.
package p2pwebrtc

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/libp2p/go-libp2p/core/connmgr"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	tpt "github.com/libp2p/go-libp2p/core/transport"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/pion/webrtc/v4"
)

const (
	// SignalingProtocol is the protocol of the stream carrying the SDP and ICE candidates.
	SignalingProtocol protocol.ID = "/webrtc-signaling/0.0.1"

	// TransportName is reported in network.ConnectionState.Transport.
	TransportName = "webrtc"
)

// DefaultSTUNServers are the STUN servers js-libp2p's webrtc transport uses by default.
// they are not used unless passed to WithSTUN.
var DefaultSTUNServers = []string{
	"stun:stun.l.google.com:19302",
	"stun:global.stun.twilio.com:3478",
	"stun:stun.cloudflare.com:3478",
	"stun:stun.services.mozilla.com:3478",
}

// defaults for the peer connection, matching go-libp2p's webrtc-direct transport.
const (
	DefaultDisconnectedTimeout = 20 * time.Second
	DefaultFailedTimeout       = 30 * time.Second
	DefaultKeepaliveTimeout    = 15 * time.Second

	// DefaultHandshakeTimeout bounds the signaling, from the offer until the peer connection connects.
	DefaultHandshakeTimeout = 30 * time.Second

	// DefaultMaxInFlight bounds the number of concurrent handshakes.
	DefaultMaxInFlight = 32

	// sctpReceiveBufferSize is the size of the buffer for incoming messages, ten full sized messages.
	sctpReceiveBufferSize = 10 * maxReceiveMessageSize
)

var (
	// ListenAddr is the address the transport listens on. it has no network component of its own,
	// the reachable addresses are the host's relayed addresses with /webrtc appended, see Addrs.
	ListenAddr = ma.StringCast("/webrtc")

	webrtcComponent = ListenAddr[0]
)

// Transport is a tpt.Transport for /webrtc. go-libp2p's swarm sends every /p2p-circuit address to
// the relay transport, so outbound connections are made with Connect rather than through the swarm.
type Transport struct {
	host     host.Host
	rcmgr    network.ResourceManager
	gater    connmgr.ConnectionGater
	config   webrtc.Configuration
	settings []func(*webrtc.SettingEngine)

	disconnected, failed, keepalive time.Duration
	handshakeTimeout                time.Duration
	inflight                        chan struct{}

	listener atomic.Pointer[listener]

	ctx    context.Context
	cancel context.CancelFunc
	once   sync.Once
}

var (
	_ tpt.Transport = &Transport{}
	_ io.Closer     = &Transport{}
)

// Option configures the transport.
type Option func(*Transport) error

// WithSTUN adds STUN servers, the runner needs one to learn its public address.
func WithSTUN(urls ...string) Option {
	return func(t *Transport) error {
		if len(urls) > 0 {
			t.config.ICEServers = append(t.config.ICEServers, webrtc.ICEServer{URLs: urls})
		}
		return nil
	}
}

// WithICEServers adds ICE (STUN or TURN) servers.
func WithICEServers(servers ...webrtc.ICEServer) Option {
	return func(t *Transport) error {
		t.config.ICEServers = append(t.config.ICEServers, servers...)
		return nil
	}
}

// WithGater consults the gater before handing a connection to the swarm.
// the swarm also checks its own gater once the connection is added.
func WithGater(g connmgr.ConnectionGater) Option {
	return func(t *Transport) error {
		t.gater = g
		return nil
	}
}

// WithHandshakeTimeout bounds the signaling of a single connection.
func WithHandshakeTimeout(d time.Duration) Option {
	return func(t *Transport) error {
		if d <= 0 {
			return errors.New("handshake timeout must be positive")
		}
		t.handshakeTimeout = d
		return nil
	}
}

// WithMaxInFlight bounds the number of concurrent handshakes, excess signaling streams are reset.
func WithMaxInFlight(n int) Option {
	return func(t *Transport) error {
		if n <= 0 {
			return errors.New("max in flight must be positive")
		}
		t.inflight = make(chan struct{}, n)
		return nil
	}
}

// WithICETimeouts sets pion's ICE disconnected, failed and keepalive timeouts.
func WithICETimeouts(disconnected, failed, keepalive time.Duration) Option {
	return func(t *Transport) error {
		t.disconnected, t.failed, t.keepalive = disconnected, failed, keepalive
		return nil
	}
}

// WithSettingEngine adjusts pion's setting engine for every peer connection, applied after the
// transport's own settings. for example restricting the network types or interfaces.
func WithSettingEngine(fn func(*webrtc.SettingEngine)) Option {
	return func(t *Transport) error {
		t.settings = append(t.settings, fn)
		return nil
	}
}

// New creates the transport without registering it, see Add.
func New(h host.Host, opts ...Option) (*Transport, error) {
	ctx, cancel := context.WithCancel(context.Background())
	t := &Transport{
		host:             h,
		rcmgr:            h.Network().ResourceManager(),
		disconnected:     DefaultDisconnectedTimeout,
		failed:           DefaultFailedTimeout,
		keepalive:        DefaultKeepaliveTimeout,
		handshakeTimeout: DefaultHandshakeTimeout,
		inflight:         make(chan struct{}, DefaultMaxInFlight),
		ctx:              ctx,
		cancel:           cancel,
	}
	if t.rcmgr == nil {
		t.rcmgr = &network.NullResourceManager{}
	}

	for _, opt := range opts {
		if err := opt(t); err != nil {
			cancel()
			return nil, err
		}
	}

	return t, nil
}

// Add creates the transport, registers it with the host's swarm, listens on /webrtc and serves the
// signaling protocol. call it once per host, after the host is constructed.
func Add(h host.Host, opts ...Option) (*Transport, error) {
	n, ok := h.Network().(tpt.TransportNetwork)
	if !ok {
		return nil, fmt.Errorf("%T is not a transport network", h.Network())
	}

	t, err := New(h, opts...)
	if err != nil {
		return nil, err
	}

	if err = n.AddTransport(t); err != nil {
		t.Close()
		return nil, fmt.Errorf("unable to add the webrtc transport: %w", err)
	}

	if err = n.Listen(ListenAddr); err != nil {
		t.Close()
		return nil, fmt.Errorf("unable to listen on %s: %w", ListenAddr, err)
	}

	h.SetStreamHandler(SignalingProtocol, t.handleSignaling)

	return t, nil
}

// Close stops accepting connections, connections already established are left to the swarm.
func (t *Transport) Close() error {
	t.once.Do(func() {
		t.host.RemoveStreamHandler(SignalingProtocol)
		t.cancel()
		if l := t.listener.Load(); l != nil {
			l.Close()
		}
	})
	return nil
}

// Protocols implements tpt.Transport.
func (t *Transport) Protocols() []int {
	return []int{ma.P_WEBRTC}
}

// Proxy implements tpt.Transport.
func (t *Transport) Proxy() bool {
	return false
}

// CanDial implements tpt.Transport. the swarm routes /p2p-circuit addresses to the relay transport
// before asking any other transport, so this transport never dials through the swarm, see Connect.
func (t *Transport) CanDial(ma.Multiaddr) bool {
	return false
}

var errUseConnect = errors.New("p2pwebrtc: dial through Transport.Connect, the swarm cannot dial /p2p-circuit/webrtc")

// Dial implements tpt.Transport, see CanDial.
func (t *Transport) Dial(context.Context, ma.Multiaddr, peer.ID) (tpt.CapableConn, error) {
	return nil, errUseConnect
}

// Listen implements tpt.Transport, laddr must be ListenAddr and only one listener can exist.
func (t *Transport) Listen(laddr ma.Multiaddr) (tpt.Listener, error) {
	if !laddr.Equal(ListenAddr) {
		return nil, fmt.Errorf("can only listen on %s, not %s", ListenAddr, laddr)
	}

	if t.ctx.Err() != nil {
		return nil, tpt.ErrListenerClosed
	}

	l := newListener(t)
	if !t.listener.CompareAndSwap(nil, l) {
		l.Close()
		return nil, errors.New("already listening")
	}

	return l, nil
}

// acquire reserves a handshake slot without waiting.
func (t *Transport) acquire() bool {
	select {
	case t.inflight <- struct{}{}:
		return true
	default:
		return false
	}
}

func (t *Transport) release() {
	<-t.inflight
}

// deliver hands an established connection to the swarm through the listener.
func (t *Transport) deliver(ctx context.Context, c *connection) error {
	l := t.listener.Load()
	if l == nil {
		return errors.New("not listening on " + ListenAddr.String())
	}
	return l.deliver(ctx, c)
}

func (t *Transport) gate(dir network.Direction, c *connection) error {
	if t.gater == nil {
		return nil
	}
	if dir == network.DirInbound && !t.gater.InterceptAccept(c) {
		return errors.New("connection gated")
	}
	if !t.gater.InterceptSecured(dir, c.remotePeer, c) {
		return errors.New("connection gated")
	}
	return nil
}
