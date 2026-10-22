package migrations_test

import (
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/stretchr/testify/require"
)

func TestPath(t *testing.T) {
	t.Setenv("CACHE_DIRECTORY", "/var/cache/eg-test")
	require.Equal(t, filepath.Join("/var/cache/eg-test", migrations.CLI), migrations.Path(migrations.CLI))
	require.Equal(t, filepath.Join("/var/cache/eg-test", migrations.Runner), migrations.Path(migrations.Runner))
	require.NotEqual(t, migrations.Path(migrations.CLI), migrations.Path(migrations.Runner), "runner and cli use distinct files")
}
