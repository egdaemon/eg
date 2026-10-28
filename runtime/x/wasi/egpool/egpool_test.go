package egpool_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/runtime/wasi/eg"
	"github.com/egdaemon/eg/runtime/x/wasi/egpool"
	"github.com/stretchr/testify/require"
)

// ops runs each operation of the sequence, the shape the tests were written for.
func ops(size uint16) egpool.Pool[eg.OpFn] {
	return egpool.New(size, func(op eg.OpFn) eg.OpFn { return op })
}

func TestPoolRun(t *testing.T) {
	t.Run("never runs more operations at once than the size of the pool", func(t *testing.T) {
		var running, peak, ran atomic.Int32

		op := func(context.Context, eg.Op) error {
			cur := running.Add(1)
			defer running.Add(-1)

			for {
				p := peak.Load()
				if cur <= p || peak.CompareAndSwap(p, cur) {
					break
				}
			}

			time.Sleep(10 * time.Millisecond)
			ran.Add(1)
			return nil
		}

		list := make([]eg.OpFn, 12)
		for i := range list {
			list[i] = op
		}

		require.NoError(t, ops(3).Run(egpool.Slice(list...))(t.Context(), egtest.Op()))
		require.EqualValues(t, 12, ran.Load())
		require.LessOrEqual(t, peak.Load(), int32(3))
		require.Greater(t, peak.Load(), int32(1), "operations should overlap")
	})

	t.Run("a size of zero runs one at a time", func(t *testing.T) {
		var running, peak atomic.Int32

		op := func(context.Context, eg.Op) error {
			cur := running.Add(1)
			defer running.Add(-1)
			peak.Store(max(peak.Load(), cur))
			time.Sleep(5 * time.Millisecond)
			return nil
		}

		require.NoError(t, ops(0).Run(egpool.Slice[eg.OpFn](op, op, op))(t.Context(), egtest.Op()))
		require.EqualValues(t, 1, peak.Load())
	})

	t.Run("returns the first failure and still runs the other operations", func(t *testing.T) {
		var ran atomic.Int32
		boom := errors.New("boom")

		ok := func(context.Context, eg.Op) error {
			ran.Add(1)
			return nil
		}
		fail := func(context.Context, eg.Op) error {
			ran.Add(1)
			return boom
		}

		err := ops(1).Run(egpool.Slice[eg.OpFn](ok, fail, ok, ok))(t.Context(), egtest.Op())
		require.ErrorIs(t, err, boom)
		require.EqualValues(t, 4, ran.Load())
	})

	t.Run("returns the error of the sequence", func(t *testing.T) {
		cause := errors.New("sequence failed")

		err := ops(2).Run(egpool.Func(func(context.Context, func(eg.OpFn) bool) error { return cause }))(t.Context(), egtest.Op())
		require.ErrorIs(t, err, cause)
	})

	t.Run("pulls operations lazily as workers become available", func(t *testing.T) {
		var pulled, finished atomic.Int32

		seq := egpool.Func(func(ctx context.Context, yield func(eg.OpFn) bool) error {
			for range 8 {
				// a pool of one has a backlog of one, so at most the running operation, the queued
				// one, and the one being offered can have been pulled ahead of those finished.
				require.LessOrEqual(t, pulled.Load()-finished.Load(), int32(3))
				pulled.Add(1)

				if !yield(func(context.Context, eg.Op) error {
					time.Sleep(5 * time.Millisecond)
					finished.Add(1)
					return nil
				}) {
					return nil
				}
			}

			return nil
		})

		require.NoError(t, ops(1).Run(seq)(t.Context(), egtest.Op()))
		require.EqualValues(t, 8, finished.Load())
	})

	t.Run("stops queueing when the context is cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		var ran atomic.Int32

		release := make(chan struct{})
		block := func(ctx context.Context, _ eg.Op) error {
			ran.Add(1)
			<-release
			return nil
		}

		list := make([]eg.OpFn, 50)
		for i := range list {
			list[i] = block
		}

		done := make(chan error, 1)
		go func() {
			done <- ops(1).Run(egpool.Slice(list...))(ctx, egtest.Op())
		}()

		require.Eventually(t, func() bool { return ran.Load() == 1 }, time.Second, time.Millisecond)
		cancel()
		close(release)

		select {
		case err := <-done:
			require.Error(t, err)
			require.Less(t, ran.Load(), int32(50))
		case <-time.After(5 * time.Second):
			t.Fatal("pool did not return after the context was cancelled")
		}
	})

	t.Run("an empty sequence does nothing", func(t *testing.T) {
		require.NoError(t, ops(2).Run(egpool.Slice[eg.OpFn]())(t.Context(), egtest.Op()))
	})

	t.Run("runs the operation created for each item", func(t *testing.T) {
		var mu sync.Mutex
		var seen []int

		pool := egpool.New(3, func(n int) eg.OpFn {
			return func(context.Context, eg.Op) error {
				mu.Lock()
				defer mu.Unlock()
				seen = append(seen, n)
				return nil
			}
		})

		require.NoError(t, pool.Run(egpool.Slice(1, 2, 3, 4, 5))(t.Context(), egtest.Op()))
		require.ElementsMatch(t, []int{1, 2, 3, 4, 5}, seen)
	})
}
