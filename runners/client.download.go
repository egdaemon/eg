package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"runtime"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/gofrs/uuid/v5"
)

// Downloads work from control plane.
func NewDownloadClient(c *http.Client) *DownloadClient {
	return &DownloadClient{
		c:    c,
		host: eg.EnvAPIHostDefault(),
	}
}

type DownloadClient struct {
	c    *http.Client
	host string
}

func (t DownloadClient) Download(ctx context.Context, workload *EnqueuedDequeueResponse) (err error) {
	var (
		encoded []byte
		req     *http.Request
	)

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/c/q/%s/download", t.host, workload.Enqueued.Id), bytes.NewReader(encoded))
	if err != nil {
		return err
	}

	download, err := httpx.AsError(t.c.Do(req))
	defer func() { errorsx.Log(httpx.AutoClose(download)) }()
	if err != nil {
		return err
	}

	dirs := DefaultSpoolDirs()

	uid, err := uuid.FromString(workload.Enqueued.Id)
	if err != nil {
		return errorsx.Wrap(err, "invalid enqueued downloaded, malformed uid")
	}

	if err = dirs.Download(uid, "archive.tar.gz", download.Body); err != nil {
		return errorsx.Wrap(err, "unable to receive kernel archive")
	}

	if encoded, err = json.Marshal(&workload); err != nil {
		return errorsx.Wrap(err, "unable to write metadata")
	}

	if err = dirs.Download(uid, "metadata.json", bytes.NewBuffer(encoded)); err != nil {
		return errorsx.Wrap(err, "unable to write metadata")
	}

	if err = dirs.Enqueue(uid); err != nil {
		return errorsx.Wrap(err, "unable to enqueue kernel archive")
	}

	return nil
}

// retrieves available workloads
func NewWorkloadClient(c *http.Client, rm *ResourceManager) *WorkloadClient {
	return &WorkloadClient{
		c:    c,
		m:    rm,
		host: eg.EnvAPIHostDefault(),
	}
}

type WorkloadClient struct {
	c    *http.Client
	m    *ResourceManager
	host string
}

func (t WorkloadClient) Download(ctx context.Context, limits RuntimeResources) (_ *EnqueuedDequeueResponse, err error) {
	var (
		encoded []byte
		req     = EnqueuedSearchRequest{
			Os:     runtime.GOOS,
			Arch:   runtime.GOARCH,
			Cores:  limits.Cores,
			Memory: limits.Memory,
		}
		resp EnqueuedDequeueResponse
	)

	if encoded, err = json.Marshal(&req); err != nil {
		return nil, err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/q/dequeue", t.host), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}

	dhttpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(dhttpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(dhttpresp.Body).Decode(&resp); err != nil {
		return nil, err
	}

	return &resp, nil
}
