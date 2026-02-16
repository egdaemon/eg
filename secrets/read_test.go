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

func TestRead_File(t *testing.T) {
	t.Run("success read plaintext file", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "secret.txt")
		expected := "my plaintext secret"
		require.NoError(t, os.WriteFile(secretPath, []byte(expected), 0644))

		uri := "file://" + secretPath
		result, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, expected, string(result))
	})

	t.Run("failure missing file", func(t *testing.T) {
		uri := "file:///non/existent/path/secret.txt"
		_, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.Error(t, err)
		require.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("empty file returns empty content", func(t *testing.T) {
		tmp := t.TempDir()
		secretPath := filepath.Join(tmp, "empty.txt")
		require.NoError(t, os.WriteFile(secretPath, []byte(""), 0644))

		uri := "file://" + secretPath
		result, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "", string(result))
	})

	t.Run("relative path", func(t *testing.T) {
		tmp := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(tmp, "rel.txt"), []byte("relative content"), 0644))

		// file:./rel.txt uses Opaque, not Path
		uri := "file:" + filepath.Join(tmp, "rel.txt")
		result, err := io.ReadAll(secrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "relative content", string(result))
	})

	t.Run("works with NewReader", func(t *testing.T) {
		tmp := t.TempDir()
		p1 := filepath.Join(tmp, "a.txt")
		p2 := filepath.Join(tmp, "b.txt")
		require.NoError(t, os.WriteFile(p1, []byte("alpha"), 0644))
		require.NoError(t, os.WriteFile(p2, []byte("bravo"), 0644))

		result, err := io.ReadAll(secrets.NewReader(t.Context(), "file://"+p1, "file://"+p2))
		require.NoError(t, err)
		require.Equal(t, "alpha\nbravo\n", string(result))
	})
}

func TestNewReader(t *testing.T) {
	writeSecret := func(t *testing.T, dir, name, passphrase, content string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		encrypted, err := encryptChaCha(passphrase, []byte(content))
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(path, encrypted, 0644))
		return "chachasm://" + passphrase + "@" + path
	}

	t.Run("single uri", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeSecret(t, tmp, "s1.chacha", "pass1", "secret-one")

		result, err := io.ReadAll(secrets.NewReader(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "secret-one\n", string(result))
	})

	t.Run("multiple uris concatenated with newlines", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := writeSecret(t, tmp, "s1.chacha", "pass1", "first")
		uri2 := writeSecret(t, tmp, "s2.chacha", "pass2", "second")
		uri3 := writeSecret(t, tmp, "s3.chacha", "pass3", "third")

		result, err := io.ReadAll(secrets.NewReader(t.Context(), uri1, uri2, uri3))
		require.NoError(t, err)
		require.Equal(t, "first\nsecond\nthird\n", string(result))
	})

	t.Run("no uris produces empty output", func(t *testing.T) {
		result, err := io.ReadAll(secrets.NewReader(t.Context()))
		require.NoError(t, err)
		require.Equal(t, "", string(result))
	})

	t.Run("error propagates from bad uri", func(t *testing.T) {
		tmp := t.TempDir()
		good := writeSecret(t, tmp, "good.chacha", "pass", "ok")
		bad := "chachasm:///does/not/exist.chacha"

		_, err := io.ReadAll(secrets.NewReader(t.Context(), good, bad))
		require.Error(t, err)
	})
}
