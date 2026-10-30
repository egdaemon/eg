package egautogen

import (
	"context"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/iterx"
	"github.com/egdaemon/eg/internal/sqltestx"
	"github.com/stretchr/testify/require"
)

func TestFunctionsPaths(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkingDirectory, "/workload/eg")

	seq := functions(func(context.Context) iterx.Seq[coverage.Report] {
		return iterx.Slice(
			coverage.Report{Path: "/workload/eg/authn/authn.go", Fnname: "A", Hits: 2},
			coverage.Report{Path: "authn/b.go", Fnname: "B"},
			coverage.Report{Path: "/workload/other/c.go", Fnname: "C"},
		)
	})

	var paths []string
	for fn := range seq.Each(t.Context()) {
		paths = append(paths, fn.Path)
	}
	require.NoError(t, seq.Err())
	require.Equal(t, []string{"authn/authn.go", "authn/b.go", "/workload/other/c.go"}, paths)
}

func TestFromPaths(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkingDirectory, "/workload/eg")

	var paths []string
	for fn := range FromSpecified(Fn{Path: "/workload/eg/a.go"}, Fn{Path: "b.go"}, Fn{Path: "/other/c.go"}).Each(t.Context()) {
		paths = append(paths, fn.Path)
	}
	require.Equal(t, []string{"a.go", "b.go", "/other/c.go"}, paths)
}

func TestFrom(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkingDirectory, "/workload/eg")

	db := sqltestx.DuckDB(t)
	sqltestx.Exec(t, db, `CREATE TABLE 'eg.metrics.coverage' (
		id UUID PRIMARY KEY DEFAULT uuidv7(),
		path TEXT NOT NULL,
		statements FLOAT4 NOT NULL,
		branches FLOAT4 NOT NULL,
		fnname TEXT NOT NULL DEFAULT '',
		hits BIGINT NOT NULL DEFAULT 0,
		generated BOOLEAN NOT NULL DEFAULT false
	)`)
	sqltestx.Exec(t, db, `INSERT INTO 'eg.metrics.coverage' (path, statements, branches, fnname, hits) VALUES
		('/workload/eg/a.go', 50, 1, 'A', 3),
		('/workload/eg/b.go', 10, 1, 'B', 1),
		('/workload/eg/c.go', 90, 1, 'C', 2),
		('/workload/eg/d.py', 0, 1, 'D', 0)`)

	t.Run("yields the least covered functions first with paths relative to the workspace", func(t *testing.T) {
		seq := From(db, Worst(Language("go")))

		var fns []Fn
		for fn := range seq.Each(t.Context()) {
			fns = append(fns, fn)
		}
		require.NoError(t, seq.Err())
		require.Equal(t, []Fn{{Path: "b.go", Name: "B", Hits: 1}, {Path: "a.go", Name: "A", Hits: 3}, {Path: "c.go", Name: "C", Hits: 2}}, fns)
	})

	t.Run("applies the filters before the limit", func(t *testing.T) {
		seq := From(db, Worst(Language("go"), Coverage(40, 100))).Limit(1)

		var names []string
		for fn := range seq.Each(t.Context()) {
			names = append(names, fn.Name)
		}
		require.Equal(t, []string{"A"}, names)
	})

	t.Run("leaves the database open", func(t *testing.T) {
		for range From(db, Worst()).Each(t.Context()) {
		}

		require.NoError(t, db.PingContext(t.Context()))
	})
}
