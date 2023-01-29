package main

import (
	"encoding/hex"
	"log"
	"net"

	"github.com/james-lawrence/eg/cmd/cmdopts"
	"github.com/james-lawrence/eg/cmd/eg/daemons"
	"github.com/james-lawrence/eg/internal/cryptox"
	"github.com/james-lawrence/eg/internal/sshx"
	"golang.org/x/crypto/ssh"
)

type daemon struct {
	AccountID  string `name:"account" help:"account to register runner with" default:"${vars_account_id}" required:"true"`
	Seed       string `name:"secret" help:"seed for generating ssh credentials in a consistent manner"`
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	CacheDir   string `name:"directory" help:"root directory of the repository" default:"${vars_cache_directory}"`
}

// essentially we use ssh forwarding from the control plane to the local http server
// allowing the control plane to interogate
func (t daemon) Run(ctx *cmdopts.Global) (err error) {
	var (
		signer   ssh.Signer
		httpl    net.Listener
		sshproxy net.Listener
	)

	if signer, err = sshx.AutoCached(sshx.NewKeyGen(sshx.OptionKeyGenRand(cryptox.NewPRNGSHA512([]byte(t.Seed)))), t.SSHKeyPath); err != nil {
		return err
	}

	log.Println("running daemon initiated")
	defer log.Println("running daemon completed")
	log.Println("cache directory", t.CacheDir)

	config := &ssh.ClientConfig{
		User: t.AccountID,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			log.Println("hostkey", hostname, remote.String(), hex.EncodeToString(key.Marshal()))
			return nil
		},
	}

	if err = daemons.Register(ctx, t.AccountID, signer.PublicKey()); err != nil {
		return err
	}

	if httpl, err = daemons.HTTP(ctx); err != nil {
		return err
	}
	defer httpl.Close()

	if sshproxy, err = daemons.SSHProxy(ctx, config, signer, httpl); err != nil {
		return err
	}
	defer sshproxy.Close()

	<-ctx.Context.Done()
	return ctx.Context.Err()
}
