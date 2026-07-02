package runners

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
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

func TestSpoolBlock(t *testing.T) {
	t.Run("claims an unclaimed key", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)

		uid := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))
		rundir, err := dirs.Dequeue()
		require.NoError(t, err)

		require.NoError(t, dirs.Block("repo1", rundir))
		require.DirExists(t, filepath.Join(dirs.Blocked, "repo1"))
		require.DirExists(t, rundir)
	})

	t.Run("parks a second claim for an already active key", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)

		uid1 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid1, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid1))
		rundir1, err := dirs.Dequeue()
		require.NoError(t, err)
		require.NoError(t, dirs.Block("repo1", rundir1))

		uid2 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid2, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid2))
		rundir2, err := dirs.Dequeue()
		require.NoError(t, err)

		err = dirs.Block("repo1", rundir2)
		require.ErrorIs(t, err, ErrRepoBlocked)
		require.NoDirExists(t, rundir2)
		require.DirExists(t, filepath.Join(dirs.Blocked, "repo1", Queued().Dirname(uid2)))
	})

	t.Run("unblock drains parked items back to queued and removes the marker", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)

		uid1 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid1, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid1))
		rundir1, err := dirs.Dequeue()
		require.NoError(t, err)
		require.NoError(t, dirs.Block("repo1", rundir1))

		uid2 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid2, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid2))
		rundir2, err := dirs.Dequeue()
		require.NoError(t, err)
		require.ErrorIs(t, dirs.Block("repo1", rundir2), ErrRepoBlocked)

		require.NoError(t, dirs.Unblock("repo1"))
		require.NoDirExists(t, filepath.Join(dirs.Blocked, "repo1"))
		require.DirExists(t, filepath.Join(dirs.Queued, Queued().Dirname(uid2)))
	})

	t.Run("unblock is a no-op for a key that was never blocked", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		require.NoError(t, dirs.Unblock("never-claimed"))
	})

	t.Run("concurrent claims for the same key elect exactly one winner", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)

		const n = 8
		rundirs := make([]string, n)
		for i := range rundirs {
			uid := uuid.Must(uuid.NewV7())
			require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
			require.NoError(t, dirs.Enqueue(uid))
			rundir, err := dirs.Dequeue()
			require.NoError(t, err)
			rundirs[i] = rundir
		}

		var (
			wg              sync.WaitGroup
			mu              sync.Mutex
			claimed, parked int
		)

		wg.Add(n)
		for _, rundir := range rundirs {
			go func(rundir string) {
				defer wg.Done()

				err := dirs.Block("repo1", rundir)

				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					claimed++
				case errors.Is(err, ErrRepoBlocked):
					parked++
				default:
					t.Errorf("unexpected error: %v", err)
				}
			}(rundir)
		}
		wg.Wait()

		require.Equal(t, 1, claimed)
		require.Equal(t, n-1, parked)

		entries, err := os.ReadDir(filepath.Join(dirs.Blocked, "repo1"))
		require.NoError(t, err)
		require.Len(t, entries, n-1)
	})
}
