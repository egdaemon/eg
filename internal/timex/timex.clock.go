package timex

import (
	"sync"
	"time"
)

// AdjustableClock is a Clock whose current time is set explicitly rather
// than read from the system clock. Intended for tests that need to control
// time deterministically instead of sleeping for real durations.
type AdjustableClock struct {
	mu  sync.Mutex
	now time.Time
}

// NewAdjustableClock returns an AdjustableClock initialized to now.
func NewAdjustableClock(now time.Time) *AdjustableClock {
	return &AdjustableClock{now: now}
}

func (t *AdjustableClock) Now() time.Time {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.now
}

// Set the clock's current time.
func (t *AdjustableClock) Set(now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.now = now
}

// Advance the clock's current time by d.
func (t *AdjustableClock) Advance(d time.Duration) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.now = t.now.Add(d)
}
