package lan_test

import (
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/lan"
	"github.com/stretchr/testify/require"
)

func TestPath(t *testing.T) {
	t.Run("within a given directory", func(t *testing.T) {
		require.Equal(t, filepath.Join("/var/cache/eg-daemon", "lan.runner.duckdb"), lan.PathIn("/var/cache/eg-daemon", lan.RoleRunner))
		require.Equal(t, filepath.Join("/home/me/.cache/eg", "lan.client.duckdb"), lan.PathIn("/home/me/.cache/eg", lan.RoleClient))
	})

	require.NotEqual(t, lan.Path(lan.RoleClient), lan.Path(lan.RoleRunner), "roles use distinct files")
	require.Equal(t, "lan.client.duckdb", filepath.Base(lan.Path(lan.RoleClient)))
	require.Equal(t, "lan.runner.duckdb", filepath.Base(lan.Path(lan.RoleRunner)))
}
