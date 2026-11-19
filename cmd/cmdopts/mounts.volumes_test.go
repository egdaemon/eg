package cmdopts_test

import (
	"testing"

	"github.com/egdaemon/eg/cmd/cmdopts"
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

	t.Run("invalid specification", func(t *testing.T) {
		_, err := cmdopts.Mounts{MountReadOnly: []string{"/b"}}.Volumes()
		require.Error(t, err)
	})

	t.Run("missing source", func(t *testing.T) {
		_, err := cmdopts.Mounts{Mount: []string{t.TempDir() + "/missing:/x"}}.Volumes()
		require.Error(t, err)
	})
}
