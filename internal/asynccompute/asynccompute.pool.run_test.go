package asynccompute_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/asynccompute"
	"github.com/stretchr/testify/require"
)

func TestPoolRun(t *testing.T) {
	type key struct{}

	t.Run("runs the workload with the context it was queued with", func(t *testing.T) {
		got := make(chan any, 1)

		pool := asynccompute.New(func(ctx context.Context, n int) error {
			got <- ctx.Value(key{})
			return nil
		})

		require.NoError(t, pool.Run(context.WithValue(t.Context(), key{}, "queued"), 1))
		require.NoError(t, pool.Close())
		require.Equal(t, "queued", <-got)
	})

	t.Run("a full backlog blocks until the context ends", func(t *testing.T) {
		release := make(chan struct{})
		pool := asynccompute.New(func(ctx context.Context, n int) error {
			<-release
			return nil
		}, asynccompute.Compose(asynccompute.Workers[int](1), asynccompute.Backlog[int](0)))

		// the only worker takes the first workload and blocks.
		require.NoError(t, pool.Run(t.Context(), 1))

		cause := errors.New("gave up waiting")
		ctx, cancel := context.WithCancelCause(t.Context())
		go func() {
			time.Sleep(20 * time.Millisecond)
			cancel(cause)
		}()

		require.ErrorIs(t, pool.Run(ctx, 2), cause)

		close(release)
		require.NoError(t, pool.Close())
	})
}
