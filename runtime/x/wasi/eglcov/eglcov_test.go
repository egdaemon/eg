package eglcov_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/runtime/x/wasi/eglcov"
	"github.com/stretchr/testify/require"
)

const document = `TN:
SF:src/a.ts
FN:1,Fa
FNDA:3,Fa
LF:10
LH:5
BRF:4
BRH:1
end_of_record
SF:src/b.ts
LF:4
LH:4
end_of_record
`

func TestCoverage(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkspaceDirectory, "/ws")

	t.Run("directory", func(t *testing.T) {
		ws := t.TempDir()
		t.Setenv(_eg.EnvComputeWorkspaceDirectory, ws)

		dir := filepath.Join(ws, "proj", "coverage")
		require.NoError(t, os.MkdirAll(dir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "lcov.info"), []byte(document), 0600))

		var got []*coverage.Report
		for r, err := range eglcov.Coverage(dir).Seq(t.Context()) {
			require.NoError(t, err)
			got = append(got, r)
		}

		require.Len(t, got, 3) // two files and one function.
		require.Equal(t, "proj/src/a.ts", got[0].Path)
		require.EqualValues(t, 50, got[0].Statements)
		require.EqualValues(t, 25, got[0].Branches)
		require.Equal(t, "Fa", got[1].Fnname)
		require.EqualValues(t, 3, got[1].Hits)
		require.Equal(t, "proj/src/b.ts", got[2].Path)
		require.EqualValues(t, 100, got[2].Statements)
	})

	t.Run("an explicit root overrides the parent of the directory", func(t *testing.T) {
		ws := t.TempDir()
		t.Setenv(_eg.EnvComputeWorkspaceDirectory, ws)

		dir := filepath.Join(ws, "proj", "coverage")
		require.NoError(t, os.MkdirAll(dir, 0700))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "lcov.info"), []byte(document), 0600))

		var paths []string
		for r, err := range eglcov.Coverage(dir).Root(filepath.Join(ws, "other")).Seq(t.Context()) {
			require.NoError(t, err)
			paths = append(paths, r.Path)
		}

		require.Equal(t, []string{"other/src/a.ts", "other/src/a.ts", "other/src/b.ts"}, paths)
	})

	t.Run("root makes paths workspace relative", func(t *testing.T) {
		var paths []string
		for r, err := range eglcov.CoverageFrom(strings.NewReader(document)).Root("/ws/egmeta/console").Seq(t.Context()) {
			require.NoError(t, err)
			paths = append(paths, r.Path)
		}

		require.Equal(t, []string{"egmeta/console/src/a.ts", "egmeta/console/src/a.ts", "egmeta/console/src/b.ts"}, paths)
	})

	t.Run("root outside the workspace is kept", func(t *testing.T) {
		for r, err := range eglcov.CoverageFrom(strings.NewReader(document)).Root("/elsewhere").Seq(t.Context()) {
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(r.Path, "/elsewhere/src/"), r.Path)
		}
	})

	t.Run("invalid lcov is an error", func(t *testing.T) {
		var failed error
		for _, err := range eglcov.CoverageFrom(strings.NewReader("FNDA:x\n")).Seq(t.Context()) {
			if err != nil {
				failed = err
			}
		}

		require.Error(t, failed)
	})
}
