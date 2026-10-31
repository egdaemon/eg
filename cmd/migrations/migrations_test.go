package migrations_test

import (
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/stretchr/testify/require"
)

func TestPath(t *testing.T) {
	t.Setenv(userx.EnvCacheDirectory, "/var/cache/eg-test")
	require.Equal(t, filepath.Join("/var/cache/eg-test", migrations.CLI), migrations.Path(migrations.CLI))
	require.Equal(t, filepath.Join("/var/cache/eg-test", migrations.Daemon), migrations.Path(migrations.Daemon))
	require.NotEqual(t, migrations.Path(migrations.CLI), migrations.Path(migrations.Daemon), "runner and cli use distinct files")
}

func TestOpen(t *testing.T) {
	t.Run("creates the parent directory and applies the migrations", func(t *testing.T) {
		db, err := migrations.Open(t.Context(), filepath.Join(t.TempDir(), "missing", "dir", "test.db"))
		require.NoError(t, err)
		defer db.Close()

		var n int
		require.NoError(t, db.QueryRowContext(t.Context(), "SELECT count(*) FROM lan_peers").Scan(&n))
		require.NoError(t, db.QueryRowContext(t.Context(), "SELECT count(*) FROM compute_history").Scan(&n))
	})
}
