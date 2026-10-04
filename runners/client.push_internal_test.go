package runners

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPushClientCandidates(t *testing.T) {
	t.Run("requests candidates matching the workload requirements", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/c/q/candidates", r.URL.Path)
			require.Equal(t, "4", r.FormValue("cores"))
			require.Equal(t, "8", r.FormValue("vram"))
			require.Equal(t, "true", r.FormValue("allow_shared"))
			require.Equal(t, "https://example.com/repo.git", r.FormValue("vcs_uri"))
			require.Equal(t, "3", r.FormValue("limit"))

			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"items":[{"id":"runner-1","p2pid":"peer-1"}]}`))
		}))
		defer srv.Close()

		resp, err := PushClient{c: srv.Client(), host: srv.URL}.Candidates(t.Context(), &Enqueued{
			Cores:       4,
			Vram:        8,
			AllowShared: true,
			VcsUri:      "https://example.com/repo.git",
		}, 3)
		require.NoError(t, err)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "runner-1", resp.Items[0].Id)
		require.Equal(t, "peer-1", resp.Items[0].P2Pid)
	})
}
