package simnet

import "time"

type packetWithDeliveryTimeAndOrder struct {
	*Packet
	order        int
	deliveryTime time.Time
}

// packetHeap is a min-heap ordered by delivery time, then order.
// It avoids container/heap to eliminate interface{} boxing allocations.
type packetHeap []packetWithDeliveryTimeAndOrder

func (h packetHeap) less(i, j int) bool {
	return (h[i].deliveryTime.Before(h[j].deliveryTime) ||
		h[i].deliveryTime.Equal(h[j].deliveryTime) && h[i].order < h[j].order)
}

func (h *packetHeap) push(x packetWithDeliveryTimeAndOrder) {
	*h = append(*h, x)
	h.siftUp(len(*h) - 1)
}

func (h *packetHeap) pop() packetWithDeliveryTimeAndOrder {
	old := *h
	n := len(old)
	item := old[0]
	old[0] = old[n-1]
	old[n-1] = packetWithDeliveryTimeAndOrder{} // clear for GC
	*h = old[:n-1]
	if len(*h) > 0 {
		h.siftDown(0)
	}
	return item
}

func (h packetHeap) siftUp(i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if !h.less(i, parent) {
			break
		}
		h[i], h[parent] = h[parent], h[i]
		i = parent
	}
}

func (h packetHeap) siftDown(i int) {
	n := len(h)
	for {
		left := 2*i + 1
		if left >= n {
			break
		}
		j := left
		if right := left + 1; right < n && h.less(right, left) {
			j = right
		}
		if !h.less(j, i) {
			break
		}
		h[i], h[j] = h[j], h[i]
		i = j
	}
}
