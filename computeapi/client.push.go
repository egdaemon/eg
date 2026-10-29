package computeapi

import (
	"bufio"
	"context"
	"fmt"
	"github.com/egdaemon/eg/jsonx"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/golang-jwt/jwt/v4"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/libp2p/go-libp2p/core/protocol"
)

// proxyProtocol is the runner's stream handler (see cmd/eg/daemons/p2p.go)
// that proxies an inbound libp2p stream into the runner's local HTTP listener.
const proxyProtocol = protocol.ID("/egdaemon/proxy")

// bounds how long a single runner gets to accept a stream before we move on to
// the next candidate. covers dialing through the relay and hole punching.
const dialTimeout = 15 * time.Second

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

// ErrPaymentRequired is returned when the control plane has no runners for the account, i.e.) it is on the free
// tier. only runners on the local network and the p2p network can be used.
const ErrPaymentRequired = errorsx.String("payment required")

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
	if httpx.IsStatusError(err, http.StatusPaymentRequired) != nil {
		return nil, fmt.Errorf("%w: %w", ErrPaymentRequired, err)
	} else if err != nil {
		return nil, err
	}

	if err = jsonx.UnmarshalRead(httpresp.Body, &resp); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode candidates")
	}

	return &resp, nil
}

// TryUpload attempts to upload the workload to each candidate in order,
// returning the runner that accepted it and the workload as recorded, or nils
// when none did.
func TryUpload(ctx context.Context, p2p host.Host, candidates []*compute.Compute, req *EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (*compute.Compute, *Enqueued) {
	debugx.Println("direct upload candidates", len(candidates))
	for _, c := range candidates {
		if recorded, err := UploadOne(ctx, p2p, c, req, archive, environ); err != nil {
			log.Println("direct upload failed", c.Id, c.P2Pid, err)
		} else if recorded != nil {
			return c, recorded
		} else {
			log.Println("direct upload declined, the runner is likely at capacity", c.Id, c.P2Pid)
		}
	}

	return nil, nil
}

// tokenClaims returns the p2p addresses of the relay holding the runner's
// reservation and, when present, the runner's p2p id from a token issued by the
// control plane (candidate or log tokens). the token is only verified by the
// runner/control plane, the client just reads the claims.
func tokenClaims(token string) (relay []string, p2pid string) {
	var claims struct {
		jwt.RegisteredClaims
		Relay []string `json:"relay"`
		P2P   string   `json:"p2p"`
	}

	if _, _, err := jwt.NewParser().ParseUnverified(token, &claims); err != nil {
		debugx.Println("unable to read token claims", err)
		return nil, ""
	}

	return claims.Relay, claims.P2P
}

// UploadOne dials the candidate over p2p and uploads the workload, along with the
// candidate's token, to the runner's POST /c/upload, returning the workload as
// recorded by the control plane. a nil workload with a nil error means the runner
// declined the workload (i.e. it is at capacity); try the next candidate.
func UploadOne(ctx context.Context, p2p host.Host, candidate *compute.Compute, req *EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (recorded *Enqueued, err error) {
	if err = iox.Rewind(archive); err != nil {
		return nil, errorsx.Wrap(err, "unable to rewind archive")
	}

	if err = iox.Rewind(environ); err != nil {
		return nil, errorsx.Wrap(err, "unable to rewind environ")
	}

	stream, err := dialRunner(ctx, p2p, candidate.P2Pid, candidate.Token)
	if err != nil {
		return nil, err
	}
	defer stream.Close()

	debugx.Println("direct upload stream opened", candidate.Id, stream.Conn().RemotePeer(), stream.Conn().RemoteMultiaddr())

	errorsx.Log(stream.SetDeadline(time.Now().Add(uploadTimeout)))

	mimetype, body, err := NewDirectUpload(req, candidate.Token, archive, environ)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to build upload request")
	}
	defer body.Close()

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://runner/c/upload", body)
	if err != nil {
		return nil, err
	}
	httpreq.Header.Set("Content-Type", mimetype)

	if err = httpreq.Write(stream); err != nil {
		return nil, fmt.Errorf("unable to write request over stream: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReader(stream), httpreq)
	if err != nil {
		return nil, fmt.Errorf("unable to read response from stream: %w", err)
	}
	defer resp.Body.Close()

	debugx.Println("direct upload response", candidate.Id, stream.Conn().RemotePeer(), resp.Status)

	if httpx.IsSuccess(resp.StatusCode) {
		// the runner responds with the workload as recorded by the control plane,
		// including the log token for streaming its logs.
		recorded = &Enqueued{}
		if err = jsonx.UnmarshalRead(resp.Body, recorded); err != nil {
			return nil, errorsx.Wrap(err, "unable to decode recorded workload")
		}

		return recorded, nil
	}

	// conflict is what runners before the switch to unavailable respond with.
	if httpx.CheckStatusCode(resp.StatusCode, http.StatusServiceUnavailable, http.StatusConflict) {
		return nil, nil
	}

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<10))
	return nil, fmt.Errorf("runner rejected upload with status %d: %s", resp.StatusCode, string(b))
}

// dialRunner opens a stream over p2p to the http proxy of the runner p2pid, reaching
// it through the relay carried by token's claims when no addresses are known.
func dialRunner(ctx context.Context, p2p host.Host, p2pid string, token string) (network.Stream, error) {
	pid, err := peer.Decode(p2pid)
	if err != nil {
		return nil, fmt.Errorf("invalid peer id %q: %w", p2pid, err)
	}

	// tokens only carry the peer id; without addresses the routed host
	// falls back to a dht lookup. reach the runner through the relay holding its
	// reservation (carried by the token), and the relays we're connected
	// to, instead. hole punching upgrades it to a direct connection.
	if len(p2p.Peerstore().Addrs(pid)) == 0 {
		relay, _ := tokenClaims(token)
		addrs := append(libp2px.RelayCircuitAddrs(relay...), libp2px.CircuitAddrs(p2p)...)
		p2p.Peerstore().AddAddrs(pid, addrs, peerstore.TempAddrTTL)
	}

	debugx.Println("dialing runner", pid, p2p.Peerstore().Addrs(pid))

	dctx, done := context.WithTimeout(ctx, dialTimeout)
	defer done()

	stream, err := p2p.NewStream(dctx, pid, proxyProtocol)
	if err != nil {
		return nil, fmt.Errorf("unable to open stream to %s: %w", p2pid, err)
	}
	return stream, nil
}

// FollowLogs streams the log of workload id into dst as it is written, until the
// workload completes or ctx is cancelled. token is the workload's log token issued by
// the control plane (Enqueued.LogsToken); its claims identify the runner to dial.
// offset resumes the stream from the given byte offset.
func FollowLogs(ctx context.Context, p2p host.Host, token string, id string, offset int64, dst io.Writer) (err error) {
	_, p2pid := tokenClaims(token)
	if p2pid == "" {
		return errorsx.String("log token missing runner")
	}

	stream, err := dialRunner(ctx, p2p, p2pid, token)
	if err != nil {
		return err
	}
	defer stream.Close()

	// no deadline; the stream lives as long as the workload. cancelling ctx closes it.
	stop := context.AfterFunc(ctx, func() { _ = stream.Reset() })
	defer stop()

	httpreq, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://runner/c/logs/%s?offset=%d", url.PathEscape(id), offset), nil)
	if err != nil {
		return err
	}
	httpreq.Header.Set("Authorization", token)

	if err = httpreq.Write(stream); err != nil {
		return fmt.Errorf("unable to write request over stream: %w", err)
	}

	resp, err := httpx.AsError(http.ReadResponse(bufio.NewReader(stream), httpreq))
	defer func() { errorsx.Log(httpx.AutoClose(resp)) }()
	if err != nil {
		return errorsx.Wrap(err, "unable to stream logs")
	}

	if _, err = io.Copy(dst, resp.Body); ctx.Err() != nil {
		return ctx.Err()
	}

	return errorsx.Wrap(err, "log stream interrupted")
}
