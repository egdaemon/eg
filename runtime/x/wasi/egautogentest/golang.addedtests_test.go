package egautogentest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddedTests(t *testing.T) {
	existing := []byte("package sample\n\nimport \"testing\"\n\nfunc TestOld(t *testing.T) {}\n\nfunc TestOlder(t *testing.T) {}\n")

	t.Run("runs only the tests that were added", func(t *testing.T) {
		require.Equal(t, []string{"TestNew"}, addedTests(existing, "sample", []string{"TestOld", "TestNew", "TestOlder"}))
	})

	t.Run("runs everything when nothing was added", func(t *testing.T) {
		require.Equal(t, []string{"TestOld", "TestOlder"}, addedTests(existing, "sample", []string{"TestOld", "TestOlder"}))
	})

	t.Run("runs everything without an existing file", func(t *testing.T) {
		require.Equal(t, []string{"TestNew"}, addedTests(nil, "sample", []string{"TestNew"}))
	})

	t.Run("runs everything when the existing file has no tests", func(t *testing.T) {
		require.Equal(t, []string{"TestNew"}, addedTests([]byte("package sample\n"), "sample", []string{"TestNew"}))
	})
}
