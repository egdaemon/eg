package compute

import (
	"cmp"
	"context"
	"database/sql"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/computeapi"
	"github.com/egdaemon/eg/jsonx"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/asynccompute"
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
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/peerstore"
	"github.com/multiformats/go-multiaddr"
	"google.golang.org/protobuf/proto"
)

// uploadoptions are the flags of the commands that upload a workload to a runner: straight to one when
// possible, otherwise through the cluster, optionally following the workload once it is running.
type uploadoptions struct {
	P2P            int           `name:"p2p" help:"priority of uploading directly to a runner the control plane suggests before falling back to the cluster, higher goes first, 0 disables" default:"1" env:"EG_P2P_PRIORITY"`
	LAN            int           `name:"lan" help:"priority of uploading directly to a runner remembered from the local network (see 'eg compute lan'), higher goes first, 0 disables including scanning for runners" default:"2" env:"EG_LAN_PRIORITY"`
	P2POnly        bool          `name:"p2p-only" help:"never fall back to uploading to the cluster, fail if no runner accepts a direct upload" env:"EG_P2P_ONLY"`
	CacheDir       string        `name:"cache-directory" help:"directory the remembered systems on the local network are stored in" default:"${vars_cache_directory}" env:"EG_CACHE_DIRECTORY"`
	LANScanTimeout time.Duration `name:"lan-scan-timeout" help:"how long the background scan of the local network runs for" default:"3s"`
	LANPeerTTL     time.Duration `name:"lan-peer-ttl" type:"durationinf" help:"how long systems on the local network are remembered after they were last seen" default:"720h" env:"EG_LAN_PEER_TTL"`
	Follow         bool          `name:"follow" help:"stream the workload's logs once it is running, requires a runner supporting log streaming" negatable:"" default:"true"`
}

// remember scans the local network for runners and remembers them while the workload is being built, the scan
// only feeds the cache so it never delays the upload. the cache is optional, nil when unavailable.
// stop must be called exactly once.
func (t uploadoptions) remember(ctx context.Context, find libp2px.MDNSLookup) (lancache *sql.DB, stop func()) {
	if t.LAN <= 0 {
		return nil, func() {}
	}

	lancache, err := openlan(ctx, filepath.Join(t.CacheDir, migrations.CLI), t.LANPeerTTL)
	if err != nil {
		log.Println(errorsx.Wrap(err, "the local network cache is unavailable"))
		return nil, func() {}
	}

	stopscan := autoscan(ctx, lancache, find, t.LANScanTimeout)
	return lancache, func() {
		stopscan()
		errorsx.Log(lancache.Close())
	}
}

// submit uploads the workload to a runner directly when enabled and possible, otherwise to the cluster at endpoint,
// and follows it when requested.
func (t uploadoptions) submit(ctx context.Context, chttp *http.Client, endpoint string, lancache *sql.DB, enq *computeapi.Enqueued, archiveio, environio io.ReadSeeker) error {
	if order := priorities(t.LAN, t.P2P); len(order) > 0 {
		// a nil runner means nothing accepted the workload. otherwise it was uploaded and
		// the error, if any, is the result of following it, so we must not upload it again.
		accepted, cause := t.p2p(ctx, chttp, lancache, order, enq, archiveio, environio)
		if accepted != nil {
			log.Println("enqueued directly", enq.Id, accepted.Id, accepted.P2Pid)
			return cause
		}

		if t.P2POnly {
			return errorsx.Wrap(cause, "direct upload unavailable and --p2p-only is set")
		}

		log.Println("direct upload unavailable, falling back to the cluster", cause)

		if _, err := archiveio.Seek(0, io.SeekStart); err != nil {
			return errorsx.Wrap(err, "unable to rewind kernel archive")
		}
	} else if t.P2POnly {
		return errorsx.Errorf("--p2p-only is set but no direct upload strategy is enabled, see --p2p and --lan")
	}

	recorded, err := t.enqueue(ctx, chttp, endpoint, enq, archiveio)
	if err != nil {
		return err
	}

	log.Println("enqueued", spew.Sdump(recorded))

	return t.followenqueued(ctx, chttp, recorded.Enqueued.Id)
}

// enqueue uploads the workload to the cluster.
func (t uploadoptions) enqueue(ctx context.Context, chttp *http.Client, endpoint string, enq *computeapi.Enqueued, archive io.Reader) (_ *computeapi.EnqueuedCreateResponse, err error) {
	var (
		e computeapi.EnqueuedCreateResponse
	)

	mimetype, buf, err := computeapi.NewEnqueueUpload(enq, archive)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to generate multipart upload")
	}
	defer buf.Close()

	r := iox.TimeoutReader(10*time.Second, buf)
	defer r.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, r)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to create kernel upload request")
	}
	req.Header.Set("Content-Type", mimetype)

	debugx.Println("upload initiated", endpoint)
	resp, err := httpx.AsError(chttp.Do(req)) //nolint:golint,bodyclose
	defer httpx.TryClose(resp)
	debugx.Println("upload completed", endpoint)

	if err != nil {
		return nil, errorsx.Wrap(err, "unable to upload kernel for processing")
	}

	if err = jsonx.UnmarshalRead(resp.Body, &e); err != nil {
		return nil, errorsx.Wrap(err, "unable to decode response")
	}

	if e.Enqueued == nil {
		return nil, errorsx.New("enqueued response missing workload")
	}

	return &e, nil
}

// strategy is a way of reaching a runner peer to peer. uploading to the cluster is not
// one of them, it is always the last resort.
type strategy int

const (
	strategyLAN    strategy = iota // runners remembered from the local network.
	strategyDirect                 // runners the control plane suggests, reached through the p2p network.
)

func (t strategy) String() string {
	switch t {
	case strategyLAN:
		return "local network"
	default:
		return "p2p network"
	}
}

// priorities orders the enabled strategies, highest priority first. a priority of zero or less
// disables the strategy, ties favor the local network.
func priorities(lanpriority, directpriority int) []strategy {
	type ranked struct {
		strategy strategy
		priority int
	}

	enabled := slices.DeleteFunc([]ranked{{strategyLAN, lanpriority}, {strategyDirect, directpriority}}, func(r ranked) bool {
		return r.priority <= 0
	})

	slices.SortStableFunc(enabled, func(a, b ranked) int {
		return cmp.Compare(b.priority, a.priority)
	})

	return slicesx.MapTransform(func(r ranked) strategy { return r.strategy }, enabled...)
}

// autoscan looks for runners on the local network and remembers them for later uploads. it
// never blocks the caller, failures are logged. the returned function stops the scan and waits for it,
// and must be called exactly once.
func autoscan(parent context.Context, db *sql.DB, find libp2px.MDNSLookup, timeout time.Duration) (stop func()) {
	ctx, cancel := context.WithCancel(parent)

	pool := asynccompute.New(func(ctx context.Context, _ struct{}) error {
		if err := lan.Discover(ctx, db, find, timeout); err != nil && ctx.Err() == nil {
			return errorsx.Wrap(err, "unable to scan the local network")
		}

		return nil
	}, asynccompute.Compose(asynccompute.Workers[struct{}](1), asynccompute.Backlog[struct{}](1)))

	// the backlog has room for the single scan so this never blocks. the pool logs failures.
	errorsx.Log(pool.Run(ctx, struct{}{}))

	return func() {
		cancel()
		_ = pool.Close() // waits for the scan, a failure was already logged by the pool.
	}
}

// outcome is a runner that accepted the workload and the p2p host it was reached with.
type outcome struct {
	host     host.Host
	runner   *compute.Compute
	recorded *computeapi.Enqueued
}

// p2p attempts to upload the workload straight to a runner, trying the strategies in order and
// returning the runner that accepted it. the runner records the workload with the cluster using
// the candidate token issued for it. db is the cache of remembered systems and may be nil.
// a nil runner means none accepted the workload and the caller should fall back to the cluster. otherwise
// the error is the result of following the workload (see follow) and it must not be uploaded again.
func (t uploadoptions) p2p(ctx context.Context, chttp *http.Client, db *sql.DB, order []strategy, enq *computeapi.Enqueued, archive, environ io.ReadSeeker) (_ *compute.Compute, err error) {
	// clone so the caller's enqueued remains untouched for the cluster fallback.
	req := &computeapi.EnqueuedDequeueResponse{
		Enqueued: proto.Clone(enq).(*computeapi.Enqueued),
	}

	candidates, err := runners.NewPushClient(chttp).Candidates(ctx, req.Enqueued, directCandidates)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve candidate runners")
	}

	if db != nil {
		errorsx.Log(errorsx.Wrap(lan.RememberCandidates(ctx, db, candidates.Items...), "unable to remember the candidate runners"))
	}

	if len(candidates.Items) == 0 {
		return nil, errorsx.New("the control plane has no runners available that match the workload")
	}

	var failures []error
	for _, s := range order {
		var o outcome

		switch s {
		case strategyLAN:
			o, err = t.lan(ctx, db, candidates.Items, req, archive, environ)
		default:
			o, err = t.direct(ctx, chttp, candidates.Items, req, archive, environ)
		}

		if err != nil {
			failures = append(failures, errorsx.Wrapf(err, "%s upload failed", s))
			continue
		}
		defer o.host.Close()

		debugx.Println("enqueued", spew.Sdump(o.recorded))
		return o.runner, t.follow(ctx, chttp, o)
	}

	// every enabled strategy failed, report all of them so the reason for each is visible.
	return nil, errors.Join(failures...)
}

// lan uploads to the candidates that were remembered from the local network. it only consults the
// cache, scanning the network is done in the background (see autoscan).
func (t uploadoptions) lan(ctx context.Context, db *sql.DB, candidates []*compute.Compute, req *computeapi.EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (_ outcome, err error) {
	if db == nil {
		return outcome{}, errorsx.String("the local network cache is unavailable")
	}

	var remembered []lan.Peer
	if err = sqlx.ScanInto(lan.PeerLookupLive(ctx, db, int64(t.LANPeerTTL)), &remembered); err != nil {
		return outcome{}, errorsx.Wrap(err, "unable to retrieve the remembered systems")
	}

	known := make(map[string]lan.Peer, len(remembered))
	for _, p := range remembered {
		if !p.Disabled && len(p.AddrList()) > 0 {
			known[p.PeerID] = p
		}
	}

	// the control plane ranks the candidates, keep its order.
	nearby := slices.DeleteFunc(slices.Clone(candidates), func(c *compute.Compute) bool {
		_, found := known[c.P2Pid]
		return !found
	})

	if len(nearby) == 0 {
		return outcome{}, errorsx.Errorf("none of the %d candidates are known on the local network (%d systems remembered)", len(candidates), len(known))
	}

	p2p, err := libp2px.NewLocalClient()
	if err != nil {
		return outcome{}, err
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

	accepted, recorded := runners.TryUpload(ctx, p2p, nearby, req, archive, environ)
	if accepted == nil {
		errorsx.Log(p2p.Close())
		return outcome{}, errorsx.Errorf("no runner on the local network accepted the workload, %d candidates", len(nearby))
	}

	errorsx.Log(errorsx.Wrap(lan.PeerTouch(ctx, db, accepted.P2Pid).Scan(&lan.Peer{}), "unable to refresh the remembered system"))

	return outcome{host: p2p, runner: accepted, recorded: recorded}, nil
}

// direct uploads to the candidates through the p2p network, using the relays when they are not directly reachable.
func (t uploadoptions) direct(ctx context.Context, chttp *http.Client, candidates []*compute.Compute, req *computeapi.EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (_ outcome, err error) {
	meta, err := registration.NewPingClient(chttp).Meta(ctx)
	if err != nil {
		return outcome{}, errorsx.Wrap(err, "unable to retrieve p2p bootstrap addresses")
	}

	p2p, err := libp2px.NewClient(ctx, libp2px.StringsToPeers(meta.Bootstrap...)...)
	if err != nil {
		return outcome{}, errorsx.Wrap(err, "unable to join p2p network")
	}

	accepted, recorded := runners.TryUpload(ctx, p2p, candidates, req, archive, environ)
	if accepted == nil {
		errorsx.Log(p2p.Close())
		return outcome{}, errorsx.Errorf("no runner accepted the workload, %d candidates", len(candidates))
	}

	return outcome{host: p2p, runner: accepted, recorded: recorded}, nil
}

// follow streams the workload's logs when requested and reports how the workload ended, nil when it succeeded.
// the workload is already enqueued; an error here must not trigger another upload.
func (t uploadoptions) follow(ctx context.Context, chttp *http.Client, o outcome) error {
	if !t.Follow {
		return nil
	}

	if stringsx.Blank(o.recorded.LogsToken) {
		log.Println("runner does not support log streaming", o.runner.Id, o.runner.P2Pid)
		return computex.Await(ctx, o.recorded.Id, runners.NewPushClient(chttp).HistoryFind)
	}

	if err := runners.FollowLogs(ctx, o.host, o.recorded.LogsToken, o.recorded.Id, 0, os.Stdout); err != nil {
		return errorsx.Wrap(err, "unable to stream workload logs")
	}

	// the runner records the outcome itself, the control plane is only consulted when it can't answer.
	return computex.Await(
		ctx,
		o.recorded.Id,
		func(ctx context.Context, id string) (*compute.History, error) {
			return runners.RunnerHistoryFind(ctx, o.host, o.recorded.LogsToken, id)
		},
		runners.NewPushClient(chttp).HistoryFind,
	)
}

// followenqueued streams the logs of a workload enqueued with the cluster. it waits for the cluster to
// schedule the workload on a runner, then dials that runner through the p2p network. when the workload
// completes before it can be followed its stored log is printed instead.
func (t uploadoptions) followenqueued(ctx context.Context, chttp *http.Client, id string) (err error) {
	if !t.Follow {
		return nil
	}

	return followenqueued(ctx, chttp, id)
}

// followenqueued streams the logs of a workload enqueued with the cluster, see upload.followenqueued.
func followenqueued(ctx context.Context, chttp *http.Client, id string) (err error) {
	// how often the cluster is polled for the workload to be scheduled on a runner.
	const pollinterval = 2 * time.Second

	log.Println("waiting for the workload to start", id)
	found, err := runners.NewPushClient(chttp).AwaitLogsToken(ctx, id, pollinterval)
	if errors.Is(err, runners.ErrWorkloadCompleted) {
		if err = computex.Logs(ctx, id, os.Stdout, runners.NewPushClient(chttp).HistoryLogs); err != nil {
			return err
		}

		return computex.Await(ctx, id, runners.NewPushClient(chttp).HistoryFind)
	} else if errors.Is(err, runners.ErrLogsUnavailable) {
		log.Println("unable to stream the workload logs, waiting for it to complete", err)
		return computex.Await(ctx, id, runners.NewPushClient(chttp).HistoryFind)
	} else if err != nil {
		return errorsx.Wrap(err, "unable to stream workload logs")
	}

	meta, err := registration.NewPingClient(chttp).Meta(ctx)
	if err != nil {
		return errorsx.Wrap(err, "unable to retrieve p2p bootstrap addresses")
	}

	p2p, err := libp2px.NewClient(ctx, libp2px.StringsToPeers(meta.Bootstrap...)...)
	if err != nil {
		return errorsx.Wrap(err, "unable to join p2p network")
	}
	defer p2p.Close()

	if err = runners.FollowLogs(ctx, p2p, found.LogsToken, found.Id, 0, os.Stdout); err != nil {
		return errorsx.Wrap(err, "unable to stream workload logs")
	}

	return computex.Await(ctx, id, runners.NewPushClient(chttp).HistoryFind)
}
