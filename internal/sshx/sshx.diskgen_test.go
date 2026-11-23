package sshx_test

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/sshx"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestDiskGen(t *testing.T) {
	t.Run("returns the private key bytes as stored and the matching public key", func(t *testing.T) {
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		block, err := ssh.MarshalPrivateKey(priv, "")
		require.NoError(t, err)
		encoded := pem.EncodeToMemory(block)

		path := filepath.Join(t.TempDir(), "id_ed25519")
		require.NoError(t, os.WriteFile(path, encoded, 0o600))

		epriv, epub, err := sshx.NewKeyGenPath(path).Generate()
		require.NoError(t, err)
		require.Equal(t, encoded, epriv)

		sshpub, err := ssh.NewPublicKey(pub)
		require.NoError(t, err)
		require.Equal(t, sshpub.Marshal(), epub)
	})

	t.Run("a missing file is an error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "missing")

		epriv, epub, err := sshx.NewKeyGenPath(path).Generate()
		require.ErrorIs(t, err, os.ErrNotExist)
		require.Nil(t, epriv)
		require.Nil(t, epub)
	})

	t.Run("a directory is an error", func(t *testing.T) {
		epriv, epub, err := sshx.NewKeyGenPath(t.TempDir()).Generate()
		require.Error(t, err)
		require.Nil(t, epriv)
		require.Nil(t, epub)
	})

	t.Run("a file that is not a private key is an error", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "id_ed25519")
		require.NoError(t, os.WriteFile(path, []byte("not a private key"), 0o600))

		epriv, epub, err := sshx.NewKeyGenPath(path).Generate()
		require.Error(t, err)
		require.Nil(t, epriv)
		require.Nil(t, epub)
	})

	t.Run("a passphrase protected key is an error", func(t *testing.T) {
		_, priv, err := ed25519.GenerateKey(rand.Reader)
		require.NoError(t, err)

		block, err := ssh.MarshalPrivateKeyWithPassphrase(priv, "", []byte("secret"))
		require.NoError(t, err)

		path := filepath.Join(t.TempDir(), "id_ed25519")
		require.NoError(t, os.WriteFile(path, pem.EncodeToMemory(block), 0o600))

		_, _, err = sshx.NewKeyGenPath(path).Generate()
		require.Error(t, err)
		var missing *ssh.PassphraseMissingError
		require.ErrorAs(t, err, &missing)
	})
}
