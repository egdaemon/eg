package simnet

import (
	"time"

	"golang.org/x/time/rate"
)

type RateLink struct {
	*rate.Limiter
	BitsPerSecond int
	Receiver      PacketReceiver
}

// Creates a new RateLimiter with the following parameters:
// bandwidth (in bits/sec).
// burstSize is in Bytes
func newRateLimiter(bandwidth int, burstSize int) *rate.Limiter {
	// Convert bandwidth from bits/sec to bytes/sec
	bytesPerSecond := rate.Limit(float64(bandwidth) / 8.0)
	return rate.NewLimiter(bytesPerSecond, burstSize)
}

func NewRateLink(bandwidth int, burstSize int, receiver PacketReceiver) *RateLink {
	return &RateLink{
		Limiter:  newRateLimiter(bandwidth, burstSize),
		Receiver: receiver,
	}
}

func (l *RateLink) Reserve(now time.Time, packetSize int) time.Duration {
	r := l.Limiter.ReserveN(now, packetSize)
	return r.DelayFrom(now)
}

func (l *RateLink) RecvPacket(p Packet) {
	l.Receiver.RecvPacket(p)
}
