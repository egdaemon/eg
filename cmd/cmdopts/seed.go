package cmdopts

import (
	"encoding/base64"
	"io"
	"math/rand/v2"

	"github.com/egdaemon/eg/internal/cryptox"
	"github.com/egdaemon/eg/internal/langx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/gofrs/uuid/v5"
)

// Entropy derives a long, high-entropy, base64-encoded value from a raw seed
// string, falling back to a fresh random UUID when the seed is empty. Bound
// once via kong.Bind in main.go and taken as an explicit Run parameter by
// bootstrap-env and authorize-seed, instead of each independently calling its
// own copy of this logic -- it's a pure function of a non-empty seed, so
// giving the same raw seed to both commands (each invoking the same injected
// Entropy) yields the identical derived value, with no shared state required
// between the two sibling commands.
type Entropy func(seed string) (string, error)

// GenerateEntropy is the production implementation bound to Entropy in main.go.
func GenerateEntropy(seed string) (string, error) {
	prng := cryptox.NewChaCha8(langx.FirstNonZero(seed, uuid.Must(uuid.NewV4()).String()))
	b, err := io.ReadAll(io.LimitReader(prng, rand.New(prng).Int64N(128)))
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// KeyGenSeeded builds the ed25519 identity generator for a given seed string.
// Bound once via kong.Bind in main.go and taken as an explicit Run parameter
// by authorize-seed and daemon, instead of each calling sshx.NewKeyGenSeeded
// directly -- same reasoning as Entropy above.
type KeyGenSeeded func(seed string) *sshx.KeyGen
