package cryptox

import (
	"crypto/sha256"
	"math/rand/v2"
)

func NewChaCha8[T ~[]byte | string](seed T) *rand.ChaCha8 {
	return rand.NewChaCha8(sha256.Sum256([]byte(seed)))
}
