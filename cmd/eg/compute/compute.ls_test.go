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
	"github.com/egdaemon/eg/cmd/cmdtypography"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/internal/timex"
	"github.com/stretchr/testify/require"
)

func TestLsList(t *testing.T) {
	t.Run("follows the pages and returns every workload", func(t *testing.T) {
		var offsets []string
		srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			offsets = append(offsets, r.URL.Query().Get("offset"))
			require.Equal(t, "build", r.URL.Query().Get("query"))
			require.Equal(t, "https://example.com/r.git", r.URL.Query().Get("vcs_uri"))
			if r.URL.Query().Get("offset") == "0" {
				_, _ = w.Write([]byte(`{"next":{"offset":"1","limit":"100"},"items":[{"id":"a","description":"first"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"next":{"offset":"0","limit":"100"},"items":[{"id":"b"}]}`))
		}))
		defer srv.Close()

		tr := srv.Client().Transport.(*http.Transport)
		tr.TLSClientConfig.InsecureSkipVerify = true
		tr.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
		}

		items, err := lsCmd{VCS: cmdopts.VCS{VcsURI: "https://example.com/r.git"}, Query: "build", Limit: 100}.list(t.Context(), computeapi.NewWorkloadClient(srv.Client()))
		require.NoError(t, err)
		require.Equal(t, []string{"0", "1"}, offsets)
		require.Len(t, items, 2)
		require.Equal(t, "a", items[0].GetId())
		require.Equal(t, "b", items[1].GetId())
	})
}

func TestRenderWorkloads(t *testing.T) {
	t.Run("prints a row per workload with its state", func(t *testing.T) {
		out := ansi.Strip(renderWorkloads(
			&computeapi.Enqueued{Id: "a", Description: "queued one", Os: "linux", Arch: "amd64", VcsUri: "https://example.com/r.git", VcsCommit: "0123456789abcdef", CreatedAt: "garbage"},
			&computeapi.Enqueued{Id: "b", Os: "linux", Arch: "arm64", InitiatedAt: "2026-01-02T03:04:05Z"},
		))

		require.Contains(t, out, "workloads: 2")
		require.Regexp(t, `│ ⏸\s+│ \s*│ \s*│ a - queued one\s+│ linux/amd64\s+│ https://example.com/r.git\s+│ 01234567\s+│ garbage\s+│`, out)
		require.Regexp(t, `│ ▶\s+│\s+│ \S+\s+│ b\s+│ linux/arm64\s+│`, out)
	})

	t.Run("no workloads still prints the header", func(t *testing.T) {
		out := ansi.Strip(renderWorkloads())
		require.Contains(t, out, "workloads: 0")
		require.Contains(t, out, "WORKLOAD")
	})
}

func TestTypographyWorkloadState(t *testing.T) {
	require.Equal(t, "⏸", fmt.Sprintf("%v", TypographyWorkloadState(&computeapi.Enqueued{})))
	require.Equal(t, "▶", fmt.Sprintf("%v", TypographyWorkloadState(&computeapi.Enqueued{InitiatedAt: "2026-01-02T03:04:05Z"})))
	require.Equal(t, "✔", fmt.Sprintf("%v", TypographyWorkloadState(&computeapi.Enqueued{InitiatedAt: "2026-01-02T03:04:05Z", CompletedAt: "2026-01-02T03:04:06Z"})))
	require.Equal(t, "▶ |", fmt.Sprintf("%-2v|", TypographyWorkloadState(&computeapi.Enqueued{InitiatedAt: "2026-01-02T03:04:05Z"})))
}

func TestTypographyWorkloadQueued(t *testing.T) {
	now := time.Now()
	require.Equal(t, "", fmt.Sprintf("%v", TypographyWorkloadQueued(&computeapi.Enqueued{CreatedAt: "garbage"})))
	require.Equal(t, "1m0s", fmt.Sprintf("%v", TypographyWorkloadQueued(&computeapi.Enqueued{CreatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano)})))
	require.Equal(t, "30s", fmt.Sprintf("%v", TypographyWorkloadQueued(&computeapi.Enqueued{CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), InitiatedAt: now.Add(-time.Hour + 30*time.Second).Format(time.RFC3339Nano)})))
}

func TestTypographyWorkloadRunning(t *testing.T) {
	now := time.Now()
	require.Equal(t, "", fmt.Sprintf("%v", TypographyWorkloadRunning(&computeapi.Enqueued{CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano)})))
	require.Equal(t, "30s", fmt.Sprintf("%v", TypographyWorkloadRunning(&computeapi.Enqueued{InitiatedAt: now.Add(-30 * time.Second).Format(time.RFC3339Nano)})))
	require.Equal(t, "10s", fmt.Sprintf("%v", TypographyWorkloadRunning(&computeapi.Enqueued{InitiatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), CompletedAt: now.Add(-time.Hour + 10*time.Second).Format(time.RFC3339Nano)})))
}

func TestWorkloadInfinityTimestamps(t *testing.T) {
	now := time.Now()
	inf := timex.RFC3339Inf().Format(time.RFC3339Nano)
	w := &computeapi.Enqueued{CreatedAt: now.Add(-time.Minute).Format(time.RFC3339Nano), InitiatedAt: inf, CompletedAt: inf}
	require.Equal(t, "⏸", fmt.Sprintf("%v", TypographyWorkloadState(w)))
	require.Equal(t, "1m0s", fmt.Sprintf("%v", TypographyWorkloadQueued(w)))
	require.Equal(t, "", fmt.Sprintf("%v", TypographyWorkloadRunning(w)))
	require.Equal(t, "", fmt.Sprintf("%v", cmdtypography.TimestampRFC3339(inf).Zero("").Infinity("", "")))

	w = &computeapi.Enqueued{CreatedAt: now.Add(-time.Hour).Format(time.RFC3339Nano), InitiatedAt: now.Add(-30 * time.Second).Format(time.RFC3339Nano), CompletedAt: inf}
	require.Equal(t, "▶", fmt.Sprintf("%v", TypographyWorkloadState(w)))
	require.Equal(t, "30s", fmt.Sprintf("%v", TypographyWorkloadRunning(w)))
}
