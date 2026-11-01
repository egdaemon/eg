// Package egx holds experimental eg functionality, its api may change or move into the core runtime.
package egx

import (
	"context"
	"log"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/runtime/wasi/eg"
)

// Retrier retries an operation when it fails. it is configured by chaining, each method returns
// a new Retrier and leaves the receiver unchanged.
//
//	egx.Retry(5).Delay(30 * time.Second).Run(op)
type Retrier struct {
	attempts int
	strategy backoff.Strategy
}

// Retry creates a Retrier that runs an operation at most attempts times, zero or less retries until the
// context ends. until configured it waits a second between attempts.
func Retry(attempts int) Retrier {
	return Retrier{attempts: attempts, strategy: backoff.Constant(time.Second)}
}

// Delay waits d between attempts, it replaces any previous Delay or Backoff.
func (t Retrier) Delay(d time.Duration) Retrier {
	return t.Backoff(backoff.Constant(d))
}

// Backoff waits between attempts as the strategy dictates, it replaces any previous Delay or Backoff.
func (t Retrier) Backoff(s backoff.Strategy) Retrier {
	t.strategy = s
	return t
}

// Exponential waits scale * 2^attempt between attempts, doubling after each failure, it replaces any
// previous Delay or Backoff. scale must not be zero.
func (t Retrier) Exponential(scale time.Duration) Retrier {
	return t.Backoff(backoff.Exponential(scale))
}

// Cycle waits first, then the remaining delays in order, between attempts, starting over at first once
// they are used up. it replaces any previous Delay or Backoff.
func (t Retrier) Cycle(first time.Duration, delays ...time.Duration) Retrier {
	cycle := backoff.Cycle(append([]time.Duration{first}, delays...)...)
	// the first retry is attempt 1, shift it so the first delay is the first one waited.
	return t.Backoff(backoff.StrategyFunc(func(attempt int64) time.Duration {
		return cycle.Backoff(attempt - 1)
	}))
}

// Run creates the operation that runs op, retrying it when it fails. when the attempts are used up or
// the context ends the last error of op is returned.
func (t Retrier) Run(op eg.OpFn) eg.OpFn {
	return func(ctx context.Context, o eg.Op) (err error) {
		for tries := 1; ; tries++ {
			if err = op(ctx, o); err == nil {
				return nil
			}

			if t.attempts > 0 && tries >= t.attempts {
				return err
			}

			log.Println("operation failed, retrying", err)

			select {
			case <-ctx.Done():
				return err
			case <-time.After(t.strategy.Backoff(int64(tries))):
			}
		}
	}
}
