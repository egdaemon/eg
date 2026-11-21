package computeapi

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/jsonx"
)

// Enqueue registers the workload with the control plane, which queues it for a runner. unlike an upload there is
// no archive, the runner clones the workload's repository, so the vcs uri and entry must be set. returns the
// workload as recorded, carrying the id assigned to it.
func (t PushClient) Enqueue(ctx context.Context, enq *Enqueued) (_ *Enqueued, err error) {
	var (
		resp    EnqueuedCreateResponse
		encoded []byte
	)

	if encoded, err = jsonx.Marshal(&EnqueuedCreateRequest{Enqueued: enq}); err != nil {
		return nil, errorsx.Wrap(err, "unable to encode workload")
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/q/workload", t.host), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}
	httpreq.Header.Set("Content-Type", "application/json")

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = jsonx.UnmarshalRead(httpresp.Body, &resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode workload")
	}

	if resp.Enqueued == nil {
		return nil, errorsx.String("enqueue response missing workload")
	}

	return resp.Enqueued, nil
}
