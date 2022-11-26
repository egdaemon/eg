package sshx

import (
	"bytes"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"strings"

	"golang.org/x/crypto/ssh"
)

func FingerprintSHA256(d []byte) string {
	digest := sha256.Sum256(d)
	return hex.EncodeToString(digest[:])
}

// PublicKey returns a public key from the pem encoded private key.
func PublicKey(pemkey []byte) (pub []byte, err error) {
	var (
		pkey   *rsa.PrivateKey
		pubkey ssh.PublicKey
	)

	blk, _ := pem.Decode(pemkey) // assumes a single valid pem encoded key.

	if pkey, err = x509.ParsePKCS1PrivateKey(blk.Bytes); err != nil {
		return pub, err
	}

	// log.Println("PUBKEY", spew.Sdump(pkey.N), spew.Sdump(pkey.PublicKey))
	if pubkey, err = ssh.NewPublicKey(&pkey.PublicKey); err != nil {
		return pub, err
	}

	return ssh.MarshalAuthorizedKey(pubkey), nil
}

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
