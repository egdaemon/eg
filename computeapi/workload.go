package computeapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/jsonx"
)

// NewWorkloadClient queries the control plane for the workloads of the account.
func NewWorkloadClient(c *http.Client) *WorkloadClient {
	return &WorkloadClient{
		c:    c,
		host: eg.EnvAPIHostDefault(),
	}
}

type WorkloadClient struct {
	c    *http.Client
	host string
}

// Search retrieves a page of the account's active (queued or running) workloads. the response's Next carries
// the request for the following page, an offset of zero means there are no more pages.
func (t WorkloadClient) Search(ctx context.Context, req *EnqueuedSearchRequest) (_ *EnqueuedSearchResponse, err error) {
	var (
		resp EnqueuedSearchResponse
	)

	form := url.Values{}
	form.Set("query", req.GetQuery())
	form.Set("arch", req.GetArch())
	form.Set("os", req.GetOs())
	form.Set("cores", strconv.FormatUint(req.GetCores(), 10))
	form.Set("memory", strconv.FormatUint(req.GetMemory(), 10))
	form.Set("vram", strconv.FormatUint(req.GetVram(), 10))
	form.Set("vcs_uri", req.GetVcsUri())
	form.Set("offset", strconv.FormatUint(req.GetOffset(), 10))
	form.Set("limit", strconv.FormatUint(req.GetLimit(), 10))

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/c/q/?%s", t.host, form.Encode()), nil)
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = jsonx.UnmarshalRead(httpresp.Body, &resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode workloads")
	}

	return &resp, nil
}
