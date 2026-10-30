package egautogen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectTypes(t *testing.T) {
	pkg, err := loadPackage(".fixtures/golang/example.1")
	require.NoError(t, err)

	decl := findFuncDecl(pkg, "Greet")
	require.NotNil(t, decl)

	rendered := collectTypes(pkg, decl)

	t.Run("renders the types the function references", func(t *testing.T) {
		require.Contains(t, rendered, "type Account struct")
		require.Contains(t, rendered, "Name string")
	})

	t.Run("ignores types the function does not reference", func(t *testing.T) {
		require.NotContains(t, rendered, "fixture")
	})
}
