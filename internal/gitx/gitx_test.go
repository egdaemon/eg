package gitx

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/compute"
	"github.com/stretchr/testify/require"
)

func TestBearer(t *testing.T) {
	t.Run("returns password from token file", func(t *testing.T) {
		dir := t.TempDir()
		writeTokenFile(t, dir, "filetoken")
		require.Equal(t, "filetoken", Bearer(dir))
	})

	t.Run("empty when file absent", func(t *testing.T) {
		require.Equal(t, "", Bearer(t.TempDir()))
	})
}

func writeTokenFile(t *testing.T, dir, password string) {
	t.Helper()
	encoded, err := json.Marshal(&compute.GitCredentialsHTTP{Password: password})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vcsaccess.token"), encoded, 0600))
}
