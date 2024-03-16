package main

import (
	"context"
	"encoding/hex"
	"log"
	"net"
	"net/http"

	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type daemon struct {
	AccountID  string `name:"account" help:"account to register runner with" default:"${vars_account_id}" required:"true"`
	MachineID  string `name:"machine" help:"unique id for this particular machine" default:"${vars_machine_id}" required:"true"`
	Seed       string `name:"secret" help:"seed for generating ssh credentials in a consistent manner"`
	SSHKeyPath string `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	CacheDir   string `name:"directory" help:"root directory of the repository" default:"${vars_cache_directory}"`
}

// essentially we use ssh forwarding from the control plane to the local http server
// allowing the control plane to interogate
func (t daemon) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		signer     ssh.Signer
		httpl      net.Listener
		grpcl      net.Listener
		authclient *http.Client
	)

	if httpl, err = net.Listen("tcp", "127.0.1.1:8093"); err != nil {
		return err
	}

	if signer, err = sshx.AutoCached(sshx.NewKeyGenSeeded(t.Seed), t.SSHKeyPath); err != nil {
		return err
	}

	ctx := context.WithValue(gctx.Context, oauth2.HTTPClient, tlsc.DefaultClient())
	if authclient, err = authn.OAuth2SSHHTTPClient(ctx, signer); err != nil {
		return err
	}

	log.Println("running daemon initiated")
	defer log.Println("running daemon completed")
	log.Println("cache directory", t.CacheDir)

	config := &ssh.ClientConfig{
		User: t.MachineID,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			log.Println("hostkey", hostname, remote.String(), hex.EncodeToString(key.Marshal()))
			return nil
		},
	}

	if err = daemons.Register(gctx, tlsc, t.AccountID, t.MachineID, signer); err != nil {
		return err
	}

	if err = daemons.HTTP(gctx, httpl); err != nil {
		return err
	}
	defer httpl.Close()

	if grpcl, err = daemons.DefaultAgentListener(); err != nil {
		return err
	}

	if err = daemons.Agent(gctx, grpcl); err != nil {
		return err
	}

	if err = daemons.SSHProxy(gctx, config, signer, httpl); err != nil {
		return err
	}

	go runners.AutoDownload(gctx.Context, authclient)

	return runners.Queue(gctx.Context)
}
