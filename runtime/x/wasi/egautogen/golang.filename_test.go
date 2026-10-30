package egautogen

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGolangFilename(t *testing.T) {
	fn := Fn{Path: "/src/backoff/backoff.go", Name: "DynamicHashHour"}

	t.Run("default pattern", func(t *testing.T) {
		name, err := golangFilename(DefaultGolangFilename, fn)
		require.NoError(t, err)
		require.Equal(t, "backoff.dynamichashhour.autogen_test.go", name)
	})

	t.Run("methods are named type_method", func(t *testing.T) {
		name, err := golangFilename(DefaultGolangFilename, Fn{Path: "/src/sample/sample.go", Name: "Account.Rename"})
		require.NoError(t, err)
		require.Equal(t, "sample.account_rename.autogen_test.go", name)
	})

	t.Run("custom pattern", func(t *testing.T) {
		name, err := golangFilename("gen_%fn.name%_test.go", fn)
		require.NoError(t, err)
		require.Equal(t, "gen_dynamichashhour_test.go", name)
	})

	t.Run("a pattern without tokens names every function the same", func(t *testing.T) {
		name, err := golangFilename("autogen_test.go", fn)
		require.NoError(t, err)
		require.Equal(t, "autogen_test.go", name)
	})

	t.Run("replaces git patterns", func(t *testing.T) {
		t.Setenv("EG_GIT_HEAD_COMMIT", "0123456789abcdef0123456789abcdef01234567")
		name, err := golangFilename("%fn.name%.%git.hash%_test.go", fn)
		require.NoError(t, err)
		require.Equal(t, "dynamichashhour.0123456789abcdef0123456789abcdef01234567_test.go", name)
	})

	t.Run("rejects a path", func(t *testing.T) {
		_, err := golangFilename("../%fn.name%_test.go", fn)
		require.Error(t, err)
	})

	t.Run("rejects a name that is not a test file", func(t *testing.T) {
		_, err := golangFilename("%fn.name%.go", fn)
		require.Error(t, err)
	})
}

func TestGolangOptionsFilename(t *testing.T) {
	t.Run("defaults to the default pattern", func(t *testing.T) {
		require.Equal(t, DefaultGolangFilename, Golang(FromSpecified()).resolve().filename)
	})

	t.Run("overrides the pattern", func(t *testing.T) {
		require.Equal(t, "x_test.go", Golang(FromSpecified()).Filename("x_test.go").resolve().filename)
	})
}
