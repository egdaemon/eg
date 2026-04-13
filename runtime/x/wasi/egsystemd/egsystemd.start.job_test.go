package egsystemd

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

// mockPerform returns a PerformFn that sends result to the await channel in a
// goroutine (mirroring how the real dbus methods work) and returns dispatchErr
// from the initial call.
func mockPerform(result string, dispatchErr error) PerformFn {
	return func(_ context.Context, _ string, _ string, ch chan<- string) (int, error) {
		if dispatchErr != nil {
			return 0, dispatchErr
		}
		go func() { ch <- result }()
		return 0, nil
	}
}

func TestStartJob(t *testing.T) {
	t.Run("no_targets_returns_nil", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		err := startJob(ctx, mockPerform("done", nil))
		require.NoError(t, err)
	})

	t.Run("single_target_done_result_returns_nil", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		err := startJob(ctx, mockPerform("done", nil), "foo.service")
		require.NoError(t, err)
	})

	t.Run("single_target_non_done_result_returns_error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		err := startJob(ctx, mockPerform("failed", nil), "foo.service")
		require.EqualError(t, err, "failed")
	})

	t.Run("dispatch_error_returned_immediately", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		sentinel := errors.New("dbus dispatch error")
		err := startJob(ctx, mockPerform("", sentinel), "foo.service")
		require.ErrorIs(t, err, sentinel)
	})

	t.Run("multiple_targets_all_succeed_returns_nil", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		err := startJob(ctx, mockPerform("done", nil), "foo.service", "bar.service", "baz.service")
		require.NoError(t, err)
	})

	t.Run("multiple_targets_second_fails_returns_error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		calls := 0
		d := func(_ context.Context, _ string, _ string, ch chan<- string) (int, error) {
			calls++
			result := "done"
			if calls == 2 {
				result = "failed"
			}
			go func() { ch <- result }()
			return 0, nil
		}

		err := startJob(ctx, d, "foo.service", "bar.service")
		require.EqualError(t, err, "failed")
		require.Equal(t, 2, calls)
	})

	t.Run("context_cancelled_before_result_returns_context_error", func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())

		// dispatch succeeds but never sends to the channel; we cancel instead.
		d := func(_ context.Context, _ string, _ string, _ chan<- string) (int, error) {
			cancel()
			return 0, nil
		}

		err := startJob(ctx, d, "foo.service")
		require.ErrorIs(t, err, context.Canceled)
	})
}
