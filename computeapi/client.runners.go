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

// RunnersSearch retrieves a page of the account's live runners. the request is never global, runners
// shared by other accounts are not searchable here. the response's Next carries the request for the following
// page, an offset of zero means there are no more pages.
func (t PushClient) RunnersSearch(ctx context.Context, req *compute.SearchRequest) (_ *compute.SearchResponse, err error) {
	var (
		resp compute.SearchResponse
	)

	form := url.Values{}
	form.Set("query", req.GetQuery())
	form.Set("arch", req.GetArch())
	form.Set("os", req.GetOs())
	form.Set("cores", strconv.FormatUint(req.GetCores(), 10))
	form.Set("memory", strconv.FormatUint(req.GetMemory(), 10))
	form.Set("vram", strconv.FormatUint(req.GetVram(), 10))
	form.Set("offset", strconv.FormatUint(req.GetOffset(), 10))
	form.Set("limit", strconv.FormatUint(req.GetLimit(), 10))

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/c/runners/?%s", t.host, form.Encode()), nil)
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = jsonx.UnmarshalRead(httpresp.Body, &resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode runners")
	}

	return &resp, nil
}

// RunnersDestroy removes the runner of the account, it reappears if the runner is still running and registers again.
func (t PushClient) RunnersDestroy(ctx context.Context, id string) (err error) {
	httpreq, err := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/c/runners/%s", t.host, url.PathEscape(id)), nil)
	if err != nil {
		return err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	return err
}
