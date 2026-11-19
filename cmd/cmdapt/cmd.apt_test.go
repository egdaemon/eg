package cmdapt

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egdaemon/eg/apt"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

func TestSources(t *testing.T) {
	t.Run("should default to the egs transport", func(t *testing.T) {
		out := sources("apt.egdaemon.com", "acct", "repo", "stable", "main", false)
		require.Contains(t, out, "deb [signed-by=/etc/apt/keyrings/repo.asc] egs://acct.apt.egdaemon.com/repo stable main")
		require.Contains(t, out, "sudo eg apt repo key repo --account-id=acct --output=/etc/apt/keyrings/repo.asc")
		require.Contains(t, out, "/etc/apt/sources.list.d/repo.list")
		require.NotContains(t, out, "auth.conf.d")
	})

	t.Run("should use https with basic authentication", func(t *testing.T) {
		out := sources("apt.egdaemon.com", "acct", "repo", "testing", "contrib", true)
		require.Contains(t, out, "deb [signed-by=/etc/apt/keyrings/repo.asc] https://acct.apt.egdaemon.com/repo testing contrib")
		require.Contains(t, out, "curl -fsSL https://acct.apt.egdaemon.com/repo/key.asc")
		require.Contains(t, out, `printf 'machine acct.apt.egdaemon.com/repo\nlogin token\npassword %s\n' "$(eg authz ci token --account-id=acct)"`)
		require.Contains(t, out, "/etc/apt/auth.conf.d/repo.conf")
		require.NotContains(t, out, "egs://")
	})
}

func debs(t *testing.T, names ...string) []string {
	dir := t.TempDir()
	paths := make([]string, 0, len(names))
	for _, n := range names {
		p := filepath.Join(dir, n)
		require.NoError(t, os.WriteFile(p, []byte(n), 0600))
		paths = append(paths, p)
	}
	return paths
}

func TestUpload(t *testing.T) {
	gctx := &cmdopts.Global{Context: context.Background()}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/repo/upload/")
		switch name {
		case "staged.deb":
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"filename":"pool/staged.deb","sha256":"s1","bytes":10,"staged":true}`)
		case "same.deb":
			fmt.Fprint(w, `{"filename":"pool/same.deb","sha256":"s2","bytes":8,"staged":false}`)
		case "conflict.deb":
			w.WriteHeader(http.StatusConflict)
		case "invalid.deb":
			w.WriteHeader(http.StatusBadRequest)
		default:
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	c := apt.New(srv.Client(), srv.URL+"/repo")

	t.Run("should report each package", func(t *testing.T) {
		var out bytes.Buffer
		paths := debs(t, "staged.deb", "same.deb")
		require.NoError(t, upload(gctx, c, &out, paths...))
		require.Equal(t, fmt.Sprintf("staged %s pool/staged.deb s1\nunchanged %s pool/same.deb s2\n", paths[0], paths[1]), out.String())
	})

	t.Run("should attempt every package and fail on conflicts and invalid packages", func(t *testing.T) {
		var out bytes.Buffer
		paths := debs(t, "conflict.deb", "staged.deb", "invalid.deb", "broken.deb")
		err := upload(gctx, c, &out, paths...)
		require.Error(t, err)
		require.ErrorContains(t, err, "already published")
		require.ErrorContains(t, err, "not a valid debian package")
		require.ErrorContains(t, err, "upload "+paths[3])
		require.True(t, apt.Status(err, http.StatusConflict))
		require.Equal(t, fmt.Sprintf("conflict %s\nstaged %s pool/staged.deb s1\ninvalid %s\nfailed %s\n", paths[0], paths[1], paths[2], paths[3]), out.String())
	})
}

func TestPublish(t *testing.T) {
	gctx := &cmdopts.Global{Context: context.Background()}
	nobackoff := func(int) time.Duration { return time.Millisecond }

	t.Run("should print the version", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) == 1 {
				w.WriteHeader(http.StatusConflict)
				return
			}
			fmt.Fprint(w, `{"vid":"v3"}`)
		}))
		defer srv.Close()

		var out bytes.Buffer
		require.NoError(t, publish(gctx, apt.New(srv.Client(), srv.URL+"/repo"), &out, 3, nobackoff))
		require.Equal(t, "v3\n", out.String())
	})

	t.Run("should explain a publish that keeps conflicting", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusConflict)
		}))
		defer srv.Close()

		err := publish(gctx, apt.New(srv.Client(), srv.URL+"/repo"), io.Discard, 1, nobackoff)
		require.ErrorContains(t, err, "another publish is in progress after 1 retries")
	})
}

func TestWriteKey(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keyrings", "repo.asc")

	t.Run("should install the key", func(t *testing.T) {
		require.NoError(t, writekey(path, func(w io.Writer) error {
			_, err := io.WriteString(w, "key-1")
			return err
		}))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "key-1", string(data))

		info, err := os.Stat(path)
		require.NoError(t, err)
		require.Equal(t, os.FileMode(0644), info.Mode().Perm())
	})

	t.Run("should keep the existing key when the fetch fails", func(t *testing.T) {
		require.Error(t, writekey(path, func(w io.Writer) error {
			_, _ = io.WriteString(w, "partial")
			return errors.New("boom")
		}))

		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, "key-1", string(data))

		entries, err := os.ReadDir(filepath.Dir(path))
		require.NoError(t, err)
		require.Len(t, entries, 1)
	})
}
