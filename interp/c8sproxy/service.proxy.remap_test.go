package c8sproxy

import (
	"log"
	"path/filepath"
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/runtime/wasi/egunsafe"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServiceProxyRemap(t *testing.T) {
	ws := workspaces.Context{
		Root:         "/w/.eg.workload/m.bbbbbbbb.d",
		RuntimeDir:   "/w/.eg.workload/m.bbbbbbbb.d/.eg.runtime",
		CacheDir:     "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache",
		WorkspaceDir: "/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace",
		WorkingDir:   "/w/.eg.workload/m.bbbbbbbb.d/eg",
	}

	// the standard proxy, used by local and upload, runs in the same container as the module. every path the module
	// sends is already valid there, so nothing is remapped.
	t.Run("standard remap", func(t *testing.T) {
		remap := NewServiceProxy(
			log.Default(),
			ws,
			ServiceProxyOptionCommandEnviron("A=1"),
			ServiceProxyOptionContainerOptions("--network", "host"),
		).remap

		// directories of the container.
		assert.Equal(t, eg.DefaultRuntimeDirectory("eg.default.module"), remap(eg.DefaultRuntimeDirectory("eg.default.module")))
		assert.Equal(t, eg.DefaultWorkspaceDirectory("x"), remap(eg.DefaultWorkspaceDirectory("x")))
		assert.Equal(t, eg.DefaultWorkingDirectory("x"), remap(eg.DefaultWorkingDirectory("x")))
		assert.Equal(t, eg.DefaultCacheDirectory("x"), remap(eg.DefaultCacheDirectory("x")))

		// everything else.
		assert.Equal(t, ".eg/Containerfile", remap(".eg/Containerfile"))
		assert.Equal(t, filepath.Join(ws.RuntimeDir, "eg.default.module"), remap(filepath.Join(ws.RuntimeDir, "eg.default.module")))
		assert.Equal(t, "/etc/ssl/certs/x", remap("/etc/ssl/certs/x"))
	})

	// the baremetal proxy runs on the host, the paths the module sends must resolve to the host directories.
	t.Run("baremetal remap", func(t *testing.T) {
		log.SetFlags(log.Flags() | log.Lshortfile)
		remap := NewServiceProxy(log.Default(), ws, ServiceProxyOptionBaremetal).remap

		log.Println("workspace", spew.Sdump(ws))
		// directories of the container resolve to the host.
		require.Equal(t, filepath.Join(ws.CacheDir, "x"), remap(eg.DefaultCacheDirectory("x")))
		require.Equal(t, filepath.Join(ws.WorkspaceDir, "x"), remap(eg.DefaultWorkspaceDirectory("x")))
		require.Equal(t, filepath.Join(ws.WorkingDir, "x"), remap(eg.DefaultWorkingDirectory("x")))
		require.Equal(t, filepath.Join(ws.RuntimeDir, "eg.default.module"), remap(eg.DefaultRuntimeDirectory("eg.default.module")))

		// baremetal provides the host directories to the module, see compute.baremetal.go.
		// set the environment to simulate the virtual environment egunsafe.RuntimeDirectory expects.
		t.Setenv(eg.EnvComputeRuntimeDirectory, ws.RuntimeDir)
		require.Equal(t, filepath.Join(ws.RuntimeDir, "eg.default.module"), remap(egunsafe.RuntimeDirectory("eg.default.module")))

		// host paths are unchanged.
		require.Equal(t, ws.RuntimeDir, remap(ws.RuntimeDir))
		require.Equal(t, filepath.Join(ws.RuntimeDir, "eg.default.module"), remap(filepath.Join(ws.RuntimeDir, "eg.default.module")))
		require.Equal(t, filepath.Join(ws.CacheDir, "x"), remap(filepath.Join(ws.CacheDir, "x")))
		require.Equal(t, filepath.Join(ws.WorkspaceDir, "x"), remap(filepath.Join(ws.WorkspaceDir, "x")))
		require.Equal(t, filepath.Join(ws.WorkingDir, "x"), remap(filepath.Join(ws.WorkingDir, "x")))
		require.Equal(t, "/etc/ssl/certs/x", remap("/etc/ssl/certs/x"))
	})
}

func TestServiceProxyRemapBoundaries(t *testing.T) {
	ws := workspaces.Context{
		Root:         "/w/.eg.workload/m.bbbbbbbb.d",
		RuntimeDir:   "/w/.eg.workload/m.bbbbbbbb.d/.eg.runtime",
		CacheDir:     "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache",
		WorkspaceDir: "/w/.eg.workload/m.bbbbbbbb.d/.eg.workspace",
		WorkingDir:   "/w/.eg.workload/m.bbbbbbbb.d/eg",
	}

	t.Run("baremetal remap", func(t *testing.T) {
		remap := NewServiceProxy(log.Default(), ws, ServiceProxyOptionBaremetal).remap

		// the directories of the container themselves resolve to the root.
		assert.Equal(t, ws.Root, remap(eg.DefaultRootDirectory()))
		assert.Equal(t, ws.Root, remap(eg.DefaultMountRoot()))

		// a path leaving a directory of the container is not within it, nothing is rooted.
		assert.Equal(t, "/workload/../etc/passwd", remap("/workload/../etc/passwd"))
		assert.Equal(t, "/eg.mnt/../etc/passwd", remap("/eg.mnt/../etc/passwd"))

		// a path leaving the root of the host is not within it either, it is not a directory of the container.
		assert.Equal(t, ws.Root+"/../etc/passwd", remap(ws.Root+"/../etc/passwd"))

		// a sibling sharing the name of a directory of the container as a prefix is not within it.
		assert.Equal(t, "/workload-other/x", remap("/workload-other/x"))
		assert.Equal(t, "/eg.mnt-other/x", remap("/eg.mnt-other/x"))
		assert.Equal(t, ws.Root+"-other/x", remap(ws.Root+"-other/x"))
	})
}
