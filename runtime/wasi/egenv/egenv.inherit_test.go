package egenv_test

import (
	"slices"
	"testing"

	"github.com/egdaemon/eg/runtime/wasi/egenv"
	"github.com/stretchr/testify/require"
)

// the assertions are booleans so a failure doesn't print the whole process environment, it holds secrets.
func TestInherit(t *testing.T) {
	t.Run("InheritEnv_matches_Inherit", func(t *testing.T) {
		t.Setenv("EGENV_TEST_INHERIT", "value")

		require.True(t, slices.Equal(egenv.MustEnviron(egenv.Inherit()), egenv.InheritEnv()))
	})

	t.Run("copies_the_process_environment", func(t *testing.T) {
		t.Setenv("EGENV_TEST_INHERIT", "value")

		environ := egenv.MustEnviron(egenv.Inherit())
		require.True(t, slices.Contains(environ, "EGENV_TEST_INHERIT=value"))
	})

	t.Run("later_entries_follow_inherited_values", func(t *testing.T) {
		t.Setenv("EGENV_TEST_INHERIT", "host")

		environ := egenv.MustEnviron(egenv.Inherit().Var("EGENV_TEST_INHERIT", "override"))
		host := slices.Index(environ, "EGENV_TEST_INHERIT=host")
		override := slices.Index(environ, "EGENV_TEST_INHERIT=override")
		require.True(t, host >= 0 && override > host, "the override must come after the inherited value so it wins")
	})
}
