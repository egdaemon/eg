package secrets_test

import (
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/secrets"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/chacha20poly1305"
)

func encryptChaCha(passphrase string, plaintext []byte) ([]byte, error) {
	salt := make([]byte, 16)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, err
	}

	nonce := make([]byte, chacha20poly1305.NonceSize)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	key := argon2.IDKey([]byte(passphrase), salt, 1, 64*1024, 4, 32)
	aead, err := chacha20poly1305.New(key)
	if err != nil {
		return nil, err
	}

	ciphertext := aead.Seal(nil, nonce, plaintext, nil)

	// Format: [SALT(16)][NONCE(12)][CIPHERTEXT+TAG]
	res := append(salt, nonce...)
	res = append(res, ciphertext...)
	return res, nil
}

func TestRead_ChaCha(t *testing.T) {
	t.Run("success_decrypt_chachasm", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "secret.chacha")
		passphrase := "super-secret-password"
		expectedData := "hello world secret content"

		encrypted, err := encryptChaCha(passphrase, []byte(expectedData))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(secretPath, encrypted, 0644))

		// Test Download via URI
		uri := "chachasm://" + secretPath
		reader := secrets.Read(t.Context(), uri, secrets.WithPassphrase(passphrase))
		require.NotNil(t, reader)

		result, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, expectedData, string(result))
	})

	t.Run("success_decrypt_with_uri_embedded_passphrase", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "secret_uri.chacha")
		passphrase := "pass123"
		expectedData := "embedded pass content"

		encrypted, err := encryptChaCha(passphrase, []byte(expectedData))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(secretPath, encrypted, 0644))

		// Test Download with passphrase in URI
		uri := "chachasm://" + passphrase + "@" + secretPath
		reader := secrets.Read(t.Context(), uri)
		require.NotNil(t, reader)

		result, err := io.ReadAll(reader)
		require.NoError(t, err)
		require.Equal(t, expectedData, string(result))
	})

	t.Run("failure_invalid_passphrase", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "secret_fail.chacha")
		correctPass := "correct"
		wrongPass := "wrong"

		encrypted, err := encryptChaCha(correctPass, []byte("some data"))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(secretPath, encrypted, 0644))

		uri := "chachasm://" + secretPath
		reader := secrets.Read(t.Context(), uri, secrets.WithPassphrase(wrongPass))

		_, err = io.ReadAll(reader)
		require.Error(t, err)
		require.Contains(t, err.Error(), "decryption failed")
	})

	t.Run("failure_missing_file", func(t *testing.T) {
		uri := "chachasm:///non/existent/path/file.chacha"
		reader := secrets.Read(t.Context(), uri, secrets.WithPassphrase("any"))

		_, err := io.ReadAll(reader)
		require.Error(t, err)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("failure_no_passphrase_provided", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "no_pass.chacha")
		require.NoError(t, os.WriteFile(secretPath, []byte("too short to be valid anyway"), 0644))

		uri := "chachasm://" + secretPath
		reader := secrets.Read(t.Context(), uri) // No WithPassphrase and no UserInfo

		_, err := io.ReadAll(reader)
		require.Error(t, err)
		require.Contains(t, err.Error(), "passphrase not provided")
	})
}
