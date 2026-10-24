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
	"slices"
	"time"

	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/asynccompute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/history"
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

// strategy is a way of reaching a runner peer to peer. uploading to the cluster is not
// one of them, it is always the last resort.
type strategy int

const (
	strategyLAN    strategy = iota // runners remembered from the local network.
	strategyDirect                 // runners the control plane suggests, reached through the p2p network.
)

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
	recorded *runners.Enqueued
}

// p2p attempts to upload the workload straight to a runner, trying the strategies in order and
// returning the runner that accepted it. the runner records the workload with the cluster using
// the candidate token issued for it. db is the cache of remembered systems and may be nil.
// a nil runner means none accepted the workload and the caller should fall back to the cluster. otherwise
// the error is the result of following the workload (see follow) and it must not be uploaded again.
func (t upload) p2p(ctx context.Context, chttp *http.Client, db *sql.DB, order []strategy, enq *runners.Enqueued, archive, environ io.ReadSeeker) (_ *compute.Compute, err error) {
	// clone so the caller's enqueued remains untouched for the cluster fallback.
	req := &runners.EnqueuedDequeueResponse{
		Enqueued: proto.Clone(enq).(*runners.Enqueued),
	}

	candidates, err := runners.NewPushClient(chttp).Candidates(ctx, req.Enqueued, directCandidates)
	if err != nil {
		return nil, errorsx.Wrap(err, "unable to retrieve candidate runners")
	}

	if db != nil {
		errorsx.Log(errorsx.Wrap(lan.RememberCandidates(ctx, db, candidates.Items...), "unable to remember the candidate runners"))
	}

	var failures error
	for _, s := range order {
		var o outcome

		switch s {
		case strategyLAN:
			o, err = t.lan(ctx, db, candidates.Items, req, archive, environ)
		default:
			o, err = t.direct(ctx, chttp, candidates.Items, req, archive, environ)
		}

		if err != nil {
			log.Println("upload strategy unavailable", err)
			failures = errorsx.Compact(failures, err)
			continue
		}
		defer o.host.Close()

		return o.runner, t.follow(ctx, chttp, o)
	}

	return nil, errorsx.Compact(failures, errorsx.Errorf("no runner accepted the workload, %d candidates", len(candidates.Items)))
}

// lan uploads to the candidates that were remembered from the local network. it only consults the
// cache, scanning the network is done in the background (see autoscan).
func (t upload) lan(ctx context.Context, db *sql.DB, candidates []*compute.Compute, req *runners.EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (_ outcome, err error) {
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
		return outcome{}, errorsx.Errorf("none of the %d candidates are known on the local network", len(candidates))
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
func (t upload) direct(ctx context.Context, chttp *http.Client, candidates []*compute.Compute, req *runners.EnqueuedDequeueResponse, archive, environ io.ReadSeeker) (_ outcome, err error) {
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
func (t upload) follow(ctx context.Context, chttp *http.Client, o outcome) error {
	if !t.Follow {
		return nil
	}

	if stringsx.Blank(o.recorded.LogsToken) {
		log.Println("runner does not support log streaming", o.runner.Id, o.runner.P2Pid)
		return history.Await(ctx, o.recorded.Id, runners.NewPushClient(chttp).HistoryFind)
	}

	if err := runners.FollowLogs(ctx, o.host, o.recorded.LogsToken, o.recorded.Id, 0, os.Stdout); err != nil {
		return errorsx.Wrap(err, "unable to stream workload logs")
	}

	// the runner records the outcome itself, the control plane is only consulted when it can't answer.
	return history.Await(
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
func (t upload) followenqueued(ctx context.Context, chttp *http.Client, id string) (err error) {
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
		if err = history.Logs(ctx, id, os.Stdout, runners.NewPushClient(chttp).HistoryLogs); err != nil {
			return err
		}

		return history.Await(ctx, id, runners.NewPushClient(chttp).HistoryFind)
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

	return history.Await(ctx, id, runners.NewPushClient(chttp).HistoryFind)
}
