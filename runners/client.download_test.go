package runners_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/testx"
	"github.com/egdaemon/eg/runners"
	"github.com/gofrs/uuid/v5"
	"github.com/stretchr/testify/require"
)

func buildArchive(t *testing.T, entrypoint string, src io.Reader) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	content, err := io.ReadAll(src)
	require.NoError(t, err)
	require.NoError(t, tw.WriteHeader(&tar.Header{Name: entrypoint, Size: int64(len(content))}))
	_, err = tw.Write(content)
	require.NoError(t, err)
	require.NoError(t, tw.Close())
	require.NoError(t, gw.Close())
	return buf.Bytes()
}

func TestDownloadClient(t *testing.T) {
	t.Run("retries_on_409_without_exhausting_connections", func(t *testing.T) {
		// The bug: defer inside the retry loop means 409 response bodies are
		// never closed during retries. With MaxConnsPerHost=1, the unreleased
		// body holds the only connection, so the next request blocks forever.
		const conflictRetries = 3

		uid := errorsx.Must(uuid.NewV4())
		archive := buildArchive(t, "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))

		attempts := 0
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempts < conflictRetries {
				attempts++
				// WriteEmptyJSON writes a body — the connection cannot be
				// reused until it is drained and closed.
				errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusConflict))
				return
			}
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive)
		}))
		defer srv.Close()

		// One connection only — exposes the connection exhaustion caused by
		// unclosed 409 response bodies accumulating across loop iterations.
		transport := &http.Transport{MaxConnsPerHost: 1}
		client := &http.Client{Transport: transport}

		dirs := runners.NewSpoolDir(t.TempDir())
		dc := runners.NewDownloadClient(client, runners.DownloadClientOptionHost(srv.URL), runners.DownloadClientOptionDirs(dirs))

		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()

		workload := &runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{
				Id:    uid.String(),
				Entry: "entry.wasm",
			},
		}

		err := dc.Download(ctx, workload)
		require.NoError(t, err)
	})

	t.Run("success_on_first_attempt_writes_spool_files", func(t *testing.T) {
		uid := errorsx.Must(uuid.NewV4())
		archive := buildArchive(t, "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive)
		}))
		defer srv.Close()

		root := t.TempDir()
		dirs := runners.NewSpoolDir(root)
		dc := runners.NewDownloadClient(http.DefaultClient, runners.DownloadClientOptionHost(srv.URL), runners.DownloadClientOptionDirs(dirs))

		err := dc.Download(t.Context(), &runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uid.String(), Entry: "entry.wasm"},
		})
		require.NoError(t, err)

		// After a successful download the item moves from downloading → queued.
		dirname := runners.Queued().Dirname(uid)
		_, err = os.Stat(filepath.Join(dirs.Downloading, dirname))
		require.True(t, os.IsNotExist(err), "downloading dir should be gone after enqueue")

		require.FileExists(t, filepath.Join(dirs.Queued, dirname, "archive.tar.gz"))
		require.FileExists(t, filepath.Join(dirs.Queued, dirname, "metadata.json"))
	})

	t.Run("context_cancelled_during_409_retry_loop", func(t *testing.T) {
		uid := errorsx.Must(uuid.NewV4())

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			errorsx.Log(httpx.WriteEmptyJSON(w, http.StatusConflict))
		}))
		defer srv.Close()

		dirs := runners.NewSpoolDir(t.TempDir())
		dc := runners.NewDownloadClient(http.DefaultClient, runners.DownloadClientOptionHost(srv.URL), runners.DownloadClientOptionDirs(dirs))

		ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
		defer cancel()

		err := dc.Download(ctx, &runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uid.String(), Entry: "entry.wasm"},
		})
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("non_retryable_http_error_propagates", func(t *testing.T) {
		uid := errorsx.Must(uuid.NewV4())

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		dirs := runners.NewSpoolDir(t.TempDir())
		dc := runners.NewDownloadClient(http.DefaultClient, runners.DownloadClientOptionHost(srv.URL), runners.DownloadClientOptionDirs(dirs))

		err := dc.Download(t.Context(), &runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: uid.String(), Entry: "entry.wasm"},
		})
		require.Error(t, err)
	})

	t.Run("invalid_uuid_returns_error", func(t *testing.T) {
		archive := buildArchive(t, "entry.wasm", testx.Read(testx.Fixture("example.1.wasm")))

		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(archive)
		}))
		defer srv.Close()

		dirs := runners.NewSpoolDir(t.TempDir())
		dc := runners.NewDownloadClient(http.DefaultClient, runners.DownloadClientOptionHost(srv.URL), runners.DownloadClientOptionDirs(dirs))

		err := dc.Download(t.Context(), &runners.EnqueuedDequeueResponse{
			Enqueued: &runners.Enqueued{Id: "not-a-valid-uuid", Entry: "entry.wasm"},
		})
		require.ErrorContains(t, err, "malformed uid")
	})
}
