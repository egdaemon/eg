package cmdopts

import (
	"crypto/rand"

	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/jwtx"
)

// EnvironSecret is the secret that signs the tokens authenticating requests for the environment
// file a workload was provided, see interp/environ. it is generated once per process, held only
// in memory, and bound as a singleton via kong.BindSingletonProvider in main.go.
type EnvironSecret struct {
	secret []byte
}

func NewEnvironSecret() (*EnvironSecret, error) {
	b := make([]byte, 64)
	if _, err := rand.Read(b); err != nil {
		return nil, errorsx.Wrap(err, "unable to generate environ secret")
	}

	return &EnvironSecret{secret: b}, nil
}

// Source is the secret in the form the jwtx package consumes.
func (t *EnvironSecret) Source() jwtx.JWTSecretSource {
	return func() []byte { return t.secret }
}
