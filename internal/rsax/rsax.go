package rsax

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"io"
	"os"
	"path/filepath"

	"github.com/james-lawrence/eg/internal/systemx"
	"github.com/pkg/errors"
)

const (
	defaultBits = 4096 // 4096 bit keysize, reasonable default.
)

func AutoBits() int {
	return defaultBits
}

// Auto generates a RSA key using package defined defaults.
func Auto() (pkey []byte, err error) {
	return Generate(defaultBits)
}

// CachedAuto loads/generates an RSA key at the provided filepath.
func CachedAuto(path string) (pkey []byte, err error) {
	if systemx.FileExists(path) {
		return os.ReadFile(path)
	}

	if pkey, err = Auto(); err != nil {
		return nil, err
	}

	if err = os.WriteFile(path, pkey, 0600); err != nil {
		return nil, err
	}

	return pkey, nil
}

// CachedAutoDeterministic loads/generates an RSA key at the provided filepath.
func CachedAutoDeterministic(seed []byte, path string) (pkey []byte, err error) {
	if systemx.FileExists(path) {
		return os.ReadFile(path)
	}

	if pkey, err = AutoDeterministic(seed); err != nil {
		return nil, err
	}

	if err = os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}

	if err = os.WriteFile(path, pkey, 0600); err != nil {
		return nil, err
	}

	return pkey, nil
}

// CachedGenerate loads/generates an SSH key at the provided filepath.
func CachedGenerate(path string, bits int) (pkey []byte, err error) {
	if systemx.FileExists(path) {
		return os.ReadFile(path)
	}

	if pkey, err = Generate(bits); err != nil {
		return nil, err
	}

	if err = os.WriteFile(path, pkey, 0600); err != nil {
		return nil, err
	}

	return pkey, nil
}

// AutoDerive ...
func AutoDeterministic(key []byte) (pkey []byte, err error) {
	return Deterministic(key, defaultBits)
}

// Deterministic rsa private key based on the seed. uses a SHA512 hash as
// a csprng.
func Deterministic(seed []byte, bits int) (pkey []byte, err error) {
	return generate(NewSHA512CSPRNG(seed), bits, deterministicGenerateKey)
}

// UnsafeAuto generates a ssh key using unsafe defaults, this method is used to
// generate an ssh key quickly for cases were we do not care about safety, i.e.
// tests.
func UnsafeAuto() (pkey []byte, err error) {
	return Generate(128)
}

// Generate a RSA private key with the given bits size, returns the pem encoded bytes.
func Generate(bits int) (encoded []byte, err error) {
	return generate(rand.Reader, bits, rsa.GenerateKey)
}

func generate(r io.Reader, bits int, gen func(io.Reader, int) (*rsa.PrivateKey, error)) (encoded []byte, err error) {
	var (
		pkey *rsa.PrivateKey
	)

	if pkey, err = private(r, bits, gen); err != nil {
		return encoded, err
	}

	// Get ASN.1 DER format
	marshalled := x509.MarshalPKCS1PrivateKey(pkey)

	return pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: marshalled,
	}), nil
}

// generatePrivateKey creates a RSA Private Key of specified byte size
func private(r io.Reader, bits int, gen func(io.Reader, int) (*rsa.PrivateKey, error)) (k *rsa.PrivateKey, err error) {
	// Private Key generation
	if k, err = gen(r, bits); err != nil {
		return k, err
	}

	// Validate Private Key
	return k, k.Validate()
}

// PublicKey returns a public key from the pem encoded private key.
func PublicKey(pemkey []byte) (pub []byte, err error) {
	var (
		pkey *rsa.PrivateKey
	)

	blk, _ := pem.Decode(pemkey) // assumes a single valid pem encoded key.

	if pkey, err = x509.ParsePKCS1PrivateKey(blk.Bytes); err != nil {
		return pub, err
	}

	return x509.MarshalPKCS1PublicKey(&pkey.PublicKey), nil
}

func DecodeFile(path string) (priv *rsa.PrivateKey, err error) {
	var (
		encoded []byte
	)

	if encoded, err = os.ReadFile(path); err != nil {
		return nil, err
	}

	return Decode(encoded)
}

// Decode decode a RSA private key.
func Decode(encoded []byte) (priv *rsa.PrivateKey, err error) {
	b, _ := pem.Decode(encoded)
	if priv, err = x509.ParsePKCS1PrivateKey(b.Bytes); err != nil {
		return nil, errors.WithStack(err)
	}

	return priv, nil
}

// DecodePKCS1PrivateKey decode PEM to x509.PKCS1PrivateKey bytes
func DecodePKCS1PrivateKey(encoded []byte) []byte {
	b, _ := pem.Decode(encoded)
	return b.Bytes
}

// MaybeDecode decodes RSA from an encoded array and possible error.
func MaybeDecode(encoded []byte, err error) (priv *rsa.PrivateKey, _ error) {
	if err != nil {
		return priv, err
	}
	return Decode(encoded)
}

// FingerprintSHA256 of the key
func FingerprintSHA256(b []byte) string {
	digest := sha256.Sum256(b)
	return hex.EncodeToString(digest[:])
}
