package asynccompute_test

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/egdaemon/eg/internal/asynccompute"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	t.Run("runs every workload", func(t *testing.T) {
		var total atomic.Int64

		pool := asynccompute.New(func(ctx context.Context, n int) error {
			total.Add(int64(n))
			return nil
		})

		for i := 1; i <= 10; i++ {
			require.NoError(t, pool.Run(t.Context(), i))
		}

		require.NoError(t, pool.Close())
		require.EqualValues(t, 55, total.Load())
	})

	t.Run("never exceeds the configured workers", func(t *testing.T) {
		var (
			active, peak atomic.Int64
			release      = make(chan struct{})
			started      = make(chan struct{}, 6)
		)

		pool := asynccompute.New(func(ctx context.Context, n int) error {
			now := active.Add(1)
			for old := peak.Load(); now > old && !peak.CompareAndSwap(old, now); old = peak.Load() {
			}
			started <- struct{}{}
			<-release
			active.Add(-1)
			return nil
		}, asynccompute.Compose(asynccompute.Workers[int](2), asynccompute.Backlog[int](8)))

		for i := 0; i < 6; i++ {
			require.NoError(t, pool.Run(t.Context(), i))
		}

		// both workers are busy, the rest are queued.
		<-started
		<-started
		require.EqualValues(t, 2, active.Load())
		close(release)

		require.NoError(t, pool.Close())
		require.EqualValues(t, 2, peak.Load())
	})
}
