package coverage_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/stretchr/testify/require"
)

func TestFrom(t *testing.T) {
	t.Run("yields the rows of the query and closes the database", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		sqltestx.Exec(t, db, `CREATE TABLE 'eg.metrics.coverage' (
			id UUID PRIMARY KEY DEFAULT uuidv7(),
			path TEXT NOT NULL,
			statements FLOAT4 NOT NULL,
			branches FLOAT4 NOT NULL,
			fnname TEXT NOT NULL DEFAULT '',
			hits BIGINT NOT NULL DEFAULT 0
		)`)
		sqltestx.Exec(t, db, `INSERT INTO 'eg.metrics.coverage' (path, statements, branches, fnname, hits) VALUES
			('a.go', 1, 1, 'A', 3),
			('b.go', 1, 1, 'B', 1),
			('c.go', 1, 1, 'C', 2)`)

		seq := coverage.From(func(context.Context) (*sql.DB, error) { return db, nil }, coverage.WorstSearchBuilder())

		var names []string
		for r := range seq.Each(t.Context()) {
			names = append(names, r.Fnname)
		}
		require.NoError(t, seq.Err())
		require.Equal(t, []string{"B", "C", "A"}, names)
		require.Error(t, db.PingContext(t.Context()))
	})

	t.Run("closes the database when iteration stops early", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		sqltestx.Exec(t, db, `CREATE TABLE 'eg.metrics.coverage' (
			id UUID PRIMARY KEY DEFAULT uuidv7(),
			path TEXT NOT NULL,
			statements FLOAT4 NOT NULL,
			branches FLOAT4 NOT NULL,
			fnname TEXT NOT NULL DEFAULT '',
			hits BIGINT NOT NULL DEFAULT 0
		)`)
		sqltestx.Exec(t, db, `INSERT INTO 'eg.metrics.coverage' (path, statements, branches, fnname, hits) VALUES
			('a.go', 1, 1, 'A', 3),
			('b.go', 1, 1, 'B', 1)`)

		seq := coverage.From(func(context.Context) (*sql.DB, error) { return db, nil }, coverage.WorstSearchBuilder())

		var names []string
		for r := range seq.Each(t.Context()) {
			names = append(names, r.Fnname)
			break
		}
		require.NoError(t, seq.Err())
		require.Equal(t, []string{"B"}, names)
		require.Error(t, db.PingContext(t.Context()))
	})

	t.Run("an error opening the database is the error of the sequence", func(t *testing.T) {
		cause := errors.New("unavailable")
		seq := coverage.From(func(context.Context) (*sql.DB, error) { return nil, cause }, coverage.WorstSearchBuilder())

		var count int
		for range seq.Each(t.Context()) {
			count++
		}
		require.Zero(t, count)
		require.ErrorIs(t, seq.Err(), cause)
	})
}

func TestQuery(t *testing.T) {
	t.Run("yields the rows of the query and leaves the database open", func(t *testing.T) {
		db := sqltestx.DuckDB(t)
		sqltestx.Exec(t, db, `CREATE TABLE 'eg.metrics.coverage' (
			id UUID PRIMARY KEY DEFAULT uuidv7(),
			path TEXT NOT NULL,
			statements FLOAT4 NOT NULL,
			branches FLOAT4 NOT NULL,
			fnname TEXT NOT NULL DEFAULT '',
			hits BIGINT NOT NULL DEFAULT 0
		)`)
		sqltestx.Exec(t, db, `INSERT INTO 'eg.metrics.coverage' (path, statements, branches, fnname, hits) VALUES
			('a.go', 1, 1, 'A', 3),
			('b.go', 1, 1, 'B', 1),
			('c.go', 1, 1, 'C', 2)`)

		seq := coverage.Query(db, coverage.WorstSearchBuilder())

		var names []string
		for r := range seq.Each(t.Context()) {
			names = append(names, r.Fnname)
		}
		require.NoError(t, seq.Err())
		require.Equal(t, []string{"B", "C", "A"}, names)
		require.NoError(t, db.PingContext(t.Context()))
	})
}
