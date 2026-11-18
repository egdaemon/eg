package compute

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/computeapi"
	"github.com/stretchr/testify/require"
)

func TestHistoryList(t *testing.T) {
	t.Run("retrieves a single page only", func(t *testing.T) {
		var requests int
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests++
			require.Equal(t, "build", r.URL.Query().Get("query"))
			require.Equal(t, "https://example.com/r.git", r.URL.Query().Get("vcs_uri"))
			require.Equal(t, "0", r.URL.Query().Get("offset"))
			require.Equal(t, "20", r.URL.Query().Get("limit"))
			_, _ = w.Write([]byte(`{"next":{"offset":1,"limit":20},"items":[{"id":"a","result":2}]}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		items, err := historyCmd{VCS: cmdopts.VCS{VcsURI: "https://example.com/r.git"}, Query: "build", Limit: 20}.list(t.Context(), computeapi.NewWorkloadClient(srv.Client()))
		require.NoError(t, err)
		require.Equal(t, 1, requests)
		require.Len(t, items, 1)
		require.Equal(t, "a", items[0].GetId())
	})
}

func TestRenderHistory(t *testing.T) {
	t.Run("prints a row per record with its result", func(t *testing.T) {
		out := ansi.Strip(renderHistory(
			&compute.History{Id: "a", Description: "built", Os: "linux", Arch: "amd64", VcsUri: "https://example.com/r.git", VcsCommit: "0123456789abcdef", Result: 2, Duration: int64(90 * time.Second), InitiatedAt: "garbage", CompletedAt: "garbage"},
			&compute.History{Id: "b", Result: 1},
			&compute.History{Id: "c", Result: 3},
		))

		require.Contains(t, out, "workloads: 3")
		require.Contains(t, out, "QUEUED")
		require.Contains(t, out, "ELAPSED")
		require.Regexp(t, `│ ✔\s+│\s+│ 1m30s\s+│ a\s+│ built\s+│ linux/amd64\s+│ r.git@01234567\s+│\s+│ garbage\s+│ garbage\s+│`, out)
		require.Regexp(t, `│ ✖\s+│\s+│ 0s\s+│ b\s+│`, out)
		require.Regexp(t, `│ ⊘\s+│\s+│ 0s\s+│ c\s+│`, out)
	})
}

func TestTypographyHistoryResult(t *testing.T) {
	require.Equal(t, "?", fmt.Sprintf("%v", TypographyHistoryResult(&compute.History{})))
	require.Equal(t, "✖", fmt.Sprintf("%v", TypographyHistoryResult(&compute.History{Result: 1})))
	require.Equal(t, "✔", fmt.Sprintf("%v", TypographyHistoryResult(&compute.History{Result: 2})))
	require.Equal(t, "⊘", fmt.Sprintf("%v", TypographyHistoryResult(&compute.History{Result: 3})))
}

func TestTypographyHistoryQueued(t *testing.T) {
	require.Equal(t, "", fmt.Sprintf("%v", TypographyHistoryQueued(&compute.History{CreatedAt: "2026-01-02T03:04:05Z"})))
	require.Equal(t, "5s", fmt.Sprintf("%v", TypographyHistoryQueued(&compute.History{CreatedAt: "2026-01-02T03:04:05Z", InitiatedAt: "2026-01-02T03:04:10Z"})))
}

func TestTypographyVCSCommit(t *testing.T) {
	require.Equal(t, "retrovibed/retrovibed.git@59d8af54", fmt.Sprintf("%v", TypographyVCSCommit("git@github.com:retrovibed/retrovibed.git", "59d8af54ffffffff")))
	require.Equal(t, "retrovibed/retrovibed.git@59d8af54", fmt.Sprintf("%v", TypographyVCSCommit("https://github.com/retrovibed/retrovibed.git", "59d8af54")))
	require.Equal(t, "retrovibed/retrovibed.git@59d8af54", fmt.Sprintf("%v", TypographyVCSCommit("ssh://git@github.com/retrovibed/retrovibed.git", "59d8af54")))
	require.Equal(t, "59d8af54", fmt.Sprintf("%v", TypographyVCSCommit("", "59d8af54")))
	require.Equal(t, "a/b.git", fmt.Sprintf("%v", TypographyVCSCommit("git@h:a/b.git", "")))
}

func TestTypographyDuration(t *testing.T) {
	require.Equal(t, "1m2.345s", fmt.Sprintf("%v", TypographyDuration(62*time.Second+345*time.Millisecond+400*time.Microsecond)))
}
