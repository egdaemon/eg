package simnet

import (
	"math"
	"time"
)

// coDelQueue is a FIFO queue with CoDel bufferbloat control.
// Refer to RFC 8289
type coDelQueue struct {
	q ringBuffer[packetWithTimestamp]

	byteCount uint64
	MTU       uint16

	// CoDel state
	target   time.Duration // target queue delay (e.g., 5ms)
	interval time.Duration // interval for sustained bad queue (e.g., 100ms)

	dropping   bool
	firstAbove time.Time
	dropNext   time.Time
	count      int
	lastCount  int
}

type packetWithTimestamp struct {
	Packet
	ts time.Time
}

func newCoDelQueue(target, interval time.Duration) coDelQueue {
	return coDelQueue{
		target:   target,
		interval: interval,
		q:        newRingBuffer[packetWithTimestamp](128),
	}
}

// Enqueue adds a packet to the queue
func (q *coDelQueue) Enqueue(p Packet) {
	q.byteCount += uint64(len(p.buf))
	q.q.PushBack(packetWithTimestamp{p, time.Now()})
}

// Dequeue removes and returns the next packet when it's ready for delivery
// This blocks until a packet is available AND its delivery time has been reached
// Uses a timer that can be reset if a packet with earlier delivery time arrives
func (q *coDelQueue) Dequeue() (Packet, bool) {
	now := time.Now()
	p, okayToDrop := q.doDequeue(now)

	if q.dropping {
		if !okayToDrop {
			// sojourn below target leave dropping
			q.dropping = false
		}

		for !now.Before(q.dropNext) && q.dropping {
			// implicitly drop the packet
			q.drop(p)
			q.count++
			p, okayToDrop = q.doDequeue(now)
			if !okayToDrop {
				// leave drop state
				q.dropping = false
			} else {
				// schedule next drop
				q.dropNext = controlLaw(q.dropNext, q.interval, q.count)
			}
		}
	} else if okayToDrop {
		// If we get here, we're not in drop state. The `okToDrop`
		// return from doDequeue means that the sojourn time has been above
		// 'TARGET' for 'INTERVAL', so enter drop state.
		q.drop(p)
		p, _ = q.doDequeue(now)
		q.dropping = true

		// If min went above TARGET close to when it last went
		// below, assume that the drop rate that controlled the
		// queue on the last cycle is a good starting point to
		// control it now. (`dropNext` will be at most 'INTERVAL'
		// later than the time of the last drop, so 'now - dropNext'
		// is a good approximation of the time from the last drop
		// until now.) Implementations vary slightly here; this is
		// the Linux version, which is more widely deployed and
		// tested.
		delta := q.count - q.lastCount
		q.count = 1
		if delta > 1 && now.Sub(q.dropNext) < 16*q.interval {
			q.count = delta
		}

		q.dropNext = controlLaw(now, q.interval, q.count)
		q.lastCount = q.count
	}

	return p.Packet, len(p.Packet.buf) > 0
}

func (q *coDelQueue) drop(p packetWithTimestamp) {
	// TODO add stats
}

func (q *coDelQueue) doDequeue(now time.Time) (p packetWithTimestamp, okToDrop bool) {
	if q.q.Empty() {
		q.firstAbove = time.Time{}
		return
	}

	p = q.q.PopFront()
	q.byteCount -= uint64(len(p.Packet.buf))
	sojournTime := now.Sub(p.ts)
	if sojournTime < q.target || q.byteCount < uint64(q.MTU) {
		q.firstAbove = time.Time{}
	} else {
		if q.firstAbove.IsZero() {
			// Just went above from below. If still above later will say it's
			// okay to drop
			q.firstAbove = now.Add(q.interval)
		} else if !now.Before(q.firstAbove) {
			okToDrop = true
		}
	}

	return
}

func controlLaw(t time.Time, interval time.Duration, count int) time.Time {
	return t.Add(time.Duration(
		float64(time.Second) *
			(interval.Seconds() / math.Sqrt(float64(count)))))
}
