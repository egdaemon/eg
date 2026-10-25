package schedules_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/schedules"
	"github.com/stretchr/testify/require"
)

// newclient a client whose requests, whatever their host, are served by h.
func newclient(t *testing.T, h http.HandlerFunc) *http.Client {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return &http.Client{Transport: httpx.RewriteHostTransport(errorsx.Must(url.Parse(srv.URL)), nil)}
}

func TestCreate(t *testing.T) {
	t.Run("should post the schedule using the control plane's json names", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Schedule struct {
					Cron  string `json:"cron"`
					Entry string `json:"entrypoint"`
					TTL   string `json:"ttl"`
				} `json:"schedule"`
			}

			require.Equal(t, http.MethodPost, r.Method)
			require.Equal(t, "/r/schedule/", r.URL.Path)
			require.Equal(t, "application/json", r.Header.Get("Content-Type"))
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			require.Equal(t, "1d", body.Schedule.Cron)
			require.Equal(t, "tests/stress", body.Schedule.Entry)
			require.Equal(t, "3600000", body.Schedule.TTL)
			fmt.Fprint(w, `{"schedule":{"id":"s1","cron":"1d","entrypoint":"tests/stress","nextrun_at":"2026-10-17T00:00:00Z"}}`)
		})

		s, err := schedules.Create(t.Context(), c, &schedules.Schedule{Cron: "1d", Entry: "tests/stress", Ttl: 3600000})
		require.NoError(t, err)
		require.Equal(t, "s1", s.GetId())
		require.Equal(t, "2026-10-17T00:00:00Z", s.GetNextrunAt())
	})

	t.Run("should discard fields it does not know", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, `{"schedule":{"id":"s1","cid":"00000000-0000-0000-0000-000000000000"}}`)
		})

		s, err := schedules.Create(t.Context(), c, &schedules.Schedule{})
		require.NoError(t, err)
		require.Equal(t, "s1", s.GetId())
	})

	t.Run("should return the status of a failed request", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusBadRequest) })

		_, err := schedules.Create(t.Context(), c, &schedules.Schedule{Cron: "0 3 * * *"})
		require.Error(t, err)
		require.NotNil(t, httpx.IsStatusError(err, http.StatusBadRequest))
	})
}

func TestSearch(t *testing.T) {
	t.Run("should encode the request as query parameters", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodGet, r.Method)
			require.Equal(t, "/r/schedule/", r.URL.Path)
			require.Equal(t, "git@github.com:egdaemon/eg.git", r.URL.Query().Get("vcs_uri"))
			require.Equal(t, "nightly", r.URL.Query().Get("query"))
			require.Equal(t, "2", r.URL.Query().Get("offset"))
			require.Equal(t, "50", r.URL.Query().Get("limit"))
			fmt.Fprint(w, `{"next":{"offset":"3","limit":"50"},"items":[{"id":"s1","cron":"1d","entrypoint":"tests/stress","cores":"8"}]}`)
		})

		page, err := schedules.Search(t.Context(), c, &schedules.ScheduleSearchRequest{
			Query:  "nightly",
			VcsUri: "git@github.com:egdaemon/eg.git",
			Offset: 2,
			Limit:  50,
		})
		require.NoError(t, err)
		require.Equal(t, uint64(3), page.GetNext().GetOffset())
		require.Len(t, page.GetItems(), 1)
		require.Equal(t, "1d", page.GetItems()[0].GetCron())
		require.Equal(t, "tests/stress", page.GetItems()[0].GetEntry())
		require.Equal(t, uint64(8), page.GetItems()[0].GetCores())
	})

	t.Run("should return the status of a failed request", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })

		_, err := schedules.Search(t.Context(), c, &schedules.ScheduleSearchRequest{})
		require.NotNil(t, httpx.IsStatusError(err, http.StatusForbidden))
	})
}

func TestFind(t *testing.T) {
	// pages mimics the control plane's pagination: a full page advances the offset, a partial page resets it.
	pages := func(t *testing.T, limit int, ids ...string) *http.Client {
		return newclient(t, func(w http.ResponseWriter, r *http.Request) {
			resp := schedules.ScheduleSearchResponse{Next: &schedules.ScheduleSearchRequest{Limit: uint64(limit)}}
			offset := errorsx.Must(strconv.Atoi(r.URL.Query().Get("offset")))

			for _, id := range ids[min(offset*limit, len(ids)):min((offset+1)*limit, len(ids))] {
				resp.Items = append(resp.Items, &schedules.Schedule{Id: id, VcsUri: r.URL.Query().Get("vcs_uri")})
			}

			resp.Next.Offset = uint64(offset) + 1
			if len(resp.Items) < limit {
				resp.Next.Offset = 0
			}

			require.NoError(t, json.NewEncoder(w).Encode(&resp))
		})
	}

	t.Run("should locate a schedule on the first page", func(t *testing.T) {
		s, err := schedules.Find(t.Context(), pages(t, 100, "s1", "s2"), "git@github.com:egdaemon/eg.git", "s2")
		require.NoError(t, err)
		require.Equal(t, "s2", s.GetId())
		require.Equal(t, "git@github.com:egdaemon/eg.git", s.GetVcsUri())
	})

	t.Run("should locate a schedule on a later page", func(t *testing.T) {
		s, err := schedules.Find(t.Context(), pages(t, 2, "s1", "s2", "s3", "s4", "s5"), "", "s5")
		require.NoError(t, err)
		require.Equal(t, "s5", s.GetId())
	})

	t.Run("should return nil when the schedule does not exist", func(t *testing.T) {
		s, err := schedules.Find(t.Context(), pages(t, 2, "s1", "s2", "s3", "s4"), "", "missing")
		require.NoError(t, err)
		require.Nil(t, s)
	})

	t.Run("should return nil when there are no schedules", func(t *testing.T) {
		s, err := schedules.Find(t.Context(), pages(t, 100), "", "missing")
		require.NoError(t, err)
		require.Nil(t, s)
	})

	t.Run("should return the status of a failed search", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })

		_, err := schedules.Find(t.Context(), c, "", "s1")
		require.NotNil(t, httpx.IsStatusError(err, http.StatusInternalServerError))
	})
}

func TestUpdate(t *testing.T) {
	t.Run("should patch the full schedule by id", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			var req schedules.ScheduleUpdateRequest

			require.Equal(t, http.MethodPatch, r.Method)
			require.Equal(t, "/r/schedule/s1", r.URL.Path)
			require.NoError(t, json.NewDecoder(r.Body).Decode(&req))
			require.Equal(t, "6h", req.GetSchedule().GetCron())
			require.Equal(t, "2026-10-16T00:00:00Z", req.GetSchedule().GetLastrunAt())
			require.NoError(t, json.NewEncoder(w).Encode(&schedules.ScheduleUpdateResponse{Schedule: req.GetSchedule()}))
		})

		s, err := schedules.Update(t.Context(), c, "s1", &schedules.Schedule{Id: "s1", Cron: "6h", LastrunAt: "2026-10-16T00:00:00Z"})
		require.NoError(t, err)
		require.Equal(t, "6h", s.GetCron())
	})

	t.Run("should escape the id", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "/r/schedule/a%2Fb", r.URL.EscapedPath())
			fmt.Fprint(w, `{"schedule":{}}`)
		})

		_, err := schedules.Update(t.Context(), c, "a/b", &schedules.Schedule{})
		require.NoError(t, err)
	})

	t.Run("should return the status of a failed request", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden) })

		_, err := schedules.Update(t.Context(), c, "s1", &schedules.Schedule{})
		require.NotNil(t, httpx.IsStatusError(err, http.StatusForbidden))
	})
}

func TestRemove(t *testing.T) {
	t.Run("should delete the schedule by id", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, http.MethodDelete, r.Method)
			require.Equal(t, "/r/schedule/s1", r.URL.Path)
			b, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.Empty(t, b)
			fmt.Fprint(w, `{"schedule":{"id":"s1"}}`)
		})

		require.NoError(t, schedules.Remove(t.Context(), c, "s1"))
	})

	t.Run("should return the status of a failed request", func(t *testing.T) {
		c := newclient(t, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusInternalServerError) })

		err := schedules.Remove(t.Context(), c, "s1")
		require.NotNil(t, httpx.IsStatusError(err, http.StatusInternalServerError))
	})
}
