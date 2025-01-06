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
	"github.com/gofrs/uuid"
	"github.com/pbnjay/memory"
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

func (t DownloadClient) Download(ctx context.Context) (err error) {
	var (
		encoded []byte
		req     = EnqueuedSearchRequest{
			Os:     runtime.GOOS,
			Arch:   runtime.GOARCH,
			Cores:  uint64(runtime.NumCPU()),
			Memory: memory.TotalMemory(),
		}
		resp EnqueuedDequeueResponse
	)

	if encoded, err = json.Marshal(&req); err != nil {
		return err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/q/dequeue", t.host), bytes.NewReader(encoded))
	if err != nil {
		return err
	}

	dhttpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(dhttpresp)) }()
	if err != nil {
		return err
	}

	if err = json.NewDecoder(dhttpresp.Body).Decode(&resp); err != nil {
		return err
	}

	httpreq, err = http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/c/q/%s/download", t.host, resp.Enqueued.Id), bytes.NewReader(encoded))
	if err != nil {
		return err
	}

	download, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(download)) }()
	if err != nil {
		return err
	}

	dirs := DefaultSpoolDirs()

	uid := uuid.FromStringOrNil(resp.Enqueued.Id)
	if err = dirs.Download(uid, "archive.tar.gz", download.Body); err != nil {
		return errorsx.Wrap(err, "unable to receive kernel archive")
	}

	if encoded, err = json.Marshal(&resp); err != nil {
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
