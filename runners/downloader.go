package runners

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

func NewDownloadClient(c *http.Client) *DownloadClient {
	return &DownloadClient{
		c:    c,
		host: envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost),
	}
}

type DownloadClient struct {
	c    *http.Client
	host string
}

func (t DownloadClient) Download(ctx context.Context) (err error) {
	var (
		encoded []byte
		req     EnqueuedSearchRequest
		resp    EnqueuedDequeueResponse
	)

	if encoded, err = json.Marshal(&req); err != nil {
		return err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/eg/registration/", t.host), bytes.NewReader(encoded))
	if err != nil {
		return err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.MaybeLog(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return err
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return err
	}

	return nil
}
