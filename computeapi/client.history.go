package computeapi

import (
	"bufio"
	"context"
	"fmt"
	"github.com/egdaemon/eg/jsonx"
	"io"
	"net/http"
	"net/url"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/libp2p/go-libp2p/core/host"
)

// runnerget performs a GET against the runner the log token was issued for, over the p2p network.
// the caller owns closing the response.
func runnerget(ctx context.Context, p2p host.Host, token string, path string) (_ *http.Response, closer func(), err error) {
	_, p2pid := tokenClaims(token)
	if p2pid == "" {
		return nil, nil, errorsx.String("log token missing runner")
	}

	stream, err := dialRunner(ctx, p2p, p2pid, token)
	if err != nil {
		return nil, nil, err
	}

	stop := context.AfterFunc(ctx, func() { _ = stream.Reset() })
	closer = func() {
		stop()
		errorsx.Log(stream.Close())
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://runner"+path, nil)
	if err != nil {
		closer()
		return nil, nil, err
	}
	httpreq.Header.Set("Authorization", token)

	if err = httpreq.Write(stream); err != nil {
		closer()
		return nil, nil, fmt.Errorf("unable to write request over stream: %w", err)
	}

	resp, err := httpx.AsError(http.ReadResponse(bufio.NewReader(stream), httpreq))
	if err != nil {
		errorsx.Log(httpx.AutoClose(resp))
		closer()
		return nil, nil, err
	}

	// closer is a named result, capture the stream's closer or the returned function calls itself.
	closestream := closer

	return resp, func() {
		errorsx.Log(httpx.AutoClose(resp))
		closestream()
	}, nil
}

// RunnerHistoryFind retrieves the record of a workload from the runner that ran it. a status error of
// http.StatusNotFound means the runner hasn't recorded it (yet), or the record expired.
func RunnerHistoryFind(ctx context.Context, p2p host.Host, token string, id string) (_ *compute.History, err error) {
	var (
		out compute.HistoryFindResponse
	)

	resp, closer, err := runnerget(ctx, p2p, token, fmt.Sprintf("/c/history/%s", url.PathEscape(id)))
	if err != nil {
		return nil, err
	}
	defer closer()

	if err = jsonx.UnmarshalRead(resp.Body, &out); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode history")
	}

	if out.History == nil {
		return nil, errorsx.String("history response missing record")
	}

	return out.History, nil
}

// RunnerHistoryLogs writes the recorded log of a workload from the runner that ran it into dst. a status error of
// http.StatusNotFound means the runner hasn't recorded it (yet), or the record expired.
func RunnerHistoryLogs(ctx context.Context, p2p host.Host, token string, id string, dst io.Writer) (err error) {
	resp, closer, err := runnerget(ctx, p2p, token, fmt.Sprintf("/c/history/%s/logs", url.PathEscape(id)))
	if err != nil {
		return err
	}
	defer closer()

	_, err = io.Copy(dst, resp.Body)
	return errorsx.Wrap(err, "unable to read stored logs")
}
