package tarx_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/internal/tarx"
	"github.com/stretchr/testify/require"
)

func TestSize(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested"), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "a.txt"), bytes.Repeat([]byte("a"), 10), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "nested", "b.txt"), bytes.Repeat([]byte("b"), 32), 0600))

	single := filepath.Join(t.TempDir(), "c.txt")
	require.NoError(t, os.WriteFile(single, bytes.Repeat([]byte("c"), 7), 0600))

	buf := bytes.NewBuffer(nil)
	require.NoError(t, tarx.Pack(buf, dir, single))

	n, err := tarx.Size(buf)
	require.NoError(t, err)
	require.Equal(t, int64(49), n)
}

func TestSizeEmpty(t *testing.T) {
	buf := bytes.NewBuffer(nil)
	require.NoError(t, tarx.Pack(buf))

	n, err := tarx.Size(buf)
	require.NoError(t, err)
	require.Zero(t, n)
}

func TestSizeInvalid(t *testing.T) {
	_, err := tarx.Size(bytes.NewBufferString("not an archive"))
	require.Error(t, err)
}
