package compute

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/egdaemon/eg/computeapi"
	"github.com/golang-jwt/jwt/v4"
	"github.com/stretchr/testify/require"
)

func TestDownloadTo(t *testing.T) {
	t.Run("writes the output atomically", func(t *testing.T) {
		out := filepath.Join(t.TempDir(), "w1.analytics.db")

		require.NoError(t, downloadto(out, "ignored", func(dst io.Writer) error {
			_, err := dst.Write([]byte("data"))
			return err
		}))

		b, err := os.ReadFile(out)
		require.NoError(t, err)
		require.Equal(t, "data", string(b))

		entries, err := os.ReadDir(filepath.Dir(out))
		require.NoError(t, err)
		require.Len(t, entries, 1, "no temporary files remain")
	})

	t.Run("blank output uses the default name", func(t *testing.T) {
		wd, err := os.Getwd()
		require.NoError(t, err)
		dir := t.TempDir()
		require.NoError(t, os.Chdir(dir))
		defer func() { require.NoError(t, os.Chdir(wd)) }()

		require.NoError(t, downloadto("", "w1.log", func(dst io.Writer) error {
			_, err := dst.Write([]byte("log"))
			return err
		}))
		require.FileExists(t, filepath.Join(dir, "w1.log"))
	})

	t.Run("a failed download leaves nothing behind", func(t *testing.T) {
		dir := t.TempDir()
		out := filepath.Join(dir, "w1.analytics.db")

		require.Error(t, downloadto(out, "ignored", func(dst io.Writer) error {
			_, _ = dst.Write([]byte("partial"))
			return errors.New("boom")
		}))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Empty(t, entries)
	})
}

func TestDownloadAnalytics(t *testing.T) {
	t.Run("running workload is downloaded from its runner", func(t *testing.T) {
		var auth string

		runner := newlogrunner(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/c/analytics/w1", r.URL.Path)
			auth = r.Header.Get("Authorization")
			_, _ = w.Write([]byte("live analytics"))
		}))

		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"p2p": runner.ID().String()}).SignedString([]byte("secret"))
		require.NoError(t, err)

		chttp, seen := fakeapi(t, map[string]string{
			"/c/runners/meta": fmt.Sprintf(`{"bootstrap":[%q]}`, fmt.Sprintf("%s/p2p/%s", runner.Addrs()[0], runner.ID())),
			"/c/q/w1":         fmt.Sprintf(`{"enqueue":{"id":"w1","logs_token":%q,"completed_at":"9999-12-31T23:59:59Z"}}`, token),
		})

		var dst bytes.Buffer
		require.NoError(t, computeapi.DownloadAnalytics(t.Context(), chttp, "w1", &dst))
		require.Equal(t, "live analytics", dst.String())
		require.Equal(t, token, auth)
		require.Equal(t, []string{"/c/q/w1", "/c/runners/meta"}, seen())
	})

	t.Run("a workload completed since being scheduled is downloaded from the runner's history", func(t *testing.T) {
		runner := newlogrunner(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/c/analytics/w1":
				w.WriteHeader(http.StatusNotFound)
			case "/c/history/w1/analytics":
				_, _ = w.Write([]byte("recorded analytics"))
			default:
				w.WriteHeader(http.StatusBadRequest)
			}
		}))

		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"p2p": runner.ID().String()}).SignedString([]byte("secret"))
		require.NoError(t, err)

		chttp, _ := fakeapi(t, map[string]string{
			"/c/runners/meta": fmt.Sprintf(`{"bootstrap":[%q]}`, fmt.Sprintf("%s/p2p/%s", runner.Addrs()[0], runner.ID())),
			"/c/q/w1":         fmt.Sprintf(`{"enqueue":{"id":"w1","logs_token":%q,"completed_at":"9999-12-31T23:59:59Z"}}`, token),
		})

		var dst bytes.Buffer
		require.NoError(t, computeapi.DownloadAnalytics(t.Context(), chttp, "w1", &dst))
		require.Equal(t, "recorded analytics", dst.String())
	})

	t.Run("a workload that completed earlier is downloaded from the cluster's history", func(t *testing.T) {
		chttp, seen := fakeapi(t, map[string]string{
			"/c/q/w1":                 `{"enqueue":{"id":"w1","completed_at":"2020-01-01T00:00:00Z"}}`,
			"/c/history/w1/analytics": "stored analytics",
		})

		var dst bytes.Buffer
		require.NoError(t, computeapi.DownloadAnalytics(t.Context(), chttp, "w1", &dst))
		require.Equal(t, "stored analytics", dst.String())
		require.Equal(t, []string{"/c/q/w1", "/c/history/w1/analytics"}, seen())
	})

	t.Run("unknown id", func(t *testing.T) {
		chttp, _ := fakeapi(t, nil)

		require.Error(t, computeapi.DownloadAnalytics(t.Context(), chttp, "missing", io.Discard))
	})
}

func TestDownloadLogs(t *testing.T) {
	t.Run("a workload that completed earlier is downloaded from the cluster's history", func(t *testing.T) {
		chttp, _ := fakeapi(t, map[string]string{
			"/c/q/w1":            `{"enqueue":{"id":"w1","completed_at":"2020-01-01T00:00:00Z"}}`,
			"/c/history/w1/logs": "stored logs\n",
		})

		var dst bytes.Buffer
		require.NoError(t, computeapi.DownloadLogs(t.Context(), chttp, "w1", &dst))
		require.Equal(t, "stored logs\n", dst.String())
	})
}
