// adapted from github.com/libp2p/go-libp2p/p2p/transport/webrtc/connection.go (v0.50.0), MIT licensed, see LICENSE.go-libp2p.

package p2pwebrtc

import (
	"context"
	"errors"
	"fmt"
	"net"
	"sync"

	ic "github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	tpt "github.com/libp2p/go-libp2p/core/transport"

	ma "github.com/multiformats/go-multiaddr"
	"github.com/pion/datachannel"
	"github.com/pion/sctp"
	"github.com/pion/webrtc/v4"
)

var _ tpt.CapableConn = &connection{}

type errConnectionTimeout struct{}

var _ net.Error = &errConnectionTimeout{}

func (errConnectionTimeout) Error() string   { return "connection timeout" }
func (errConnectionTimeout) Timeout() bool   { return true }
func (errConnectionTimeout) Temporary() bool { return false }

var errConnClosed = errors.New("connection closed")

type dataChannel struct {
	stream  datachannel.ReadWriteCloser
	channel *webrtc.DataChannel
}

// connection is a libp2p connection over a webrtc peer connection, every stream is a datachannel.
// unlike webrtc-direct there is no noise handshake on channel 0: the peers authenticated each other
// on the relayed connection that carried the SDP, and the SDP carries the DTLS fingerprints.
// channel ids are picked by pion from the DTLS role (client even, server odd).
type connection struct {
	pc        *webrtc.PeerConnection
	transport *Transport
	scope     network.ConnManagementScope

	closeOnce sync.Once
	closeErr  error

	localPeer      peer.ID
	localMultiaddr ma.Multiaddr

	remotePeer      peer.ID
	remoteKey       ic.PubKey
	remoteMultiaddr ma.Multiaddr

	m       sync.Mutex
	streams map[uint16]*stream

	acceptQueue chan dataChannel

	ctx    context.Context
	cancel context.CancelFunc
}

func newConnection(
	pc *webrtc.PeerConnection,
	transport *Transport,
	scope network.ConnManagementScope,

	localPeer peer.ID,
	localMultiaddr ma.Multiaddr,

	remotePeer peer.ID,
	remoteKey ic.PubKey,
	remoteMultiaddr ma.Multiaddr,
	incomingDataChannels chan dataChannel,
	peerConnectionClosedCh chan struct{},
) (*connection, error) {
	ctx, cancel := context.WithCancel(context.Background())
	c := &connection{
		pc:        pc,
		transport: transport,
		scope:     scope,

		localPeer:      localPeer,
		localMultiaddr: localMultiaddr,

		remotePeer:      remotePeer,
		remoteKey:       remoteKey,
		remoteMultiaddr: remoteMultiaddr,
		ctx:             ctx,
		cancel:          cancel,
		streams:         make(map[uint16]*stream),

		acceptQueue: incomingDataChannels,
	}

	pc.OnConnectionStateChange(c.onConnectionStateChange)
	pc.SCTP().OnClose(func(err error) {
		if err != nil {
			c.closeWithError(fmt.Errorf("%w: %w", errConnClosed, err))
		}
		c.closeWithError(errConnClosed)
	})
	select {
	case <-peerConnectionClosedCh:
		c.Close()
		return nil, errConnClosed
	default:
	}
	// the state may have changed before the callback above was installed.
	c.onConnectionStateChange(pc.ConnectionState())
	if c.IsClosed() {
		return nil, c.closeErr
	}
	return c, nil
}

// ConnState implements transport.CapableConn
func (c *connection) ConnState() network.ConnectionState {
	return network.ConnectionState{Transport: TransportName}
}

// Close closes the underlying peerconnection.
func (c *connection) Close() error {
	c.closeWithError(errConnClosed)
	return nil
}

func (c *connection) As(target any) bool {
	if target, ok := target.(**webrtc.PeerConnection); ok {
		*target = c.pc
		return true
	}
	return false
}

// CloseWithError closes the connection ignoring the error code. As there's no way to signal
// the remote peer on closing the underlying peerconnection, we ignore the error code.
func (c *connection) CloseWithError(_ network.ConnErrorCode) error {
	return c.Close()
}

// closeWithError is used to Close the connection when the underlying DTLS connection fails
func (c *connection) closeWithError(err error) {
	c.closeOnce.Do(func() {
		c.closeErr = err
		// cancel must be called after closeErr is set. This ensures interested goroutines waiting on
		// ctx.Done can read closeErr without holding the conn lock.
		c.cancel()
		// closing peerconnection will close the datachannels associated with the streams
		c.pc.Close()

		c.m.Lock()
		streams := c.streams
		c.streams = nil
		c.m.Unlock()
		for _, s := range streams {
			s.closeForShutdown(err)
		}
		c.scope.Done()
	})
}

func (c *connection) IsClosed() bool {
	return c.ctx.Err() != nil
}

func (c *connection) OpenStream(ctx context.Context) (network.MuxedStream, error) {
	if c.IsClosed() {
		return nil, c.closeErr
	}

	// the spec requires an empty label, the id is assigned by pion from the dtls role.
	dc, err := c.pc.CreateDataChannel("", nil)
	if err != nil {
		return nil, err
	}
	rwc, err := c.detachChannel(ctx, dc)
	if err != nil {
		// There's a race between webrtc.SCTP.OnClose callback and the underlying
		// association closing. It's nicer to close the connection here.
		if errors.Is(err, sctp.ErrStreamClosed) {
			c.closeWithError(errConnClosed)
			return nil, c.closeErr
		}
		dc.Close()
		return nil, fmt.Errorf("detach channel failed: %w", err)
	}
	if dc.ID() == nil {
		rwc.Close()
		return nil, errors.New("datachannel opened without an id")
	}
	streamID := *dc.ID()
	str := newStream(dc, rwc, maxSendMessageSize, func() { c.removeStream(streamID) })
	if err := c.addStream(str); err != nil {
		str.Reset()
		return nil, fmt.Errorf("failed to add stream(%d) to connection: %w", streamID, err)
	}
	return str, nil
}

func (c *connection) AcceptStream() (network.MuxedStream, error) {
	select {
	case <-c.ctx.Done():
		return nil, c.closeErr
	case dc := <-c.acceptQueue:
		streamID := *dc.channel.ID()
		str := newStream(dc.channel, dc.stream, maxSendMessageSize, func() { c.removeStream(streamID) })
		if err := c.addStream(str); err != nil {
			str.Reset()
			return nil, err
		}
		return str, nil
	}
}

func (c *connection) LocalPeer() peer.ID            { return c.localPeer }
func (c *connection) RemotePeer() peer.ID           { return c.remotePeer }
func (c *connection) RemotePublicKey() ic.PubKey    { return c.remoteKey }
func (c *connection) LocalMultiaddr() ma.Multiaddr  { return c.localMultiaddr }
func (c *connection) RemoteMultiaddr() ma.Multiaddr { return c.remoteMultiaddr }
func (c *connection) Scope() network.ConnScope      { return c.scope }
func (c *connection) Transport() tpt.Transport      { return c.transport }

func (c *connection) addStream(str *stream) error {
	c.m.Lock()
	defer c.m.Unlock()
	if c.streams == nil {
		return c.closeErr
	}
	if _, ok := c.streams[str.id]; ok {
		return errors.New("stream ID already exists")
	}
	c.streams[str.id] = str
	return nil
}

func (c *connection) removeStream(id uint16) {
	c.m.Lock()
	defer c.m.Unlock()
	delete(c.streams, id)
}

func (c *connection) onConnectionStateChange(state webrtc.PeerConnectionState) {
	if state == webrtc.PeerConnectionStateFailed || state == webrtc.PeerConnectionStateClosed {
		c.closeWithError(errConnectionTimeout{})
	}
}

// detachChannel detaches an outgoing channel by taking into account the context
// passed to `OpenStream` as well the closure of the underlying peerconnection.
// see the go-libp2p original for why channels are detached rather than read through OnMessage.
func (c *connection) detachChannel(ctx context.Context, dc *webrtc.DataChannel) (datachannel.ReadWriteCloser, error) {
	done := make(chan struct{})

	var rwc datachannel.ReadWriteCloser
	var err error
	// OnOpen will return immediately for detached datachannels
	dc.OnOpen(func() {
		rwc, err = dc.Detach()
		// this is safe since the function should return instantly if the peerconnection is closed
		close(done)
	})
	select {
	case <-c.ctx.Done():
		return nil, c.closeErr
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		return rwc, err
	}
}
