package daemons

import (
	"crypto/tls"
	"log"
	"net/http"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/golang-jwt/jwt/v4"
	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/jwtx"
	"github.com/james-lawrence/eg/notary"
	"github.com/james-lawrence/eg/registration"
	"golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"
)

func Register(global *cmdopts.Global, aid string, s ssh.Signer) (err error) {
	fingerprint := ssh.FingerprintSHA256(s.PublicKey())
	log.Println("registering daemon with control plane initiated", aid, fingerprint)
	defer log.Println("registering daemon with control plane completed", aid, fingerprint)

	ctransport := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}

	c := jwtx.NewHTTP(
		&http.Client{Transport: ctransport, Timeout: 10 * time.Second},
		jwtx.SignerFn(func() (signed string, err error) {
			signed, err = jwt.NewWithClaims(
				notary.NewJWTSigner(),
				jwtx.NewJWTClaims(
					fingerprint,
					jwtx.ClaimsOptionAuthnExpiration(),
					jwtx.ClaimsOptionIssuer(aid),
				),
			).SignedString(s)
			if err != nil {
				return "", err
			}

			return signed, err
		}),
	)

	rc := registration.NewRegistrationClient(c)

	r := rate.NewLimiter(rate.Every(5*time.Second), 1)

	for err := r.Wait(global.Context); err == nil; err = r.Wait(global.Context) {
		regreq := &registration.RegistrationRequest{
			Id:        fingerprint,
			Labels:    []string{"linux"},
			Publickey: ssh.MarshalAuthorizedKey(s.PublicKey()),
		}

		reg, err := rc.Registration(global.Context, regreq)

		if err == nil {
			log.Println("registration accepted", spew.Sdump(reg))
			return nil
		}

		switch err.(type) {
		case errorsx.Temporary:
			log.Println("waiting for registration to be authorized")
		default:
			log.Println("registration failed", err)
		}
	}

	return nil
}