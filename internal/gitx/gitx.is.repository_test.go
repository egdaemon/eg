package gitx

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsRepository(t *testing.T) {
	t.Run("true for a git repository", func(t *testing.T) {
		dir := t.TempDir()
		initRepo(t, dir, map[string]string{"main.go": "package main"})
		require.True(t, IsRepository(dir))
	})

	t.Run("false for a plain directory", func(t *testing.T) {
		require.False(t, IsRepository(t.TempDir()))
	})
}
