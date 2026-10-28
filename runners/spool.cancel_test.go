package runners

import (
	"bytes"
	"context"
	"os"
	"testing"
	"time"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestSpoolCancel(t *testing.T) {
	t.Run("unknown workload", func(t *testing.T) {
		dirs := NewSpoolDir(t.TempDir())
		require.ErrorIs(t, dirs.Cancel(uuid.Must(uuid.NewV7())), ErrWorkloadNotFound)
	})

	t.Run("marks queued and running workloads and is idempotent", func(t *testing.T) {
		dirs := NewSpoolDir(t.TempDir())
		uid := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))

		require.NoError(t, dirs.Cancel(uid))
		require.NoError(t, dirs.Cancel(uid))

		// the marker travels with the directory.
		rundir, err := dirs.Dequeue()
		require.NoError(t, err)
		require.True(t, IsCancelled(rundir))
	})
}

func TestWatchCancel(t *testing.T) {
	t.Run("cancels the context when the marker is created", func(t *testing.T) {
		dirs := NewSpoolDir(t.TempDir())
		uid := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))
		rundir, err := dirs.Dequeue()
		require.NoError(t, err)

		ctx, stop, err := watchcancel(t.Context(), rundir)
		require.NoError(t, err)
		defer stop()

		require.NoError(t, ctx.Err())
		require.NoError(t, dirs.Cancel(uid))

		select {
		case <-ctx.Done():
			require.ErrorIs(t, context.Cause(ctx), ErrCancelled)
		case <-time.After(5 * time.Second):
			t.Fatal("context was not cancelled")
		}
	})

	t.Run("marker created before the watch", func(t *testing.T) {
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(dir+"/"+cancelmarker, nil, 0600))

		ctx, stop, err := watchcancel(t.Context(), dir)
		require.NoError(t, err)
		defer stop()

		require.ErrorIs(t, context.Cause(ctx), ErrCancelled)
	})

	t.Run("stopping does not mark the workload cancelled", func(t *testing.T) {
		ctx, stop, err := watchcancel(t.Context(), t.TempDir())
		require.NoError(t, err)
		stop()
		require.NotErrorIs(t, context.Cause(ctx), ErrCancelled)
	})
}

func TestRecoverDiscardsCancelled(t *testing.T) {
	dirs := NewSpoolDir(t.TempDir())
	uid := uuid.Must(uuid.NewV7())
	require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
	require.NoError(t, dirs.Enqueue(uid))
	rundir, err := dirs.Dequeue()
	require.NoError(t, err)
	require.NoError(t, dirs.Cancel(uid))

	require.NoError(t, recover(t.Context(), metadata{dirs: &dirs}))

	_, ok := dirs.Locate(uid)
	require.False(t, ok)
	_, err = os.Stat(rundir)
	require.True(t, os.IsNotExist(err))
}
