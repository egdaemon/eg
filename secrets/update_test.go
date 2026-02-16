package secrets_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
)

func TestUpdate_ChaCha(t *testing.T) {
	t.Run("success_update_new_file", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "new_secret.chacha")
		passphrase := "update-passphrase"
		newData := "updated secret content"

		uri := "chachasm://" + secretPath
		err := secrets.Update(t.Context(), uri, bytes.NewReader([]byte(newData)), secrets.WithPassphrase(passphrase))
		require.NoError(t, err)

		// Verify by downloading it back
		reader := secrets.Read(t.Context(), uri, secrets.WithPassphrase(passphrase))
		result, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, newData, string(result))
	})

	t.Run("success_overwrite_existing_file", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "overwrite.chacha")
		passphrase := "same-pass"

		// Initial write
		require.NoError(t, secrets.Update(t.Context(), "chachasm://"+secretPath, bytes.NewReader([]byte("old")), secrets.WithPassphrase(passphrase)))

		// Overwrite
		updatedData := "new-improved-data"
		err := secrets.Update(t.Context(), "chachasm://"+secretPath, bytes.NewReader([]byte(updatedData)), secrets.WithPassphrase(passphrase))
		require.NoError(t, err)

		reader := secrets.Read(t.Context(), "chachasm://"+secretPath, secrets.WithPassphrase(passphrase))
		result, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, updatedData, string(result))
	})

	t.Run("success_update_using_uri_passphrase", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "uri_pass.chacha")
		passphrase := "secret123"
		data := "uri data"

		uri := "chachasm://" + passphrase + "@" + secretPath
		err := secrets.Update(t.Context(), uri, bytes.NewReader([]byte(data)))
		require.NoError(t, err)

		reader := secrets.Read(t.Context(), uri)
		result, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, data, string(result))
	})

	t.Run("failure_no_passphrase", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "fail.chacha")

		uri := "chachasm://" + secretPath
		err := secrets.Update(t.Context(), uri, bytes.NewReader([]byte("data")))
		require.Error(t, err)
		require.Contains(t, err.Error(), "passphrase required")
	})

	t.Run("failure_invalid_path", func(t *testing.T) {
		// Attempting to write to a directory that doesn't exist/unauthorized
		uri := "chachasm:///this/path/should/not/exist/secret.chacha"
		err := secrets.Update(t.Context(), uri, bytes.NewReader([]byte("data")), secrets.WithPassphrase("pass"))

		require.Error(t, err)
		require.True(t, os.IsNotExist(err) || os.IsPermission(err))
	})
}

func TestUpdate_File(t *testing.T) {
	t.Run("success write new file", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "secret.txt")
		data := "plaintext secret"

		uri := "file://" + secretPath
		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte(data))))

		result, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, data, string(result))
	})

	t.Run("success overwrite existing file", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "secret.txt")
		uri := "file://" + secretPath

		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte("old"))))
		require.NoError(t, secrets.Update(t.Context(), uri, bytes.NewReader([]byte("new"))))

		result, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "new", string(result))
	})

	t.Run("failure invalid path", func(t *testing.T) {
		uri := "file:///this/path/should/not/exist/secret.txt"
		err := secrets.Update(t.Context(), uri, bytes.NewReader([]byte("data")))
		require.Error(t, err)
	})
}
