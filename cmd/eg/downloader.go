package main

import (
	"context"
	"log"
	"net/http"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/runners"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"
)

type downloader struct {
	runtimecfg   cmdopts.RuntimeResources
	AccountID    string   `name:"account" help:"account to register runner with" default:"${vars_account_id}" required:"true"`
	MachineID    string   `name:"machine" help:"unique id for this particular machine" default:"${vars_machine_id}" required:"true"`
	Seed         string   `name:"seed" help:"seed for generating ssh credentials in a consistent manner" default:"${vars_entropy_seed}"`
	SSHKeyPath   string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	SSHAgentPath string   `name:"sshagentpath" help:"ssh agent socket path" default:"${vars_eg_runtime_directory}/ssh.agent.socket"`
	Autodownload bool     `name:"autodownload" help:"enable/disable the basic download scheduler" default:"true"`
	CacheDir     string   `name:"directory" help:"local cache directory" default:"${vars_cache_directory}"`
	MountDirs    []string `name:"mounts" short:"m" help:"folders to mount using podman mount specs" default:""`
	EnvVars      []string `name:"env" short:"e" help:"environment variables to import" default:""`
}

// essentially we use ssh forwarding from the control plane to the local http server
// allowing the control plane to interogate
func (t downloader) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		signer     ssh.Signer
		authclient *http.Client
	)

	log.Println("running daemon initiated")
	defer log.Println("running daemon completed")
	log.Println("cache directory", t.CacheDir)
	log.Println("detected runtime configuration", spew.Sdump(t.runtimecfg))

	if signer, err = sshx.AutoCached(sshx.NewKeyGenSeeded(t.Seed), t.SSHKeyPath); err != nil {
		return errorsx.Wrap(err, "unable to retrieve identity credentials")
	}

	if err = daemons.Register(gctx, tlsc, &t.runtimecfg, t.AccountID, t.MachineID, signer); err != nil {
		return err
	}

	tokensrc := compute.NewAuthzTokenSource(tlsc.DefaultClient(), signer, authn.EndpointCompute())
	authclient = oauth2.NewClient(
		context.WithValue(gctx.Context, oauth2.HTTPClient, tlsc.DefaultClient()),
		tokensrc,
	)

	go runners.AutoDownload(gctx.Context, authclient)

	return nil
}
