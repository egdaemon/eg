package runners_test

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func TestSpoolDirsWake(t *testing.T) {
	t.Run("wake reaches copies", func(t *testing.T) {
		d := runners.NewSpoolDir(t.TempDir())
		cp := d
		require.Eventually(t, func() bool {
			d.Wake()
			select {
			case err := <-cp.Awake():
				return err == nil
			case <-time.After(10 * time.Millisecond):
				return false
			}
		}, time.Second, 10*time.Millisecond)
	})
}
