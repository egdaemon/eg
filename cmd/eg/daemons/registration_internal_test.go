package daemons

import (
	"testing"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/bytesx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestGenRegistration(t *testing.T) {
	t.Run("includes runtime resources", func(t *testing.T) {
		priv, _, err := sshx.NewKeyGenSeeded(t.Name()).Generate()
		require.NoError(t, err)
		s, err := ssh.ParsePrivateKey(priv)
		require.NoError(t, err)

		runtimecfg := cmdopts.RuntimeResources{
			OS:     "linux",
			Arch:   "amd64",
			Cores:  4,
			Memory: 8 * bytesx.GiB,
			Vram:   16 * bytesx.GiB,
			Labels: []string{"foo", "eg:gpu:amdgpu"},
		}

		reg := genregistration(s, peer.ID("derp"), &runtimecfg)
		require.Equal(t, runtimecfg.OS, reg.Os)
		require.Equal(t, runtimecfg.Arch, reg.Arch)
		require.Equal(t, runtimecfg.Cores, reg.Cores)
		require.Equal(t, uint64(runtimecfg.Memory), reg.Memory)
		require.Equal(t, uint64(runtimecfg.Vram), reg.Vram)
		require.Equal(t, runtimecfg.Labels, reg.Labels)
	})
}
