package sshx

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"os"
	"strings"

	"golang.org/x/crypto/ssh"
)

// IsNoKeyFound check if ssh key is not found.
func IsNoKeyFound(err error) bool {
	return err.Error() == "ssh: no key found"
}

// Comment adds comment to the ssh public key.
func Comment(encoded []byte, comment string) []byte {
	if strings.TrimSpace(comment) == "" {
		return encoded
	}

	comment = " " + comment + "\r\n"
	return append(bytes.TrimSpace(encoded), []byte(comment)...)
}

type option func(*KeyGen)

func OptionKeyGenRand(src io.Reader) option {
	return func(kg *KeyGen) {
		kg.src = src
	}
}

func NewKeyGen(options ...option) *KeyGen {
	kg := KeyGen{
		src: rand.Reader,
	}

	for _, opt := range options {
		opt(&kg)
	}

	return &kg
}

type KeyGen struct {
	src io.Reader
}

func (t KeyGen) Generate() (epriv, epub []byte, err error) {
	var (
		priv   ed25519.PrivateKey
		pub    ed25519.PublicKey
		pubkey ssh.PublicKey
		mpriv  []byte
	)

	if pub, priv, err = ed25519.GenerateKey(rand.Reader); err != nil {
		return nil, nil, err
	}

	if pubkey, err = ssh.NewPublicKey(pub); err != nil {
		return nil, nil, err
	}

	if mpriv, err = x509.MarshalPKCS8PrivateKey(priv); err != nil {
		return nil, nil, err
	}

	pemKey := &pem.Block{
		Type:  "PRIVATE KEY",
		Bytes: mpriv,
	}

	return pem.EncodeToMemory(pemKey), ssh.MarshalAuthorizedKey(pubkey), nil
}

type keygen interface {
	Generate() (epriv, epub []byte, err error)
}

func loadcached(path string) (s ssh.Signer, err error) {
	var (
		privencoded []byte
	)

	if privencoded, err = os.ReadFile(path); err != nil {
		return nil, err
	}

	return ssh.ParsePrivateKey(privencoded)
}

func AutoCached(kg keygen, path string) (s ssh.Signer, err error) {
	var (
		privencoded, pubencoded []byte
	)

	if s, err = loadcached(path); err == nil {
		return s, nil
	}

	if privencoded, pubencoded, err = kg.Generate(); err != nil {
		return nil, err
	}

	if s, err = ssh.ParsePrivateKey(privencoded); err != nil {
		return nil, err
	}

	if err = os.WriteFile(path, privencoded, 0600); err != nil {
		return nil, err
	}

	if err = os.WriteFile(fmt.Sprintf("%s.pub", path), pubencoded, 0600); err != nil {
		return nil, err
	}

	return s, err
}
