package runners

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// proxyProtocol is the runner's stream handler (see cmd/eg/daemons/p2p.go)
// that proxies an inbound libp2p stream into the runner's local HTTP listener.
const proxyProtocol = protocol.ID("/egdaemon/proxy")

// bounds how long a single runner gets to accept a stream before we move on to
// the next candidate.
const dialTimeout = 5 * time.Second

// bounds how long a single runner gets to receive the workload.
const uploadTimeout = 5 * time.Minute

// Pushes workloads directly to runners.
func NewPushClient(c *http.Client) *PushClient {
	return &PushClient{
		c:    c,
		host: eg.EnvAPIHostDefault(),
	}
}

type PushClient struct {
	c    *http.Client
	host string
}

// Candidates returns the runners able to run the workload, best match first.
func (t PushClient) Candidates(ctx context.Context, enq *Enqueued, limit uint64) (_ *compute.SearchResponse, err error) {
	var (
		resp compute.SearchResponse
	)

	form := url.Values{}
	form.Set("cores", strconv.FormatUint(enq.Cores, 10))
	form.Set("memory", strconv.FormatUint(enq.Memory, 10))
	form.Set("vram", strconv.FormatUint(enq.Vram, 10))
	form.Set("arch", enq.Arch)
	form.Set("os", enq.Os)
	form.Set("allow_shared", strconv.FormatBool(enq.AllowShared))
	form.Set("vcs_uri", enq.VcsUri)
	form.Set("limit", strconv.FormatUint(limit, 10))

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, fmt.Sprintf("%s/c/q/candidates", t.host), strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	httpreq.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	httpresp, err := httpx.AsError(t.c.Do(httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(httpresp)) }()
	if err != nil {
		return nil, err
	}

	if err = json.NewDecoder(httpresp.Body).Decode(&resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode candidates")
	}

	return &resp, nil
}

// TryUpload attempts to upload the workload to each candidate in order,
// returning the runner that accepted it, or nil when none did.
func TryUpload(ctx context.Context, p2p host.Host, candidates []*compute.Compute, req *EnqueuedDequeueResponse, archive, environ io.ReadSeeker) *compute.Compute {
	for _, c := range candidates {
		if accepted, err := Upload(ctx, p2p, c, req, archive, environ); err != nil {
			log.Println("direct upload failed", c.Id, c.P2Pid, err)
		} else if accepted {
			return c
		}
	}

	return nil
}

// Upload dials the candidate over p2p and uploads the workload, along with the
// candidate's token, to the runner's POST /c/upload. accepted=false with a nil
// error means the runner declined the workload (i.e. it is at capacity); try
// the next candidate.
func Upload(ctx context.Context, p2p host.Host, candidate *compute.Compute, req *EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (accepted bool, err error) {
	pid, err := peer.Decode(candidate.P2Pid)
	if err != nil {
		return false, fmt.Errorf("invalid peer id %q: %w", candidate.P2Pid, err)
	}

	if err = iox.Rewind(archive); err != nil {
		return false, errorsx.Wrap(err, "unable to rewind archive")
	}

	if err = iox.Rewind(environ); err != nil {
		return false, errorsx.Wrap(err, "unable to rewind environ")
	}

	dctx, done := context.WithTimeout(ctx, dialTimeout)
	defer done()

	stream, err := p2p.NewStream(dctx, pid, proxyProtocol)
	if err != nil {
		return false, fmt.Errorf("unable to open stream to %s: %w", candidate.P2Pid, err)
	}
	defer stream.Close()

	errorsx.Log(stream.SetDeadline(time.Now().Add(uploadTimeout)))

	mimetype, body, err := NewDirectUpload(req, candidate.Token, archive, environ)
	if err != nil {
		return false, errorsx.Wrap(err, "unable to build upload request")
	}
	defer body.Close()

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://runner/c/upload", body)
	if err != nil {
		return false, err
	}
	httpreq.Header.Set("Content-Type", mimetype)

	if err = httpreq.Write(stream); err != nil {
		return false, fmt.Errorf("unable to write request over stream: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(stream), httpreq)
	if err != nil {
		return false, fmt.Errorf("unable to read response from stream: %w", err)
	}
	defer resp.Body.Close()

	if httpx.IsSuccess(resp.StatusCode) {
		return true, nil
	}

	if resp.StatusCode == http.StatusConflict {
		return false, nil
	}

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	return false, fmt.Errorf("runner rejected upload with status %d: %s", resp.StatusCode, string(b))
}
