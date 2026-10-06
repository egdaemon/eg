package backoff_test

import (
	"math"
	"testing"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/stretchr/testify/require"
)

func TestExponential(t *testing.T) {
	t.Run("double each time", func(t *testing.T) {
		s := backoff.Exponential(1 * time.Second)
		expected := []time.Duration{1 * time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second}
		for i, want := range expected {
			require.Equal(t, want, s.Backoff(int64(i)), "attempt %d", i)
		}
	})

	t.Run("should gracefully handle overflows", func(t *testing.T) {
		s := backoff.Exponential(1 * time.Second)
		expected := []time.Duration{
			time.Second << uint(0),
			time.Second << uint(1),
			time.Second << uint(2),
			time.Second << uint(3),
			time.Second << uint(4),
			time.Second << uint(5),
			time.Second << uint(6),
			time.Second << uint(7),
			time.Second << uint(8),
			time.Second << uint(9),
			time.Second << uint(10),
			time.Second << uint(11),
			time.Second << uint(12),
			time.Second << uint(13),
			time.Second << uint(14),
			time.Second << uint(15),
			time.Second << uint(16),
			time.Second << uint(17),
			time.Second << uint(18),
			time.Second << uint(19),
			time.Second << uint(20),
			time.Second << uint(21),
			time.Second << uint(22),
			time.Second << uint(23),
			time.Second << uint(24),
			time.Second << uint(25),
			time.Second << uint(26),
			time.Second << uint(27),
			time.Second << uint(28),
			time.Second << uint(29),
			time.Second << uint(30),
			time.Second << uint(31),
			time.Second << uint(32),
			time.Second << uint(33),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 40
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 50
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 60
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 70
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 80
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 90
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64),
			time.Duration(math.MaxInt64), // 100
		}
		for i, want := range expected {
			require.Equal(t, want, s.Backoff(int64(i)), "attempt %d", i)
		}
	})

	t.Run("attempt 0", func(t *testing.T) {
		require.Equal(t, time.Duration(1*time.Second), backoff.Exponential(1*time.Second).Backoff(0))
	})

	t.Run("attempt 1", func(t *testing.T) {
		require.Equal(t, time.Duration(2*time.Second), backoff.Exponential(1*time.Second).Backoff(1))
	})

	t.Run("attempt 2", func(t *testing.T) {
		require.Equal(t, time.Duration(4*time.Second), backoff.Exponential(1*time.Second).Backoff(2))
	})

	t.Run("attempt 3", func(t *testing.T) {
		require.Equal(t, time.Duration(8*time.Second), backoff.Exponential(1*time.Second).Backoff(3))
	})

	t.Run("attempt 36", func(t *testing.T) {
		require.Equal(t, time.Duration(math.MaxInt64), backoff.Exponential(1*time.Second).Backoff(36))
	})

	t.Run("attempt 37", func(t *testing.T) {
		require.Equal(t, time.Duration(math.MaxInt64), backoff.Exponential(1*time.Second).Backoff(37))
	})

	t.Run("attempt 54 - overflow", func(t *testing.T) {
		require.Equal(t, time.Duration(math.MaxInt64), backoff.Exponential(1*time.Second).Backoff(54))
	})

	t.Run("with scaling - attempt 0", func(t *testing.T) {
		require.Equal(t, time.Duration(500*time.Millisecond), backoff.Exponential(500*time.Millisecond).Backoff(0))
	})

	t.Run("with scaling - attempt 1", func(t *testing.T) {
		require.Equal(t, time.Duration(1*time.Second), backoff.Exponential(500*time.Millisecond).Backoff(1))
	})

	t.Run("with scaling - attempt 2", func(t *testing.T) {
		require.Equal(t, time.Duration(2*time.Second), backoff.Exponential(500*time.Millisecond).Backoff(2))
	})

	t.Run("with scaling - attempt 3", func(t *testing.T) {
		require.Equal(t, time.Duration(4*time.Second), backoff.Exponential(500*time.Millisecond).Backoff(3))
	})

	t.Run("max attempt value", func(t *testing.T) {
		require.Equal(t, time.Duration(math.MaxInt64), backoff.Exponential(1*time.Second).Backoff(math.MaxInt64))
	})
}
