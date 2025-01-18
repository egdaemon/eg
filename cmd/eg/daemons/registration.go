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
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/systemx"
	"github.com/egdaemon/eg/notary"
	"github.com/egdaemon/eg/runners/registration"
	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
	"golang.org/x/time/rate"

	"github.com/libp2p/go-libp2p/core/peer"
)

func genregistration(s ssh.Signer, p2pid peer.ID, runtimecfg *cmdopts.RuntimeResources) *registration.Registration {
	return &registration.Registration{
		P2Pid:       p2pid.String(),
		Description: fmt.Sprintf("%s - %s", systemx.HostnameOrDefault("unknown.eg.lan"), ssh.FingerprintSHA256(s.PublicKey())),
		Os:          runtimecfg.OS,
		Arch:        runtimecfg.Arch,
		Cores:       runtimecfg.Cores,
		Memory:      runtimecfg.Memory,
		Publickey:   s.PublicKey().Marshal(),
		Labels:      append([]string{}, runtimecfg.Labels...),
	}
}

func Register(global *cmdopts.Global, tlsc *cmdopts.TLSConfig, runtimecfg *cmdopts.RuntimeResources, aid, machineid string, p2pid peer.ID, s ssh.Signer) (err error) {
	fingerprint := ssh.FingerprintSHA256(s.PublicKey())
	log.Println("registering daemon with control plane initiated", aid, machineid, fingerprint, p2pid.String())
	defer log.Println("registering daemon with control plane completed", aid, machineid, fingerprint)

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

	rc := registration.NewRegistrationClient(c)

	r := rate.NewLimiter(rate.Every(5*time.Second), 1)

	for err := r.Wait(global.Context); err == nil; err = r.Wait(global.Context) {
		var (
			unrecoverable errorsx.Unrecoverable
			authzedts     time.Time
		)

		regreq := &registration.RegistrationRequest{
			Registration: genregistration(s, p2pid, runtimecfg),
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
			debugx.Println("authzed timestamp", authzedts, "<", ts)
			log.Printf("waiting for registration to be accepted. run `eg actl authorize id '%s'` to accept\n", md5x.String(fingerprint))
			continue
		}

		log.Println("registration accepted", spew.Sdump(reg))
		return nil
	}

	return nil
}
