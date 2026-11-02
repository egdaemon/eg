package compute

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/egdaemon/eg/cmd/cmdidentity"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/stretchr/testify/require"
)

// the commands acting on an enqueued workload resolve the account through the injected identity, an explicit account
// needs no repository.
func TestWorkloadCommandsResolveTheAccountThroughIdentity(t *testing.T) {
	t.Run("watch follows a completed workload", func(t *testing.T) {
		tlsc, _ := enqueueapi(t)
		_, sshkey := uploadrepo(t)

		cmd := watch{SSHKeyPath: sshkey, ID: "w1"}

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, cmdidentity.New(t.Context(), "account", "", nil)))
	})

	t.Run("download logs writes the stored log", func(t *testing.T) {
		tlsc, _ := enqueueapi(t)
		_, sshkey := uploadrepo(t)
		output := filepath.Join(t.TempDir(), "w1.log")

		cmd := downloadlogs{downloadopts{SSHKeyPath: sshkey, ID: "w1", Output: output}}

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, cmdidentity.New(t.Context(), "account", "", nil)))

		b, err := os.ReadFile(output)
		require.NoError(t, err)
		require.Equal(t, "done\n", string(b))
	})

	t.Run("download analytics writes the stored database", func(t *testing.T) {
		tlsc, _ := enqueueapi(t)
		_, sshkey := uploadrepo(t)
		output := filepath.Join(t.TempDir(), "w1.analytics.db")

		cmd := downloadanalytics{downloadopts{SSHKeyPath: sshkey, ID: "w1", Output: output}}

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, cmdidentity.New(t.Context(), "account", "", nil)))

		b, err := os.ReadFile(output)
		require.NoError(t, err)
		require.Equal(t, "analytics", string(b))
	})

	t.Run("a blank account without a repository is rejected", func(t *testing.T) {
		tlsc, _ := enqueueapi(t)
		_, sshkey := uploadrepo(t)

		cmd := watch{SSHKeyPath: sshkey, ID: "w1"}

		err := cmd.Run(&cmdopts.Global{Context: t.Context()}, tlsc, cmdidentity.New(t.Context(), "", "", nil))
		require.ErrorContains(t, err, "a repository is required")
	})
}

func TestCancelRun(t *testing.T) {
	t.Run("posts the cancel for the workload", func(t *testing.T) {
		var (
			m             sync.Mutex
			method, path  string
			authorization string
		)

		json := func(w http.ResponseWriter, v string) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(v))
		}

		mux := http.NewServeMux()
		mux.HandleFunc("/oauth2/ssh/token", func(w http.ResponseWriter, r *http.Request) {
			json(w, `{"access_token":"access","token_type":"BEARER","refresh_token":"refresh","expires_in":3600}`)
		})
		mux.HandleFunc("/authn/s/", func(w http.ResponseWriter, r *http.Request) { json(w, `{}`) })
		mux.HandleFunc("/authn/ssh", func(w http.ResponseWriter, r *http.Request) { json(w, `{"profiles":[{"token":"profile"}]}`) })
		mux.HandleFunc("/authn/current", func(w http.ResponseWriter, r *http.Request) { json(w, `{"token":"session"}`) })
		mux.HandleFunc("/c/authz/", func(w http.ResponseWriter, r *http.Request) {
			json(w, fmt.Sprintf(`{"token":{"expires":%d,"bearer":"bearer"}}`, time.Now().Add(time.Hour).UnixMilli()))
		})
		mux.HandleFunc("/c/q/w1/cancel", func(w http.ResponseWriter, r *http.Request) {
			m.Lock()
			defer m.Unlock()
			method, path, authorization = r.Method, r.URL.Path, r.Header.Get("Authorization")
			json(w, `{}`)
		})

		srv := httptest.NewServer(mux)
		t.Cleanup(srv.Close)

		u, err := url.Parse(srv.URL)
		require.NoError(t, err)

		tlsc := &cmdopts.TLSConfig{Wrap: func(c *http.Client) *http.Client {
			c.Transport = redirect{to: u}
			return c
		}}
		_, sshkey := uploadrepo(t)

		cmd := cancel{SSHKeyPath: sshkey, ID: "w1"}

		require.NoError(t, cmd.Run(&cmdopts.Global{Context: t.Context(), AccountID: "account"}, tlsc, cmdidentity.New(t.Context(), "account", "", nil)))

		m.Lock()
		defer m.Unlock()
		require.Equal(t, http.MethodPost, method)
		require.Equal(t, "/c/q/w1/cancel", path)
		require.Equal(t, "Bearer bearer", authorization)
	})
}
