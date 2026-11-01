package egx_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egx"
	"github.com/stretchr/testify/require"
)

func TestRetry(t *testing.T) {
	t.Run("retries until the operation succeeds", func(t *testing.T) {
		attempts := 0
		op := func(context.Context, eg.Op) error {
			attempts++
			if attempts < 3 {
				return errors.New("not yet")
			}
			return nil
		}

		fn := egx.Retry(0).Delay(time.Millisecond).Run(op)
		require.NoError(t, fn(t.Context(), nil))
		require.Equal(t, 3, attempts)
	})

	t.Run("attempts limits the tries and returns the last error", func(t *testing.T) {
		attempts := 0
		failure := errors.New("always")
		op := func(context.Context, eg.Op) error {
			attempts++
			return failure
		}

		err := egx.Retry(4).Delay(time.Millisecond).Run(op)(t.Context(), nil)
		require.ErrorIs(t, err, failure)
		require.Equal(t, 4, attempts)
	})

	t.Run("without attempts retries until the context ends and returns the last error", func(t *testing.T) {
		ctx, done := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer done()

		failure := errors.New("always")
		op := func(context.Context, eg.Op) error { return failure }

		require.ErrorIs(t, egx.Retry(0).Delay(time.Millisecond).Run(op)(ctx, nil), failure)
	})

	t.Run("backoff is consulted with the number of the failed attempt", func(t *testing.T) {
		var seen []int64
		strategy := backoff.StrategyFunc(func(attempt int64) time.Duration {
			seen = append(seen, attempt)
			return time.Millisecond
		})
		op := func(context.Context, eg.Op) error { return errors.New("always") }

		_ = egx.Retry(3).Backoff(strategy).Run(op)(t.Context(), nil)
		require.Equal(t, []int64{1, 2}, seen)
	})

	t.Run("the last of delay and backoff wins", func(t *testing.T) {
		called := false
		strategy := backoff.StrategyFunc(func(int64) time.Duration {
			called = true
			return time.Millisecond
		})
		op := func(context.Context, eg.Op) error { return errors.New("always") }

		_ = egx.Retry(2).Backoff(strategy).Delay(time.Millisecond).Run(op)(t.Context(), nil)
		require.False(t, called)
	})

	t.Run("chaining leaves the receiver unchanged", func(t *testing.T) {
		attempts := 0
		op := func(context.Context, eg.Op) error {
			attempts++
			return errors.New("always")
		}

		base := egx.Retry(0).Delay(time.Millisecond)
		_ = base.Delay(time.Hour)

		ctx, done := context.WithTimeout(t.Context(), 30*time.Millisecond)
		defer done()
		_ = base.Run(op)(ctx, nil)
		require.Greater(t, attempts, 2)
	})

	t.Run("exponential doubles the wait after each failure", func(t *testing.T) {
		start := time.Now()
		op := func(context.Context, eg.Op) error { return errors.New("always") }

		// waits of 2ms, 4ms, 8ms.
		_ = egx.Retry(4).Exponential(time.Millisecond).Run(op)(t.Context(), nil)
		require.GreaterOrEqual(t, time.Since(start), 14*time.Millisecond)
	})

	t.Run("cycle with a single delay repeats it", func(t *testing.T) {
		start := time.Now()
		op := func(context.Context, eg.Op) error { return errors.New("always") }

		_ = egx.Retry(4).Cycle(5*time.Millisecond).Run(op)(t.Context(), nil)
		require.GreaterOrEqual(t, time.Since(start), 15*time.Millisecond)
	})

	t.Run("cycle waits each delay in order and starts over", func(t *testing.T) {
		start := time.Now()
		op := func(context.Context, eg.Op) error { return errors.New("always") }

		// waits of 1ms, 20ms, 1ms.
		_ = egx.Retry(4).Cycle(time.Millisecond, 20*time.Millisecond).Run(op)(t.Context(), nil)
		elapsed := time.Since(start)
		require.GreaterOrEqual(t, elapsed, 22*time.Millisecond)
		require.Less(t, elapsed, 40*time.Millisecond)
	})
}
