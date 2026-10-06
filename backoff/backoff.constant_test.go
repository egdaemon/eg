package backoff_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/backoff"
	"github.com/stretchr/testify/require"
)

func TestConstant(t *testing.T) {
	t.Run("should remain constant", func(t *testing.T) {
		s := backoff.Constant(1 * time.Second)
		for i := 0; i < 5; i++ {
			require.Equal(t, 1*time.Second, s.Backoff(int64(i)), "attempt %d", i)
		}
	})
}
