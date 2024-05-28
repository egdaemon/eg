package daemons

import (
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/notary"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"
)

func Ping(global *cmdopts.Global, tlsc *cmdopts.TLSConfig, runtimecfg *cmdopts.RuntimeResources, aid, machineid string, s ssh.Signer) (err error) {
	fingerprint := ssh.FingerprintSHA256(s.PublicKey())
	log.Println("periodic ping initiated", aid, machineid, fingerprint)
	defer log.Println("periodic ping  completed", aid, machineid, fingerprint)

	if stringsx.Blank(aid) {
		return errorsx.String("an account id is required to register the daemon")
	}

	c := jwtx.NewHTTP(
		tlsc.DefaultClient(),
		jwtx.SignerFn(func() (signed string, err error) {
			claims := jwtx.NewJWTClaims(
				machineid,
				jwtx.ClaimsOptionAuthnExpiration(),
				jwtx.ClaimsOptionIssuer(aid),
			)

			debugx.Println("claims", spew.Sdump(claims))

			return jwt.NewWithClaims(
				notary.NewJWTSigner(),
				claims,
			).SignedString(s)
		}),
	)

	rc := registration.NewPingClient(c)

	r := rate.NewLimiter(rate.Every(15*time.Minute), 1)

	req := registration.PingRequest{
		Registration: genregistration(s, runtimecfg),
	}

	for err := r.Wait(global.Context); err == nil; err = r.Wait(global.Context) {
		if _, cause := rc.Request(global.Context, machineid, &req); cause != nil {
			log.Println("ping failed", cause)
			continue
		}
	}

	return nil
}
