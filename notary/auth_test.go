package notary

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/stretchr/testify/require"
)

func TestNewAutoSignerPath(t *testing.T) {
	t.Run("should succeed when no key exists", func(t *testing.T) {
		_, err := newAutoSignerPath(
			filepath.Join(t.TempDir(), DefaultNotaryKey),
			"",
			sshx.UnsafeNewKeyGen(),
		)
		require.NoError(t, err)
	})

	t.Run("should fail when unable to write to disk", func(t *testing.T) {
		if userx.CurrentUserOrDefault(userx.Root()).Uid == "0" {
			log.Println("ROOT USER")
			return
		}
		tmp := t.TempDir()
		os.Stat(tmp)
		require.NoError(t, os.Chmod(tmp, 0100))
		_, err := newAutoSignerPath(
			filepath.Join(tmp, DefaultNotaryKey),
			"",
			sshx.UnsafeNewKeyGen(),
		)
		require.Error(t, err)
	})
}
