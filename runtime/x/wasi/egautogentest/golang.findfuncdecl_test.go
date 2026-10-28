package egautogentest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindFuncDecl(t *testing.T) {
	pkg, err := loadPackage(".fixtures/golang/example.1")
	require.NoError(t, err)

	t.Run("finds a function", func(t *testing.T) {
		decl := findFuncDecl(pkg, "Greet")
		require.NotNil(t, decl)
		require.Equal(t, "Greet", decl.Name.Name)
	})

	t.Run("finds a method by Type.Method", func(t *testing.T) {
		decl := findFuncDecl(pkg, "Account.Rename")
		require.NotNil(t, decl)
		require.Equal(t, "Rename", decl.Name.Name)
	})

	t.Run("does not find a method by its bare name", func(t *testing.T) {
		require.Nil(t, findFuncDecl(pkg, "Rename"))
	})

	t.Run("does not find unknown functions", func(t *testing.T) {
		require.Nil(t, findFuncDecl(pkg, "DoesNotExist"))
	})
}
