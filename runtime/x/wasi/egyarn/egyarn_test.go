package egyarn_test

import (
	"testing"

	"github.com/egdaemon/eg/runtime/x/wasi/egyarn"
	"github.com/stretchr/testify/require"
)

func TestEnv(t *testing.T) {
	t.Run("sets_the_yarn_cache_variables", func(t *testing.T) {
		environ := egyarn.Env()
		require.Contains(t, environ, "YARN_CACHE_FOLDER="+egyarn.CacheDirectory())
		require.Contains(t, environ, "COREPACK_ENABLE_DOWNLOAD_PROMPT=0")
		require.Contains(t, environ, "YARN_ENABLE_GLOBAL_CACHE=false")
	})

	t.Run("does_not_copy_the_process_environment", func(t *testing.T) {
		t.Setenv("EGYARN_TEST_INHERIT", "value")
		t.Setenv("GNUPGHOME", "/eg.mnt/.gnupg")

		environ := egyarn.Env()
		require.NotContains(t, environ, "EGYARN_TEST_INHERIT=value")
		require.NotContains(t, environ, "GNUPGHOME=/eg.mnt/.gnupg")
		require.Len(t, environ, 4)
	})
}
