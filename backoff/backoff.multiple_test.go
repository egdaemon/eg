package backoff_test

import (
	"math"
	"testing"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/stretchr/testify/require"
)

func TestMultiple(t *testing.T) {
	t.Run("scales linearly with the attempt", func(t *testing.T) {
		s := backoff.Multiple(1 * time.Second)
		expected := []time.Duration{0, 1 * time.Second, 2 * time.Second, 3 * time.Second, 4 * time.Second}
		for i, want := range expected {
			require.Equal(t, want, s.Backoff(int64(i)), "attempt %d", i)
		}
	})

	t.Run("attempt 0", func(t *testing.T) {
		require.Equal(t, time.Duration(0), backoff.Multiple(500*time.Millisecond).Backoff(0))
	})

	t.Run("with scaling - attempt 1", func(t *testing.T) {
		require.Equal(t, time.Duration(500*time.Millisecond), backoff.Multiple(500*time.Millisecond).Backoff(1))
	})

	t.Run("with scaling - attempt 2", func(t *testing.T) {
		require.Equal(t, time.Duration(1*time.Second), backoff.Multiple(500*time.Millisecond).Backoff(2))
	})

	t.Run("with scaling - attempt 3", func(t *testing.T) {
		require.Equal(t, time.Duration(1500*time.Millisecond), backoff.Multiple(500*time.Millisecond).Backoff(3))
	})

	t.Run("negative attempt returns 0", func(t *testing.T) {
		require.Equal(t, time.Duration(0), backoff.Multiple(1*time.Second).Backoff(-1))
	})

	t.Run("should gracefully handle overflows", func(t *testing.T) {
		s := backoff.Multiple(time.Duration(math.MaxInt64))
		require.Equal(t, time.Duration(math.MaxInt64), s.Backoff(2))
	})

	t.Run("max attempt value", func(t *testing.T) {
		require.Equal(t, time.Duration(math.MaxInt64), backoff.Multiple(1*time.Second).Backoff(math.MaxInt64))
	})
}
