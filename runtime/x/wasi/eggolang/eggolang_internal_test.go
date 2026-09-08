package eggolang

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBuildOption(t *testing.T) {
	t.Run("options_are_empty_by_default", func(t *testing.T) {
		require.Empty(t, Build().options())
	})

	t.Run("options_include_flags", func(t *testing.T) {
		require.Equal(t, []string{"-race", "-v"}, Build(BuildOption.Flags("-race", "-v")).options())
	})

	t.Run("options_include_debug", func(t *testing.T) {
		require.Equal(t, []string{"-x"}, Build(BuildOption.Debug(true)).options())
	})

	t.Run("options_append_tags_after_flags", func(t *testing.T) {
		require.Equal(t, []string{"-race", "-tags=a,b"}, Build(BuildOption.Flags("-race"), BuildOption.Tags("a", "b")).options())
	})
}
