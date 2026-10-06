package simnet

import (
	"container/heap"
	"fmt"
	"log/slog"
	"net"
	"net/netip"
	"sync"
	"time"
)

type DropReason string

const (
	DropReasonUnknownDestination DropReason = "unknown destination"
	DropReasonUnknownSource      DropReason = "unknown source"
	DropReasonFirewalled         DropReason = "Packet firewalled"
)

type OnDrop func(packet Packet, reason DropReason)

func LogOnDrop(logger *slog.Logger) OnDrop {
	return func(packet Packet, reason DropReason) {
		logger.Error("Dropping packet", "from", packet.From, "to", packet.To, "reason", reason)
	}
}

type ipPortKey struct {
	ip    string
	port  uint16
	isUDP bool
}

func (k *ipPortKey) FromNetAddr(addr net.Addr) error {
	switch addr := addr.(type) {
	case *net.UDPAddr:
		*k = ipPortKey{
			ip:    string(addr.IP),
			port:  uint16(addr.Port),
			isUDP: true,
		}
		return nil
	case *net.TCPAddr:
		*k = ipPortKey{
			ip:    string(addr.IP),
			port:  uint16(addr.Port),
			isUDP: false,
		}
		return nil
	default:
		ip, err := netip.ParseAddrPort(addr.String())
		if err != nil {
			return err
		}
		*k = ipPortKey{
			ip:    string(ip.Addr().AsSlice()),
			port:  ip.Port(),
			isUDP: addr.Network() == "udp",
		}
		return nil
	}
}

type addrMap[V any] struct {
	mu    sync.Mutex
	nodes map[ipPortKey]V
}

func (m *addrMap[V]) Get(addr net.Addr) (V, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var v V
	if len(m.nodes) == 0 {
		return v, false
	}
	var k ipPortKey
	if err := k.FromNetAddr(addr); err != nil {
		return v, false
	}
	v, ok := m.nodes[k]
	return v, ok
}

func (m *addrMap[V]) Set(addr net.Addr, v V) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nodes == nil {
		m.nodes = make(map[ipPortKey]V)
	}

	var k ipPortKey
	if err := k.FromNetAddr(addr); err != nil {
		return err
	}
	m.nodes[k] = v
	return nil
}

func (m *addrMap[V]) Delete(addr net.Addr) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.nodes == nil {
		m.nodes = make(map[ipPortKey]V)
	}

	var k ipPortKey
	if err := k.FromNetAddr(addr); err != nil {
		return err
	}
	delete(m.nodes, k)
	return nil
}

// PerfectRouter is a router that has no latency or jitter and can route to
// every node
type PerfectRouter struct {
	OnDrop OnDrop
	nodes  addrMap[PacketReceiver]
}

func (r *PerfectRouter) RecvPacket(p Packet) {
	conn, ok := r.nodes.Get(p.To)
	if !ok {
		if r.OnDrop != nil {
			r.OnDrop(p, DropReasonUnknownDestination)
		}
		return
	}

	conn.RecvPacket(p)
}

func (r *PerfectRouter) AddNode(addr net.Addr, conn PacketReceiver) {
	r.nodes.Set(addr, conn)
}

func (r *PerfectRouter) RemoveNode(addr net.Addr) {
	r.nodes.Delete(addr)
}

var _ Router = &PerfectRouter{}

type VariableLatencyRouter struct {
	PerfectRouter
	LatencyFunc func(packet *Packet) time.Duration
	CloseSignal chan struct{}

	packets     chan Packet
	packetCount int
	h           packetHeap
}

func (r *VariableLatencyRouter) RecvPacket(p Packet) {
	r.packets <- p
}

// wgGo is the same as Go 1.25's wg.Go. Remove this when Go 1.26 is out
func wgGo(wg *sync.WaitGroup, f func()) {
	wg.Add(1)
	go func() {
		defer wg.Done()
		f()
	}()
}

func (r *VariableLatencyRouter) Start(wg *sync.WaitGroup) {
	r.packets = make(chan Packet, 128)
	heap.Init(&r.h)

	wgGo(wg, func() {
		var nextDelivery time.Time
		deliveryTimer := time.NewTimer(0)
		deliveryTimer.Stop()

		for {
			select {
			case <-r.CloseSignal:
				return
			case p := <-r.packets:
				r.packetCount++
				latency := r.LatencyFunc(&p)
				deliveryTime := time.Now().Add(latency)
				heap.Push(&r.h, packetWithDeliveryTimeAndOrder{
					Packet:       &p,
					order:        r.packetCount,
					deliveryTime: deliveryTime,
				})
				if nextDelivery.IsZero() || deliveryTime.Before(nextDelivery) {
					nextDelivery = deliveryTime
					deliveryTimer.Reset(latency)
				}
			case <-deliveryTimer.C:
				now := time.Now()
				for len(r.h) > 0 && !r.h[0].deliveryTime.After(now) {
					p := heap.Pop(&r.h).(packetWithDeliveryTimeAndOrder).Packet
					r.PerfectRouter.RecvPacket(*p)
				}
				if len(r.h) > 0 {
					nextDelivery = r.h[0].deliveryTime
					deliveryTimer.Reset(nextDelivery.Sub(now))
				} else {
					nextDelivery = time.Time{}
				}
			}
		}
	})
}

type simpleNodeFirewall struct {
	mu                sync.Mutex
	publiclyReachable bool
	packetsOutTo      map[string]struct{}
	node              PacketReceiver
}

func (f *simpleNodeFirewall) MarkPacketSentOut(p Packet) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.packetsOutTo == nil {
		f.packetsOutTo = make(map[string]struct{})
	}
	f.packetsOutTo[p.To.String()] = struct{}{}
}

func (f *simpleNodeFirewall) IsPacketInAllowed(p Packet) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.publiclyReachable {
		return true
	}

	_, ok := f.packetsOutTo[p.From.String()]
	return ok
}

func (f *simpleNodeFirewall) String() string {
	return fmt.Sprintf("public: %v, packetsOutTo: %v", f.publiclyReachable, f.packetsOutTo)
}

type SimpleFirewallRouter struct {
	OnDrop                 OnDrop
	mu                     sync.Mutex
	nodes                  map[string]*simpleNodeFirewall
	publiclyReachableAddrs map[string]bool
}

func (r *SimpleFirewallRouter) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	nodes := make([]string, 0, len(r.nodes))
	for _, node := range r.nodes {
		nodes = append(nodes, node.String())
	}
	return fmt.Sprintf("%v", nodes)
}

func (r *SimpleFirewallRouter) SetAddrPubliclyReachable(addr net.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.publiclyReachableAddrs == nil {
		r.publiclyReachableAddrs = make(map[string]bool)
	}
	r.publiclyReachableAddrs[addr.String()] = true
}

func (r *SimpleFirewallRouter) RecvPacket(p Packet) {
	r.mu.Lock()
	defer r.mu.Unlock()
	toNode, exists := r.nodes[p.To.String()]
	if !exists {
		if r.OnDrop != nil {
			r.OnDrop(p, DropReasonUnknownDestination)
		}
		return
	}

	// Record that this node is sending a packet to the destination
	fromNode, exists := r.nodes[p.From.String()]
	if !exists {
		if r.OnDrop != nil {
			r.OnDrop(p, DropReasonUnknownSource)
		}
		return
	}
	fromNode.MarkPacketSentOut(p)

	if !toNode.IsPacketInAllowed(p) {
		if r.OnDrop != nil {
			r.OnDrop(p, DropReasonFirewalled)
		}
		return
	}

	toNode.node.RecvPacket(p)
}

func (r *SimpleFirewallRouter) AddNode(addr net.Addr, conn PacketReceiver) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.nodes == nil {
		r.nodes = make(map[string]*simpleNodeFirewall)
	}

	if publiclyReachable := r.publiclyReachableAddrs[addr.String()]; publiclyReachable {
		r.nodes[addr.String()] = &simpleNodeFirewall{
			publiclyReachable: true,
			node:              conn,
		}
		return
	}
	r.nodes[addr.String()] = &simpleNodeFirewall{
		packetsOutTo: make(map[string]struct{}),
		node:         conn,
	}
}

func (r *SimpleFirewallRouter) RemoveNode(addr net.Addr) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.nodes == nil {
		return
	}
	delete(r.nodes, addr.String())
}

var _ Router = &SimpleFirewallRouter{}
