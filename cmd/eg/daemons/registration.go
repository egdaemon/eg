package daemons

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/systemx"
	"github.com/egdaemon/eg/notary"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"
)

func Register(global *cmdopts.Global, tlsc *cmdopts.TLSConfig, runtimecfg *cmdopts.RuntimeResources, aid, machineid string, s ssh.Signer) (err error) {
	fingerprint := ssh.FingerprintSHA256(s.PublicKey())
	log.Println("registering daemon with control plane initiated", aid, machineid, fingerprint)
	defer log.Println("registering daemon with control plane completed", aid, machineid, fingerprint)

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

	rc := registration.NewRegistrationClient(c)

	r := rate.NewLimiter(rate.Every(5*time.Second), 1)

	for err := r.Wait(global.Context); err == nil; err = r.Wait(global.Context) {
		var (
			unrecoverable errorsx.Unrecoverable
			authzedts     time.Time
		)

		regreq := &registration.RegistrationRequest{
			Registration: &registration.Registration{
				Description: fmt.Sprintf("%s - %s", systemx.HostnameOrDefault("unknown.eg.lan"), fingerprint),
				Os:          runtimecfg.OS,
				Arch:        runtimecfg.Arch,
				Cores:       runtimecfg.Cores,
				Memory:      runtimecfg.Memory,
				Publickey:   s.PublicKey().Marshal(),
				Labels:      []string{},
			},
		}

		reg, err := rc.Registration(global.Context, regreq)
		if errors.Is(err, &unrecoverable) {
			return errorsx.Wrapf(err, "encountered an unrecoverable error during registration: %s %s %s", aid, machineid, fingerprint)
		} else if err != nil {
			log.Println("registration failed", err)
			continue
		}

		if authzedts, err = time.Parse(time.RFC3339Nano, reg.Registration.AuthzedAt); err != nil {
			log.Println("unable to parse authzed timestamp", reg.Registration.AuthzedAt, err)
			continue
		}

		if ts := time.Now(); authzedts.After(ts) {
			insecure := ""
			if tlsc.Insecure {
				insecure = " --insecure"
			}
			debugx.Println("authzed timestamp", authzedts, "<", ts)
			log.Printf("waiting for registration to be accepted. run `eg actl authorize --id='%s'%s` to accept\n", md5x.String(fingerprint), insecure)
			continue
		}

		log.Println("registration accepted", spew.Sdump(reg))
		return nil
	}

	return nil
}
