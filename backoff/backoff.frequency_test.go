package backoff

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/timex"
	"github.com/stretchr/testify/require"
)

func TestFrequency(t *testing.T) {
	t.Run("before the anchor returns the time remaining until it", func(t *testing.T) {
		const fq = time.Hour
		now := time.Date(2024, time.January, 1, 10, 15, 0, 0, time.UTC)
		expected := time.Date(2024, time.January, 1, 10, 20, 0, 0, time.UTC)
		clk := timex.NewAdjustableClock(now)

		// anchor at 20 mins past the hour. means this setup will run at X:20 every hour.
		s := frequency{anchor: 20 * time.Minute, fq: fq, c: clk}

		d := s.Backoff(0)

		require.Equal(t, expected, now.Add(d))
	})

	t.Run("after the anchor falls back to the window boundary", func(t *testing.T) {
		const fq = time.Hour
		now := time.Date(2024, time.January, 1, 10, 45, 0, 0, time.UTC)
		clk := timex.NewAdjustableClock(now)
		pos := now.Truncate(fq)
		anchor := now.Sub(pos) - time.Second
		s := frequency{anchor: anchor, fq: fq, c: clk}

		d := s.Backoff(0)

		expected := pos.Add(fq).Sub(now)
		require.Equal(t, expected, d)
	})

	t.Run("ignores the attempt number", func(t *testing.T) {
		clk := timex.NewAdjustableClock(time.Date(2024, time.January, 1, 10, 15, 0, 0, time.UTC))
		s := frequency{anchor: 5 * time.Minute, fq: time.Hour, c: clk}

		first := s.Backoff(0)
		second := s.Backoff(9)

		require.Equal(t, first, second)
	})

	t.Run("stays positive across iterations as the clock advances by the frequency", func(t *testing.T) {
		const fq = time.Hour
		seed := "positive-across-iterations-seed"
		clk := timex.NewAdjustableClock(time.Date(2024, time.January, 1, 0, 0, 0, 0, time.UTC))
		s := frequency{anchor: DynamicHashDuration(fq, seed), fq: fq, c: clk}

		points := make([]time.Duration, 0, 3)
		for range 3 {
			d := s.Backoff(0)
			require.Greater(t, d, time.Duration(0))
			points = append(points, d)
			clk.Advance(fq)
		}

		require.Len(t, points, 3)
	})
}
