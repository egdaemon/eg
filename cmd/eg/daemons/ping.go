package daemons

import (
	"log"
	"sync/atomic"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/libp2px"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"
)

// uploadable advertises to the control plane whether this runner accepts workloads uploaded directly to it.
func Ping(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, runtimecfg *cmdopts.RuntimeResources, aid, machineid string, p2p host.Host, relays *atomic.Pointer[[]peer.AddrInfo], s ssh.Signer, uploadable bool) (err error) {
	fingerprint := ssh.FingerprintSHA256(s.PublicKey())
	log.Println("periodic ping initiated", aid, machineid, fingerprint)
	defer log.Println("periodic ping completed", aid, machineid, fingerprint)

	if stringsx.Blank(aid) {
		return errorsx.String("an account id is required to register the daemon")
	}

	authclient := authn.New(gctx.Context, tlsc.DefaultClient(), s, aid, compute.NewAuthzTokenSource)

	rc := registration.NewPingClient(authclient)

	r := rate.NewLimiter(rate.Every(envx.Duration(15*time.Minute, eg.EnvPingMinimumDelay)), 1)

	req := registration.PingRequest{
		Registration: genregistration(s, p2p.ID(), runtimecfg),
		Uploadable:   uploadable,
	}

	for err := r.Wait(gctx.Context); err == nil; err = r.Wait(gctx.Context) {
		if resp, cause := rc.Request(gctx.Context, machineid, &req); cause != nil {
			log.Println("ping failed", cause)
			continue
		} else {
			// connect to new bootstrapping servers when detected
			errorsx.Log(
				errorsx.Wrap(
					libp2px.Connect(gctx.Context, p2p, libp2px.StringsToPeers(resp.Bootstrap...)...),
					"unable to connect to bootstrap node",
				),
			)

			// publish the current relays; the keeper started at daemon startup reserves
			// with them so clients can reach us via them.
			relays.Store(new(libp2px.StringsToPeers(resp.Bootstrap...)))
		}
	}

	return nil
}
