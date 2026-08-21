package egworkloads_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"slices"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/egworkloads"
	"github.com/stretchr/testify/require"
)

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.24\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644))

	db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "analytics.db"))
	require.NoError(t, err)
	defer db.Close()

	require.NoError(t, egworkloads.Detect(t.Context(), db, dir))

	scanner := egworkloads.DiscoveredLookup(t.Context(), db)
	defer scanner.Close()

	var paths []string
	for scanner.Next() {
		var w egworkloads.Discovered
		require.NoError(t, scanner.Scan(&w))
		require.NotEmpty(t, w.ID)
		require.False(t, w.Ts.IsZero())
		paths = append(paths, w.Path)
	}
	require.NoError(t, scanner.Err())

	slices.Sort(paths)
	require.Equal(t, []string{".", "sub"}, paths)
}
