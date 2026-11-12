package egvm

import (
	"testing"
	"testing/fstest"

	"github.com/egdaemon/eg/internal/sshx"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestHostkeys(t *testing.T) {
	// hostkey is a key of a machine's sshd, and its public half as the root holds it.
	hostkey := func(t *testing.T) (ssh.PublicKey, []byte) {
		signer, err := sshx.SignerFromGenerator(sshx.NewKeyGen())
		require.NoError(t, err)
		return signer.PublicKey(), ssh.MarshalAuthorizedKey(signer.PublicKey())
	}

	t.Run("accepts a host key the root holds", func(t *testing.T) {
		key, held := hostkey(t)
		_, other := hostkey(t)
		root := fstest.MapFS{
			"etc/ssh/ssh_host_rsa_key.pub":     {Data: other},
			"etc/ssh/ssh_host_ed25519_key.pub": {Data: held},
		}
		require.NoError(t, hostkeys(root)("egvm", nil, key))
	})

	t.Run("refuses a host key the root does not hold", func(t *testing.T) {
		key, _ := hostkey(t)
		_, other := hostkey(t)
		root := fstest.MapFS{"etc/ssh/ssh_host_ed25519_key.pub": {Data: other}}
		require.Error(t, hostkeys(root)("egvm", nil, key))
	})

	t.Run("refuses every host key when the root holds none", func(t *testing.T) {
		key, _ := hostkey(t)
		require.Error(t, hostkeys(fstest.MapFS{})("egvm", nil, key))
	})

	t.Run("reports a host key it can not parse", func(t *testing.T) {
		key, _ := hostkey(t)
		root := fstest.MapFS{"etc/ssh/ssh_host_ed25519_key.pub": {Data: []byte("not a key")}}
		require.ErrorContains(t, hostkeys(root)("egvm", nil, key), "unable to parse")
	})
}
