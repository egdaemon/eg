package egautogen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCollectUsage(t *testing.T) {
	pkg, err := loadPackage(".fixtures/golang/example.1")
	require.NoError(t, err)

	t.Run("renders the existing tests calling the function", func(t *testing.T) {
		usage := collectUsage(pkg, "Greet")
		require.Contains(t, usage, "func TestGreet")
		require.Contains(t, usage, "Greet(Account{Name: \"world\"})")
	})

	t.Run("reports when there is no usage", func(t *testing.T) {
		require.Equal(t, "no existing usage of this function was found.", collectUsage(pkg, "DoesNotExist"))
	})
}
