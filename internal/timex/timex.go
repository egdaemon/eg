package timex

import (
	"math"
	"time"
)

// Every executes the provided function every duration.
func Every(d time.Duration, do func()) {
	for range time.Tick(d) {
		do()
	}
}

// NowAndEvery executes the provided function immeditately and every duration.
func NowAndEvery(d time.Duration, do func()) {
	do()
	for range time.Tick(d) {
		do()
	}
}

// DurationOrDefault ...
func DurationOrDefault(a, b time.Duration) time.Duration {
	if a == 0 {
		return b
	}
	return a
}

// DurationMax select the maximum duration from the set.
func DurationMax(ds ...time.Duration) (d time.Duration) {
	for _, c := range ds {
		if c > d {
			d = c
		}
	}

	return d
}

// DurationMin select the minimum duration from the set.
func DurationMin(ds ...time.Duration) (d time.Duration) {
	d = math.MaxInt64

	for _, c := range ds {
		if c < d {
			d = c
		}
	}

	return d
}

// SafeReset stops and drains the timer (if necessary) and then resets.
func SafeReset(t *time.Timer, d time.Duration) {
	if !t.Stop() {
		select {
		case <-t.C:
		default:
		}
	}
	t.Reset(d)
}

type Clock struct{}

func (t Clock) Now() time.Time {
	return time.Now()
}
