package simnet

import (
	"sync"
	"time"
)

const Mibps = 1_000_000

const DefaultFlowBucketCount = 128

// packetWithDeliveryTime holds a packet along with its delivery time and enqueue time
type packetWithDeliveryTime struct {
	Packet
	DeliveryTime time.Time
}

// LinkSettings defines the network characteristics for a simulated link direction
type LinkSettings struct {
	// BitsPerSecond specifies the bandwidth limit in bits per second
	BitsPerSecond int

	// MTU (Maximum Transmission Unit) specifies the maximum packet size in bytes
	MTU int

	// FlowBucketCount sets the number of flow buckets for FQ-CoDel. If zero
	// defaults to DefaultFlowBucketCount
	FlowBucketCount int
}

// Simlink simulates a bidirectional network link with variable latency,
// bandwidth limiting, and CoDel-based bufferbloat mitigation
type Simlink struct {
	up   *linkDriver
	down *linkDriver
}

func NewSimlink(
	closeSignal chan struct{},
	linkSettings NodeBiDiLinkSettings,
	upPacketReceiver PacketReceiver,
	downPacketReceiver PacketReceiver,
) *Simlink {
	const (
		target     = 5 * time.Millisecond
		interval   = 100 * time.Millisecond
		defaultMTU = 1500
	)

	if linkSettings.Uplink.MTU == 0 {
		linkSettings.Uplink.MTU = defaultMTU
	}
	if linkSettings.Downlink.MTU == 0 {
		linkSettings.Downlink.MTU = defaultMTU
	}

	if linkSettings.Uplink.FlowBucketCount == 0 {
		linkSettings.Uplink.FlowBucketCount = DefaultFlowBucketCount
	}
	if linkSettings.Downlink.FlowBucketCount == 0 {
		linkSettings.Downlink.FlowBucketCount = DefaultFlowBucketCount
	}

	return &Simlink{
		up: newLinkDriver(
			target, interval,
			linkSettings.Uplink.MTU,
			linkSettings.Uplink.FlowBucketCount,
			linkSettings.Uplink.MTU, linkSettings.Uplink.BitsPerSecond,
			upPacketReceiver,
			closeSignal,
		),
		down: newLinkDriver(
			target, interval,
			linkSettings.Downlink.MTU,
			linkSettings.Downlink.FlowBucketCount,
			linkSettings.Downlink.MTU, linkSettings.Downlink.BitsPerSecond,
			downPacketReceiver,
			closeSignal,
		),
	}
}

func (l *Simlink) Start(wg *sync.WaitGroup) {
	l.up.Start(wg)
	l.down.Start(wg)
}

type linkDriver struct {
	newPacket   chan Packet
	q           fqCoDel
	rateLink    *RateLink
	closeSignal chan struct{}
}

func newLinkDriver(
	target, interval time.Duration,
	quantum int,
	flowCount int,
	mtu int, bandwidth int,
	receiver PacketReceiver,
	closeSignal chan struct{}) *linkDriver {
	return &linkDriver{
		newPacket:   make(chan Packet, 1_024),
		q:           newFqCoDel(target, interval, quantum, flowCount),
		closeSignal: closeSignal,
		rateLink:    NewRateLink(bandwidth, mtu, receiver),
	}
}

func (d *linkDriver) RecvPacket(p Packet) {
	d.newPacket <- p
}

func (d *linkDriver) Start(wg *sync.WaitGroup) {
	wgGo(wg, func() {
		deqTimer := time.NewTimer(0)
		deqTimer.Stop()
		var pendingPacket *Packet

		for {
			select {
			case <-d.closeSignal:
				return
			case packet := <-d.newPacket:
				d.q.Enqueue(packet)
				if pendingPacket == nil {
					deqTimer.Reset(0)
				}
			case <-deqTimer.C:
				for {
					if pendingPacket != nil {
						d.rateLink.RecvPacket(*pendingPacket)
						pendingPacket = nil
					}

					p, ok := d.q.Dequeue()
					if ok {
						pendingPacket = &p
						now := time.Now()
						if d.rateLink.AllowN(now, len(p.buf)) {
							continue
						}
						delayDeq := d.rateLink.Reserve(now, len(p.buf))
						deqTimer.Reset(delayDeq)
					}
					break
				}
			}
		}
	})
}
