package egautogentest

import (
	"context"
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/coverage"
	"github.com/stretchr/testify/require"
)

func TestFunctionsPaths(t *testing.T) {
	t.Setenv(_eg.EnvComputeWorkingDirectory, "/workload/eg")

	seq := functions(func(context.Context) ([]coverage.Report, error) {
		return []coverage.Report{
			{Path: "/workload/eg/authn/authn.go", Fnname: "A", Hits: 2},
			{Path: "authn/b.go", Fnname: "B"},
			{Path: "/workload/other/c.go", Fnname: "C"},
		}, nil
	})

	var paths []string
	for fn := range seq.Each(t.Context()) {
		paths = append(paths, fn.Path)
	}
	require.NoError(t, seq.Err())
	require.Equal(t, []string{"authn/authn.go", "authn/b.go", "/workload/other/c.go"}, paths)
}
