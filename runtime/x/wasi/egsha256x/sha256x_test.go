package egsha256x_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/runtime/x/wasi/egsha256x"
	"github.com/stretchr/testify/require"
)

const (
	helloHex    = "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	helloBase64 = "LPJNul-wow4m6DsqxbninhsWHlwfp0JecwQzYpOLmCQ"
	emptyHex    = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
)

func TestDigest(t *testing.T) {
	require.Equal(t, helloHex, egsha256x.FormatHex(egsha256x.Digest("hello")))
	require.Equal(t, helloHex, egsha256x.FormatHex(egsha256x.Digest([]byte("hello"))))
	require.Equal(t, emptyHex, egsha256x.FormatHex(egsha256x.Digest("")))
}

func TestDigestIO(t *testing.T) {
	t.Run("matches Digest", func(t *testing.T) {
		require.Equal(t, helloHex, egsha256x.FormatHex(egsha256x.DigestIO(strings.NewReader("hello"))))
	})

	t.Run("larger than the copy buffer", func(t *testing.T) {
		large := strings.Repeat("a", 100*1024)
		require.Equal(t,
			egsha256x.FormatHex(egsha256x.Digest(large)),
			egsha256x.FormatHex(egsha256x.DigestIO(strings.NewReader(large))),
		)
	})

	t.Run("read failure returns nil", func(t *testing.T) {
		require.Nil(t, egsha256x.DigestIO(errorsx.Reader(errors.New("read failure"))))
	})
}

func TestDigestFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello"), 0600))

	t.Run("single path", func(t *testing.T) {
		require.Equal(t, helloHex, egsha256x.FormatHex(egsha256x.DigestFile(filepath.Join(dir, "hello.txt"))))
	})

	t.Run("joined path elements", func(t *testing.T) {
		require.Equal(t, helloHex, egsha256x.FormatHex(egsha256x.DigestFile(dir, "hello.txt")))
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		require.Nil(t, egsha256x.DigestFile(dir, "missing.txt"))
	})
}

func TestString(t *testing.T) {
	require.Equal(t, helloBase64, egsha256x.String("hello"))
}

func TestFormatBase64(t *testing.T) {
	require.Equal(t, helloBase64, egsha256x.FormatBase64(egsha256x.Digest("hello")))
}

func TestFormatHex(t *testing.T) {
	require.Equal(t, helloHex, egsha256x.FormatHex(egsha256x.Digest("hello")))
}
