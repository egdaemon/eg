package daemons

import (
	"context"
	"log"
	"time"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/runners/registration"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
)

func Ping(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, runtimecfg *cmdopts.RuntimeResources, aid, machineid string, p2pid peer.ID, s ssh.Signer) (err error) {
	fingerprint := ssh.FingerprintSHA256(s.PublicKey())
	log.Println("periodic ping initiated", aid, machineid, fingerprint)
	defer log.Println("periodic ping completed", aid, machineid, fingerprint)

	if stringsx.Blank(aid) {
		return errorsx.String("an account id is required to register the daemon")
	}

	tokensrc := compute.NewAuthzTokenSource(tlsc.DefaultClient(), s, authn.EndpointCompute())
	authclient := oauth2.NewClient(
		context.WithValue(gctx.Context, oauth2.HTTPClient, tlsc.DefaultClient()),
		tokensrc,
	)

	rc := registration.NewPingClient(authclient)

	r := rate.NewLimiter(rate.Every(envx.Duration(5*time.Minute, eg.EnvPingMinimumDelay)), 1)

	req := registration.PingRequest{
		Registration: genregistration(s, p2pid, runtimecfg),
	}

	for err := r.Wait(gctx.Context); err == nil; err = r.Wait(gctx.Context) {
		if _, cause := rc.Request(gctx.Context, machineid, &req); cause != nil {
			log.Println("ping failed", cause)
			continue
		}
	}

	return nil
}
