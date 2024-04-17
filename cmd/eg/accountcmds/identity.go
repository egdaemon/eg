package accountcmds

import (
	"encoding/base64"
	"log"

	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/sshx"
	"golang.org/x/crypto/ssh"
)

type Identity struct {
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
}

func (t Identity) Run(gctx *cmdopts.Global, tlscfg *cmdopts.TLSConfig) (err error) {
	var (
		signer ssh.Signer
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(), t.SSHKeyPath); err != nil {
		return err
	}

	log.Println("identity", md5x.String(ssh.FingerprintSHA256(signer.PublicKey())))
	log.Println("fingerprint", ssh.FingerprintSHA256(signer.PublicKey()))
	log.Println("base64", base64.URLEncoding.EncodeToString(signer.PublicKey().Marshal()))
	return nil
}
