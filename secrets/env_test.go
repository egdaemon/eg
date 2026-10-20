package secrets

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEnvReader(t *testing.T) {
	t.Run("should read secrets through the existing readers", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "secret.env")
		require.NoError(t, os.WriteFile(path, []byte("A=b\n"), 0600))

		b, err := io.ReadAll(Env().NewReader(t.Context(), "file://"+path))
		require.NoError(t, err)
		require.Equal(t, "A=b\n", string(b))
	})
}
