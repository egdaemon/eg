package iox_test

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/stretchr/testify/require"
)

func TestTimeoutReader(t *testing.T) {
	t.Run("it should timeout", func(t *testing.T) {
		var buf [16 * bytesx.KiB]byte
		r := iox.TimeoutReader(time.Millisecond, iox.DelayReader(3*time.Millisecond, io.NopCloser(cryptox.NewChaCha8(t.Name()))))

		n, err := io.ReadFull(r, buf[:])
		require.ErrorIs(t, err, context.DeadlineExceeded)
		require.EqualValues(t, 0, n)
	})

	t.Run("it should read data", func(t *testing.T) {
		var buf [16 * bytesx.KiB]byte
		r := iox.TimeoutReader(3*time.Millisecond, iox.DelayReader(time.Millisecond, io.NopCloser(cryptox.NewChaCha8(t.Name()))))
		n, err := io.ReadFull(r, buf[:])
		require.NoError(t, err)
		require.EqualValues(t, len(buf), n)
	})
}
