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

func TestCacheResolutionClaim(t *testing.T) {
	t.Run("claims an unclaimed key", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		enq := &Enqueued{AccountId: "acct-claims", VcsUri: "repo"}

		uid := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid))
		rundir, err := dirs.Dequeue()
		require.NoError(t, err)

		bucket, err := NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir)
		require.NoError(t, err)
		require.NotEmpty(t, bucket)
		require.DirExists(t, filepath.Join(dirs.Blocked, bucket))
		require.DirExists(t, rundir)
	})

	t.Run("parks a second claim for an already active key", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		enq := &Enqueued{AccountId: "acct-park", VcsUri: "repo"}

		uid1 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid1, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid1))
		rundir1, err := dirs.Dequeue()
		require.NoError(t, err)
		bucket1, err := NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir1)
		require.NoError(t, err)

		uid2 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid2, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid2))
		rundir2, err := dirs.Dequeue()
		require.NoError(t, err)

		bucket2, err := NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir2)
		require.ErrorIs(t, err, ErrRepoBlocked)
		require.Equal(t, "", bucket2)
		require.NoDirExists(t, rundir2)
		require.DirExists(t, filepath.Join(dirs.Blocked, bucket1, Queued().Dirname(uid2)))
	})

	t.Run("claims the next candidate when the first is already active", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		enq := &Enqueued{AccountId: "acct-next", VcsUri: "repo"}

		uid1 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid1, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid1))
		rundir1, err := dirs.Dequeue()
		require.NoError(t, err)
		bucket1, err := NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir1)
		require.NoError(t, err)

		uid2 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid2, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid2))
		rundir2, err := dirs.Dequeue()
		require.NoError(t, err)

		bucket2, err := NewCacheResolution(dirs, 2).Claim(t.Context(), cachebuckets(enq), rundir2)
		require.NoError(t, err)
		require.NotEqual(t, bucket1, bucket2)
		require.DirExists(t, rundir2)
	})

	t.Run("parks under the last candidate once all are exhausted", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		enq := &Enqueued{AccountId: "acct-exhausted", VcsUri: "repo"}

		const n = 3
		var last string
		for range n {
			uid := uuid.Must(uuid.NewV7())
			require.NoError(t, dirs.Download(uid, "archive.tar.gz", bytes.NewBufferString("")))
			require.NoError(t, dirs.Enqueue(uid))
			rundir, err := dirs.Dequeue()
			require.NoError(t, err)

			bucket, err := NewCacheResolution(dirs, n).Claim(t.Context(), cachebuckets(enq), rundir)
			require.NoError(t, err)
			last = bucket
		}

		uidLast := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uidLast, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uidLast))
		rundirLast, err := dirs.Dequeue()
		require.NoError(t, err)

		bucket, err := NewCacheResolution(dirs, n).Claim(t.Context(), cachebuckets(enq), rundirLast)
		require.ErrorIs(t, err, ErrRepoBlocked)
		require.Equal(t, "", bucket)
		require.NoDirExists(t, rundirLast)
		require.DirExists(t, filepath.Join(dirs.Blocked, last, Queued().Dirname(uidLast)))
	})

	t.Run("release drains parked items back to queued and removes the marker", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		enq := &Enqueued{AccountId: "acct-release", VcsUri: "repo"}

		uid1 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid1, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid1))
		rundir1, err := dirs.Dequeue()
		require.NoError(t, err)
		bucket1, err := NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir1)
		require.NoError(t, err)

		uid2 := uuid.Must(uuid.NewV7())
		require.NoError(t, dirs.Download(uid2, "archive.tar.gz", bytes.NewBufferString("")))
		require.NoError(t, dirs.Enqueue(uid2))
		rundir2, err := dirs.Dequeue()
		require.NoError(t, err)
		_, err = NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir2)
		require.ErrorIs(t, err, ErrRepoBlocked)

		require.NoError(t, NewCacheResolution(dirs, 1).Release(bucket1))
		require.NoDirExists(t, filepath.Join(dirs.Blocked, bucket1))
		require.DirExists(t, filepath.Join(dirs.Queued, Queued().Dirname(uid2)))
	})

	t.Run("release is a no-op for a key that was never claimed", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		require.NoError(t, NewCacheResolution(dirs, 1).Release("never-claimed"))
	})

	t.Run("concurrent claims for the same key elect exactly one winner", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		enq := &Enqueued{AccountId: "acct-concurrent", VcsUri: "repo"}

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
			winner          string
			claimed, parked int
		)

		wg.Add(n)
		for _, rundir := range rundirs {
			go func(rundir string) {
				defer wg.Done()

				bucket, err := NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir)

				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					claimed++
					winner = bucket
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

		entries, err := os.ReadDir(filepath.Join(dirs.Blocked, winner))
		require.NoError(t, err)
		require.Len(t, entries, n-1)
	})

	t.Run("concurrent claims across a fan-out of candidates elect up to len(candidates) winners", func(t *testing.T) {
		sdir := t.TempDir()
		dirs := NewSpoolDir(sdir)
		enq := &Enqueued{AccountId: "acct-fanout", VcsUri: "repo"}

		const buckets = 8
		const n = buckets * 2
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
			claimedBuckets  = map[string]int{}
			claimed, parked int
		)

		wg.Add(n)
		for _, rundir := range rundirs {
			go func(rundir string) {
				defer wg.Done()

				bucket, err := NewCacheResolution(dirs, buckets).Claim(t.Context(), cachebuckets(enq), rundir)

				mu.Lock()
				defer mu.Unlock()
				switch {
				case err == nil:
					claimed++
					claimedBuckets[bucket]++
				case errors.Is(err, ErrRepoBlocked):
					parked++
				default:
					t.Errorf("unexpected error: %v", err)
				}
			}(rundir)
		}
		wg.Wait()

		require.Equal(t, buckets, claimed)
		require.Equal(t, n-buckets, parked)
		require.Len(t, claimedBuckets, buckets)
		for bucket, count := range claimedBuckets {
			require.Equalf(t, 1, count, "bucket %s claimed more than once", bucket)
		}
	})
}
