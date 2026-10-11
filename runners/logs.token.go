package runners

import (
	"time"

	"github.com/egdaemon/eg/internal/jwtx"
	"github.com/golang-jwt/jwt/v4"
)

// LogClaims authorize streaming the logs of a single workload (wid) from a
// runner. the control plane signs them with the secret the runner sent at
// registration (aud: runner compute id, iss: account, sub: profile), and the
// runner verifies them locally.
type LogClaims struct {
	jwt.RegisteredClaims
	Workload string `json:"wid,omitempty"`
}

// SignLogToken issues a log token for workload wid valid for ttl.
func SignLogToken(secret jwtx.JWTSecretSource, wid string, ttl time.Duration, options ...jwtx.Option) (string, error) {
	return jwtx.Signed(secret(), LogClaims{
		RegisteredClaims: jwtx.NewJWTClaims(wid, append([]jwtx.Option{jwtx.ClaimsOptionExpiration(ttl)}, options...)...),
		Workload:         wid,
	})
}
