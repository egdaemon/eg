package simnet

import "time"

type packetWithDeliveryTimeAndOrder struct {
	*Packet
	order        int
	deliveryTime time.Time
}

// packetHeap implements heap.Interface ordered by packet delivery time.
type packetHeap []packetWithDeliveryTimeAndOrder

func (h packetHeap) Len() int { return len(h) }

func (h packetHeap) Less(i, j int) bool {
	return (h[i].deliveryTime.Before(h[j].deliveryTime) ||
		h[i].deliveryTime.Equal(h[j].deliveryTime) && h[i].order < h[j].order)
}

func (h packetHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

func (h *packetHeap) Push(x any) {
	*h = append(*h, x.(packetWithDeliveryTimeAndOrder))
}

func (h *packetHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	*h = old[:n-1]
	return item
}
