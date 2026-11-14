package runners

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/stretchr/testify/require"
)

func TestEnviron(t *testing.T) {
	passwd := &fstest.MapFile{Data: []byte("root:x:0:0:root:/root:/bin/bash\negd:x:990:990::/home/egd:/bin/bash\n")}

	t.Run("image environment and root's home", func(t *testing.T) {
		root := fstest.MapFS{
			"etc/environment": &fstest.MapFile{Data: []byte("PATH=/usr/sbin:/usr/bin\nDEBIAN_FRONTEND=noninteractive\n")},
			"etc/passwd":      passwd,
		}

		environ, err := environ(root)
		require.NoError(t, err)
		require.Equal(t, []string{"PATH=/usr/sbin:/usr/bin", "DEBIAN_FRONTEND=noninteractive", "container=egiso", "HOME=/root"}, environ)
	})

	t.Run("missing environment is root's home alone", func(t *testing.T) {
		environ, err := environ(fstest.MapFS{"etc/passwd": passwd})
		require.NoError(t, err)
		require.Equal(t, []string{"container=egiso", "HOME=/root"}, environ)
	})

	t.Run("missing passwd fails", func(t *testing.T) {
		_, err := environ(fstest.MapFS{})
		require.Error(t, err)
	})

	t.Run("no root account sets no home, only the container", func(t *testing.T) {
		environ, err := environ(fstest.MapFS{"etc/passwd": &fstest.MapFile{Data: []byte("egd:x:990:990::/home/egd:/bin/bash\n")}})
		require.NoError(t, err)
		require.Equal(t, []string{"container=egiso"}, environ)
	})
}

func TestGPGConf(t *testing.T) {
	if _, err := exec.LookPath("gpgconf"); err != nil {
		t.Skip("gpgconf is not installed")
	}

	t.Run("finds gpg home and its agent beneath GNUPGHOME", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("GNUPGHOME", home)

		agentsock, gnupghome, err := gpgconf(t.Context())
		require.NoError(t, err)
		require.Equal(t, home, gnupghome)
		require.Equal(t, "S.gpg-agent", filepath.Base(agentsock))
	})

	t.Run("fails without gpgconf", func(t *testing.T) {
		t.Setenv("PATH", t.TempDir())
		_, _, err := gpgconf(t.Context())
		require.Error(t, err)
	})
}

func TestMachineOptionLocalGPGAgent(t *testing.T) {
	setup := func(t *testing.T, agent bool, home bool) (agentsock string, gnupghome string) {
		tmp := t.TempDir()
		agentsock = filepath.Join(tmp, "S.gpg-agent")
		gnupghome = filepath.Join(tmp, "gnupg")
		if agent {
			require.NoError(t, os.WriteFile(agentsock, nil, 0600))
		}
		if home {
			require.NoError(t, os.Mkdir(gnupghome, 0700))
		}
		return agentsock, gnupghome
	}

	t.Run("missing agent socket attaches nothing", func(t *testing.T) {
		agentsock, gnupghome := setup(t, false, true)
		envb := envx.Build()
		config := langx.Clone(machineconfig{}, machineOptionLocalGPGAgent(envb, agentsock, gnupghome))
		require.Empty(t, config.overlays)
		require.Empty(t, config.sockets)
		environ, err := envb.Environ()
		require.NoError(t, err)
		require.Equal(t, []string{"container=egiso"}, environ)
	})

	t.Run("missing gnupghome attaches nothing", func(t *testing.T) {
		agentsock, gnupghome := setup(t, true, false)
		config := langx.Clone(machineconfig{}, machineOptionLocalGPGAgent(envx.Build(), agentsock, gnupghome))
		require.Empty(t, config.overlays)
		require.Empty(t, config.sockets)
	})

	t.Run("both present overlays the home, forwards the sockets and sets GNUPGHOME", func(t *testing.T) {
		agentsock, gnupghome := setup(t, true, true)
		envb := envx.Build()
		config := langx.Clone(machineconfig{}, machineOptionLocalGPGAgent(envb, agentsock, gnupghome))

		require.Equal(t, []attachment{{host: gnupghome, path: eg.DefaultMountRoot(".gnupg")}}, config.overlays)
		require.Equal(t, []attachment{
			{host: agentsock, path: eg.DefaultMountRoot(".gnupg", "S.gpg-agent")},
			{host: filepath.Join(filepath.Dir(agentsock), "S.dirmngr"), path: eg.DefaultMountRoot(".gnupg", "S.dirmngr")},
			{host: filepath.Join(filepath.Dir(agentsock), "S.keyboxd"), path: eg.DefaultMountRoot(".gnupg", "S.keyboxd")},
		}, config.sockets)

		environ, err := envb.Environ()
		require.NoError(t, err)
		require.Equal(t, []string{"GNUPGHOME=" + eg.DefaultMountRoot(".gnupg")}, environ)
	})
}
