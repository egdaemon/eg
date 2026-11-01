package computeapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egdaemon/eg/compute"
	"github.com/stretchr/testify/require"
)

func TestPushClientRunnersSearch(t *testing.T) {
	t.Run("sends the filters and decodes the page", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/c/runners/", r.URL.Path)
			require.Equal(t, "gpu", r.URL.Query().Get("query"))
			require.Equal(t, "arm64", r.URL.Query().Get("arch"))
			require.Equal(t, "4", r.URL.Query().Get("cores"))
			require.Equal(t, "2", r.URL.Query().Get("offset"))
			require.Equal(t, "10", r.URL.Query().Get("limit"))
			require.False(t, r.URL.Query().Has("global"))
			_, _ = w.Write([]byte(`{"next":{"offset":"3","limit":"10"},"items":[{"id":"abc","description":"box","global":true}]}`))
		}))
		defer srv.Close()

		resp, err := PushClient{c: srv.Client(), host: srv.URL}.RunnersSearch(t.Context(), &compute.SearchRequest{Query: "gpu", Arch: "arm64", Cores: 4, Offset: 2, Limit: 10})
		require.NoError(t, err)
		require.Equal(t, uint64(3), resp.Next.Offset)
		require.Len(t, resp.Items, 1)
		require.Equal(t, "abc", resp.Items[0].Id)
		require.True(t, resp.Items[0].Global)
	})
}

func TestPushClientRunnersDestroy(t *testing.T) {
	t.Run("deletes the runner", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/c/runners/abc", r.URL.Path)
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		require.NoError(t, PushClient{c: srv.Client(), host: srv.URL}.RunnersDestroy(t.Context(), "abc"))
	})

	t.Run("a failure is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		require.Error(t, PushClient{c: srv.Client(), host: srv.URL}.RunnersDestroy(t.Context(), "abc"))
	})
}
