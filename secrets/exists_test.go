package secrets_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
)

func TestExists(t *testing.T) {
	t.Run("chachasm reports a missing file as not existing", func(t *testing.T) {
		exists, err := secrets.Exists(t.Context(), "chachasm://pass@"+filepath.Join(t.TempDir(), "missing.chacha"))
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("chachasm reports an existing file without needing its passphrase", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "secret.chacha")
		require.NoError(t, secrets.Update(t.Context(), "chachasm://pass@"+path, bytes.NewReader([]byte("content"))))

		exists, err := secrets.Exists(t.Context(), "chachasm://wrong@"+path)
		require.NoError(t, err)
		require.True(t, exists)

		exists, err = secrets.Exists(t.Context(), "chachasm://"+path)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("file reports missing and existing files", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plain.secret")

		exists, err := secrets.Exists(t.Context(), "file://"+path)
		require.NoError(t, err)
		require.False(t, exists)

		require.NoError(t, os.WriteFile(path, []byte("content"), 0o600))

		exists, err = secrets.Exists(t.Context(), "file://"+path)
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("unsupported schemes are an error", func(t *testing.T) {
		_, err := secrets.Exists(t.Context(), "nope://secret")
		require.ErrorContains(t, err, "unsupported scheme")
	})
}
