package backoff_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/stretchr/testify/require"
)

func TestAttemptV(t *testing.T) {
	const constantDelay = 5 * time.Millisecond
	s := backoff.Constant(constantDelay)

	t.Run("success on first attempt", func(t *testing.T) {
		ctx := t.Context()
		doFunc := func(_ context.Context, attempt uint) (string, error) {
			require.Equal(t, uint(0), attempt)
			return "success", nil
		}
		result, err := backoff.AttemptV(ctx, s, doFunc)
		require.NoError(t, err)
		require.Equal(t, "success", result)
	})

	t.Run("success on second attempt", func(t *testing.T) {
		ctx := t.Context()
		doFunc := func(_ context.Context, attempt uint) (string, error) {
			if attempt == 0 {
				return "", errors.ErrUnsupported
			}
			return "success", nil
		}
		result, err := backoff.AttemptV(ctx, s, doFunc)
		require.NoError(t, err)
		require.Equal(t, "success", result)
	})

	t.Run("stop attempts on first retry returns initial error", func(t *testing.T) {
		ctx := t.Context()
		initialErr := errors.New("initial error")
		doFunc := func(_ context.Context, attempt uint) (string, error) {
			if attempt == 0 {
				return "", initialErr
			}
			return "", backoff.ErrStopAttempts
		}
		result, err := backoff.AttemptV(ctx, s, doFunc)
		require.ErrorIs(t, err, initialErr)
		require.Equal(t, "", result)
	})

	t.Run("stop attempts on second retry returns previous error", func(t *testing.T) {
		ctx := t.Context()
		initialErr := errors.New("initial error")
		retryErr := errors.New("retry 1 error")
		doFunc := func(_ context.Context, attempt uint) (string, error) {
			if attempt == 0 {
				return "", initialErr
			}
			if attempt == 1 {
				return "", retryErr
			}
			return "", backoff.ErrStopAttempts
		}
		result, err := backoff.AttemptV(ctx, s, doFunc)
		require.ErrorIs(t, err, retryErr)
		require.Equal(t, "", result)
	})

	t.Run("stop attempts with wrapped error", func(t *testing.T) {
		ctx := t.Context()
		previousErr := errors.New("previous error")
		doFunc := func(_ context.Context, attempt uint) (int, error) {
			if attempt == 0 {
				return 0, errors.ErrUnsupported
			}
			if attempt == 1 {
				return 0, previousErr
			}
			return 0, errors.Join(backoff.ErrStopAttempts, errors.New("wrapped"))
		}
		result, err := backoff.AttemptV(ctx, s, doFunc)
		require.ErrorIs(t, err, previousErr)
		require.Equal(t, 0, result)
	})

	t.Run("timeout before first retry sleep", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()
		attempt := 0
		doFunc := func(context.Context, uint) (string, error) {
			attempt++
			return "", errors.ErrUnsupported
		}
		time.Sleep(2 * time.Millisecond)
		_, err := backoff.AttemptV(ctx, s, doFunc)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.Equal(t, 1, attempt)
	})

	t.Run("timeout after one retry sleep", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 8*time.Millisecond)
		defer cancel()
		attempt := 0
		doFunc := func(context.Context, uint) (int, error) {
			attempt++
			return 0, errors.ErrUnsupported
		}
		start := time.Now()
		_, err := backoff.AttemptV(ctx, s, doFunc)
		elapsed := time.Since(start)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.GreaterOrEqual(t, elapsed, constantDelay)
	})

	t.Run("timeout after three retry sleeps", func(t *testing.T) {
		const expectedSleeps = 3
		targetTimeout := time.Duration(expectedSleeps) * constantDelay
		ctx, cancel := context.WithTimeout(context.Background(), targetTimeout)
		defer cancel()
		attempt := 0
		doFunc := func(context.Context, uint) (float64, error) {
			attempt++
			return 0.0, errors.ErrUnsupported
		}
		start := time.Now()
		_, err := backoff.AttemptV(ctx, s, doFunc)
		elapsed := time.Since(start)
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.GreaterOrEqual(t, elapsed, targetTimeout)
	})
}
