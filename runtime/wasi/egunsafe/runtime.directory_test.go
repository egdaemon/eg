package egunsafe_test

import (
	"testing"

	_eg "github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/stretchr/testify/require"
)

func TestRuntimeDirectory(t *testing.T) {
	t.Run("environment overrides the default mount", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeRuntimeDirectory, "/runner/controlled")
		require.Equal(t, "/runner/controlled", egunsafe.RuntimeDirectory())
	})

	t.Run("environment is joined with paths", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeRuntimeDirectory, "/runner/controlled")
		require.Equal(t, "/runner/controlled/foo/bar", egunsafe.RuntimeDirectory("foo", "bar"))
	})

	t.Run("unset falls back to the default mount", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeRuntimeDirectory, "")
		require.Equal(t, _eg.DefaultMountRoot(_eg.RuntimeDirectory), egunsafe.RuntimeDirectory())
	})

	t.Run("unset falls back to the default mount joined with paths", func(t *testing.T) {
		t.Setenv(_eg.EnvComputeRuntimeDirectory, "")
		require.Equal(t, _eg.DefaultMountRoot(_eg.RuntimeDirectory, "foo", "bar"), egunsafe.RuntimeDirectory("foo", "bar"))
	})
}
