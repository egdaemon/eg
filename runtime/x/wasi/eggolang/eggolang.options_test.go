package eggolang

import (
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestOptions(t *testing.T) {
	t.Run("Timeout", func(t *testing.T) {
		require.Equal(t, 3*time.Minute, langx.Clone(config{}, New().Timeout(3*time.Minute)...).timeout)
	})

	t.Run("WorkingDirectory", func(t *testing.T) {
		require.Equal(t, "/workspace", langx.Clone(config{}, New().WorkingDirectory("/workspace")...).bctx.Dir)
	})

	t.Run("Environ_appends", func(t *testing.T) {
		c := langx.Clone(config{}, New().Environ("A=1").Environ("B=2", "C=3")...)
		require.Equal(t, []string{"A=1", "B=2", "C=3"}, c.environ)
	})

	t.Run("Verbose", func(t *testing.T) {
		require.Equal(t, "-v", langx.Clone(config{}, New().Verbose(true)...).verbose)
		require.Empty(t, langx.Clone(config{}, New().Verbose(true).Verbose(false)...).verbose)
	})

	t.Run("testflags_skips_blank_values", func(t *testing.T) {
		require.Equal(t, []string{"-covermode count"}, langx.Clone(config{coverage: "-covermode count"}, New()...).testflags())
	})
}
