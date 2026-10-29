package compute

import (
	"context"
	"database/sql"
	"io"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/computeapi"

	"github.com/egdaemon/eg/internal/asynccompute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/lan"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/sqlx"
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
	Attempts       int           `name:"attempts" help:"how many times to look for a runner to accept the workload before giving up, waiting longer between each" default:"8" env:"EG_UPLOAD_ATTEMPTS"`
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
// and follows it when requested. see computeapi.Upload.
func (t uploadoptions) submit(ctx context.Context, chttp *http.Client, endpoint string, lancache *sql.DB, enq *computeapi.Enqueued, archiveio, environio io.ReadSeeker) error {
	// avoid wrapping a nil cache in a non-nil interface.
	var db sqlx.Queryer
	if lancache != nil {
		db = lancache
	}

	return computeapi.Upload(
		ctx,
		chttp,
		db,
		enq,
		archiveio,
		environio,
		computeapi.UploadEndpoint(endpoint),
		computeapi.UploadPriorities(t.LAN, t.P2P),
		computeapi.UploadP2POnly(t.P2POnly),
		computeapi.UploadFollow(t.Follow),
		computeapi.UploadLANPeerTTL(t.LANPeerTTL),
		computeapi.UploadMaxAttempts(t.Attempts),
	)
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

// followenqueued streams the logs of a workload enqueued with the cluster. it waits for the cluster to
// schedule the workload on a runner, then dials that runner through the p2p network. when the workload
// completes before it can be followed its stored log is printed instead.
func (t uploadoptions) followenqueued(ctx context.Context, chttp *http.Client, id string) (err error) {
	if !t.Follow {
		return nil
	}

	return followenqueued(ctx, chttp, id)
}

// followenqueued streams the logs of a workload enqueued with the cluster, see computeapi.FollowEnqueued.
func followenqueued(ctx context.Context, chttp *http.Client, id string) (err error) {
	return computeapi.FollowEnqueued(ctx, chttp, id)
}
