package backoff_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/stretchr/testify/require"
)

func TestCycle(t *testing.T) {
	t.Run("more attempts than delays", func(t *testing.T) {
		s := backoff.Cycle(1*time.Second, 2*time.Second, 3*time.Second)
		expected := []time.Duration{1 * time.Second, 2 * time.Second, 3 * time.Second, 1 * time.Second, 2 * time.Second}
		for i, want := range expected {
			require.Equal(t, want, s.Backoff(int64(i)), "attempt %d", i)
		}
	})
}
