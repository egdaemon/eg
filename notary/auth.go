package notary

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/golang-jwt/jwt/v4"
	"github.com/james-lawrence/eg/internal/errorsx"
	"github.com/james-lawrence/eg/internal/sshx"
	"github.com/james-lawrence/eg/internal/userx"
	"golang.org/x/crypto/ssh"
)

const (
	DefaultNotaryKey = "eg.auth"
)

// ErrUnauthorizedKey used when the key isn't authorized by the cluster its trying to connect too.
type ErrUnauthorizedKey struct{}

func (t ErrUnauthorizedKey) Error() string {
	path := PublicKeyPath()
	return fmt.Sprintf(`your key is unauthorized and will need to be added by an authorized user.
please give the following file to an authorized user "%s".`, path)
}

// Format override standard error formatting.
func (t ErrUnauthorizedKey) Format(s fmt.State, verb rune) {
	io.WriteString(s, t.Error())
}

type keygen interface {
	Generate() (epriv, epub []byte, err error)
}

// PublicKeyPath generates the path to the public key on disk for
// a client.
func PublicKeyPath() string {
	return userx.DefaultUserDirLocation(DefaultNotaryKey) + ".pub"
}

// PrivateKeyPath generates the path to the private key on disk for
// a client.
func PrivateKeyPath() string {
	return userx.DefaultUserDirLocation(DefaultNotaryKey)
}

// ClearAutoSignerKey clears the autosigner from disk.
func ClearAutoSignerKey() error {
	return errorsx.Compact(
		os.Remove(PrivateKeyPath()),
		os.Remove(PublicKeyPath()),
	)
}

// NewAutoSigner - loads or generates a ssh key to sign RPC requests with.
// this method is only for use by clients and the new key will need to be added to the cluster.
func NewAutoSigner(comment string, kgen keygen) (s Signer, err error) {
	return newAutoSignerPath(userx.DefaultUserDirLocation(DefaultNotaryKey), comment, kgen)
}

// AutoSignerInfo returns the fingerprint and authorized ssh key.
func AutoSignerInfo() (fp string, pub []byte, err error) {
	var (
		pubk    ssh.PublicKey
		encoded []byte
	)

	if encoded, err = os.ReadFile(PublicKeyPath()); err != nil {
		return fp, pub, err
	}

	if pubk, _, _, _, err = ssh.ParseAuthorizedKey(encoded); err != nil {
		return fp, pub, err
	}

	return ssh.FingerprintSHA256(pubk), encoded, nil
}

func newAutoSignerPath(location string, comment string, kgen keygen) (s Signer, err error) {
	var (
		ss ssh.Signer
	)

	if ss, err = sshx.AutoCached(kgen, location); err != nil {
		return s, err
	}

	return Signer{
		fingerprint: ssh.FingerprintSHA256(ss.PublicKey()),
		signer:      ss,
	}, nil
}

// NewSigner a request signer from a private key.
func NewSigner(pkey []byte) (s Signer, err error) {
	var (
		ss ssh.Signer
	)

	if ss, err = ssh.ParsePrivateKey(pkey); err != nil {
		return s, err
	}

	return Signer{
		fingerprint: ssh.FingerprintSHA256(ss.PublicKey()),
		signer:      ss,
	}, nil
}

// Signer implements grpc's credentials.PerRPCCredentials
type Signer struct {
	fingerprint string
	signer      ssh.Signer
}

// AutoSignerInfo returns the fingerprint and authorized ssh key.
func (t Signer) AutoSignerInfo() (fp string, pub []byte, err error) {
	return t.fingerprint, ssh.MarshalAuthorizedKey(t.signer.PublicKey()), nil
}

func (t Signer) PublicKey() ssh.PublicKey {
	return t.signer.PublicKey()
}

func NewJWTSigner() jwt.SigningMethod {
	return jwtsigner{}
}

type jwtsigner struct{}

func (t jwtsigner) Verify(signingString, signature string, key interface{}) error {
	var (
		err    error
		sigb   []byte
		pubkey ssh.PublicKey
		sig    ssh.Signature
		ok     bool
	)

	if pubkey, ok = key.(ssh.PublicKey); !ok {
		return jwt.ErrInvalidKeyType
	}

	// Decode the signature
	if sigb, err = jwt.DecodeSegment(signature); err != nil {
		return err
	}

	if err = ssh.Unmarshal(sigb, &sig); err != nil {
		return err
	}

	if err = pubkey.Verify([]byte(signingString), &sig); err != nil {
		return err
	}

	return nil
}

func (t jwtsigner) Sign(signingString string, key interface{}) (string, error) {
	var (
		s    ssh.Signer
		sigb []byte
		ok   bool
	)

	if s, ok = key.(ssh.Signer); !ok {
		log.Printf("DERP %T\n", key)
		return "", jwt.ErrInvalidKeyType
	}

	// Sign the string and return the encoded result
	sig, err := s.Sign(rand.Reader, []byte(signingString))
	if err != nil {
		return "", err
	}

	sigb = ssh.Marshal(sig)

	return jwt.EncodeSegment(sigb), nil
}

func (t jwtsigner) Alg() string {
	return "ssh"
}
