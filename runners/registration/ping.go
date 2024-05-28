package registration

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

func NewPingClient(c *http.Client) *PingClient {
	return &PingClient{
		c:    c,
		host: envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost),
	}
}

type PingClient struct {
	c    *http.Client
	host string
}

func (t PingClient) Request(ctx context.Context, id string, req *PingRequest) (_ *PingResponse, err error) {
	var (
		encoded []byte
		resp    PingResponse
	)

	if encoded, err = json.Marshal(req); err != nil {
		return nil, err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/c/runners/%s", t.host, id), bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()

	if httpx.IsStatusError(err, http.StatusForbidden) != nil {
		return nil, errorsx.NewUnrecoverable(err)
	} else if err != nil {
		return nil, err
	}

	return &resp, nil
}
