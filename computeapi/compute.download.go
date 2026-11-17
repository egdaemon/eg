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
// workloads that already completed are read from the runner that executed them, falling back to the stored logs.
func DownloadLogs(ctx context.Context, chttp *http.Client, id string, dst io.Writer) (err error) {
	log.Println("waiting for the workload to start", id)
	found, err := NewPushClient(chttp).AwaitLogsToken(ctx, id, downloadpollinterval)
	if errors.Is(err, ErrWorkloadCompleted) {
		return downloadCompleted(ctx, chttp, id, dst, RunnerHistoryLogs, func(ctx context.Context, id string, dst io.Writer) error {
			return computex.Logs(ctx, id, dst, NewPushClient(chttp).HistoryLogs)
		})
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
// earlier is read from its runner first, falling back to the stored analytics of the cluster.
func DownloadAnalytics(ctx context.Context, chttp *http.Client, id string, dst io.Writer) (err error) {
	log.Println("waiting for the workload to start", id)
	found, err := NewPushClient(chttp).AwaitLogsToken(ctx, id, downloadpollinterval)
	if errors.Is(err, ErrWorkloadCompleted) {
		return downloadCompleted(ctx, chttp, id, dst, RunnerHistoryAnalytics, NewPushClient(chttp).HistoryAnalytics)
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

// downloadCompleted retrieves the data of a workload that already completed. the runner that executed it
// is always attempted first, located through the p2p id and log token of the history record. when the runner
// can't be located or no longer has the data, the copy stored by the cluster (stored) is used instead.
func downloadCompleted(
	ctx context.Context,
	chttp *http.Client,
	id string,
	dst io.Writer,
	fetch func(ctx context.Context, p2p host.Host, token string, id string, dst io.Writer) error,
	stored func(ctx context.Context, id string, dst io.Writer) error,
) (err error) {
	rerr := downloadFromRunner(ctx, chttp, id, dst, fetch)
	if rerr == nil {
		return nil
	}

	if ctx.Err() != nil {
		return ctx.Err()
	}

	log.Println("unable to retrieve from the runner, using the stored copy:", rerr)

	if err = stored(ctx, id, dst); err != nil {
		return errorsx.Wrapf(err, "stored workload data unavailable and the runner failed: %v", rerr)
	}

	return nil
}

// downloadFromRunner resolves the runner of a completed workload from its history record (p2p id
// and log token) and reads the recorded data directly from it.
func downloadFromRunner(
	ctx context.Context,
	chttp *http.Client,
	id string,
	dst io.Writer,
	fetch func(ctx context.Context, p2p host.Host, token string, id string, dst io.Writer) error,
) (err error) {
	found, err := NewPushClient(chttp).HistoryFindResponse(ctx, id)
	if err != nil {
		return errorsx.Wrap(err, "unable to load the history record")
	}

	if found.History.P2PId == "" || found.LogsToken == "" {
		return errorsx.String("the history has no runner to retrieve from")
	}

	p2p, err := dialp2p(ctx, chttp)
	if err != nil {
		return err
	}
	defer p2p.Close()

	return errorsx.Wrapf(fetch(ctx, p2p, found.LogsToken, id, dst), "runner %s has no recorded data for workload", found.History.P2PId)
}
