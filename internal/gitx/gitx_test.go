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
	t.Run("token takes precedence over file", func(t *testing.T) {
		dir := t.TempDir()
		writeTokenFile(t, dir, "filetoken")
		require.Equal(t, "tok", bearer(dir, "tok"))
	})

	t.Run("file when token absent", func(t *testing.T) {
		dir := t.TempDir()
		writeTokenFile(t, dir, "filetoken")
		require.Equal(t, "filetoken", bearer(dir, ""))
	})

	t.Run("empty when both absent", func(t *testing.T) {
		require.Equal(t, "", bearer(t.TempDir(), ""))
	})
}

func writeTokenFile(t *testing.T, dir, password string) {
	t.Helper()
	encoded, err := json.Marshal(&compute.GitCredentialsHTTP{Password: password})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "vcsaccess.token"), encoded, 0600))
}
