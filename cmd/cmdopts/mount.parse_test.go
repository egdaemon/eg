package cmdopts_test

import (
	"io/fs"
	"testing"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

func TestParseMount(t *testing.T) {
	t.Run("source and destination", func(t *testing.T) {
		m, err := cmdopts.ParseMount("/shared/media:/workload/.eg.cache/shared", cmdopts.AccessDefault)
		require.NoError(t, err)
		require.Equal(t, cmdopts.Mount{Source: "/shared/media", Destination: "/workload/.eg.cache/shared"}, m)
	})

	t.Run("mode user and group", func(t *testing.T) {
		m, err := cmdopts.ParseMount("/shared/media:/workload/.eg.cache/shared:0700:alice:staff", cmdopts.AccessWrite)
		require.NoError(t, err)
		require.Equal(t, cmdopts.Mount{
			Source:      "/shared/media",
			Destination: "/workload/.eg.cache/shared",
			Mode:        fs.FileMode(0o700),
			User:        "alice",
			Group:       "staff",
			Access:      cmdopts.AccessWrite,
		}, m)
	})

	t.Run("blank mode user and group", func(t *testing.T) {
		m, err := cmdopts.ParseMount("/a:/b:::", cmdopts.AccessRead)
		require.NoError(t, err)
		require.Equal(t, cmdopts.Mount{Source: "/a", Destination: "/b", Access: cmdopts.AccessRead}, m)
	})

	t.Run("invalid", func(t *testing.T) {
		for _, spec := range []string{"", "/a", ":/b", "/a:", "/a:/b:rw", "/a:/b:0700:u:g:extra"} {
			_, err := cmdopts.ParseMount(spec, cmdopts.AccessDefault)
			require.Error(t, err, spec)
		}
	})
}
