package gpgx

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	"github.com/ProtonMail/go-crypto/openpgp/packet"
	"github.com/egdaemon/eg/internal/cryptox"

	_ "crypto/sha512"
)

// DefaultDirectory returns the well-known GPG home directory for the given home directory.
func DefaultDirectory(home string) string {
	return filepath.Join(home, ".gnupg")
}

// Keyring generates a deterministic keyring from seed and places it in dir,
// loading from disk if it already exists. dir is a GPG home directory e.g. ~/.gnupg.
func Keyring(dir, seed string, options ...option) (entity *openpgp.Entity, err error) {
	if entity, err = loadkeyring(dir); err == nil {
		return entity, nil
	}

	kg := NewKeyGenSeeded(seed, options...)
	if entity, err = kg.Generate(); err != nil {
		return nil, err
	}

	return entity, savekeyring(dir, entity)
}

func loadkeyring(dir string) (*openpgp.Entity, error) {
	encoded, err := os.ReadFile(filepath.Join(dir, "private.asc"))
	if err != nil {
		return nil, err
	}

	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(encoded))
	if err != nil {
		return nil, err
	}

	if len(entities) == 0 {
		return nil, fmt.Errorf("gpgx: no key found in %s", dir)
	}

	return entities[0], nil
}

func savekeyring(dir string, entity *openpgp.Entity) (err error) {
	if err = os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	cfg := &packet.Config{}

	var privbuf bytes.Buffer
	privw, err := armor.Encode(&privbuf, openpgp.PrivateKeyType, nil)
	if err != nil {
		return err
	}
	if err = entity.SerializePrivate(privw, cfg); err != nil {
		return err
	}
	if err = privw.Close(); err != nil {
		return err
	}

	var pubbuf bytes.Buffer
	pubw, err := armor.Encode(&pubbuf, openpgp.PublicKeyType, nil)
	if err != nil {
		return err
	}
	if err = entity.Serialize(pubw); err != nil {
		return err
	}
	if err = pubw.Close(); err != nil {
		return err
	}

	if err = os.WriteFile(filepath.Join(dir, "private.asc"), privbuf.Bytes(), 0600); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(dir, "public.asc"), pubbuf.Bytes(), 0644)
}

type option func(*KeyGen)

func OptionKeyGenRand(src io.Reader) option {
	return func(kg *KeyGen) {
		kg.rand = src
	}
}

func OptionKeyGenIdentity(name, comment, email string) option {
	return func(kg *KeyGen) {
		kg.name = name
		kg.comment = comment
		kg.email = email
	}
}

func OptionKeyGenClock(fn func() time.Time) option {
	return func(kg *KeyGen) {
		kg.clock = fn
	}
}

func NewKeyGenSeeded(seed string, options ...option) *KeyGen {
	return NewKeyGen(append([]option{
		OptionKeyGenRand(cryptox.NewChaCha8(seed)),
		OptionKeyGenClock(func() time.Time { return time.Unix(0, 0) }),
	}, options...)...)
}

func UnsafeNewKeyGen() *KeyGen {
	return NewKeyGenSeeded("unsafe")
}

func NewKeyGen(options ...option) *KeyGen {
	kg := KeyGen{
		rand:  rand.Reader,
		clock: time.Now,
	}

	for _, opt := range options {
		opt(&kg)
	}

	return &kg
}

type KeyGen struct {
	rand    io.Reader
	clock   func() time.Time
	name    string
	comment string
	email   string
}

func (t KeyGen) Generate() (*openpgp.Entity, error) {
	cfg := &packet.Config{
		Rand:          t.rand,
		Time:          t.clock,
		DefaultHash:   crypto.SHA256,
		DefaultCipher: packet.CipherAES256,
		Algorithm:     packet.PubKeyAlgoEdDSA,
	}

	return openpgp.NewEntity(t.name, t.comment, t.email, cfg)
}
