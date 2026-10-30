package eggolang

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/duckproxy"
	"github.com/egdaemon/eg/duckproxyserver"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/interp/events"
	"github.com/egdaemon/eg/runtime/x/wasi/egcoverage"
	"github.com/stretchr/testify/require"
)

// analyticsproxy starts a duckdb behind a duckproxy unix socket, returns the database and a function
// that reports coverage to it through a fresh connection, the way a guest does.
func analyticsproxy(t *testing.T) (*sql.DB, func(context.Context, ...*coverage.Report) error) {
	t.Helper()

	dir := t.TempDir()
	socket := filepath.Join(dir, "analytics.socket")

	db, err := sql.Open("duckdb", filepath.Join(dir, "analytics.db"))
	require.NoError(t, err)
	t.Cleanup(func() { db.Close() })

	pctx, pdone := context.WithTimeout(t.Context(), 10*time.Second)
	defer pdone()
	require.NoError(t, events.PrepareDB(pctx, db))

	sctx, stop := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() {
		served <- duckproxyserver.ListenUnix(sctx, socket, duckproxyserver.New(db))
	}()
	t.Cleanup(func() {
		stop()
		<-served
	})

	dial := func(ctx context.Context, network, addr string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}

	require.Eventually(t, func() bool {
		c := sql.OpenDB(duckproxy.NewConnector(socket, dial))
		defer c.Close()
		return c.PingContext(context.Background()) == nil
	}, 2*time.Second, 10*time.Millisecond)

	return db, func(ctx context.Context, batch ...*coverage.Report) error {
		client := sql.OpenDB(duckproxy.NewConnector(socket, dial))
		defer client.Close()

		rows := make([]coverage.Report, 0, len(batch))
		for _, r := range batch {
			rows = append(rows, *r)
		}

		return sqlx.Discard(sqlx.Scan(coverage.NewReportBatchInsert(ctx, client, rows...)))
	}
}

// coveragemodule writes a go module containing a function per entry of hits and the matching
// coverage profile, and points the working and ephemeral directories at them.
func coveragemodule(t *testing.T, hits ...int) {
	t.Helper()

	root := t.TempDir()
	tmp := t.TempDir()
	profiles := filepath.Join(tmp, ".eg.coverage")
	require.NoError(t, os.MkdirAll(profiles, 0700))
	t.Setenv(_eg.EnvComputeWorkingDirectory, root)
	t.Setenv("TMPDIR", tmp)
	gomod := filepath.Join(root, "go.mod")

	var (
		source  strings.Builder
		profile strings.Builder
	)
	source.WriteString("package sample\n\n")
	profile.WriteString("mode: count\n")
	for i, h := range hits {
		line := 3 + i*4
		fmt.Fprintf(&source, "func F%03d() {\n\tprintln()\n}\n\n", i)
		fmt.Fprintf(&profile, "example.com/sample/sample.go:%d.13,%d.2 1 %d\n", line, line+2, h)
	}

	require.NoError(t, os.WriteFile(gomod, []byte("module example.com/sample\n"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(root, "sample.go"), []byte(source.String()), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(profiles, "sample.cov"), []byte(profile.String()), 0600))

}

func TestCoverage(t *testing.T) {
	t.Run("reports file and function coverage", func(t *testing.T) {
		db, report := analyticsproxy(t)
		coveragemodule(t, 3, 0)

		require.NoError(t, egcoverage.Into(report, Coverage()...)(t.Context(), nil))

		rows, err := db.Query(`SELECT fnname, hits FROM 'eg.metrics.coverage' WHERE fnname != '' ORDER BY fnname`)
		require.NoError(t, err)
		defer rows.Close()

		type fn struct {
			name string
			hits int64
		}
		var found []fn
		for rows.Next() {
			var f fn
			require.NoError(t, rows.Scan(&f.name, &f.hits))
			found = append(found, f)
		}
		require.NoError(t, rows.Err())
		require.Equal(t, []fn{{"F000", 3}, {"F001", 0}}, found)

		var total int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM 'eg.metrics.coverage'`).Scan(&total))
		require.Equal(t, 3, total) // two functions and the file.
	})

	t.Run("flushes full batches", func(t *testing.T) {
		db, report := analyticsproxy(t)
		coveragemodule(t, slices.Repeat([]int{1}, 130)...)

		require.NoError(t, egcoverage.Into(report, Coverage()...)(t.Context(), nil))

		var total int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM 'eg.metrics.coverage'`).Scan(&total))
		require.Equal(t, 131, total) // 130 functions and the file.
	})

	t.Run("empty profile directory", func(t *testing.T) {
		db, report := analyticsproxy(t)

		tmp := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(tmp, ".eg.coverage"), 0700))
		t.Setenv(_eg.EnvComputeWorkingDirectory, t.TempDir())
		t.Setenv("TMPDIR", tmp)

		require.NoError(t, egcoverage.Into(report, Coverage()...)(t.Context(), nil))

		var total int
		require.NoError(t, db.QueryRow(`SELECT count(*) FROM 'eg.metrics.coverage'`).Scan(&total))
		require.Zero(t, total)
	})

	t.Run("reporter errors are returned", func(t *testing.T) {
		boom := errors.New("boom")
		failing := func(context.Context, ...*coverage.Report) error { return boom }

		t.Run("final flush", func(t *testing.T) {
			coveragemodule(t, 1)
			require.ErrorIs(t, egcoverage.Into(failing, Coverage()...)(t.Context(), nil), boom)
		})

		t.Run("full batch", func(t *testing.T) {
			coveragemodule(t, slices.Repeat([]int{1}, 130)...)
			require.ErrorIs(t, egcoverage.Into(failing, Coverage()...)(t.Context(), nil), boom)
		})
	})

	t.Run("unparsable profile is an error", func(t *testing.T) {
		tmp := t.TempDir()
		profiles := filepath.Join(tmp, ".eg.coverage")
		require.NoError(t, os.MkdirAll(profiles, 0700))
		t.Setenv(_eg.EnvComputeWorkingDirectory, t.TempDir())
		t.Setenv("TMPDIR", tmp)
		require.NoError(t, os.WriteFile(filepath.Join(profiles, "bad.cov"), []byte("not a profile"), 0600))

		err := egcoverage.Into(func(context.Context, ...*coverage.Report) error { return nil }, Coverage()...)(t.Context(), nil)
		require.ErrorContains(t, err, "unable to parse profiles")
	})
}
