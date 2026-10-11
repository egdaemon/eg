package daemons

import (
	"crypto/rand"
	"sync"

	"github.com/egdaemon/eg/internal/jwtx"
)

var (
	jwtsecretonce sync.Once
	jwtsecret     []byte
)

// JWTSecret is the runner's token signing secret. it is generated once per
// process, held only in memory, and sent to the control plane on registration
// so it can sign tokens (e.g. log streaming) this runner verifies locally.
// restarting the daemon rotates it, invalidating any outstanding tokens.
func JWTSecret() jwtx.JWTSecretSource {
	jwtsecretonce.Do(func() {
		jwtsecret = make([]byte, 64)
		// crypto/rand.Read never returns an error, it panics instead.
		_, _ = rand.Read(jwtsecret)
	})

	return func() []byte {
		return jwtsecret
	}
}
