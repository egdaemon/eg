package daemons_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/runners"
	"github.com/stretchr/testify/require"
)

func newUploadHandler(t *testing.T) *daemons.UploadHandler {
	t.Helper()
	return &daemons.UploadHandler{
		Dirs: runners.NewSpoolDir(t.TempDir()),
	}
}

func newUploadRequest(t *testing.T, fields map[string]io.Reader) *http.Request {
	t.Helper()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for name, content := range fields {
		fw, err := mw.CreateFormFile(name, name)
		require.NoError(t, err)
		_, err = io.Copy(fw, content)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())

	r := httptest.NewRequest(http.MethodPost, "/b/upload", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func TestUploadHandler(t *testing.T) {
	t.Run("valid uploads are enqueued directly, no compile step", func(t *testing.T) {
		h := newUploadHandler(t)

		r := newUploadRequest(t, map[string]io.Reader{
			"kernel":  bytes.NewBufferString("kernel contents"),
			"environ": bytes.NewBufferString("environ contents"),
		})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		require.Equal(t, http.StatusOK, w.Code)

		entries, err := os.ReadDir(h.Dirs.Queued)
		require.NoError(t, err)
		require.Len(t, entries, 1)
	})

	t.Run("missing kernel file is rejected", func(t *testing.T) {
		h := newUploadHandler(t)

		r := newUploadRequest(t, map[string]io.Reader{
			"environ": bytes.NewBufferString("environ contents"),
		})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("missing environ file is rejected", func(t *testing.T) {
		h := newUploadHandler(t)

		r := newUploadRequest(t, map[string]io.Reader{
			"kernel": bytes.NewBufferString("kernel contents"),
		})
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		require.Equal(t, http.StatusBadRequest, w.Code)
	})
}
