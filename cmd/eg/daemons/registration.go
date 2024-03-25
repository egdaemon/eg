package daemons

import (
	"fmt"
	"log"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/egdaemon/eg/internal/systemx"
	"github.com/egdaemon/eg/notary"
	"github.com/egdaemon/eg/registration"
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
			signed, err = jwt.NewWithClaims(
				notary.NewJWTSigner(),
				jwtx.NewJWTClaims(
					machineid,
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
		var (
			authzedts time.Time
		)

		regreq := &registration.RegistrationRequest{
			Registration: &registration.Registration{
				Description: fmt.Sprintf("%s - %s", systemx.HostnameOrDefault("unknown.eg.lan"), fingerprint),
				Os:          runtimecfg.OS,
				Arch:        runtimecfg.Arch,
				Cores:       runtimecfg.Cores,
				Memory:      runtimecfg.Memory,
				Publickey:   ssh.MarshalAuthorizedKey(s.PublicKey()),
				Labels:      []string{},
			},
		}

		reg, err := rc.Registration(global.Context, regreq)
		if err != nil {
			log.Println("registration failed", err)
			continue
		}

		if authzedts, err = time.Parse(time.RFC3339Nano, reg.Registration.AuthzedAt); err != nil {
			log.Println("unable to parse authzed timestamp", err)
			continue
		}

		if authzedts.After(time.Now()) {
			insecure := ""
			if tlsc.Insecure {
				insecure = " --insecure"
			}
			log.Printf("waiting for registration to be accepted. run `eg actl authorize --id='%s'%s` to accept\n", reg.Registration.Id, insecure)
			continue
		}

		log.Println("registration accepted", spew.Sdump(reg))
		return nil
	}

	return nil
}
