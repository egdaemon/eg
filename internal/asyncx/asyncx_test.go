package asyncx_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/asyncx"
	"github.com/stretchr/testify/require"
)

func TestWakeup(t *testing.T) {
	t.Run("signal delivers", func(t *testing.T) {
		w := asyncx.NewWakeup(t.Context())
		defer w.Close()

		require.Eventually(t, func() bool {
			w.Signal()
			select {
			case err := <-w.C:
				return err == nil
			case <-time.After(10 * time.Millisecond):
				return false
			}
		}, time.Second, 10*time.Millisecond)
	})

	t.Run("close reports closed", func(t *testing.T) {
		w := asyncx.NewWakeup(t.Context())
		require.NoError(t, w.Close())
		require.ErrorIs(t, <-w.C, asyncx.ErrWakeupClosed)
	})
}
