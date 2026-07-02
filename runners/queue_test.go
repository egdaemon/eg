package runners_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

type completion struct {
	done context.CancelCauseFunc
	id   string
	d    time.Duration
}

func (t *completion) Upload(ctx context.Context, id string, duration time.Duration, cause error, logs io.Reader, analytics io.Reader) (err error) {
	t.id = id
	t.d = duration
	t.done(cause)
	return nil
}

func TestQueue(t *testing.T) {
	t.Run("Queue", func(t *testing.T) {
		t.Run("runs_a_single_workload_and_completes", func(t *testing.T) {
			dirs := runners.NewSpoolDir(t.TempDir())
			createTestWorkload(t, dirs.Queued, errorsx.Must(uuid.NewV4()), "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))
			rm := runners.NewResourceManager(runners.NewRuntimeResources())
			debugx.SetOutput(os.Stderr)
			reload := make(chan error, 1)
			ctx, done := context.WithCancelCause(t.Context())
			c := completion{done: done}
			err := runners.RunOne(ctx, 99, 0, rm, &dirs, reload, runners.QueueOptionCompletion(&c), runners.QueueOptionLogVerbosity(4))
			require.ErrorIs(t, err, context.Canceled)

			// Ensure the workload directory is gone
			_, err = dirs.Dequeue()
			require.ErrorIs(t, err, io.EOF)
		})

		t.Run("workload_timeout", func(t *testing.T) {
			dirs := runners.NewSpoolDir(t.TempDir())
			createTestWorkload(t, dirs.Queued, errorsx.Must(uuid.NewV4()), "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))
			rm := runners.NewResourceManager(runners.NewRuntimeResources())

			reload := make(chan error, 1)
			ctx, _done := context.WithTimeout(t.Context(), 200*time.Millisecond)
			defer _done()

			c := completion{done: func(cause error) {}}
			err := runners.RunOne(ctx, 99, 0, rm, &dirs, reload, runners.QueueOptionCompletion(&c), runners.QueueOptionLogVerbosity(4))
			require.ErrorIs(t, err, context.DeadlineExceeded)

			// Ensure the workload directory is gone
			_, err = dirs.Dequeue()
			require.ErrorIs(t, err, io.EOF)
		})

		t.Run("workload for an already active repo is blocked instead of run concurrently", func(t *testing.T) {
			dirs := runners.NewSpoolDir(t.TempDir())
			uid := errorsx.Must(uuid.NewV4())
			createTestWorkload(t, dirs.Queued, uid, "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))

			// simulate another worker already actively running a job for this
			// repo (account_id/vcs_uri match what createTestWorkload writes).
			key := md5x.String(uuid.Nil.String() + "uri")
			require.NoError(t, dirs.Block(key, ""))

			rm := runners.NewResourceManager(runners.NewRuntimeResources())
			reload := make(chan error, 1)
			ctx, done := context.WithTimeout(t.Context(), 2*time.Second)
			defer done()

			uploaded := false
			c := completion{done: func(cause error) { uploaded = true }}
			err := runners.RunOne(ctx, 99, 0, rm, &dirs, reload, runners.QueueOptionCompletion(&c), runners.QueueOptionLogVerbosity(4))
			require.ErrorIs(t, err, context.DeadlineExceeded)
			require.False(t, uploaded, "workload should not have run while repo was already active")

			// no resources should have been reserved for a workload that never ran.
			require.Equal(t, runners.RuntimeResources{}, rm.Snapshot())

			// the item should have been parked under Blocked/<key>, not run.
			entries, err := os.ReadDir(filepath.Join(dirs.Blocked, key))
			require.NoError(t, err)
			require.Len(t, entries, 1)

			// releasing the repo unblocks it back into Queued so it can run.
			require.NoError(t, dirs.Unblock(key))
			_, err = dirs.Dequeue()
			require.NoError(t, err)
		})

		t.Run("blocked workload completes once the repo is unblocked", func(t *testing.T) {
			dirs := runners.NewSpoolDir(t.TempDir())
			uid := errorsx.Must(uuid.NewV4())
			createTestWorkload(t, dirs.Queued, uid, "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))

			key := md5x.String(uuid.Nil.String() + "uri")
			require.NoError(t, dirs.Block(key, ""))
			require.NoError(t, dirs.Unblock(key))

			rm := runners.NewResourceManager(runners.NewRuntimeResources())
			reload := make(chan error, 1)
			parent, cancel := context.WithTimeout(t.Context(), 10*time.Second)
			defer cancel()

			uploaded := false
			ctx, cause := context.WithCancelCause(parent)
			c := completion{done: func(err error) { uploaded = true; cause(err) }}
			err := runners.RunOne(ctx, 99, 0, rm, &dirs, reload, runners.QueueOptionCompletion(&c), runners.QueueOptionLogVerbosity(4))
			require.ErrorIs(t, err, context.Canceled)
			require.True(t, uploaded, "workload should have run to completion once unblocked")
		})

		t.Run("workloads for different repos run without blocking each other", func(t *testing.T) {
			dirs := runners.NewSpoolDir(t.TempDir())
			createTestWorkloadFor(t, dirs.Queued, errorsx.Must(uuid.NewV4()), uuid.Nil.String(), "repo-a", "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))
			createTestWorkloadFor(t, dirs.Queued, errorsx.Must(uuid.NewV4()), uuid.Nil.String(), "repo-b", "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))

			rm := runners.NewResourceManager(runners.NewRuntimeResources())
			reload := make(chan error, 1)

			// each worker owns its own cancel-on-upload context, exactly like
			// runs_a_single_workload_and_completes above: since the two repos
			// don't collide, both should independently claim, run, and
			// complete their own item.
			ctx1, done1 := context.WithCancelCause(t.Context())
			ctx2, done2 := context.WithCancelCause(t.Context())
			c1 := completion{done: done1}
			c2 := completion{done: done2}

			var (
				wg         sync.WaitGroup
				err1, err2 error
			)
			wg.Add(2)
			go func() { defer wg.Done(); err1 = runners.RunOne(ctx1, 0, 0, rm, &dirs, reload, runners.QueueOptionCompletion(&c1)) }()
			go func() { defer wg.Done(); err2 = runners.RunOne(ctx2, 1, 0, rm, &dirs, reload, runners.QueueOptionCompletion(&c2)) }()
			wg.Wait()

			require.ErrorIs(t, err1, context.Canceled)
			require.ErrorIs(t, err2, context.Canceled)
		})

		t.Run("workload with no metadata", func(t *testing.T) {
			dirs := runners.NewSpoolDir(t.TempDir())
			uid := errorsx.Must(uuid.NewV4())
			createTestWorkload(t, dirs.Queued, uid, "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))
			fsx.PrintFS(os.DirFS(dirs.Queued))
			require.NoError(t, os.RemoveAll(filepath.Join(dirs.Queued, runners.Queued().Dirname(uid), "metadata.json")))
			fsx.PrintFS(os.DirFS(dirs.Queued))
			rm := runners.NewResourceManager(runners.NewRuntimeResources())

			reload := make(chan error, 1)
			ctx, done := context.WithCancelCause(t.Context())
			err := runners.RunOne(ctx, 99, 0, rm, &dirs, reload, runners.QueueOptionLogVerbosity(4), runners.QueueOptionFailure(done))
			require.ErrorIs(t, err, context.Canceled)

			expected := new(fs.PathError)
			require.ErrorAs(t, context.Cause(ctx), &expected)
			require.Equal(t, "open", expected.Op)
			require.Equal(t, filepath.Join(dirs.Running, runners.Queued().Dirname(uid), "metadata.json"), expected.Path)

			// Ensure the workload directory is gone
			_, err = dirs.Dequeue()
			require.ErrorIs(t, err, io.EOF)
		})
	})
}

// createTestWorkload is a helper function to set up a dummy workload directory
// for testing purposes.
func createTestWorkload(t *testing.T, baseDir string, uid uuid.UUID, entrypoint string, src io.Reader) string {
	return createTestWorkloadFor(t, baseDir, uid, uuid.Nil.String(), "uri", entrypoint, src)
}

// createTestWorkloadFor is like createTestWorkload but allows the account_id
// and vcs_uri to be set explicitly, so tests can construct workloads that
// belong to distinct (or identical) repos.
func createTestWorkloadFor(t *testing.T, baseDir string, uid uuid.UUID, accountID, vcsURI, entrypoint string, src io.Reader) string {
	workloadDir := filepath.Join(baseDir, runners.Queued().Dirname(uid))
	require.NoError(t, os.MkdirAll(workloadDir, 0755))
	dst, err := os.Create(filepath.Join(workloadDir, "archive.tar.gz"))
	require.NoError(t, err)

	metadataContent := []byte(`{"enqueued":{"id":"` + uid.String() + `", "account_id":"` + accountID + `", "vcs_uri":"` + vcsURI + `", "ttl":3600, "entry":"` + entrypoint + `"}}`)
	require.NoError(t, os.WriteFile(filepath.Join(workloadDir, "metadata.json"), metadataContent, 0644))
	gw := gzip.NewWriter(dst)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	module, err := io.ReadAll(src)
	require.NoError(t, err)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: entrypoint, Size: int64(len(module))}))
	_, err = io.Copy(tw, bytes.NewReader(module))
	require.NoError(t, err)

	return dst.Name()
}
