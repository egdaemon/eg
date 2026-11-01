package compute

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/stretchr/testify/require"
)

func TestPubList(t *testing.T) {
	t.Run("follows the pages and prints every runner", func(t *testing.T) {
		var offsets []string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			offsets = append(offsets, r.URL.Query().Get("offset"))
			require.Equal(t, "gpu", r.URL.Query().Get("query"))
			if r.URL.Query().Get("offset") == "0" {
				_, _ = w.Write([]byte(`{"next":{"offset":"1","limit":"100"},"items":[{"id":"a","description":"first","os":"linux","arch":"amd64","cores":"4","memory":"2147483648","labels":["x","y"],"global":true,"updated_at":"2026-01-02T03:04:05Z"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"next":{"offset":"0","limit":"100"},"items":[{"id":"b","description":"second","os":"linux","arch":"arm64"}]}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		runners, err := pubList{Query: "gpu"}.list(t.Context(), computeapi.NewPushClient(srv.Client()))
		require.NoError(t, err)
		require.Equal(t, []string{"0", "1"}, offsets)
		require.Len(t, runners, 2)
		require.Equal(t, "a", runners[0].Id)
		require.Equal(t, "b", runners[1].Id)
	})
}

func TestRender(t *testing.T) {
	runners := []*compute.Compute{
		{Id: "0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b", Description: "box", Os: "linux", Arch: "amd64", Cores: 4, Memory: 2147483648, Labels: []string{"x", "y"}, Global: true, UpdatedAt: "garbage"},
		{Id: "1190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b", Os: "linux", Arch: "arm64", UpdatedAt: "garbage"},
	}

	t.Run("should mark the runners shared with other accounts", func(t *testing.T) {
		out := ansi.Strip(render(runners...))

		require.Regexp(t, `^runners: 2\n`, out)
		require.Regexp(t, `\* +│ 0190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b - box +│ linux/amd64 +│ 4 +│ 2.00GiB +│ 0B +│ x,y +│ garbage`, out)
		require.Regexp(t, `  +│ 1190a1b2-c3d4-7e5f-8a9b-0c1d2e3f4a5b +│ linux/arm64 +│ 0 +│ 0B +│ 0B +│ +│ garbage`, out)
	})

	t.Run("should render the headers when there are no runners", func(t *testing.T) {
		out := ansi.Strip(render())

		require.Contains(t, out, "runners: 0")
		require.Contains(t, out, "RUNNER")
	})
}

func TestPubRemove(t *testing.T) {
	t.Run("removes every requested runner and reports the failures", func(t *testing.T) {
		var deleted []string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			deleted = append(deleted, r.URL.Path)
			if r.URL.Path == "/c/runners/bad" {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		err := pubRemove{IDs: []string{"bad", "good"}}.remove(t.Context(), computeapi.NewPushClient(srv.Client()))
		require.ErrorContains(t, err, "bad")
		require.Equal(t, []string{"/c/runners/bad", "/c/runners/good"}, deleted)
	})
}
