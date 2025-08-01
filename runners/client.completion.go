package runners

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
)

// Uploads completion details to control plane.
func NewCompletionClient(c *http.Client) *CompletionClient {
	return &CompletionClient{
		c:    c,
		host: eg.EnvAPIHostDefault(),
	}
}

type CompletionClient struct {
	c    *http.Client
	host string
}

func (t CompletionClient) Upload(ctx context.Context, id string, duration time.Duration, cause error, logs io.Reader, analytics io.Reader) (err error) {
	mimetype, body, err := NewEnqueueCompletion(cause, duration, logs, analytics)
	if err != nil {
		return err
	}

	defer body.Close()

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/q/%s/completed", t.host, id), body)
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
