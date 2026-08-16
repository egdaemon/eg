package runners

import (
	"bytes"
	"path/filepath"
	"testing"

	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func TestRecoverReleasesStaleRepoBlocks(t *testing.T) {
	sdir := t.TempDir()
	dirs := NewSpoolDir(sdir)
	enq := &Enqueued{AccountId: "acct-recover", VcsUri: "repo"}

	// item that had claimed the key (still sitting in Running, as if the
	// process crashed mid-run).
	uid1 := uuid.Must(uuid.NewV7())
	require.NoError(t, dirs.Download(uid1, "archive.tar.gz", bytes.NewBufferString("")))
	require.NoError(t, dirs.Enqueue(uid1))
	rundir1, err := dirs.Dequeue()
	require.NoError(t, err)
	bucket, err := NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir1)
	require.NoError(t, err)

	// a second item for the same repo that got parked behind it.
	uid2 := uuid.Must(uuid.NewV7())
	require.NoError(t, dirs.Download(uid2, "archive.tar.gz", bytes.NewBufferString("")))
	require.NoError(t, dirs.Enqueue(uid2))
	rundir2, err := dirs.Dequeue()
	require.NoError(t, err)
	_, err = NewCacheResolution(dirs, 1).Claim(t.Context(), cachebuckets(enq), rundir2)
	require.ErrorIs(t, err, ErrRepoBlocked)

	// fresh process start: nothing is actually running, so recover() must
	// release the stale marker and requeue everything.
	require.NoError(t, recover(t.Context(), metadata{dirs: &dirs}))

	require.NoDirExists(t, filepath.Join(dirs.Blocked, bucket))
	require.DirExists(t, filepath.Join(dirs.Queued, Queued().Dirname(uid1)))
	require.DirExists(t, filepath.Join(dirs.Queued, Queued().Dirname(uid2)))
}
