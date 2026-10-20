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
}
