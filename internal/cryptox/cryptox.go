package cryptox

import (
	"crypto/md5"
	"crypto/sha512"
	"math/rand/v2"
)

func NewChaCha8[T ~[]byte | string](seed T) *rand.ChaCha8 {
	var (
		vector [32]byte
		source = []byte(seed)
	)

	v1 := md5.Sum(source)
	v2 := md5.Sum(append(v1[:], source...))
	copy(vector[:15], v1[:])
	copy(vector[16:], v2[:])

	return rand.NewChaCha8(vector)
}

// NewPRNGSHA512 generate a csprng using sha512.
func NewPRNGSHA512(seed []byte) *sha512csprng {
	digest := sha512.Sum512(seed)
	return &sha512csprng{
		state: digest[:],
	}
}

type sha512csprng struct {
	state []byte
}

func (t *sha512csprng) Read(b []byte) (n int, err error) {
	for i := len(b); i > 0; i = i - len(t.state) {
		t.state = t.update(t.state)

		random := t.state
		if i < len(t.state) {
			random = t.state[:i]
		}

		n += copy(b[n:], random)
	}

	return n, nil
}

func (t *sha512csprng) update(state []byte) []byte {
	digest := sha512.Sum512(state)
	return digest[:]
}
