package cmdopts

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIOIn(t *testing.T) {
	t.Run("dash reads the fallback with nop close", func(t *testing.T) {
		var v IOIn
		require.NoError(t, v.UnmarshalText([]byte("-")))

		r, err := v.Open(strings.NewReader("from stdin"))
		require.NoError(t, err)

		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "from stdin", string(content))
		require.NoError(t, r.Close())
	})

	t.Run("dash does not close the fallback", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "stdin")
		require.NoError(t, os.WriteFile(path, []byte("hello"), 0600))
		fallback, err := os.Open(path)
		require.NoError(t, err)
		defer fallback.Close()

		var v IOIn
		require.NoError(t, v.UnmarshalText([]byte("-")))
		r, err := v.Open(fallback)
		require.NoError(t, err)
		require.NoError(t, r.Close())

		_, err = fallback.Stat()
		require.NoError(t, err, "the fallback must stay open")
		_, err = fallback.Read(make([]byte, 1))
		require.NoError(t, err, "the fallback must stay readable")
	})

	t.Run("path opens the file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "in.txt")
		require.NoError(t, os.WriteFile(path, []byte("hello"), 0600))

		var v IOIn
		require.NoError(t, v.UnmarshalText([]byte(path)))

		r, err := v.Open(strings.NewReader("ignored"))
		require.NoError(t, err)
		defer r.Close()

		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.Equal(t, "hello", string(content))
	})

	t.Run("a missing file is an error", func(t *testing.T) {
		var v IOIn
		require.NoError(t, v.UnmarshalText([]byte(filepath.Join(t.TempDir(), "missing"))))

		_, err := v.Open(strings.NewReader("ignored"))
		require.Error(t, err)
	})

	t.Run("marshal roundtrips path", func(t *testing.T) {
		var v IOIn
		require.NoError(t, v.UnmarshalText([]byte("/some/path")))
		text, err := v.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "/some/path", string(text))
	})
}
