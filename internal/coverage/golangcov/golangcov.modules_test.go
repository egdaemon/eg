package golangcov_test

import (
	"slices"
	"testing"

	"github.com/egdaemon/eg/internal/coverage/golangcov"
	"github.com/stretchr/testify/require"
)

func TestModules(t *testing.T) {
	resolve := golangcov.Modules(slices.Values([]string{".fixtures/example.1/go.mod"}))

	t.Run("resolves files within the module", func(t *testing.T) {
		path, ok := resolve("example.com/sample/sample.go")
		require.True(t, ok)
		require.Equal(t, ".fixtures/example.1/sample.go", path)
	})

	t.Run("resolves nested paths", func(t *testing.T) {
		path, ok := resolve("example.com/sample/nested/file.go")
		require.True(t, ok)
		require.Equal(t, ".fixtures/example.1/nested/file.go", path)
	})

	t.Run("does not match module path prefixes", func(t *testing.T) {
		path, ok := resolve("example.com/samplex/sample.go")
		require.False(t, ok)
		require.Equal(t, "example.com/samplex/sample.go", path)
	})

	t.Run("does not resolve other modules", func(t *testing.T) {
		path, ok := resolve("example.com/other/sample.go")
		require.False(t, ok)
		require.Equal(t, "example.com/other/sample.go", path)
	})
}
