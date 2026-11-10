package fsx_test

import (
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/fsx"
	"github.com/stretchr/testify/require"
)

func TestJoinIfPresent(t *testing.T) {
	t.Run("blank root stays blank", func(t *testing.T) {
		require.Equal(t, "", fsx.JoinIfPresent("", "account"))
	})

	t.Run("joins the elements onto the root", func(t *testing.T) {
		require.Equal(t, filepath.Join("/repos", "account", "a"), fsx.JoinIfPresent("/repos", "account", "a"))
	})

	t.Run("no elements is the root", func(t *testing.T) {
		require.Equal(t, "/repos", fsx.JoinIfPresent("/repos"))
	})
}
