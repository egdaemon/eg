package main

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe/egworkloads"
	"github.com/stretchr/testify/require"
)

func TestRecordDiscoveredWorkloads(t *testing.T) {
	setupFixture := func(t *testing.T) string {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module fixture\n\ngo 1.24\n"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644))
		return dir
	}

	openDB := func(t *testing.T) *sql.DB {
		t.Helper()
		db, err := sql.Open("duckdb", filepath.Join(t.TempDir(), "analytics.db"))
		require.NoError(t, err)
		t.Cleanup(func() { db.Close() })
		return db
	}

	t.Run("records workloads for a default-branch build", func(t *testing.T) {
		dir := setupFixture(t)
		db := openDB(t)

		recordDiscoveredWorkloads(t.Context(), db, dir,
			eg.EnvGitHeadCommit+"=abc123",
			eg.EnvGitBaseCommit+"=abc123",
		)

		scanner := egworkloads.DiscoveredLookup(t.Context(), db)
		defer scanner.Close()
		require.True(t, scanner.Next(), "expected a recorded workload")
		require.NoError(t, scanner.Err())
	})

	t.Run("skips a PR/feature-branch build", func(t *testing.T) {
		dir := setupFixture(t)
		db := openDB(t)

		recordDiscoveredWorkloads(t.Context(), db, dir,
			eg.EnvGitHeadCommit+"=abc123",
			eg.EnvGitBaseCommit+"=def456",
		)

		var count int
		row := db.QueryRow(`SELECT count(*) FROM information_schema.tables WHERE table_name = 'eg_workloads_discovered'`)
		require.NoError(t, row.Scan(&count))
		require.Zero(t, count, "table should never be created for a non-default-branch build")
	})
}
