package sshx_test

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/stretchr/testify/require"
)

func TestAutoCached(t *testing.T) {
	t.Run("logs the missing key when info logging is enabled", func(t *testing.T) {
		t.Setenv(eg.EnvLogsInfo, "true")
		path := filepath.Join(t.TempDir(), "id_ed25519")

		var out bytes.Buffer
		log.SetOutput(&out)
		defer log.SetOutput(os.Stderr)

		_, err := sshx.AutoCached(sshx.NewKeyGenSeeded("seed"), path)
		require.NoError(t, err)
		require.Contains(t, out.String(), "ssh key missing generating: "+path)
	})

	t.Run("stays quiet about the missing key by default", func(t *testing.T) {
		t.Setenv(eg.EnvLogsInfo, "false")
		path := filepath.Join(t.TempDir(), "id_ed25519")

		var out bytes.Buffer
		log.SetOutput(&out)
		defer log.SetOutput(os.Stderr)

		_, err := sshx.AutoCached(sshx.NewKeyGenSeeded("seed"), path)
		require.NoError(t, err)
		require.NotContains(t, out.String(), "ssh key missing")
	})

	t.Run("an existing key is loaded and not logged about", func(t *testing.T) {
		t.Setenv(eg.EnvLogsInfo, "true")
		path := filepath.Join(t.TempDir(), "id_ed25519")

		first, err := sshx.AutoCached(sshx.NewKeyGenSeeded("seed"), path)
		require.NoError(t, err)

		var out bytes.Buffer
		log.SetOutput(&out)
		defer log.SetOutput(os.Stderr)

		second, err := sshx.AutoCached(sshx.NewKeyGenSeeded("other"), path)
		require.NoError(t, err)
		require.Equal(t, first.PublicKey().Marshal(), second.PublicKey().Marshal())
		require.NotContains(t, out.String(), "ssh key missing")
	})

	t.Run("an unreadable key path is an error and is not reported as missing", func(t *testing.T) {
		t.Setenv(eg.EnvLogsInfo, "true")
		dir := t.TempDir()
		path := filepath.Join(dir, "id_ed25519")
		require.NoError(t, os.Mkdir(path, 0o700))

		var out bytes.Buffer
		log.SetOutput(&out)
		defer log.SetOutput(os.Stderr)

		_, err := sshx.AutoCached(sshx.NewKeyGenSeeded("seed"), path)
		require.Error(t, err)
		require.NotContains(t, out.String(), "ssh key missing")
	})

	t.Run("a key that cannot be parsed is an error and is left untouched", func(t *testing.T) {
		t.Setenv(eg.EnvLogsInfo, "true")
		path := filepath.Join(t.TempDir(), "id_ed25519")
		require.NoError(t, os.WriteFile(path, []byte("not a private key"), 0o600))

		var out bytes.Buffer
		log.SetOutput(&out)
		defer log.SetOutput(os.Stderr)

		_, err := sshx.AutoCached(sshx.NewKeyGenSeeded("seed"), path)
		require.Error(t, err)
		require.NotContains(t, out.String(), "ssh key missing")

		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "not a private key", string(got))
		require.NoFileExists(t, path+".pub")
	})

	t.Run("a key that cannot be read is an error and is left untouched", func(t *testing.T) {
		if os.Getuid() == 0 {
			t.Skip("permissions are not enforced for root")
		}

		path := filepath.Join(t.TempDir(), "id_ed25519")
		require.NoError(t, os.WriteFile(path, []byte("locked"), 0o000))

		_, err := sshx.AutoCached(sshx.NewKeyGenSeeded("seed"), path)
		require.ErrorIs(t, err, os.ErrPermission)

		require.NoError(t, os.Chmod(path, 0o600))
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "locked", string(got))
	})
}
