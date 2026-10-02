package golangcov

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/stretchr/testify/require"
)

const sample = `package sample

type Account struct {
	Name string
}

func Greet(a Account) string {
	if a.Name == "" {
		return "hello"
	}
	return "hello " + a.Name
}

func (t *Account) Rename(n string) {
	t.Name = n
}
`

// blocks correspond to the line/column positions within sample.
const profile = `mode: count
example.com/sample/sample.go:7.31,8.18 1 1
example.com/sample/sample.go:11.2,11.26 1 1
example.com/sample/sample.go:8.18,10.3 1 0
example.com/sample/sample.go:14.37,16.2 1 0
`

func writeSample(t *testing.T) (module string, profiles string) {
	module = t.TempDir()
	profiles = t.TempDir()

	require.NoError(t, os.WriteFile(filepath.Join(module, "go.mod"), []byte("module example.com/sample\n\ngo 1.25\n"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(module, "sample.go"), []byte(sample), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(profiles, "sample.cov"), []byte(profile), 0644))

	return module, profiles
}

func collect(t *testing.T, dir string, resolve Resolver) (reports []*coverage.Report) {
	for r, err := range Coverage(context.Background(), dir, resolve) {
		require.NoError(t, err)
		reports = append(reports, r)
	}

	return reports
}

func TestModulesResolver(t *testing.T) {
	module, _ := writeSample(t)
	resolve := Modules(slices.Values([]string{filepath.Join(module, "go.mod")}))

	path, ok := resolve("example.com/sample/sample.go")
	require.True(t, ok)
	require.Equal(t, filepath.Join(module, "sample.go"), path)

	path, ok = resolve("example.com/sample/nested/file.go")
	require.True(t, ok)
	require.Equal(t, filepath.Join(module, "nested", "file.go"), path)

	_, ok = resolve("example.com/samplex/sample.go")
	require.False(t, ok)

	_, ok = resolve("example.com/other/sample.go")
	require.False(t, ok)
}

func TestCoverageFunctions(t *testing.T) {
	module, profiles := writeSample(t)
	reports := collect(t, profiles, Modules(slices.Values([]string{filepath.Join(module, "go.mod")})))
	require.Len(t, reports, 3)

	path := filepath.Join(module, "sample.go")

	require.Equal(t, path, reports[0].Path)
	require.Equal(t, "", reports[0].Fnname)
	require.InDelta(t, 50.0, reports[0].Statements, 0.01)

	require.Equal(t, path, reports[1].Path)
	require.Equal(t, "Greet", reports[1].Fnname)
	require.Equal(t, int64(1), reports[1].Hits)
	require.InDelta(t, 66.66, reports[1].Statements, 0.01)

	require.Equal(t, path, reports[2].Path)
	require.Equal(t, "Account.Rename", reports[2].Fnname)
	require.Equal(t, int64(0), reports[2].Hits)
	require.InDelta(t, 0.0, reports[2].Statements, 0.01)
}

func TestCoverageUnresolved(t *testing.T) {
	_, profiles := writeSample(t)
	reports := collect(t, profiles, nil)
	require.Len(t, reports, 1)
	require.Equal(t, "example.com/sample/sample.go", reports[0].Path)
	require.Equal(t, "", reports[0].Fnname)
}
