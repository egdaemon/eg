package simnet

import (
	"log/slog"
	"net"
	"sync"
	"time"
)

func StaticLatency(duration time.Duration) func(*Packet) time.Duration {
	return func(*Packet) time.Duration {
		return duration
	}
}

// Simnet is a simulated network that manages connections between nodes
// with configurable network conditions.
type Simnet struct {
	// LatencyFunc defines the latency added when routing a given packet.
	// The latency is allowed to be dynamic and change packet to packet (which
	// could lead to packet reordering).
	//
	// A simple use case can use `StaticLatency(duration)` to set a static
	// latency for all packets.
	//
	// More complex use cases can define a latency map between endpoints and
	// have this function return the expected latency.
	LatencyFunc func(*Packet) time.Duration

	// Optional, if unset will use the default slog logger.
	Logger *slog.Logger

	started     bool
	closeSignal chan struct{}
	wg          sync.WaitGroup
	router      VariableLatencyRouter
	links       []*Simlink
}

// NodeBiDiLinkSettings defines the bidirectional link settings for a network node.
// It specifies separate configurations for downlink (incoming) and uplink (outgoing)
// traffic, allowing asymmetric network conditions to be simulated.
type NodeBiDiLinkSettings struct {
	// Downlink configures the settings for incoming traffic to this node
	Downlink LinkSettings
	// Uplink configures the settings for outgoing traffic from this node
	Uplink LinkSettings
}

// Start starts the simulated network and related goroutines
func (n *Simnet) Start() {
	n.init()
	n.started = true
	if n.Logger == nil {
		n.Logger = slog.Default()
	}
	// Log whenever the router fails to route a packet (likely a test setup bug).
	n.router.OnDrop = LogOnDrop(n.Logger)
	n.router.LatencyFunc = n.LatencyFunc
	n.router.CloseSignal = n.closeSignal
	n.router.Start(&n.wg)
	for _, link := range n.links {
		link.Start(&n.wg)
	}
}

func (n *Simnet) Close() {
	close(n.closeSignal)
	n.wg.Wait()
}

func (n *Simnet) init() {
	if n.closeSignal == nil {
		n.closeSignal = make(chan struct{})
	}
}

func (n *Simnet) NewEndpoint(addr *net.UDPAddr, linkSettings NodeBiDiLinkSettings) *SimConn {
	n.init()
	c := NewBlockingSimConn(addr)
	link := NewSimlink(
		n.closeSignal,
		linkSettings,
		&n.router,
		c,
	)
	c.SetUpPacketReceiver(link.up)
	n.router.AddNode(addr, link.down)

	n.links = append(n.links, link)
	if n.started {
		link.Start(&n.wg)
	}

	return c
}
