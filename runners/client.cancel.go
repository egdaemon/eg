package runners

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// Cancel cancels the workload with the control plane, which tells the runner executing it to stop. a status
// error of http.StatusNotFound means the workload is unknown or already completed.
func (t PushClient) Cancel(ctx context.Context, id string) (err error) {
	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/q/%s/cancel", t.host, url.PathEscape(id)), nil)
	if err != nil {
		return err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	return err
}
