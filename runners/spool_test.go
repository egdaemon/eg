package runners

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestSpoolDequeue(t *testing.T) {
	t.Run("handle conflicts", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)

		uid := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))

		// dequeue to create running
		ruid0, err := dirs.Dequeue()
		require.NoError(t, err)
		require.Equal(t, filepath.Join(dirs.Running, Queued().Dirname(uid)), ruid0)

		// enqueue again to create conflict
		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))

		ruid, err := dirs.Dequeue()
		require.ErrorIs(t, err, os.ErrExist)
		require.Equal(t, "", ruid)
	})
}
