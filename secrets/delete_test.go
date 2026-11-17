package secrets_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
)

func TestDelete(t *testing.T) {
	t.Run("chachasm removes the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "secret.chacha")
		require.NoError(t, secrets.Update(t.Context(), "chachasm://pass@"+path, bytes.NewReader([]byte("content"))))

		require.NoError(t, secrets.Delete(t.Context(), "chachasm://pass@"+path))

		exists, err := secrets.Exists(t.Context(), "chachasm://pass@"+path)
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("file removes the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plain.secret")
		require.NoError(t, os.WriteFile(path, []byte("content"), 0o600))

		require.NoError(t, secrets.Delete(t.Context(), "file://"+path))

		_, err := os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("missing file matches os.ErrNotExist", func(t *testing.T) {
		err := secrets.Delete(t.Context(), "file://"+filepath.Join(t.TempDir(), "missing"))
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("unsupported schemes are an error", func(t *testing.T) {
		require.ErrorContains(t, secrets.Delete(t.Context(), "nope://secret"), "unsupported scheme")
	})
}
