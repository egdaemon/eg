package daemons

import (
	"log"

	"github.com/james-lawrence/eg/cmd/cmdopts"
	"golang.org/x/crypto/ssh"
)

func Register(global *cmdopts.Global, aid string, pkey ssh.PublicKey) (err error) {
	fingerprint := ssh.FingerprintSHA256(pkey)
	log.Println("registering daemon with control plane initiated", aid, fingerprint)
	defer log.Println("registering daemon with control plane completed", aid, fingerprint)

	return nil
}
