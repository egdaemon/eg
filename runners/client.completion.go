package runners

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// Uploads completion details to control plane.
func NewCompletionClient(c *http.Client) *CompletionClient {
	return &CompletionClient{
		c:    c,
		host: envx.String(eg.EnvEGAPIHostDefault, eg.EnvEGAPIHost),
	}
}

type CompletionClient struct {
	c    *http.Client
	host string
}

func (t CompletionClient) Upload(ctx context.Context, id string, cause error, logs io.Reader) (err error) {
	mimetype, body, err := NewEnqueueCompletion(cause, logs)
	if err != nil {
		return err
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/manager/completed/%s", t.host, id), body)
	if err != nil {
		return err
	}
	httpreq.Header.Set("Content-Type", mimetype)

	download, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(download)) }()
	if err != nil {
		return err
	}

	return nil
}
