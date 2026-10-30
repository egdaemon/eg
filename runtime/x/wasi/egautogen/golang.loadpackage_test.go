package egautogen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadPackage(t *testing.T) {
	t.Run("loads the package name and every go file", func(t *testing.T) {
		pkg, err := loadPackage(".fixtures/golang/example.1")
		require.NoError(t, err)
		require.Equal(t, "sample", pkg.name)
		require.Len(t, pkg.files, 2)
	})

	t.Run("fails without go files", func(t *testing.T) {
		_, err := loadPackage(t.TempDir())
		require.Error(t, err)
	})
}
