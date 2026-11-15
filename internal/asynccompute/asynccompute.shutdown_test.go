package asynccompute_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/asynccompute"
	"github.com/stretchr/testify/require"
)

func TestShutdown(t *testing.T) {
	t.Run("returns once the workers are done", func(t *testing.T) {
		pool := asynccompute.New(func(ctx context.Context, n int) error { return nil })
		require.NoError(t, pool.Run(t.Context(), 1))
		require.NoError(t, asynccompute.Shutdown(t.Context(), pool))
	})

	t.Run("reports a workload failure", func(t *testing.T) {
		cause := errors.New("boom")
		pool := asynccompute.New(func(ctx context.Context, n int) error { return cause })
		require.NoError(t, pool.Run(t.Context(), 1))
		require.ErrorIs(t, asynccompute.Shutdown(t.Context(), pool), cause)
	})

	t.Run("gives up when the context ends before the workers do", func(t *testing.T) {
		release := make(chan struct{})
		defer close(release)

		pool := asynccompute.New(func(ctx context.Context, n int) error {
			<-release
			return nil
		}, asynccompute.Workers[int](1))
		require.NoError(t, pool.Run(t.Context(), 1))

		ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
		defer cancel()

		require.ErrorIs(t, asynccompute.Shutdown(ctx, pool), context.DeadlineExceeded)
	})

	t.Run("shuts the pools down in the order given", func(t *testing.T) {
		var printed []int
		printing := asynccompute.New(func(ctx context.Context, n int) error {
			printed = append(printed, n)
			return nil
		}, asynccompute.Workers[int](1))
		detecting := asynccompute.New(func(ctx context.Context, root string) error {
			return printing.Run(ctx, len(root))
		}, asynccompute.Workers[string](2))

		require.NoError(t, detecting.Run(t.Context(), "a"))
		require.NoError(t, detecting.Run(t.Context(), "bb"))
		require.NoError(t, asynccompute.Shutdown(t.Context(), detecting, printing))
		require.ElementsMatch(t, []int{1, 2}, printed)
	})

	t.Run("a failing pool does not stop the others from being shut down", func(t *testing.T) {
		cause := errors.New("boom")
		failing := asynccompute.New(func(ctx context.Context, n int) error { return cause })
		ran := make(chan int, 1)
		other := asynccompute.New(func(ctx context.Context, n int) error { ran <- n; return nil })

		require.NoError(t, failing.Run(t.Context(), 1))
		require.NoError(t, other.Run(t.Context(), 2))
		require.ErrorIs(t, asynccompute.Shutdown(t.Context(), failing, other), cause)
		require.Len(t, ran, 1)
	})
}
