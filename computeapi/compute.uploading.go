package computeapi

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/backoff"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/computex"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/slicesx"
	"github.com/egdaemon/eg/internal/sqlx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/jsonx"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	circuitv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/client"
	pbv2 "github.com/libp2p/go-libp2p/p2p/protocol/circuitv2/pb"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"
)

// uploadstrategy is a way of reaching a runner.
type uploadstrategy int

const (
	uploadLAN     uploadstrategy = iota // runners remembered from the local network.
	uploadDirect                        // runners the control plane suggests, reached through the p2p network.
	uploadCluster                       // the cluster schedules the workload, always the last resort.
)

func (t uploadstrategy) String() string {
	switch t {
	case uploadLAN:
		return "local network"
	case uploadCluster:
		return "cluster"
	default:
		return "p2p network"
	}
}

// uploadpriorities orders the enabled peer to peer strategies, highest priority first. a priority of zero or
// less disables the strategy, ties favor the local network.
func uploadpriorities(lanpriority, directpriority int) []uploadstrategy {
	type ranked struct {
		strategy uploadstrategy
		priority int
	}

	enabled := slices.DeleteFunc([]ranked{{uploadLAN, lanpriority}, {uploadDirect, directpriority}}, func(r ranked) bool {
		return r.priority <= 0
	})

	slices.SortStableFunc(enabled, func(a, b ranked) int {
		return cmp.Compare(b.priority, a.priority)
	})

	return slicesx.MapTransform(func(r ranked) uploadstrategy { return r.strategy }, enabled...)
}

// uploadopt configures Upload.
type uploadopt func(*uploader)

// UploadEndpoint sets the cluster endpoint the workload is enqueued with when it is the last resort.
func UploadEndpoint(endpoint string) uploadopt {
	return func(u *uploader) { u.endpoint = endpoint }
}

// UploadPriorities sets the priority of the local network and the p2p network, higher goes first,
// zero or less disables the strategy.
func UploadPriorities(lan, p2p int) uploadopt {
	return func(u *uploader) { u.lanpriority, u.p2ppriority = lan, p2p }
}

// UploadP2POnly never falls back to the cluster.
func UploadP2POnly(b bool) uploadopt {
	return func(u *uploader) { u.p2ponly = b }
}

// UploadFollow streams the workload's logs and waits for it to complete.
func UploadFollow(b bool) uploadopt {
	return func(u *uploader) { u.follow = b }
}

// UploadLANPeerTTL is how long systems on the local network are remembered after they were last seen.
func UploadLANPeerTTL(d time.Duration) uploadopt {
	return func(u *uploader) { u.lanttl = d }
}

// UploadMaxAttempts bounds how many times candidates are identified and connected to, defaults to DefaultUploadAttempts.
func UploadMaxAttempts(n int) uploadopt {
	return func(u *uploader) { u.attemptslimit = n }
}

// UploadBackoff sets how long to wait between attempts, defaults to exponential starting at 200ms and capped at 5s.
func UploadBackoff(s backoff.Strategy) uploadopt {
	return func(u *uploader) { u.backoff = s }
}

type uploader struct {
	httpc    *http.Client
	db       sqlx.Queryer // cache of remembered systems, may be nil.
	enq      *Enqueued
	archive  io.ReadSeeker
	environ  io.ReadSeeker
	endpoint string

	lanpriority     int
	p2ppriority     int
	p2ponly         bool
	follow          bool
	lanttl          time.Duration
	backoff         backoff.Strategy
	candidateslimit uint64 // maximum number of candidates to consider.
	attemptslimit   int    // maximum number of attempts.
	attempt         int
	free            bool       // the control plane answered payment required, only the lan/p2p sources are available.
	nocluster       bool       // the cluster answered payment required, it is not attempted again.
	retry           bool       // some candidate failed for a reason that may pass, e.g.) no reservation.
	cause           error      // why the most recent attempt failed.
	logs            *logwriter // where the workload's logs are written, tracks how much was written so following can resume.
	result          error
}

// logwriter counts the bytes written so a log stream interrupted part way can resume from its offset.
type logwriter struct {
	dst io.Writer
	n   int64
}

func (t *logwriter) Write(p []byte) (n int, err error) {
	n, err = t.dst.Write(p)
	t.n += int64(n)
	return n, err
}

// streamfailure marks a failure to stream the logs of a workload that was accepted, as opposed to the
// workload not succeeding.
type streamfailure struct{ cause error }

func (t streamfailure) Error() string { return t.cause.Error() }
func (t streamfailure) Unwrap() error { return t.cause }

// Upload uploads the workload and follows it, if enabled, until it completes. candidates are identified and
// connected to in a cycle until one accepts the workload, the context ends or the attempts are exhausted.
// the cluster is the last candidate unless the control plane reported the account as free tier or p2p only is set,
// in which case the cycle waits and retries when runners decline or are missing a reservation.
//
// states:
//
//	identifying -> connecting -> watching -> completed
//	     ^             |           |  ^  |
//	     +--- delay ---+           |  |  +-> awaiting -> completed
//	                               |  |         |
//	                               v  |         +-> failed
//	                           watchdelay
//
// a workload that was accepted is never uploaded again. when its logs cannot be streamed, e.g.) the runner has
// no relay reservation yet, following waits and retries before falling back to waiting for the result.
func Upload(ctx context.Context, httpc *http.Client, db sqlx.Queryer, enq *Enqueued, archive, environ io.ReadSeeker, options ...uploadopt) error {
	u := &uploader{
		httpc:           httpc,
		db:              db,
		enq:             enq,
		archive:         archive,
		environ:         environ,
		attemptslimit:   16,
		candidateslimit: 8,
		backoff:         backoff.New(backoff.Exponential(time.Second), backoff.Maximum(32*time.Second)),
		lanttl:          lan.DefaultTTL,
		logs:            &logwriter{dst: os.Stdout},
	}

	for _, opt := range options {
		opt(u)
	}

	for s := uploadstate(uploadidentifying{u}); s != nil; {
		s = s.Update(ctx)
	}

	return u.result
}

// strategies are the enabled strategies in the order they are attempted.
func (t *uploader) strategies() []uploadstrategy {
	order := uploadpriorities(t.lanpriority, t.p2ppriority)
	if !t.free && !t.nocluster && !t.p2ponly {
		order = append(order, uploadCluster)
	}

	return order
}

type uploadstate interface {
	Update(context.Context) uploadstate
}

func (t *uploader) fail(cause error) uploadstate {
	return uploadfailed{u: t, cause: cause}
}

// uploadidentifying finds the candidates for the attempt.
type uploadidentifying struct{ u *uploader }

func (t uploadidentifying) Update(ctx context.Context) uploadstate {
	u := t.u

	if err := ctx.Err(); err != nil {
		return u.fail(err)
	}

	u.attempt++
	u.retry = false
	u.cause = nil

	if len(u.strategies()) == 0 {
		return u.fail(errorsx.Errorf("--p2p-only is set but no direct upload strategy is enabled, see --p2p and --lan"))
	}

	if len(uploadpriorities(u.lanpriority, u.p2ppriority)) == 0 {
		return uploadconnecting{u: u}
	}

	var candidates []*compute.Compute

	if !u.free {
		found, err := NewPushClient(u.httpc).Candidates(ctx, u.enq, uint64(u.candidateslimit))
		switch {
		case errors.Is(err, ErrPaymentRequired):
			log.Println("free tier, only self hosted runners are available")
			u.free = true
		case err != nil:
			u.cause = errorsx.Wrap(err, "unable to retrieve candidate runners")
		default:
			if u.db != nil {
				errorsx.Log(errorsx.Wrap(lan.RememberCandidates(ctx, u.db, found.Items...), "unable to remember the candidate runners"))
			}
			candidates = found.Items
		}
	}

	if u.free {
		remembered, err := lanknown(ctx, u.db, u.lanttl)
		if err != nil {
			u.cause = err
		}

		for _, p := range remembered {
			candidates = append(candidates, &compute.Compute{Id: p.PeerID, P2Pid: p.PeerID, Description: p.Description})
		}
	}

	if len(candidates) == 0 {
		// no runner is currently reserved for the workload, which may change.
		u.retry = true
		u.cause = errorsx.Compact(u.cause, errorsx.New("no runners available that match the workload"))
	}

	return uploadconnecting{u: u, candidates: candidates}
}

// uploadconnecting attempts the strategies in order until one accepts the workload.
type uploadconnecting struct {
	u          *uploader
	candidates []*compute.Compute
}

func (t uploadconnecting) Update(ctx context.Context) uploadstate {
	u := t.u

	// clone so the caller's enqueued remains untouched for the cluster.
	req := &EnqueuedDequeueResponse{
		Enqueued: proto.Clone(u.enq).(*Enqueued),
	}

	for _, s := range u.strategies() {
		if err := ctx.Err(); err != nil {
			return u.fail(err)
		}

		var (
			o     outcome
			retry bool
			err   error
		)

		switch s {
		case uploadLAN:
			o, retry, err = uploadlan(ctx, u.db, u.lanttl, t.candidates, req, u.archive, u.environ)
		case uploadDirect:
			o, retry, err = uploaddirect(ctx, u.httpc, t.candidates, req, u.archive, u.environ)
		default:
			o, err = uploadenqueue(ctx, u.httpc, u.endpoint, u.enq, u.archive)
		}

		u.retry = u.retry || retry
		if s == uploadCluster && httpx.IsStatusError(err, http.StatusPaymentRequired) != nil {
			// free tier, keep why the runners failed since that is the actionable error.
			log.Println("api workload orchestration is unavailable on the free tier, only lan and p2p are available")
			u.nocluster = true
			u.cause = errorsx.Compact(u.cause, errorsx.Wrapf(err, "%s upload failed", s))

			continue
		} else if err != nil {
			u.cause = errorsx.Wrapf(err, "%s upload failed", s)
			continue
		}

		debugx.Println("enqueued", spew.Sdump(o.recorded))

		if o.host != nil {
			log.Printf("enqueued workload(%s) runner(%s) peer(%s)\n", u.enq.Id, o.runner.Id, o.runner.P2Pid)
		} else {
			log.Printf("enqueued workload(%s)\n", o.recorded.Id)
		}

		// the attempts bound consecutive failures, the workload was accepted.
		u.attempt = 0

		return uploadwatching{u: u, o: o}
	}

	if u.retry && u.attempt < u.attemptslimit {
		return uploaddelay{u: u}
	}

	cause := u.cause
	if u.p2ponly {
		cause = errorsx.Wrap(cause, "direct upload unavailable and --p2p-only is set")
	}

	return u.fail(errorsx.Wrapf(cause, "unable to upload the workload after %d attempts", u.attempt))
}

// uploaddelay waits before identifying candidates again.
type uploaddelay struct{ u *uploader }

func (t uploaddelay) Update(ctx context.Context) uploadstate {
	d := t.u.backoff.Backoff(int64(t.u.attempt - 1))
	log.Printf("(attempt %d/%d - %s) no runner accepted the workload\n", t.u.attempt, t.u.attemptslimit, d)

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return t.u.fail(errors.Join(ctx.Err(), t.u.cause))
	case <-timer.C:
		return uploadidentifying{t.u}
	}
}

// uploadwatching follows a workload that was accepted. the workload is never uploaded again, an
// error here is the result of the workload unless the logs could not be streamed.
type uploadwatching struct {
	u *uploader
	o outcome // the host is only set when a runner accepted the workload directly.
}

func (t uploadwatching) Update(ctx context.Context) uploadstate {
	var (
		err      error
		failure  streamfailure
		retrying bool
		u        = t.u
		before   = u.logs.n
	)

	if t.o.host != nil {
		defer func() {
			if !retrying {
				errorsx.Log(t.o.host.Close())
			}
		}()
	}

	if u.follow && t.o.host != nil {
		err = uploadwatch(ctx, u.httpc, t.o, u.logs.n, u.logs)
	} else if u.follow {
		err = followenqueued(ctx, u.httpc, t.o.recorded.Id, u.logs.n, u.logs)
	}

	if err == nil {
		return uploadcompleted{u}
	}

	if !errors.As(err, &failure) || ctx.Err() != nil {
		return u.fail(err)
	}

	// the stream connected and made progress, the failures are no longer consecutive.
	if u.logs.n > before {
		u.attempt = 0
	}

	if reason := retryreason(err); reason != "" && u.attempt < u.attemptslimit {
		u.attempt++
		retrying = true
		debugx.Println("unable to stream workload logs", t.o.recorded.Id, err)
		return uploadwatchdelay{u: u, o: t.o, reason: reason}
	}

	log.Println("unable to stream the workload logs, waiting for it to complete", t.o.recorded.Id, err)
	return uploadawaiting{u: u, id: t.o.recorded.Id}
}

// uploadwatchdelay waits before following the logs of the workload again.
type uploadwatchdelay struct {
	u      *uploader
	o      outcome
	reason string
}

func (t uploadwatchdelay) Update(ctx context.Context) uploadstate {
	d := t.u.backoff.Backoff(int64(t.u.attempt - 1))
	log.Printf("(attempt %d/%d - %s) unable to stream the workload logs, %s\n", t.u.attempt, t.u.attemptslimit, d, t.reason)

	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		if t.o.host != nil {
			errorsx.Log(t.o.host.Close())
		}
		return t.u.fail(ctx.Err())
	case <-timer.C:
		return uploadwatching{u: t.u, o: t.o}
	}
}

// uploadawaiting waits for the result of a workload whose logs cannot be streamed, the stored log is printed
// once it completes when none of it was streamed.
type uploadawaiting struct {
	u  *uploader
	id string
}

func (t uploadawaiting) Update(ctx context.Context) uploadstate {
	c := NewPushClient(t.u.httpc)

	err := computex.Await(ctx, t.id, c.HistoryFind)
	if ctx.Err() == nil && t.u.logs.n == 0 {
		errorsx.Log(computex.Logs(ctx, t.id, t.u.logs, c.HistoryLogs))
	}

	if err != nil {
		return t.u.fail(err)
	}

	return uploadcompleted{t.u}
}

type uploadcompleted struct{ u *uploader }

func (t uploadcompleted) Update(context.Context) uploadstate {
	t.u.result = nil
	return nil
}

type uploadfailed struct {
	u     *uploader
	cause error
}

func (t uploadfailed) Update(context.Context) uploadstate {
	t.u.result = t.cause
	return nil
}

// outcome is a runner that accepted the workload and the p2p host it was reached with.
type outcome struct {
	host     host.Host
	runner   *compute.Compute
	recorded *Enqueued
}

// retryreason is a brief reason the failure may pass if tried again, blank when it isn't retryable.
func retryreason(err error) string {
	var reservation circuitv2.ReservationError

	switch {
	case errors.As(err, &reservation), strings.Contains(err.Error(), pbv2.Status_NO_RESERVATION.String()):
		return "the runner has no relay reservation"
	case errors.Is(err, network.ErrReset):
		return "the runner reset the stream"
	default:
		return ""
	}
}

// retryable reports whether the failure may pass if tried again, i.e.) the runner has no relay reservation
// or reset the stream.
func retryable(err error) bool {
	return retryreason(err) != ""
}

// tryupload attempts to upload the workload to each candidate in order, returning the runner that accepted it
// and the workload as recorded. retry is set when any candidate failed for a reason that may pass or declined.
func tryupload(ctx context.Context, p2p host.Host, candidates []*compute.Compute, req *EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (accepted *compute.Compute, recorded *Enqueued, retry bool) {
	debugx.Println("direct upload candidates", len(candidates))
	for _, c := range candidates {
		recorded, err := UploadOne(ctx, p2p, c, req, archive, environ)
		if err != nil {
			if reason := retryreason(err); reason != "" {
				// the full error lists every address dialed, which is noise unless debugging.
				log.Println("direct upload failed", c.Id, c.P2Pid, reason)
				debugx.Println("direct upload failed", c.Id, c.P2Pid, err)
				retry = true
			} else {
				log.Println("direct upload failed", c.Id, c.P2Pid, err)
			}
			continue
		}

		if recorded == nil {
			log.Println("direct upload declined, the runner is likely at capacity", c.Id, c.P2Pid)
			retry = true
			continue
		}

		return c, recorded, retry
	}

	return nil, nil, retry
}

// lanknown are the live systems remembered from the local network that can be reached, by peer id.
func lanknown(ctx context.Context, db sqlx.Queryer, ttl time.Duration) (_ map[string]lan.Peer, err error) {
	if db == nil {
		return nil, errorsx.String("the local network cache is unavailable")
	}

	var remembered []lan.Peer
	if err = sqlx.ScanInto(lan.PeerLookupLive(ctx, db, int64(ttl)), &remembered); err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve the remembered systems")
	}

	known := make(map[string]lan.Peer, len(remembered))
	for _, p := range remembered {
		if !p.Disabled && len(p.AddrList()) > 0 {
			known[p.PeerID] = p
		}
	}

	return known, nil
}

// uploadlan uploads to the candidates that were remembered from the local network. it only consults the
// cache, scanning the network is done in the background.
func uploadlan(ctx context.Context, db sqlx.Queryer, ttl time.Duration, candidates []*compute.Compute, req *EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (_ outcome, retry bool, err error) {
	known, err := lanknown(ctx, db, ttl)
	if err != nil {
		return outcome{}, false, err
	}

	// the control plane ranks the candidates, keep its order.
	nearby := slices.DeleteFunc(slices.Clone(candidates), func(c *compute.Compute) bool {
		_, found := known[c.P2Pid]
		return !found
	})

	if len(nearby) == 0 {
		return outcome{}, false, errorsx.Errorf("none of the %d candidates are known on the local network (%d systems remembered)", len(candidates), len(known))
	}

	p2p, err := libp2px.NewLocalClient()
	if err != nil {
		return outcome{}, false, err
	}

	for _, c := range nearby {
		pid, err := peer.Decode(c.P2Pid)
		if err != nil {
			continue
		}

		for _, raw := range known[c.P2Pid].AddrList() {
			if addr, err := multiaddr.NewMultiaddr(raw); err == nil {
				p2p.Peerstore().AddAddr(pid, addr, peerstore.TempAddrTTL)
			}
		}
	}

	accepted, recorded, retry := tryupload(ctx, p2p, nearby, req, archive, environ)
	if accepted == nil {
		errorsx.Log(p2p.Close())
		return outcome{}, retry, errorsx.Errorf("no runner on the local network accepted the workload, %d candidates", len(nearby))
	}

	errorsx.Log(errorsx.Wrap(lan.PeerTouch(ctx, db, accepted.P2Pid).Scan(&lan.Peer{}), "unable to refresh the remembered system"))

	return outcome{host: p2p, runner: accepted, recorded: recorded}, retry, nil
}

// uploaddirect uploads to the candidates through the p2p network, using the relays when they are not directly reachable.
func uploaddirect(ctx context.Context, httpc *http.Client, candidates []*compute.Compute, req *EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (_ outcome, retry bool, err error) {
	if len(candidates) == 0 {
		return outcome{}, false, errorsx.String("no candidates")
	}

	meta, err := registration.NewPingClient(httpc).Meta(ctx)
	if err != nil {
		return outcome{}, false, errorsx.Wrap(err, "unable to retrieve p2p bootstrap addresses")
	}

	p2p, err := libp2px.NewClient(ctx, libp2px.StringsToPeers(meta.Bootstrap...)...)
	if err != nil {
		return outcome{}, false, errorsx.Wrap(err, "unable to join p2p network")
	}

	accepted, recorded, retry := tryupload(ctx, p2p, candidates, req, archive, environ)
	if accepted == nil {
		errorsx.Log(p2p.Close())
		return outcome{}, retry, errorsx.Errorf("no runner accepted the workload, %d candidates", len(candidates))
	}

	return outcome{host: p2p, runner: accepted, recorded: recorded}, retry, nil
}

// uploadenqueue uploads the workload to the cluster, the outcome has no host or runner.
func uploadenqueue(ctx context.Context, httpc *http.Client, endpoint string, enq *Enqueued, archive io.ReadSeeker) (_ outcome, err error) {
	var (
		e EnqueuedCreateResponse
	)

	if err = iox.Rewind(archive); err != nil {
		return outcome{}, errorsx.Wrap(err, "unable to rewind kernel archive")
	}

	mimetype, buf, err := NewEnqueueUpload(enq, archive)
	if err != nil {
		return outcome{}, errorsx.Wrap(err, "unable to generate multipart upload")
	}
	defer buf.Close()

	r := iox.TimeoutReader(10*time.Second, buf)
	defer r.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, r)
	if err != nil {
		return outcome{}, errorsx.Wrap(err, "unable to create kernel upload request")
	}
	req.Header.Set("Content-Type", mimetype)

	debugx.Println("upload initiated", endpoint)
	resp, err := httpx.AsError(httpc.Do(req)) //nolint:golint,bodyclose
	defer httpx.TryClose(resp)
	debugx.Println("upload completed", endpoint)

	if err != nil {
		return outcome{}, errorsx.Wrap(err, "unable to upload kernel for processing")
	}

	if err = jsonx.UnmarshalRead(resp.Body, &e); err != nil {
		return outcome{}, errorsx.Wrap(err, "unable to decode response")
	}

	if e.Enqueued == nil {
		return outcome{}, errorsx.New("enqueued response missing workload")
	}

	debugx.Println("enqueued", spew.Sdump(&e))

	return outcome{recorded: e.Enqueued}, nil
}

// uploadwatch streams the workload's logs from the offset and reports how the workload ended, nil when it succeeded.
// the workload is already enqueued; an error here must not trigger another upload, failing to stream is a streamfailure.
func uploadwatch(ctx context.Context, httpc *http.Client, o outcome, offset int64, dst io.Writer) error {
	if stringsx.Blank(o.recorded.LogsToken) {
		log.Println("runner does not support log streaming", o.runner.Id, o.runner.P2Pid)
		return computex.Await(ctx, o.recorded.Id, NewPushClient(httpc).HistoryFind)
	}

	if err := FollowLogs(ctx, o.host, o.recorded.LogsToken, o.recorded.Id, offset, dst); err != nil {
		return streamfailure{errorsx.Wrap(err, "unable to stream workload logs")}
	}

	// the runner records the outcome itself, the control plane is only consulted when it can't answer.
	return computex.Await(
		ctx,
		o.recorded.Id,
		func(ctx context.Context, id string) (*compute.History, error) {
			return RunnerHistoryFind(ctx, o.host, o.recorded.LogsToken, id)
		},
		NewPushClient(httpc).HistoryFind,
	)
}

// FollowEnqueued streams the logs of a workload enqueued with the cluster. it waits for the cluster to
// schedule the workload on a runner, then dials that runner through the p2p network. when the workload
// completes before it can be followed its stored log is printed instead.
func FollowEnqueued(ctx context.Context, chttp *http.Client, id string) (err error) {
	return followenqueued(ctx, chttp, id, 0, os.Stdout)
}

// followenqueued follows the logs of the enqueued workload from the offset into dst. failing to stream the logs
// is a streamfailure.
func followenqueued(ctx context.Context, chttp *http.Client, id string, offset int64, dst io.Writer) (err error) {
	// how often the cluster is polled for the workload to be scheduled on a runner.
	const pollinterval = 2 * time.Second

	log.Println("waiting for the workload to start", id)
	found, err := NewPushClient(chttp).AwaitLogsToken(ctx, id, pollinterval)
	if errors.Is(err, ErrWorkloadCompleted) {
		// the stored log is the whole log, it would repeat what was already streamed.
		if offset == 0 {
			if err = computex.Logs(ctx, id, dst, NewPushClient(chttp).HistoryLogs); err != nil {
				return err
			}
		}

		return computex.Await(ctx, id, NewPushClient(chttp).HistoryFind)
	} else if errors.Is(err, ErrLogsUnavailable) {
		log.Println("unable to stream the workload logs, waiting for it to complete", err)
		return computex.Await(ctx, id, NewPushClient(chttp).HistoryFind)
	} else if err != nil {
		return streamfailure{errorsx.Wrap(err, "unable to stream workload logs")}
	}

	meta, err := registration.NewPingClient(chttp).Meta(ctx)
	if err != nil {
		return streamfailure{errorsx.Wrap(err, "unable to retrieve p2p bootstrap addresses")}
	}

	p2p, err := libp2px.NewClient(ctx, libp2px.StringsToPeers(meta.Bootstrap...)...)
	if err != nil {
		return streamfailure{errorsx.Wrap(err, "unable to join p2p network")}
	}
	defer p2p.Close()

	if err = FollowLogs(ctx, p2p, found.LogsToken, found.Id, offset, dst); err != nil {
		return streamfailure{errorsx.Wrap(err, "unable to stream workload logs")}
	}

	return computex.Await(ctx, id, NewPushClient(chttp).HistoryFind)
}
