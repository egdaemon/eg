package backoff

import (
	"math"
	"math/bits"
	"math/rand"
	"sync/atomic"
	"time"

	"github.com/egdaemon/eg/internal/timex"
)

// Strategy strategy to compute how long to wait before retrying message.
type Strategy interface {
	Backoff(attempt int64) time.Duration
}

// Option consumes a strategy and returns a new strategy.
type Option func(Strategy) Strategy

// Maximum sets an upper bound for the strategy.
func Maximum(d time.Duration) Option {
	return func(s Strategy) Strategy {
		return StrategyFunc(func(attempt int64) time.Duration {
			if x := s.Backoff(attempt); x < d {
				return x
			}

			return d
		})
	}
}

// Jitter set a jitter frame for the strategy.
func Jitter(multiplier float64) Option {
	return func(s Strategy) Strategy {
		return StrategyFunc(func(attempt int64) time.Duration {
			x := s.Backoff(attempt)
			if x == math.MaxInt64 {
				return x
			}

			d := math.Floor(float64(x) * multiplier)
			return timex.DurationMax(
				x,
				x+time.Duration(rand.Intn(int(d))),
			)
		})
	}
}

func JitterRandWindow(d time.Duration) Option {
	return func(s Strategy) Strategy {
		return StrategyFunc(func(attempt int64) time.Duration {
			delta := time.Duration(rand.Intn(int(d)/2) - rand.Intn(int(d)))
			x := s.Backoff(attempt)
			if x == math.MaxInt64 && delta > 0 {
				return x
			}

			return x + delta
		})
	}
}

// New backoff
func New(s Strategy, options ...Option) Strategy {
	for _, opt := range options {
		s = opt(s)
	}
	return s
}

// StrategyFunc convience helper to convert a pure function into a backoff strategy.
type StrategyFunc func(attempt int64) time.Duration

// Backoff implements Strategy
func (t StrategyFunc) Backoff(attempt int64) time.Duration {
	return t(attempt)
}

// Constant always returns the provided duration regardless of the attempt.
func Constant(d time.Duration) Strategy {
	return StrategyFunc(func(attempt int64) time.Duration {
		return d
	})
}

type exponential struct {
	scale time.Duration
}

func (t *exponential) Backoff(attempt int64) (exp time.Duration) {
	// if the exponential wraps around fall back to using maximum.
	exp = time.Duration(1 << uint64(attempt))
	if exp <= 0 {
		return time.Duration(math.MaxInt64)
	}

	hi, lo := bits.Mul64(uint64(exp), uint64(t.scale))

	// check if we overflowed into hi bits, or if the low bits
	// are negative.
	if hi != 0 || (lo)&(1<<63) == (1<<63) {
		return time.Duration(math.MaxInt64)
	}

	return time.Duration(lo)
}

// Exponential implements expoential backoff.
func Exponential(scale time.Duration) Strategy {
	if scale == 0 {
		panic("exponential backoff can't be scaled by 0")
	}

	return &exponential{
		scale: scale,
	}
}

// Explicit an explicit set of delays to use. if the attempt is larger than
// the number of values it restarts at the first delay.
func Explicit(delays ...time.Duration) Strategy {
	return explicit{delays: delays}
}

type explicit struct {
	delays []time.Duration
}

func (t explicit) Backoff(attempt int64) time.Duration {
	return t.delays[attempt%int64(len(t.delays))]
}

// Attempt with a backoff strategy.
func Attempt(d Strategy, do func(int64) int64) {
	attempt := do(0)
	for {
		duration := d.Backoff(attempt)
		// log.Println("BACKOFF ATTEMPT SLEEPING", duration)
		time.Sleep(duration)
		attempt = do(attempt)
	}
}

type Awaiter interface {
	Await(Strategy) <-chan time.Time
	Reset() // reset attempts
}

type awaiter struct {
	attempts int64
}

func (t *awaiter) Reset() {
	atomic.StoreInt64(&t.attempts, 0)
}

func (t *awaiter) Await(d Strategy) <-chan time.Time {
	defer atomic.AddInt64(&t.attempts, 1)
	delay := d.Backoff(t.attempts)
	return time.After(delay)
}

func Waiter() Awaiter {
	return &awaiter{}
}
