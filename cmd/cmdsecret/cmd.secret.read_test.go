package cmdsecret_test

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
)

func TestSecretRead(t *testing.T) {
	t.Run("a missing secret fails by default", func(t *testing.T) {
		tmp := t.TempDir()
		out := filepath.Join(tmp, "out.txt")

		err := runSecretCLI(t, []string{"secret", "read", "chachasm://pass@" + filepath.Join(tmp, "missing.chacha"), "-o", out})
		require.ErrorContains(t, err, "failed to read secrets")

		got, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("allow-missing skips a missing secret", func(t *testing.T) {
		tmp := t.TempDir()
		out := filepath.Join(tmp, "out.txt")

		require.NoError(t, runSecretCLI(t, []string{"secret", "read", "--allow-missing", "chachasm://pass@" + filepath.Join(tmp, "missing.chacha"), "-o", out}))

		got, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("allow-missing reads the remaining secrets in order", func(t *testing.T) {
		tmp := t.TempDir()
		out := filepath.Join(tmp, "out.txt")
		first := "chachasm://pass@" + filepath.Join(tmp, "first.chacha")
		second := "chachasm://pass@" + filepath.Join(tmp, "second.chacha")
		missing := "chachasm://pass@" + filepath.Join(tmp, "missing.chacha")

		require.NoError(t, secrets.Update(t.Context(), first, bytes.NewReader([]byte("A=1"))))
		require.NoError(t, secrets.Update(t.Context(), second, bytes.NewReader([]byte("B=2"))))

		require.NoError(t, runSecretCLI(t, []string{"secret", "read", "--allow-missing", first, missing, second, "-o", out}))

		got, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, "A=1\nB=2\n", string(got))
	})

	t.Run("allow-missing reads an existing secret as before", func(t *testing.T) {
		tmp := t.TempDir()
		out := filepath.Join(tmp, "out.txt")
		uri := "chachasm://pass@" + filepath.Join(tmp, "only.chacha")

		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte("A=1"))))
		require.NoError(t, runSecretCLI(t, []string{"secret", "read", "--allow-missing", uri, "-o", out}))

		got, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, "A=1", string(got))
	})

	t.Run("allow-missing applies to the file scheme", func(t *testing.T) {
		tmp := t.TempDir()
		out := filepath.Join(tmp, "out.txt")
		present := filepath.Join(tmp, "present.secret")

		require.NoError(t, os.WriteFile(present, []byte("A=1"), 0o600))
		require.NoError(t, runSecretCLI(t, []string{"secret", "read", "--allow-missing", "file://" + filepath.Join(tmp, "missing.secret"), "file://" + present, "-o", out}))

		got, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, "A=1", string(got))
	})

	t.Run("the warning does not repeat the passphrase", func(t *testing.T) {
		tmp := t.TempDir()
		out := filepath.Join(tmp, "out.txt")

		var logged bytes.Buffer
		log.SetOutput(&logged)
		defer log.SetOutput(os.Stderr)

		require.NoError(t, runSecretCLI(t, []string{"secret", "read", "--allow-missing", "chachasm://hunter2@" + filepath.Join(tmp, "missing.chacha"), "-o", out}))
		require.Contains(t, logged.String(), "WARN")
		require.NotContains(t, logged.String(), "hunter2")
	})
}
