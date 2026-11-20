package cmdopts_test

import (
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/stretchr/testify/require"
)

func TestMountsVolumes(t *testing.T) {
	t.Run("access is decided by the flag", func(t *testing.T) {
		a, b, c := t.TempDir(), t.TempDir(), t.TempDir()
		volumes, err := cmdopts.Mounts{
			Mount:          []string{a + ":/x:0700:u:g"},
			MountReadOnly:  []string{b + ":/y:0700:u:g"},
			MountReadWrite: []string{c + ":/z"},
		}.Volumes()
		require.NoError(t, err)
		require.Equal(t, []string{a + ":/x", b + ":/y:ro", c + ":/z:rw"}, volumes)
	})

	t.Run("within the root directory are mounted within the mount root", func(t *testing.T) {
		a := t.TempDir()
		volumes, err := cmdopts.Mounts{MountReadWrite: []string{a + ":/workload/.agent.d/claude"}}.Volumes()
		require.NoError(t, err)
		require.Equal(t, []string{a + ":/eg.mnt/.agent.d/claude:rw"}, volumes)
	})

	t.Run("invalid specification", func(t *testing.T) {
		_, err := cmdopts.Mounts{MountReadOnly: []string{"/b"}}.Volumes()
		require.Error(t, err)
	})

	t.Run("missing source", func(t *testing.T) {
		_, err := cmdopts.Mounts{Mount: []string{t.TempDir() + "/missing:/x"}}.Volumes()
		require.Error(t, err)
	})
}

func TestMountsRemap(t *testing.T) {
	mounts := cmdopts.Mounts{
		Mount:          []string{"/c:/workloadx", "/d:/workload", "/f:/workload/../etc", "/g:/workload/"},
		MountReadOnly:  []string{"/a:/etc/a", "/e:/workload/.agent.d/vibe"},
		MountReadWrite: []string{"/b:/workload/.agent.d/claude"},
	}

	t.Run("only the destinations within the root directory are remapped", func(t *testing.T) {
		dirs, err := mounts.Remap()
		require.NoError(t, err)
		require.Equal(t, []string{"/workload/.agent.d/vibe", "/workload/.agent.d/claude"}, dirs)
	})

	t.Run("appended to the directories already remapped", func(t *testing.T) {
		envb := envx.Build().FromEnviron(eg.EnvUnsafeRemapDirectory + "=/workload/x")
		require.NoError(t, mounts.RemapEnviron(envb))
		environ, err := envb.Environ()
		require.NoError(t, err)
		require.Contains(t, environ, eg.EnvUnsafeRemapDirectory+"=/workload/x:/workload/.agent.d/vibe:/workload/.agent.d/claude")
	})

	t.Run("set when none are", func(t *testing.T) {
		envb := envx.Build()
		require.NoError(t, mounts.RemapEnviron(envb))
		environ, err := envb.Environ()
		require.NoError(t, err)
		require.Contains(t, environ, eg.EnvUnsafeRemapDirectory+"=/workload/.agent.d/vibe:/workload/.agent.d/claude")
	})

	t.Run("nothing to remap", func(t *testing.T) {
		envb := envx.Build()
		require.NoError(t, cmdopts.Mounts{}.RemapEnviron(envb))
		environ, err := envb.Environ()
		require.NoError(t, err)
		for _, e := range environ {
			require.NotContains(t, e, eg.EnvUnsafeRemapDirectory+"=")
		}
	})
}

func TestMountMountpoint(t *testing.T) {
	t.Run("outside the root directory is its destination", func(t *testing.T) {
		require.Equal(t, "/x", cmdopts.Mount{Destination: "/x"}.Mountpoint())
	})

	t.Run("within the root directory is within the mount root", func(t *testing.T) {
		require.Equal(t, eg.DefaultMountRoot(".agent.d", "claude"), cmdopts.Mount{Destination: eg.DefaultRootDirectory(".agent.d", "claude")}.Mountpoint())
	})

	t.Run("the root directory itself is its destination", func(t *testing.T) {
		require.Equal(t, eg.DefaultRootDirectory(), cmdopts.Mount{Destination: eg.DefaultRootDirectory()}.Mountpoint())
	})
}
