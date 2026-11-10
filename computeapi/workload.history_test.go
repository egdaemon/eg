package computeapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/stretchr/testify/require"
)

func TestWorkloadClientHistorySearch(t *testing.T) {
	t.Run("sends the filters and decodes the page", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/c/history/", r.URL.Path)
			require.Equal(t, "build", r.URL.Query().Get("query"))
			require.Equal(t, "2", r.URL.Query().Get("offset"))
			require.Equal(t, "10", r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{"next":{"offset":3,"limit":10},"items":[{"id":"abc","description":"box","duration":"2000000000","result":2,"uploaded_by":"me"}]}`))
		}))
		defer srv.Close()

		resp, err := WorkloadClient{c: srv.Client(), host: srv.URL}.HistorySearch(t.Context(), &compute.SearchHistoryRequest{Query: "build", Offset: 2, Limit: 10})
		require.NoError(t, err)
		require.Equal(t, uint64(3), resp.GetNext().GetOffset())
		require.Len(t, resp.GetItems(), 1)
		require.Equal(t, "abc", resp.GetItems()[0].GetId())
		require.Equal(t, int64(2*time.Second), resp.GetItems()[0].GetDuration())
		require.Equal(t, int32(2), resp.GetItems()[0].GetResult())
	})

	t.Run("decodes the duration as egmeta emits it", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(`{"next":{"offset":0,"limit":100},"items":[{"id":"abc","duration":2000000000}]}`))
		}))
		defer srv.Close()

		resp, err := WorkloadClient{c: srv.Client(), host: srv.URL}.HistorySearch(t.Context(), &compute.SearchHistoryRequest{})
		require.NoError(t, err)
		require.Equal(t, int64(2*time.Second), resp.GetItems()[0].GetDuration())
	})
}
