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

func TestCmdEnv(t *testing.T) {
	t.Run("no command prints resolved env vars", func(t *testing.T) {
		tmp := t.TempDir()
		uri := "chachasm://p@" + filepath.Join(tmp, "env.chacha")
		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte("FOO=bar\nBAZ=qux"))))

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runSecretCLI(t, []string{"secret", "env", "--uri", uri})

		w.Close()
		os.Stdout = oldStdout
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = io.Copy(&buf, r)
		require.NoError(t, err)

		require.Equal(t, "FOO=bar\nBAZ=qux\n", buf.String())
	})

	t.Run("command runs with secret env vars set", func(t *testing.T) {
		tmp := t.TempDir()
		uri := "chachasm://p@" + filepath.Join(tmp, "env.chacha")
		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte("EGTEST_SECRET=hello"))))

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runSecretCLI(t, []string{"secret", "env", "--uri", uri, "printenv", "EGTEST_SECRET"})

		w.Close()
		os.Stdout = oldStdout
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = io.Copy(&buf, r)
		require.NoError(t, err)

		require.Equal(t, "hello\n", buf.String())
	})

	t.Run("multiple uris are merged", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := "chachasm://p@" + filepath.Join(tmp, "1.chacha")
		uri2 := "chachasm://p@" + filepath.Join(tmp, "2.chacha")
		require.NoError(t, secrets.Update(t.Context(), uri1, bytes.NewReader([]byte("EGTEST_A=one"))))
		require.NoError(t, secrets.Update(t.Context(), uri2, bytes.NewReader([]byte("EGTEST_B=two"))))

		oldStdout := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := runSecretCLI(t, []string{"secret", "env", "--uri", uri1, "--uri", uri2, "printenv", "EGTEST_A", "EGTEST_B"})

		w.Close()
		os.Stdout = oldStdout
		require.NoError(t, err)

		var buf bytes.Buffer
		_, err = io.Copy(&buf, r)
		require.NoError(t, err)

		require.Equal(t, "one\ntwo\n", buf.String())
	})
}
