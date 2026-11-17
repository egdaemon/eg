package runners

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/jsonx"
)

// ReadAccessToken returns the access token the control plane issued with the workload stored in
// the run directory, blank when the workload was handed over without one.
func ReadAccessToken(dir string) string {
	var recorded computeapi.EnqueuedDequeueResponse

	encoded, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		log.Println(errorsx.Wrap(err, "unable to read the workload metadata"))
		return ""
	}

	if err = jsonx.Unmarshal(encoded, &recorded); err != nil {
		log.Println(errorsx.Wrap(err, "unable to decode the workload metadata"))
		return ""
	}

	return recorded.AccessToken
}

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
	mimetype, body, err := computeapi.NewEnqueueCompletionRequest(&computeapi.EnqueuedCompletedRequest{
		Duration:    uint64(duration.Milliseconds()),
		Successful:  cause == nil,
		AccessToken: computeapi.CompletionTokenFromContext(ctx),
	}, logs, analytics)
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
