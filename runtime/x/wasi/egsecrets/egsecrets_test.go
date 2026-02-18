package egsecrets_test

import (
	"bytes"
	"crypto/rand"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/egtest"
	"github.com/egdaemon/eg/runtime/x/wasi/egsecrets"
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

	res := append(salt, nonce...)
	res = append(res, ciphertext...)
	return res, nil
}

func writeFileSecret(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
	return "file://" + path
}

func writeChaChaSecret(t *testing.T, dir, name, passphrase, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	encrypted, err := encryptChaCha(passphrase, []byte(content))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, encrypted, 0644))
	return "chachasm://" + passphrase + "@" + path
}

func TestRead(t *testing.T) {
	t.Run("file scheme", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "secret.txt", "my-secret-value")

		result, err := io.ReadAll(egsecrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "my-secret-value", string(result))
	})

	t.Run("chacha scheme", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeChaChaSecret(t, tmp, "secret.chacha", "pass", "encrypted-value")

		result, err := io.ReadAll(egsecrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "encrypted-value", string(result))
	})
}

func TestNewReader(t *testing.T) {
	t.Run("multiple file uris", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := writeFileSecret(t, tmp, "a.txt", "alpha")
		uri2 := writeFileSecret(t, tmp, "b.txt", "bravo")

		result, err := io.ReadAll(egsecrets.NewReader(t.Context(), uri1, uri2))
		require.NoError(t, err)
		require.Equal(t, "alpha\nbravo\n", string(result))
	})

	t.Run("mixed file and chacha uris", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := writeFileSecret(t, tmp, "plain.txt", "plain")
		uri2 := writeChaChaSecret(t, tmp, "enc.chacha", "key", "encrypted")

		result, err := io.ReadAll(egsecrets.NewReader(t.Context(), uri1, uri2))
		require.NoError(t, err)
		require.Equal(t, "plain\nencrypted\n", string(result))
	})
}

func TestUpdate(t *testing.T) {
	t.Run("file round trip", func(t *testing.T) {
		tmp := t.TempDir()
		uri := "file://" + filepath.Join(tmp, "secret.txt")

		require.NoError(t, egsecrets.Update(t.Context(), uri, bytes.NewReader([]byte("written"))))

		result, err := io.ReadAll(egsecrets.Read(t.Context(), uri))
		require.NoError(t, err)
		require.Equal(t, "written", string(result))
	})
}

func TestEnv(t *testing.T) {
	t.Run("single file with env vars", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "env.txt", "FOO=bar\nBAZ=qux")

		environ := egsecrets.Env(t.Context(), uri)
		require.Equal(t, []string{"FOO=bar", "BAZ=qux"}, environ)
	})

	t.Run("chacha encrypted env vars", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeChaChaSecret(t, tmp, "env.chacha", "pass", "SECRET_KEY=abc123\nDB_PASS=hunter2")

		environ := egsecrets.Env(t.Context(), uri)
		require.Equal(t, []string{"SECRET_KEY=abc123", "DB_PASS=hunter2"}, environ)
	})

	t.Run("multiple uris merged", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := writeFileSecret(t, tmp, "a.env", "A=1")
		uri2 := writeFileSecret(t, tmp, "b.env", "B=2")

		environ := egsecrets.Env(t.Context(), uri1, uri2)
		require.Equal(t, []string{"A=1", "B=2"}, environ)
	})

	t.Run("skips malformed lines", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "mixed.txt", "GOOD=value\nnot-a-var\nALSO_GOOD=yes")

		environ := egsecrets.Env(t.Context(), uri)
		require.Equal(t, []string{"GOOD=value", "ALSO_GOOD=yes"}, environ)
	})

	t.Run("empty secret returns nil", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "empty.txt", "")

		environ := egsecrets.Env(t.Context(), uri)
		require.Nil(t, environ)
	})
}

func TestCopyInto(t *testing.T) {
	t.Run("single uri", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "secret.txt", "content")

		var buf bytes.Buffer
		require.NoError(t, egsecrets.CopyInto(t.Context(), &buf, uri))
		require.Equal(t, "content", buf.String())
	})

	t.Run("multiple uris", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := writeFileSecret(t, tmp, "a.txt", "first")
		uri2 := writeChaChaSecret(t, tmp, "b.chacha", "pass", "second")

		var buf bytes.Buffer
		require.NoError(t, egsecrets.CopyInto(t.Context(), &buf, uri1, uri2))
		require.Equal(t, "first\nsecond\n", buf.String())
	})
}

func TestCopyIntoFile(t *testing.T) {
	t.Run("single uri", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "secret.txt", "file-content")
		dst := filepath.Join(tmp, "output.txt")

		require.NoError(t, egsecrets.CopyIntoFile(t.Context(), dst, uri))

		result, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, "file-content", string(result))
	})

	t.Run("multiple uris", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := writeFileSecret(t, tmp, "a.txt", "alpha")
		uri2 := writeFileSecret(t, tmp, "b.txt", "bravo")
		dst := filepath.Join(tmp, "combined.txt")

		require.NoError(t, egsecrets.CopyIntoFile(t.Context(), dst, uri1, uri2))

		result, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, "alpha\nbravo\n", string(result))
	})

	t.Run("invalid path", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "secret.txt", "data")

		err := egsecrets.CopyIntoFile(t.Context(), "/no/such/dir/out.txt", uri)
		require.Error(t, err)
	})

	t.Run("removes file when copy fails", func(t *testing.T) {
		tmp := t.TempDir()
		dst := filepath.Join(tmp, "output.txt")

		err := egsecrets.CopyIntoFile(t.Context(), dst, "file:///no/such/secret.txt")
		require.Error(t, err)
		_, statErr := os.Stat(dst)
		require.True(t, os.IsNotExist(statErr), "expected output file to be removed after failed copy")
	})
}

func TestCopyIntoFileOp(t *testing.T) {
	t.Run("single uri", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "secret.txt", "file-content")
		dst := filepath.Join(tmp, "output.txt")

		require.NoError(t, egsecrets.CopyIntoFileOp(dst, uri)(t.Context(), egtest.Op()))

		result, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, "file-content", string(result))
	})

	t.Run("multiple uris", func(t *testing.T) {
		tmp := t.TempDir()
		uri1 := writeFileSecret(t, tmp, "a.txt", "alpha")
		uri2 := writeFileSecret(t, tmp, "b.txt", "bravo")
		dst := filepath.Join(tmp, "combined.txt")

		require.NoError(t, egsecrets.CopyIntoFileOp(dst, uri1, uri2)(t.Context(), egtest.Op()))

		result, err := os.ReadFile(dst)
		require.NoError(t, err)
		require.Equal(t, "alpha\nbravo\n", string(result))
	})

	t.Run("invalid path", func(t *testing.T) {
		tmp := t.TempDir()
		uri := writeFileSecret(t, tmp, "secret.txt", "data")

		err := egsecrets.CopyIntoFileOp("/no/such/dir/out.txt", uri)(t.Context(), egtest.Op())
		require.Error(t, err)
	})
}
