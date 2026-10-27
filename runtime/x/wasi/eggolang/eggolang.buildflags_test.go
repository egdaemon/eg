package eggolang

import (
	"testing"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestBuildFlags(t *testing.T) {
	t.Run("are_empty_by_default", func(t *testing.T) {
		require.Empty(t, langx.Clone(config{}, New()...).buildflags())
	})

	t.Run("Flags_adds_flags", func(t *testing.T) {
		require.Equal(t, []string{"-race", "-v"}, langx.Clone(config{}, New().Flags("-race", "-v")...).buildflags())
	})

	t.Run("Debug_adds_x_flag", func(t *testing.T) {
		require.Equal(t, []string{"-x"}, langx.Clone(config{}, New().Debug(true)...).buildflags())
	})

	t.Run("Debug_false_adds_nothing", func(t *testing.T) {
		require.Empty(t, langx.Clone(config{}, New().Debug(false)...).buildflags())
	})

	t.Run("Tags_are_appended_after_flags", func(t *testing.T) {
		require.Equal(t, []string{"-race", "-tags=a,b"}, langx.Clone(config{}, New().Flags("-race").Tags("a", "b")...).buildflags())
	})
}
