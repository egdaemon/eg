package cmdsecret_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
)

func TestSecretCp(t *testing.T) {
	t.Run("copies the secret to a new destination", func(t *testing.T) {
		tmp := t.TempDir()
		src := "chachasm://pass@" + filepath.Join(tmp, "src.chacha")
		dst := "chachasm://pass@" + filepath.Join(tmp, "dst.chacha")

		require.NoError(t, secrets.Update(t.Context(), src, bytes.NewReader([]byte("A=1"))))
		require.NoError(t, runSecretCLI(t, []string{"secret", "cp", src, dst}))

		got, err := io.ReadAll(secrets.Read(t.Context(), dst))
		require.NoError(t, err)
		require.Equal(t, "A=1", string(got))
	})

	t.Run("copies across schemes", func(t *testing.T) {
		tmp := t.TempDir()
		src := "chachasm://pass@" + filepath.Join(tmp, "src.chacha")
		dst := "file:" + filepath.Join(tmp, "dst.txt")

		require.NoError(t, secrets.Update(t.Context(), src, bytes.NewReader([]byte("A=1"))))
		require.NoError(t, runSecretCLI(t, []string{"secret", "cp", src, dst}))

		got, err := os.ReadFile(filepath.Join(tmp, "dst.txt"))
		require.NoError(t, err)
		require.Equal(t, "A=1", string(got))
	})

	t.Run("an existing destination is not replaced by default", func(t *testing.T) {
		tmp := t.TempDir()
		src := "chachasm://pass@" + filepath.Join(tmp, "src.chacha")
		dst := "chachasm://pass@" + filepath.Join(tmp, "dst.chacha")

		require.NoError(t, secrets.Update(t.Context(), src, bytes.NewReader([]byte("new"))))
		require.NoError(t, secrets.Update(t.Context(), dst, bytes.NewReader([]byte("old"))))

		err := runSecretCLI(t, []string{"secret", "cp", src, dst})
		require.ErrorContains(t, err, "--overwrite")

		got, err := io.ReadAll(secrets.Read(t.Context(), dst))
		require.NoError(t, err)
		require.Equal(t, "old", string(got))
	})

	t.Run("overwrite replaces an existing destination", func(t *testing.T) {
		tmp := t.TempDir()
		src := "chachasm://pass@" + filepath.Join(tmp, "src.chacha")
		dst := "chachasm://pass@" + filepath.Join(tmp, "dst.chacha")

		require.NoError(t, secrets.Update(t.Context(), src, bytes.NewReader([]byte("new"))))
		require.NoError(t, secrets.Update(t.Context(), dst, bytes.NewReader([]byte("old"))))
		require.NoError(t, runSecretCLI(t, []string{"secret", "cp", "--overwrite", src, dst}))

		got, err := io.ReadAll(secrets.Read(t.Context(), dst))
		require.NoError(t, err)
		require.Equal(t, "new", string(got))
	})

	t.Run("a missing source fails and does not create the destination", func(t *testing.T) {
		tmp := t.TempDir()
		src := "chachasm://pass@" + filepath.Join(tmp, "missing.chacha")
		dst := filepath.Join(tmp, "dst.chacha")

		err := runSecretCLI(t, []string{"secret", "cp", src, "chachasm://pass@" + dst})
		require.ErrorContains(t, err, "failed to read the source secret")
		require.NoFileExists(t, dst)
	})
}
