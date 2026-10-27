package eggolang

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	t.Run("sets_cache_directories", func(t *testing.T) {
		environ, err := env()
		require.NoError(t, err)
		require.Contains(t, environ, fmt.Sprintf("GOCACHE=%s", CacheBuildDirectory()))
		require.Contains(t, environ, fmt.Sprintf("GOMODCACHE=%s", CacheModuleDirectory()))
	})

	t.Run("xdg_cache_home_defaults_to_cache_directory", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "")

		environ, err := env()
		require.NoError(t, err)
		require.Contains(t, environ, fmt.Sprintf("XDG_CACHE_HOME=%s", CacheDirectory()))
	})

	t.Run("xdg_cache_home_respects_existing_value", func(t *testing.T) {
		t.Setenv("XDG_CACHE_HOME", "/custom/xdg")

		environ, err := env()
		require.NoError(t, err)
		require.Contains(t, environ, "XDG_CACHE_HOME=/custom/xdg")
	})

	t.Run("inherits_process_environment", func(t *testing.T) {
		t.Setenv("EGGOLANG_TEST_INHERIT", "value")

		environ, err := env()
		require.NoError(t, err)
		require.Contains(t, environ, "EGGOLANG_TEST_INHERIT=value")
	})

	t.Run("Env_matches_env", func(t *testing.T) {
		expected, err := env()
		require.NoError(t, err)
		require.Equal(t, expected, Env())
	})
}
