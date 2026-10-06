package simnet

import (
	"hash/maphash"
	"time"
)

type coDelQueueWithCredits struct {
	coDelQueue
	credits int
	bucket  int
}

// fqCoDel is an implementation of FQ-CoDel per RFC 8290.
type fqCoDel struct {
	flows []*coDelQueueWithCredits

	newFlows        linkedList[*coDelQueueWithCredits]
	oldFlows        linkedList[*coDelQueueWithCredits]
	activeFlowCount int

	quantum  int
	target   time.Duration
	interval time.Duration

	hash maphash.Hash

	// Note: No packet limit is implemented in this fqCoDel implementation. This
	// is not for production use.
}

func newFqCoDel(target, interval time.Duration, quantum, flowCount int) fqCoDel {
	return fqCoDel{
		flows:           make([]*coDelQueueWithCredits, flowCount),
		activeFlowCount: 0,

		quantum:  quantum,
		target:   target,
		interval: interval,
	}
}

func (q *fqCoDel) Enqueue(p Packet) {
	bucket := int(p.Hash(&q.hash) % uint64(len(q.flows)))

	fq := q.flows[bucket]
	if fq == nil {
		fq = &coDelQueueWithCredits{
			coDelQueue: newCoDelQueue(q.target, q.interval),
			credits:    q.quantum,
			bucket:     bucket,
		}
		q.flows[bucket] = fq
		q.newFlows.append(&listNode[*coDelQueueWithCredits]{v: fq})
	}

	fq.Enqueue(p)
}

// Dequeue implements the Fq-CoDel Dequeue algorithm. The state transition of
// queues between new, old, and empty are represented by this diagram.
//
/* +-----------------+                +------------------+
/* |                 |     Empty      |                  |
/* |     Empty       |<---------------+       Old        +----+
/* |                 |                |                  |    |
/* +-------+---------+                +------------------+    |
/*         |                             ^            ^       |Credits
/*         |Arrival                      |            |       |Exhausted
/*         v                             |            |       |
/* +-----------------+                   |            |       |
/* |                 |      Empty or     |            |       |
/* |      New        +-------------------+            +-------+
/* |                 | Credits Exhausted
/* +-----------------+
*/
func (q *fqCoDel) Dequeue() (Packet, bool) {

	for !q.newFlows.empty() {
		fq := q.newFlows.peek()
		if fq.credits < 0 {
			// For the first part, the scheduler first looks at the list of new
			// queues; for the queue at the head of that list, if that queue has a
			// negative number of credits (i.e., it has already dequeued at least a
			// quantum of bytes), it is given an additional quantum of credits, the
			// queue is put onto _the end of_ the list of old queues, and the
			// routine selects the next queue and starts again.
			fq.credits += q.quantum
			n := q.newFlows.removeFirst()
			q.oldFlows.append(n)

			continue
		}

		// Otherwise, that queue is selected for dequeue.
		if p, ok := fq.Dequeue(); ok {
			fq.credits -= len(p.buf)
			return p, true
		} else {
			// If the CoDel algorithm does not return a packet, then the
			// queue must be empty, and the scheduler does one of two things. If
			// the queue selected for dequeue came from the list of new queues, it
			// is moved to _the end of_ the list of old queues.
			//
			// The step that moves an empty queue from the list of new queues to the
			// end of the list of old queues before it is removed is crucial to
			// prevent starvation.  Otherwise, the queue could reappear (the next
			// time a packet arrives for it) before the list of old queues is
			// visited; this can go on indefinitely, even with a small number of
			// active flows, if the flow providing packets to the queue in question
			// transmits at just the right rate.  This is prevented by first moving
			// the queue to the end of the list of old queues, forcing the scheduler
			// to service all old queues before the empty queue is removed and thus
			// preventing starvation.
			n := q.newFlows.removeFirst()
			q.oldFlows.append(n)
			continue
		}
	}

	// If the list of new queues is empty, the scheduler proceeds down the
	// list of old queues in the same fashion (checking the credits and
	// either selecting the queue for dequeueing or adding credits and
	// putting the queue back at the end of the list).
	for !q.oldFlows.empty() {
		fq := q.oldFlows.peek()
		if fq.credits < 0 {
			fq.credits += q.quantum
			n := q.oldFlows.removeFirst()
			q.oldFlows.append(n)
			continue
		}

		if p, ok := fq.Dequeue(); ok {
			// If, instead, the scheduler _did_ get a packet back from the CoDel
			// algorithm, it subtracts the size of the packet from the byte credits
			// for the selected queue and returns the packet as the result of the
			// dequeue operation.
			fq.credits -= len(p.buf)
			return p, true
		} else {
			// Finally, if the CoDel algorithm does not return a packet, then the
			// queue must be empty, and the scheduler does one of two things.  If
			// the queue selected for dequeue came from the list of new queues, it
			// is moved to _the end of_ the list of old queues.  If instead it came
			// from the list of old queues, that queue is removed from the list, to
			// be added back (as a new queue) the next time a packet arrives that
			// hashes to that queue.  Then (since no packet was available for
			// dequeue), the whole dequeue process is restarted from the beginning.
			q.oldFlows.removeFirst()
			q.flows[fq.bucket] = nil
			continue
		}
	}

	return Packet{}, false
}

type linkedList[T any] struct {
	head *listNode[T]
	tail *listNode[T]
}

func (l *linkedList[T]) append(item *listNode[T]) {
	item.next = nil
	if l.tail == nil {
		l.head = item
		l.tail = l.head
	} else {
		l.tail.next = item
		l.tail = l.tail.next
	}
}

func (l *linkedList[T]) empty() bool {
	return l.head == nil
}

func (l *linkedList[T]) peek() T {
	return l.head.v
}

func (l *linkedList[T]) removeFirst() *listNode[T] {
	if l.head == nil {
		return nil
	}
	node := l.head
	l.head = l.head.next
	if l.head == nil {
		l.tail = nil
	}
	node.next = nil
	return node
}

type listNode[T any] struct {
	v    T
	next *listNode[T]
}
