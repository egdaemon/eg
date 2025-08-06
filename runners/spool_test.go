package runners

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestSpoolDequeueShouldHandleConflicts(t *testing.T) {
	sdir := t.TempDir()
	dirs := NewSpoolDir(sdir)

	for i := 0; i < 10; i++ {
		uid := uuid.Must(uuid.NewV7())

		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))
		require.NoError(t, os.Mkdir(filepath.Join(sdir, "r", iddirname(uid)), 0700))
	}

	uid := uuid.Must(uuid.NewV7())
	require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
	require.NoError(t, dirs.Enqueue(uid))

	ruid, err := dirs.Dequeue()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(sdir, "r", iddirname(uid)), ruid)
}
