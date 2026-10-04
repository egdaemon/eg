package runners

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// Records workloads uploaded directly to this runner with the control plane.
func NewDirectClient(c *http.Client) *DirectClient {
	return &DirectClient{
		c:    c,
		host: eg.EnvAPIHostDefault(),
	}
}

type DirectClient struct {
	c    *http.Client
	host string
}

// Reserve uploads the workload's archive to the control plane along with the
// candidate token the client presented, recording the workload against the
// client's account as claimed by this runner, and returns its metadata.
func (t DirectClient) Reserve(ctx context.Context, enq *Enqueued, archive io.Reader, token string) (_ *EnqueuedDequeueResponse, err error) {
	var (
		resp EnqueuedDequeueResponse
	)

	mimetype, body, err := NewEnqueueUploadReserved(enq, archive, token)
	if err != nil {
		return nil, err
	}
	defer body.Close()

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/q/reserved", t.host), body)
	if err != nil {
		return nil, err
	}
	httpreq.Header.Set("Content-Type", mimetype)

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode workload metadata")
	}

	if resp.Enqueued == nil {
		return nil, errorsx.New("workload metadata missing enqueued information")
	}

	return &resp, nil
}
