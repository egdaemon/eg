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
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
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
	workloadDir := filepath.Join(baseDir, runners.Queued().Dirname(uid))
	require.NoError(t, os.MkdirAll(workloadDir, 0755))
	dst, err := os.Create(filepath.Join(workloadDir, "archive.tar.gz"))
	require.NoError(t, err)

	metadataContent := []byte(`{"enqueued":{"id":"` + uid.String() + `", "account_id":"` + uuid.Nil.String() + `", "vcs_uri":"uri", "ttl":3600, "entry":"` + entrypoint + `"}}`)
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
