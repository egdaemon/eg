package p2pwebrtc

import (
	"context"
	"net"

	tpt "github.com/libp2p/go-libp2p/core/transport"
	ma "github.com/multiformats/go-multiaddr"
)

// listener hands the connections established over signaling streams to the swarm. it does not own a
// socket, the swarm's accept loop is the only way for a transport to add a connection to the swarm.
// the swarm records every connection it accepts as inbound, including the ones made by Connect.
type listener struct {
	t      *Transport
	conns  chan *connection
	ctx    context.Context
	cancel context.CancelFunc
}

var _ tpt.Listener = &listener{}

func newListener(t *Transport) *listener {
	ctx, cancel := context.WithCancel(t.ctx)
	return &listener{
		t:      t,
		conns:  make(chan *connection),
		ctx:    ctx,
		cancel: cancel,
	}
}

func (t *listener) Accept() (tpt.CapableConn, error) {
	select {
	case c := <-t.conns:
		return c, nil
	case <-t.ctx.Done():
		return nil, tpt.ErrListenerClosed
	}
}

func (t *listener) deliver(ctx context.Context, c *connection) error {
	select {
	case t.conns <- c:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-t.ctx.Done():
		return tpt.ErrListenerClosed
	}
}

func (t *listener) Close() error {
	t.cancel()
	t.t.listener.CompareAndSwap(t, nil)
	return nil
}

func (t *listener) Addr() net.Addr {
	return addr{}
}

func (t *listener) Multiaddr() ma.Multiaddr {
	return ListenAddr
}

type addr struct{}

func (addr) Network() string { return TransportName }
func (addr) String() string  { return ListenAddr.String() }
