package runners

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/timex"
)

// ErrWorkloadCompleted is returned when a workload completes before it could be followed.
var ErrWorkloadCompleted = errors.New("workload completed")

// Find retrieves the workload as recorded by the control plane. while the workload is
// running the response carries a log token for streaming its logs from its runner.
func (t PushClient) Find(ctx context.Context, id string) (_ *Enqueued, err error) {
	var (
		resp EnqueuedFindResponse
	)

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/c/q/%s", t.host, url.PathEscape(id)), nil)
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode workload")
	}

	if resp.Enqueued == nil {
		return nil, errorsx.String("find response missing workload")
	}

	return resp.Enqueued, nil
}

// AwaitLogsToken polls the control plane until the workload is running on a runner that
// supports log streaming, returning the workload carrying its log token. it errors when the
// workload completes before a token is issued, or ctx ends.
func (t PushClient) AwaitLogsToken(ctx context.Context, id string, interval time.Duration) (_ *Enqueued, err error) {
	tick := time.NewTicker(interval)
	defer tick.Stop()

	for {
		found, err := t.Find(ctx, id)
		if err != nil {
			return nil, err
		}

		if !stringsx.Blank(found.LogsToken) {
			return found, nil
		}

		// completed workloads no longer issue tokens, an uncompleted one encodes completion as infinity.
		if done, perr := time.Parse(time.RFC3339Nano, found.CompletedAt); perr == nil && done.Before(timex.RFC3339Inf()) {
			return nil, errorsx.Wrapf(ErrWorkloadCompleted, "workload %s completed before its logs could be streamed", id)
		}

		select {
		case <-tick.C:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}

// HistoryLogs writes the stored log of a completed workload into dst. a status error
// of http.StatusNotFound means the log isn't recorded (yet).
func (t PushClient) HistoryLogs(ctx context.Context, id string, dst io.Writer) (err error) {
	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/c/history/%s/logs", t.host, url.PathEscape(id)), nil)
	if err != nil {
		return err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return err
	}

	_, err = io.Copy(dst, httpresp.Body)
	return errorsx.Wrap(err, "unable to read stored logs")
}

// HistoryFind retrieves the record of a completed workload, including its result.
// a status error of http.StatusNotFound means the record isn't written (yet).
func (t PushClient) HistoryFind(ctx context.Context, id string) (_ *compute.History, err error) {
	var (
		resp compute.HistoryFindResponse
	)

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/c/history/%s", t.host, url.PathEscape(id)), nil)
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode history")
	}

	if resp.History == nil {
		return nil, errorsx.String("history response missing record")
	}

	return resp.History, nil
}
