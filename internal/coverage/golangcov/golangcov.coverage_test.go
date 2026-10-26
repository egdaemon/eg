package golangcov_test

import (
	"slices"
	"testing"

	"github.com/egdaemon/eg/internal/coverage"
	"github.com/egdaemon/eg/internal/coverage/golangcov"
	"github.com/stretchr/testify/require"
)

func TestCoverage(t *testing.T) {
	t.Run("reports per file and per function coverage when resolved", func(t *testing.T) {
		resolve := golangcov.Modules(slices.Values([]string{".fixtures/example.1/go.mod"}))

		var reports []*coverage.Report
		for r, err := range golangcov.Coverage(t.Context(), ".fixtures/example.1/profiles", resolve) {
			require.NoError(t, err)
			reports = append(reports, r)
		}
		require.Len(t, reports, 3)

		path := ".fixtures/example.1/sample.go"

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
	})

	t.Run("reports per file coverage only when unresolved", func(t *testing.T) {
		var reports []*coverage.Report
		for r, err := range golangcov.Coverage(t.Context(), ".fixtures/example.1/profiles", golangcov.Files) {
			require.NoError(t, err)
			reports = append(reports, r)
		}
		require.Len(t, reports, 1)
		require.Equal(t, "example.com/sample/sample.go", reports[0].Path)
		require.Equal(t, "", reports[0].Fnname)
	})

	t.Run("stops without panicking when the consumer breaks early", func(t *testing.T) {
		resolve := golangcov.Modules(slices.Values([]string{".fixtures/example.1/go.mod"}))

		var count int
		for _, err := range golangcov.Coverage(t.Context(), ".fixtures/example.1/profiles", resolve) {
			require.NoError(t, err)
			count++
			break
		}
		require.Equal(t, 1, count)
	})
}
