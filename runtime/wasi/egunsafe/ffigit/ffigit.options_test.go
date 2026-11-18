package ffigit

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCloneOptions(t *testing.T) {
	t.Run("defaults to the process environment", func(t *testing.T) {
		t.Setenv("EG_TEST_CLONE_OPTIONS", "value")
		o := newcloneopts()
		require.Equal(t, os.Environ(), o.environ)
		require.Contains(t, o.environ, "EG_TEST_CLONE_OPTIONS=value")
	})

	t.Run("environ replaces the process environment", func(t *testing.T) {
		t.Setenv("EG_TEST_CLONE_OPTIONS", "value")
		o := newcloneopts(Environ("A=1", "B=2"))
		require.Equal(t, []string{"A=1", "B=2"}, o.environ)
	})

	t.Run("the last environ wins", func(t *testing.T) {
		o := newcloneopts(Environ("A=1"), Environ("B=2"))
		require.Equal(t, []string{"B=2"}, o.environ)
	})
}
