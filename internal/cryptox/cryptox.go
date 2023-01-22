package cryptox

import "crypto/sha512"

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
