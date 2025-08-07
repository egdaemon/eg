package timex

import (
	"context"
	"log"
	"math"
	"time"
)

// Inf - positive infinity no time can be larger.
// see https://stackoverflow.com/questions/25065055/what-is-the-maximum-time-time-in-go/32620397
func Inf() time.Time {
	return time.Unix(math.MaxInt64-62135596800, 999999999)
}

func NegInf() time.Time {
	return time.Unix(math.MinInt64, math.MinInt64)
}

// Run the provided function after the duration.
func After(d time.Duration, do func()) {
	go func() {
		log.Println("sleepy")
		time.Sleep(d)
		log.Println("awake")
		do()
	}()
}

// Every executes the provided function every duration.
func Every(d time.Duration, do func()) {
	for range time.Tick(d) {
		do()
	}
}

// NowAndEvery executes the provided function immeditately and every duration.
func NowAndEvery(ctx context.Context, d time.Duration, do func(context.Context) error) error {
	if err := do(ctx); err != nil {
		return err
	}

	t := time.NewTicker(d)
	defer t.Stop()

	for {
		select {
		case <-t.C:
			if err := do(ctx); err != nil {
				return err
			}
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// DurationFirstNonZero ...
func DurationFirstNonZero(d ...time.Duration) time.Duration {
	for _, a := range d {
		if a == 0 {
			continue
		}

		return a
	}

	return 0
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

// RFC3339NanoMax truncate to the maximum value for RFC3339.
func RFC3339NanoMax(t time.Time) time.Time {
	ts := RFC3339Inf()
	if t.Before(ts) {
		return t
	}

	return ts
}

// RFC3339NanoMin truncate to the minimum value for RFC3339.
func RFC3339NanoMin(t time.Time) time.Time {
	ts := RFC3339NegInf()
	if t.After(ts) && !t.Equal(NegInf()) {
		return t
	}

	return ts
}

// RFC3339NanoEncode truncate time to RFC3339NanoEncode
func RFC3339NanoEncode(t time.Time) time.Time {
	return RFC3339NanoMax(RFC3339NanoMin(t))
}

// RFC3339Nano truncate time to RFC3339Nano
func RFC3339NanoDecode(t time.Time) time.Time {
	return RFC3339NanoMaxDecode(RFC3339NanoMinDecode(t))
}

// RFC3339NanoMinDecode convert minimum value for RFC3339 to time.Time.
func RFC3339NanoMinDecode(t time.Time) time.Time {
	ts := RFC3339NegInf()
	if t.After(ts) && !t.Equal(ts) {
		return t
	}

	return NegInf()
}

// RFC3339NanoMaxDecode truncate to the maximum value for RFC3339.
func RFC3339NanoMaxDecode(t time.Time) time.Time {
	ts := RFC3339Inf()
	if t.Before(ts) || !t.Equal(ts) {
		return t
	}

	return Inf()
}

// RFC3339NegInf neg infinity representation
func RFC3339NegInf() time.Time {
	return time.Date(0000, 01, 1, 1, 1, 1, 0, time.UTC)
}

// RFC3339Inf  infinity representation
func RFC3339Inf() time.Time {
	return time.Date(9999, time.December, 31, 23, 59, 59, 999000000, time.UTC)
}
