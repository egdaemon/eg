package interp

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/workspaces"
	"github.com/stretchr/testify/require"
)

func TestOptionRemap(t *testing.T) {
	cachepath := eg.DefaultCacheDirectory("git", "x.git")

	t.Run("container paths are used as is by default", func(t *testing.T) {
		// in a container the root of the workspace is the working directory, see workspaces.FromEnv. mapping the
		// well known directories within it moved clones out of the cache the module's containers mount.
		r := newrunner()
		require.Equal(t, cachepath, r.remap(cachepath))
		require.Equal(t, eg.DefaultWorkingDirectory("x"), r.remap(eg.DefaultWorkingDirectory("x")))
		require.Equal(t, "x/y", r.remap("x/y"))
	})

	t.Run("remap is custom", func(t *testing.T) {
		r := newrunner(OptionRemap(func(s string) string { return "/host" + s }))
		require.Equal(t, "/host"+cachepath, r.remap(cachepath))
	})

	t.Run("baremetal paths are mapped to the host", func(t *testing.T) {
		ws := workspaces.Context{
			Root:       "/w/.eg.workload/m.bbbbbbbb.d",
			WorkingDir: "/w/.eg.workload/m.bbbbbbbb.d/eg",
		}

		r := newrunner(OptionRemap(workspaces.Remap(ws)))
		require.Equal(t, "/w/.eg.workload/m.bbbbbbbb.d/.eg.cache/git/x.git", r.remap(cachepath))
	})
}
