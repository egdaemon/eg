package eggolang

import (
	"testing"

	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestTestFlags(t *testing.T) {
	t.Run("includes_all_flags_in_order", func(t *testing.T) {
		c := langx.Clone(config{coverage: "-covermode count"}, New().Flags("-race").Randomize(7).Count(3).Verbose(true)...)
		require.Equal(t, []string{"-race", "-shuffle 7", "-count 3", "-v", "-covermode count"}, c.testflags())
	})

	t.Run("Randomize_maps_seed_to_flag", func(t *testing.T) {
		require.Equal(t, "-shuffle off", langx.Clone(config{}, New().Randomize(0)...).randomize)
		require.Equal(t, "-shuffle on", langx.Clone(config{}, New().Randomize(1)...).randomize)
		require.Equal(t, "-shuffle 42", langx.Clone(config{}, New().Randomize(42)...).randomize)
	})

	t.Run("Count_zero_uses_go_default", func(t *testing.T) {
		require.Empty(t, langx.Clone(config{}, New().Count(3).Count(0)...).count)
	})

	t.Run("NoCache_sets_count_when_unset", func(t *testing.T) {
		require.Equal(t, "-count 1", langx.Clone(config{}, New().NoCache()...).count)
	})

	t.Run("NoCache_does_not_override_count", func(t *testing.T) {
		require.Equal(t, "-count 5", langx.Clone(config{}, New().Count(5).NoCache()...).count)
	})
}
