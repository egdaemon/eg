package runners

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/stringsx"
)

// CandidateTokenFile is where a workload pushed to this runner keeps the
// candidate token it was pushed with (see the POST /c/enqueue handler); the
// token is presented to the control plane to reserve the workload and to
// report its status.
const CandidateTokenFile = "candidate.token"

// phases reported to the control plane for a workload pushed to this runner.
const (
	StatusCompiling = "compiling"
	StatusQueued    = "queued"
	StatusRunning   = "running"
	StatusFailed    = "failed"
)

// ReadCandidateToken returns the candidate token stored in a workload's
// directory, blank when the workload was not pushed to this runner.
func ReadCandidateToken(dir string) string {
	encoded, err := os.ReadFile(filepath.Join(dir, CandidateTokenFile))
	if err != nil {
		if !os.IsNotExist(err) {
			log.Println(errorsx.Wrap(err, "unable to read candidate token"))
		}
		return ""
	}

	return strings.TrimSpace(string(encoded))
}

// Reports the progress of workloads pushed to this runner to the control plane.
func NewStatusClient(c *http.Client) *StatusClient {
	return &StatusClient{
		c:    c,
		host: eg.EnvAPIHostDefault(),
	}
}

type StatusClient struct {
	c    *http.Client
	host string
}

// Status reports the workload reached phase; workloads without a candidate
// token (i.e. not pushed to this runner) are not reported.
func (t StatusClient) Status(ctx context.Context, token string, phase string, enq *Enqueued) error {
	if stringsx.Blank(token) || enq == nil {
		return nil
	}

	form := url.Values{
		"token":      {token},
		"phase":      {phase},
		"vcs_uri":    {enq.VcsUri},
		"vcs_commit": {enq.VcsCommit},
	}

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/q/status", t.host), strings.NewReader(form.Encode()))
	if err != nil {
		return errorsx.Wrapf(err, "unable to report status %s", phase)
	}
	httpreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(resp)) }()
	if err != nil {
		return errorsx.Wrapf(err, "unable to report status %s", phase)
	}

	return nil
}
