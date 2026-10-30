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

func TestSecretEditCmd(t *testing.T) {
	t.Run("creates a new chacha secret when the file does not exist yet", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "new.chacha")
		uri := "chachasm://pass@" + path

		editor := filepath.Join(tmp, "editor.sh")
		require.NoError(t, os.WriteFile(editor, []byte("#!/bin/sh\nprintf 'created' > \"$1\"\n"), 0o700))
		t.Setenv("EDITOR", editor)

		require.NoError(t, runSecretCLI(t, []string{"secret", "edit", uri}))

		got, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "created", string(got))

		ciphertext, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NotContains(t, string(ciphertext), "created")
	})

	t.Run("opens the decrypted secret in the editor and stores the edited result", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "existing.chacha")
		capture := filepath.Join(tmp, "capture.txt")
		uri := "chachasm://pass@" + path

		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte("EG_ACCOUNT=one\n"))))

		editor := filepath.Join(tmp, "editor.sh")
		require.NoError(t, os.WriteFile(editor, []byte("#!/bin/sh\ncat \"$1\" > \""+capture+"\"\nprintf 'EG_ENTROPY_SEED=two\\n' >> \"$1\"\n"), 0o700))
		t.Setenv("EDITOR", editor)

		require.NoError(t, runSecretCLI(t, []string{"secret", "edit", uri}))

		seen, err := os.ReadFile(capture)
		require.NoError(t, err)
		require.Equal(t, "EG_ACCOUNT=one\n", string(seen))

		got, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "EG_ACCOUNT=one\nEG_ENTROPY_SEED=two\n", string(got))
	})

	t.Run("leaves the file untouched when the editor changes nothing", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "unchanged.chacha")
		uri := "chachasm://pass@" + path

		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte("same"))))
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		editor := filepath.Join(tmp, "editor.sh")
		require.NoError(t, os.WriteFile(editor, []byte("#!/bin/sh\nexit 0\n"), 0o700))
		t.Setenv("EDITOR", editor)

		require.NoError(t, runSecretCLI(t, []string{"secret", "edit", uri}))

		// every encryption uses a fresh salt and nonce, so identical bytes mean it was not rewritten.
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, before, after)
	})

	t.Run("fails without running the editor when the passphrase is wrong", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "locked.chacha")
		marker := filepath.Join(tmp, "editor.ran")

		require.NoError(t, secrets.Update(t.Context(), "chachasm://right@"+path, bytes.NewReader([]byte("secret"))))
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		editor := filepath.Join(tmp, "editor.sh")
		require.NoError(t, os.WriteFile(editor, []byte("#!/bin/sh\ntouch \""+marker+"\"\nprintf 'replaced' > \"$1\"\n"), 0o700))
		t.Setenv("EDITOR", editor)

		err = runSecretCLI(t, []string{"secret", "edit", "chachasm://wrong@" + path})
		require.ErrorContains(t, err, "failed to read secret for editing")
		require.NoFileExists(t, marker)

		after, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, before, after)
	})

	t.Run("leaves the secret unchanged when the editor fails", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "failing.chacha")
		uri := "chachasm://pass@" + path

		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte("original"))))

		editor := filepath.Join(tmp, "editor.sh")
		require.NoError(t, os.WriteFile(editor, []byte("#!/bin/sh\nprintf 'half written' > \"$1\"\nexit 1\n"), 0o700))
		t.Setenv("EDITOR", editor)

		err := runSecretCLI(t, []string{"secret", "edit", uri})
		require.ErrorContains(t, err, "editor failed")

		got, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "original", string(got))
	})

	t.Run("requires a passphrase", func(t *testing.T) {
		tmp := t.TempDir()
		path := filepath.Join(tmp, "nopass.chacha")

		editor := filepath.Join(tmp, "editor.sh")
		require.NoError(t, os.WriteFile(editor, []byte("#!/bin/sh\nprintf 'content' > \"$1\"\n"), 0o700))
		t.Setenv("EDITOR", editor)

		err := runSecretCLI(t, []string{"secret", "edit", "chachasm://" + path})
		require.ErrorContains(t, err, "passphrase")
		require.NoFileExists(t, path)
	})
}
