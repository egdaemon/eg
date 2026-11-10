package computeapi

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/jsonx"
)

// HistorySearch retrieves a page of the account's completed workloads, most recently completed first. the response's
// Next carries the request for the following page, an offset of zero means there are no more pages.
func (t WorkloadClient) HistorySearch(ctx context.Context, req *compute.SearchHistoryRequest) (_ *compute.SearchHistoryResponse, err error) {
	var (
		resp compute.SearchHistoryResponse
	)

	form := url.Values{}
	form.Set("query", req.GetQuery())
	form.Set("vcs_uri", req.GetVcsUri())
	form.Set("offset", strconv.FormatUint(req.GetOffset(), 10))
	form.Set("limit", strconv.FormatUint(req.GetLimit(), 10))

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/c/history/?%s", t.host, form.Encode()), nil)
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = jsonx.UnmarshalRead(httpresp.Body, &resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode history")
	}

	return &resp, nil
}
