package computeapi

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWorkloadClientSearch(t *testing.T) {
	t.Run("sends the filters and decodes the page", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/c/q/", r.URL.Path)
			require.Equal(t, "build", r.URL.Query().Get("query"))
			require.Equal(t, "arm64", r.URL.Query().Get("arch"))
			require.Equal(t, "2", r.URL.Query().Get("offset"))
			require.Equal(t, "10", r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{"next":{"offset":"3","limit":"10"},"items":[{"id":"abc","description":"box"}]}`))
		}))
		defer srv.Close()

		resp, err := WorkloadClient{c: srv.Client(), host: srv.URL}.Search(t.Context(), &EnqueuedSearchRequest{Query: "build", Arch: "arm64", Offset: 2, Limit: 10})
		require.NoError(t, err)
		require.Equal(t, uint64(3), resp.GetNext().GetOffset())
		require.Len(t, resp.GetItems(), 1)
		require.Equal(t, "abc", resp.GetItems()[0].GetId())
	})

	t.Run("a failure is an error", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer srv.Close()

		_, err := WorkloadClient{c: srv.Client(), host: srv.URL}.Search(t.Context(), &EnqueuedSearchRequest{})
		require.Error(t, err)
	})
}
