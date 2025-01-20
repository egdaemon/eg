package jwtx

import (
	"crypto/rand"

	"github.com/golang-jwt/jwt/v4"
	"golang.org/x/crypto/ssh"
)

func NewSSHSigner() jwt.SigningMethod {
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
