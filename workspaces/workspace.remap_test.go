package workspaces_test

import (
	"testing"

	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestRemap(t *testing.T) {
	ws := workspaces.Context{
		Root:         "/w/.eg.workload/m.bbbbbbbb.d",
		RuntimeDir:   "/w/.eg.workload/m.bbbbbbbb.d/.eg.runtime",
		CacheDir:     "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache",
		WorkspaceDir: "/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace",
		WorkingDir:   "/w/.eg.workload/m.bbbbbbbb.d/eg",
	}

	t.Run("passthrough", func(t *testing.T) {
		require.Equal(t, "/workload/.eg.cache/x", workspaces.RemapPassthrough("/workload/.eg.cache/x"))
		require.Equal(t, "x/y", workspaces.RemapPassthrough("x/y"))
	})

	t.Run("baremetal maps absolute paths and leaves relative paths", func(t *testing.T) {
		remap := workspaces.RemapBaremetal(ws)
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache/x", remap("/workload/.eg.cache/x"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/eg/x", remap("/workload/eg/x"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/eg/x", remap("/w/.eg.workload/m.bbbbbbbb.d/eg/x"))
		require.Equal(t, "/etc/ssl/certs/x", remap("/etc/ssl/certs/x"))
		require.Equal(t, "x/y", remap("x/y"))
	})

	t.Run("remap maps every well known directory of the container to the host", func(t *testing.T) {
		remap := workspaces.Remap(ws)

		// the directories themselves.
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d", remap("/workload"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d", remap("/eg.mnt"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache", remap("/workload/.eg.cache"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.runtime", remap("/eg.mnt/.eg.runtime"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace", remap("/workload/.eg.workspace"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/eg", remap("/workload/eg"))

		// paths within them.
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache/x/y", remap("/workload/.eg.cache/x/y"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.runtime/x/y", remap("/eg.mnt/.eg.runtime/x/y"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace/x/y", remap("/workload/.eg.workspace/x/y"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/eg/x/y", remap("/workload/eg/x/y"))
	})

	t.Run("remap leaves paths that are already within the root", func(t *testing.T) {
		remap := workspaces.Remap(ws)

		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d", remap("/w/.eg.workload/m.bbbbbbbb.d"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache/x", remap("/w/.eg.workload/m.bbbbbbbb.d/.eg.cache/x"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.runtime/x", remap("/w/.eg.workload/m.bbbbbbbb.d/.eg.runtime/x"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace/x", remap("/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace/x"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/eg/x", remap("/w/.eg.workload/m.bbbbbbbb.d/eg/x"))
	})

	t.Run("remap maps relative paths to the working directory", func(t *testing.T) {
		remap := workspaces.Remap(ws)

		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/eg/x/y", remap("x/y"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache/x", remap("../.eg.cache/x"))
	})

	t.Run("remap never resolves outside of the root", func(t *testing.T) {
		remap := workspaces.Remap(ws)

		// absolute paths that are not directories of the container are placed under the root.
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/etc/ssl/certs/x", remap("/etc/ssl/certs/x"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/workload-other/x", remap("/workload-other/x"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/eg.mnt-other/x", remap("/eg.mnt-other/x"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/w/.eg.workload/m.bbbbbbbb.d-other/x", remap("/w/.eg.workload/m.bbbbbbbb.d-other/x"))

		// paths leaving the root, or a directory of the container, are contained.
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/etc/passwd", remap("/workload/../etc/passwd"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/etc/passwd", remap("/eg.mnt/../etc/passwd"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/w/.eg.workload/etc/passwd", remap("/w/.eg.workload/m.bbbbbbbb.d/../etc/passwd"))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/etc/passwd", remap("../../../../../../../../etc/passwd"))
	})
}
