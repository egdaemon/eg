package backoff

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func testBackoff(t testing.TB, attempts int, s Strategy, expected ...time.Duration) {
	t.Helper()
	for i := 0; i < attempts; i++ {
		require.Equal(t, expected[i], s.Backoff(int64(i)), fmt.Sprintf("attempt %d", i))
	}
}

func expectedDurationTest(t testing.TB, attempt int, s Strategy, expected time.Duration) {
	t.Helper()
	require.Equal(t, expected, s.Backoff(int64(attempt)))
}

func expectedDurationRangeTest(t testing.TB, attempt int, s Strategy, expected, delta time.Duration) {
	t.Helper()
	b := s.Backoff(int64(attempt))
	diff := b - expected
	if diff < 0 {
		diff = -diff
	}
	require.LessOrEqual(t, diff, delta)
}

func TestBackoff(t *testing.T) {
	t.Run("Explicit/more attempts than delays", func(t *testing.T) {
		testBackoff(t, 5, Explicit(1*time.Second, 2*time.Second, 3*time.Second),
			1*time.Second, 2*time.Second, 3*time.Second, 1*time.Second, 2*time.Second)
	})

	t.Run("Exponential/should double each time", func(t *testing.T) {
		testBackoff(t, 5, Exponential(1*time.Second),
			1*time.Second, 2*time.Second, 4*time.Second, 8*time.Second, 16*time.Second)
	})

	t.Run("Exponential/should gracefully handle overflows", func(t *testing.T) {
		testBackoff(t, 101, Exponential(1*time.Second),
			time.Second<<uint(0),
			time.Second<<uint(1),
			time.Second<<uint(2),
			time.Second<<uint(3),
			time.Second<<uint(4),
			time.Second<<uint(5),
			time.Second<<uint(6),
			time.Second<<uint(7),
			time.Second<<uint(8),
			time.Second<<uint(9),
			time.Second<<uint(10),
			time.Second<<uint(11),
			time.Second<<uint(12),
			time.Second<<uint(13),
			time.Second<<uint(14),
			time.Second<<uint(15),
			time.Second<<uint(16),
			time.Second<<uint(17),
			time.Second<<uint(18),
			time.Second<<uint(19),
			time.Second<<uint(20),
			time.Second<<uint(21),
			time.Second<<uint(22),
			time.Second<<uint(23),
			time.Second<<uint(24),
			time.Second<<uint(25),
			time.Second<<uint(26),
			time.Second<<uint(27),
			time.Second<<uint(28),
			time.Second<<uint(29),
			time.Second<<uint(30),
			time.Second<<uint(31),
			time.Second<<uint(32),
			time.Second<<uint(33),
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
		)
	})

	t.Run("Constant/should remain constant", func(t *testing.T) {
		testBackoff(t, 5, Constant(1*time.Second),
			1*time.Second, 1*time.Second, 1*time.Second, 1*time.Second, 1*time.Second)
	})

	for _, tc := range []struct {
		name     string
		attempt  int
		strategy Strategy
		expected time.Duration
	}{
		{"Exponential Backoff/attempt 0", 0, Exponential(1 * time.Second), 1 * time.Second},
		{"Exponential Backoff/attempt 1", 1, Exponential(1 * time.Second), 2 * time.Second},
		{"Exponential Backoff/attempt 2", 2, Exponential(1 * time.Second), 4 * time.Second},
		{"Exponential Backoff/attempt 3", 3, Exponential(1 * time.Second), 8 * time.Second},
		{"Exponential Backoff/attempt 36", 36, Exponential(1 * time.Second), time.Duration(math.MaxInt64)},
		{"Exponential Backoff/attempt 37", 37, Exponential(1 * time.Second), time.Duration(math.MaxInt64)},
		{"Exponential Backoff/attempt 54 - overflow", 54, Exponential(1 * time.Second), time.Duration(math.MaxInt64)},
		{"Exponential Backoff/with scaling - attempt 0", 0, Exponential(500 * time.Millisecond), 500 * time.Millisecond},
		{"Exponential Backoff/with scaling - attempt 1", 1, Exponential(500 * time.Millisecond), 1 * time.Second},
		{"Exponential Backoff/with scaling - attempt 2", 2, Exponential(500 * time.Millisecond), 2 * time.Second},
		{"Exponential Backoff/with scaling - attempt 3", 3, Exponential(500 * time.Millisecond), 4 * time.Second},
		{"Exponential Backoff/max attempt value", math.MaxInt64, Exponential(1 * time.Second), time.Duration(math.MaxInt64)},
		{"Jitter/example 1 - with jitter", 57, New(Exponential(1*time.Second), Jitter(0.25)), time.Duration(math.MaxInt64)},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			expectedDurationTest(t, tc.attempt, tc.strategy, tc.expected)
		})
	}

	t.Run("JitterRandWindow/example 1 - with jitter range", func(t *testing.T) {
		expectedDurationRangeTest(t, 0,
			New(Constant(1*time.Second), JitterRandWindow(200*time.Millisecond)),
			time.Second, 200*time.Millisecond)
	})
}
