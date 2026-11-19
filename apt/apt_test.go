package apt

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestValidName(t *testing.T) {
	for _, name := range []string{"eg", "0", "a.b-c", "debian12"} {
		require.NoError(t, ValidName(name), name)
	}

	for _, name := range []string{"", "-a", ".a", "A", "a_b", "a/b"} {
		require.Error(t, ValidName(name), name)
	}
}

func TestURL(t *testing.T) {
	require.Equal(t, "egs://acct.apt.egdaemon.com/repo", URL(Scheme, "apt.egdaemon.com", "acct", "repo"))
	require.Equal(t, "https://acct.apt.eg.lawrencia.network/repo", URL("https", "apt.eg.lawrencia.network", "acct", "repo"))
}

func ptr[T any](v T) *T { return &v }

func TestSettings(t *testing.T) {
	t.Run("should only encode the fields that are set", func(t *testing.T) {
		encoded, err := json.Marshal(Settings{Public: ptr(false), KeepN: ptr(3), Architectures: []string{"amd64", "arm64"}, KeepNGracePeriod: ptr("2160h")})
		require.NoError(t, err)
		require.JSONEq(t, `{"public":false,"keepn":3,"architectures":["amd64","arm64"],"keepn_grace_period":"2160h"}`, string(encoded))
	})

	t.Run("should encode nothing when empty", func(t *testing.T) {
		encoded, err := json.Marshal(Settings{})
		require.NoError(t, err)
		require.JSONEq(t, `{}`, string(encoded))
	})
}

func TestClient(t *testing.T) {
	ctx := context.Background()

	t.Run("should create the repository with the settings", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/repo", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			body, _ := io.ReadAll(r.Body)
			require.JSONEq(t, `{"suite":"testing","architectures":["amd64"]}`, string(body))
			w.WriteHeader(http.StatusCreated)
			fmt.Fprint(w, `{"name":"repo"}`)
		}))
		defer srv.Close()

		out, err := New(srv.Client(), srv.URL+"/repo").Create(ctx, Settings{Suite: ptr("testing"), Architectures: []string{"amd64"}})
		require.NoError(t, err)
		require.JSONEq(t, `{"name":"repo"}`, string(out))
	})

	t.Run("should patch the settings", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/repo", r.URL.Path)
			body, _ := io.ReadAll(r.Body)
			require.JSONEq(t, `{"public":true}`, string(body))
			fmt.Fprint(w, `{"name":"repo","public":true}`)
		}))
		defer srv.Close()

		out, err := New(srv.Client(), srv.URL+"/repo").Set(ctx, Settings{Public: ptr(true)})
		require.NoError(t, err)
		require.JSONEq(t, `{"name":"repo","public":true}`, string(out))
	})

	t.Run("should report payment required when making a repository public", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "paid account required", http.StatusPaymentRequired)
		}))
		defer srv.Close()

		_, err := New(srv.Client(), srv.URL+"/repo").Set(ctx, Settings{Public: ptr(true)})
		require.True(t, Status(err, http.StatusPaymentRequired))
		require.ErrorContains(t, err, "paid account required")
	})

	t.Run("should show the repository", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/repo", r.URL.Path)
			fmt.Fprint(w, `{"name":"repo"}`)
		}))
		defer srv.Close()

		out, err := New(srv.Client(), srv.URL+"/repo/").Show(ctx)
		require.NoError(t, err)
		require.JSONEq(t, `{"name":"repo"}`, string(out))
	})

	t.Run("should fetch the key", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/repo/key.asc", r.URL.Path)
			fmt.Fprint(w, "-----BEGIN PGP PUBLIC KEY BLOCK-----")
		}))
		defer srv.Close()

		var buf bytes.Buffer
		require.NoError(t, New(srv.Client(), srv.URL+"/repo").Key(ctx, &buf))
		require.Equal(t, "-----BEGIN PGP PUBLIC KEY BLOCK-----", buf.String())
	})

	t.Run("should upload the raw deb", func(t *testing.T) {
		deb := filepath.Join(t.TempDir(), "eg_1.0_amd64.deb")
		require.NoError(t, os.WriteFile(deb, []byte("!<arch>\n"), 0600))

		statuses := []int{http.StatusCreated, http.StatusOK}
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPut, r.Method)
			require.Equal(t, "/repo/upload/eg_1.0_amd64.deb", r.URL.Path)
			require.Equal(t, ContentTypeDeb, r.Header.Get("Content-Type"))
			require.Equal(t, int64(8), r.ContentLength)
			body, _ := io.ReadAll(r.Body)
			require.Equal(t, "!<arch>\n", string(body))
			status := statuses[calls.Add(1)-1]
			w.WriteHeader(status)
			fmt.Fprintf(w, `{"filename":"pool/main/e/eg/eg_1.0_amd64.deb","sha256":"abc","bytes":8,"staged":%t}`, status == http.StatusCreated)
		}))
		defer srv.Close()

		c := New(srv.Client(), srv.URL+"/repo")
		out, err := c.Upload(ctx, deb)
		require.NoError(t, err)
		require.Equal(t, Upload{Filename: "pool/main/e/eg/eg_1.0_amd64.deb", SHA256: "abc", Bytes: 8, Staged: true}, out)

		out, err = c.Upload(ctx, deb)
		require.NoError(t, err)
		require.False(t, out.Staged)
	})

	t.Run("should report conflicts and invalid debs", func(t *testing.T) {
		deb := filepath.Join(t.TempDir(), "eg_1.0_amd64.deb")
		require.NoError(t, os.WriteFile(deb, []byte("x"), 0600))

		for _, code := range []int{http.StatusConflict, http.StatusBadRequest} {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))

			_, err := New(srv.Client(), srv.URL+"/repo").Upload(ctx, deb)
			require.True(t, Status(err, code), code)
			srv.Close()
		}
	})

	t.Run("should publish", func(t *testing.T) {
		for body, expected := range map[string]string{`{"vid":"v1"}`: "v1", `{"vid":42}`: "42"} {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, http.MethodPost, r.Method)
				require.Equal(t, "/repo/publish", r.URL.Path)
				fmt.Fprint(w, body)
			}))

			vid, err := New(srv.Client(), srv.URL+"/repo").Publish(ctx)
			require.NoError(t, err)
			require.Equal(t, expected, vid)
			srv.Close()
		}
	})

	t.Run("should retry concurrent publishes with backoff", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if calls.Add(1) <= 2 {
				w.WriteHeader(http.StatusConflict)
				return
			}
			fmt.Fprint(w, `{"vid":"v2"}`)
		}))
		defer srv.Close()

		var waits []int
		vid, err := New(srv.Client(), srv.URL+"/repo").PublishRetry(ctx, 3, func(n int) time.Duration {
			waits = append(waits, n)
			return time.Millisecond
		})
		require.NoError(t, err)
		require.Equal(t, "v2", vid)
		require.Equal(t, []int{0, 1}, waits)
	})

	t.Run("should give up after the retries", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusConflict)
		}))
		defer srv.Close()

		_, err := New(srv.Client(), srv.URL+"/repo").PublishRetry(ctx, 2, func(int) time.Duration { return time.Millisecond })
		require.True(t, Status(err, http.StatusConflict))
		require.Equal(t, int32(3), calls.Load())
	})

	t.Run("should not retry other failures", func(t *testing.T) {
		var calls atomic.Int32
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			w.WriteHeader(http.StatusForbidden)
		}))
		defer srv.Close()

		_, err := New(srv.Client(), srv.URL+"/repo").PublishRetry(ctx, 3, func(int) time.Duration { return time.Millisecond })
		require.True(t, Status(err, http.StatusForbidden))
		require.Equal(t, int32(1), calls.Load())
	})

	t.Run("should double the backoff", func(t *testing.T) {
		b := Backoff(time.Second)
		require.Equal(t, []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}, []time.Duration{b(0), b(1), b(2)})
	})
}
