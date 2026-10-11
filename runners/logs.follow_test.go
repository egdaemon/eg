package runners

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

// syncbuffer is a bytes.Buffer safe for concurrent writes/reads.
type syncbuffer struct {
	m   sync.Mutex
	buf bytes.Buffer
}

func (t *syncbuffer) Write(b []byte) (int, error) {
	t.m.Lock()
	defer t.m.Unlock()
	return t.buf.Write(b)
}

func (t *syncbuffer) String() string {
	t.m.Lock()
	defer t.m.Unlock()
	return t.buf.String()
}

func createlog(t *testing.T, dirs SpoolDirs, uid uuid.UUID) *os.File {
	t.Helper()
	runtimedir := dirs.Path(uid).Resolve(workdirname, eg.RuntimeDirectory)
	require.NoError(t, os.MkdirAll(runtimedir, 0700))
	logs, err := os.Create(filepath.Join(runtimedir, DaemonLogName))
	require.NoError(t, err)
	t.Cleanup(func() { _ = logs.Close() })
	return logs
}

func follow(ctx context.Context, dirs SpoolDirs, uid uuid.UUID, offset int64, dst *syncbuffer) chan error {
	done := make(chan error, 1)
	go func() {
		done <- dirs.Follow(ctx, uid, offset, dst, func() {})
	}()
	return done
}

func awaitFollow(t *testing.T, done chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		require.FailNow(t, "follow did not complete")
		return nil
	}
}

func TestSpoolFollow(t *testing.T) {
	t.Run("unknown workload", func(t *testing.T) {
		dirs := NewSpoolDir(t.TempDir())
		var out syncbuffer
		require.NoError(t, awaitFollow(t, follow(t.Context(), dirs, uuid.Must(uuid.NewV7()), 0, &out)))
		require.Empty(t, out.String())
	})

	t.Run("existing log from offset", func(t *testing.T) {
		dirs := NewSpoolDir(t.TempDir())
		uid := uuid.Must(uuid.NewV7())
		logs := createlog(t, dirs, uid)
		_, err := logs.WriteString("hello world\n")
		require.NoError(t, err)

		var out syncbuffer
		done := follow(t.Context(), dirs, uid, 6, &out)
		require.Eventually(t, func() bool { return out.String() == "world\n" }, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, os.RemoveAll(dirs.Path(uid).Resolve()))
		require.NoError(t, awaitFollow(t, done))
		require.Equal(t, "world\n", out.String())
	})

	t.Run("appended writes arrive in order", func(t *testing.T) {
		dirs := NewSpoolDir(t.TempDir())
		uid := uuid.Must(uuid.NewV7())
		logs := createlog(t, dirs, uid)

		var out syncbuffer
		done := follow(t.Context(), dirs, uid, 0, &out)

		expected := ""
		for _, line := range []string{"one\n", "two\n", "three\n"} {
			_, err := logs.WriteString(line)
			require.NoError(t, err)
			expected += line
			require.Eventually(t, func() bool { return out.String() == expected }, 5*time.Second, 10*time.Millisecond)
		}

		// written after the last event but before completion; must still be delivered.
		_, err := logs.WriteString("final\n")
		require.NoError(t, err)
		require.NoError(t, os.RemoveAll(dirs.Path(uid).Resolve()))
		require.NoError(t, awaitFollow(t, done))
		require.Equal(t, expected+"final\n", out.String())
	})

	t.Run("waits for a queued workload to start", func(t *testing.T) {
		dirs := NewSpoolDir(t.TempDir())
		uid := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))

		var out syncbuffer
		done := follow(t.Context(), dirs, uid, 0, &out)

		// give the follower a chance to start waiting.
		time.Sleep(50 * time.Millisecond)
		require.NoError(t, os.Rename(filepath.Join(dirs.Queued, Queued().Dirname(uid)), dirs.Path(uid).Resolve()))
		logs := createlog(t, dirs, uid)
		_, err := logs.WriteString("started\n")
		require.NoError(t, err)
		require.Eventually(t, func() bool { return out.String() == "started\n" }, 5*time.Second, 10*time.Millisecond)

		require.NoError(t, os.Rename(dirs.Path(uid).Resolve(), filepath.Join(dirs.Tombstoned, Queued().Dirname(uid))))
		require.NoError(t, awaitFollow(t, done))
		require.Equal(t, "started\n", out.String())
	})

	t.Run("context cancellation", func(t *testing.T) {
		dirs := NewSpoolDir(t.TempDir())
		uid := uuid.Must(uuid.NewV7())
		createlog(t, dirs, uid)

		ctx, cancel := context.WithCancel(t.Context())
		var out syncbuffer
		done := follow(ctx, dirs, uid, 0, &out)
		time.Sleep(50 * time.Millisecond)
		cancel()
		require.ErrorIs(t, awaitFollow(t, done), context.Canceled)
	})
}
