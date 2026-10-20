package asynccompute_test

import (
	"context"
	"errors"
	"testing"

	"github.com/egdaemon/eg/internal/asynccompute"
	"github.com/stretchr/testify/require"
)

func TestPoolClose(t *testing.T) {
	t.Run("waits for queued workloads", func(t *testing.T) {
		done := make(chan int, 4)
		pool := asynccompute.New(func(ctx context.Context, n int) error {
			done <- n
			return nil
		}, asynccompute.Workers[int](1))

		for i := range 4 {
			require.NoError(t, pool.Run(t.Context(), i))
		}

		require.NoError(t, pool.Close())
		require.Len(t, done, 4)
	})

	t.Run("reports the first failure", func(t *testing.T) {
		first, second := errors.New("first"), errors.New("second")

		pool := asynccompute.New(func(ctx context.Context, n int) error {
			switch n {
			case 1:
				return first
			case 2:
				return second
			default:
				return nil
			}
		}, asynccompute.Workers[int](1))

		for i := range 4 {
			require.NoError(t, pool.Run(t.Context(), i))
		}

		require.ErrorIs(t, pool.Close(), first)
	})

	t.Run("a successful workload doesn't hide a later failure", func(t *testing.T) {
		cause := errors.New("later failure")

		pool := asynccompute.New(func(ctx context.Context, n int) error {
			if n == 3 {
				return cause
			}

			return nil
		}, asynccompute.Workers[int](1))

		for i := range 5 {
			require.NoError(t, pool.Run(t.Context(), i))
		}

		require.ErrorIs(t, pool.Close(), cause)
	})
}
