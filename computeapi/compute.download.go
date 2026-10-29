package computeapi

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/libp2p/go-libp2p/core/host"
)

// how often the cluster is polled for the workload to be scheduled on a runner.
const downloadpollinterval = 2 * time.Second

// DownloadLogs writes the logs of a workload enqueued with the cluster into dst. it waits for the cluster to
// schedule the workload on a runner, then streams the logs from that runner until the workload completes.
// workloads that already completed are served from the stored logs.
func DownloadLogs(ctx context.Context, chttp *http.Client, id string, dst io.Writer) (err error) {
	log.Println("waiting for the workload to start", id)
	found, err := NewPushClient(chttp).AwaitLogsToken(ctx, id, downloadpollinterval)
	if errors.Is(err, ErrWorkloadCompleted) {
		return computex.Logs(ctx, id, dst, NewPushClient(chttp).HistoryLogs)
	} else if err != nil {
		return errorsx.Wrap(err, "unable to download workload logs")
	}

	p2p, err := dialp2p(ctx, chttp)
	if err != nil {
		return err
	}
	defer p2p.Close()

	return errorsx.Wrap(FollowLogs(ctx, p2p, found.LogsToken, found.Id, 0, dst), "unable to download workload logs")
}

// DownloadAnalytics writes the analytics database of a workload enqueued with the cluster into dst, downloaded
// directly from the runner. it waits for the cluster to schedule the workload on a runner. a running workload
// is snapshotted, one that completed in the meantime is served from the runner's history, and one that completed
// earlier is served from the stored analytics of the cluster.
func DownloadAnalytics(ctx context.Context, chttp *http.Client, id string, dst io.Writer) (err error) {
	log.Println("waiting for the workload to start", id)
	found, err := NewPushClient(chttp).AwaitLogsToken(ctx, id, downloadpollinterval)
	if errors.Is(err, ErrWorkloadCompleted) {
		return errorsx.Wrap(NewPushClient(chttp).HistoryAnalytics(ctx, id, dst), "stored workload analytics unavailable")
	} else if err != nil {
		return errorsx.Wrap(err, "unable to locate workload on the cluster")
	}

	p2p, err := dialp2p(ctx, chttp)
	if err != nil {
		return err
	}
	defer p2p.Close()

	err = RunnerAnalytics(ctx, p2p, found.LogsToken, found.Id, dst)
	if httpx.IsStatusError(err, http.StatusNotFound) != nil {
		// no longer running, the runner keeps the analytics of workloads it completed for a while.
		return errorsx.Wrapf(RunnerHistoryAnalytics(ctx, p2p, found.LogsToken, found.Id, dst), "runner has no recorded analytics for workload (/c/history/%s/analytics)", found.Id)
	}

	return errorsx.Wrap(err, "unable to snapshot running workload analytics")
}

// dialp2p joins the p2p network through the bootstrap peers of the cluster.
func dialp2p(ctx context.Context, chttp *http.Client) (_ host.Host, err error) {
	meta, err := registration.NewPingClient(chttp).Meta(ctx)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve p2p bootstrap addresses")
	}

	p2p, err := libp2px.NewClient(ctx, libp2px.StringsToPeers(meta.Bootstrap...)...)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to join p2p network")
	}

	return p2p, nil
}
