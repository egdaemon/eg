package egautogen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractTest(t *testing.T) {
	t.Run("source is forced into the package", func(t *testing.T) {
		response := "package wrong\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {\n}\n\nfunc TestGreetEmpty(t *testing.T) {\n}\n\nfunc helper() {}\n"
		src, tests, err := extractTest(response, "sample")
		require.NoError(t, err)
		require.Equal(t, []string{"TestGreet", "TestGreetEmpty"}, tests)
		require.Contains(t, string(src), "package sample\n")
	})

	t.Run("responses without tests are rejected", func(t *testing.T) {
		_, _, err := extractTest("package sample\n\nfunc helper() {}\n", "sample")
		require.Error(t, err)
	})

	t.Run("invalid go is rejected", func(t *testing.T) {
		_, _, err := extractTest("i couldn't write a test for this", "sample")
		require.Error(t, err)
	})

	t.Run("fenced code is rejected", func(t *testing.T) {
		_, _, err := extractTest("```go\npackage sample\n\nimport \"testing\"\n\nfunc TestGreet(t *testing.T) {}\n```", "sample")
		require.Error(t, err)
	})
}
