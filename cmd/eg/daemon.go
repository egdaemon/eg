package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strconv"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/authn"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/eg/daemons"
	"github.com/egdaemon/eg/compute"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/httpx"
	"github.com/egdaemon/eg/internal/runtimex"
	"github.com/egdaemon/eg/internal/sshx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/runners"
	"golang.org/x/crypto/ssh"
	"golang.org/x/oauth2"

	"github.com/libp2p/go-libp2p/core/host"
)

type daemon struct {
	cmdopts.RuntimeResources
	AccountID     string   `name:"account" help:"account to register runner with" default:"${vars_account_id}" required:"true"`
	MachineID     string   `name:"machine" help:"unique id for this particular machine" default:"${vars_machine_id}" required:"true"`
	Seed          string   `name:"seed" help:"seed for generating ssh credentials in a consistent manner" default:"${vars_entropy_seed}"`
	SSHKeyPath    string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	SSHAgentPath  string   `name:"sshagentpath" help:"ssh agent socket path" default:"${vars_runtime_directory}/ssh.agent.socket"`
	SSHKnownHosts string   `name:"sshknownhostspath" help:"ssh known hosts path" default:"${vars_ssh_known_hosts_path}"`
	Autodownload  bool     `name:"autodownload" help:"enable/disable the basic download scheduler" default:"true"`
	CacheDir      string   `name:"directory" help:"local cache directory" default:"${vars_cache_directory}"`
	MountDirs     []string `name:"mounts" short:"m" help:"folders to mount using podman mount specs" default:""`
	EnvVars       []string `name:"env" short:"e" help:"environment variables to import" default:""`
}

// essentially we use ssh forwarding from the control plane to the local http server
// allowing the control plane to interogate
func (t daemon) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig) (err error) {
	var (
		signer     ssh.Signer
		httpl      net.Listener
		grpcl      net.Listener
		authclient *http.Client
		p2p        host.Host
	)
	log.Println("running daemon initiated")
	defer log.Println("running daemon completed")

	rm := runners.NewResourceManager(runners.NewRuntimeResources())

	// we want to set the umask to 0002 to ensure that the cache (and other) directory are readable by the group.
	runtimex.Umask(0002)

	log.Println("cache directory", t.CacheDir)
	log.Println("detected runtime configuration", spew.Sdump(t.RuntimeResources))

	if httpl, err = net.Listen("unix", userx.DefaultRuntimeDirectory("main.socket")); err != nil {
		return err
	}

	if signer, err = sshx.AutoCached(sshx.NewKeyGenSeeded(t.Seed), t.SSHKeyPath); err != nil {
		return errorsx.Wrap(err, "unable to retrieve identity credentials")
	}

	if p2p, err = daemons.P2PProxy(gctx.Context, t.Seed, append([]byte(machineID()), signer.PublicKey().Marshal()...), httpl); err != nil {
		return errorsx.Wrap(err, "unable to initialize p2p")
	}

	if err = daemons.Register(gctx, tlsc, &t.RuntimeResources, t.AccountID, t.MachineID, p2p, signer); err != nil {
		return err
	}

	c := httpx.BindRetryTransport(tlsc.DefaultClient(), http.StatusTooManyRequests, http.StatusBadGateway)
	tokensrc := compute.NewAuthzTokenSource(c, signer, authn.EndpointCompute())
	authclient = oauth2.NewClient(
		context.WithValue(gctx.Context, oauth2.HTTPClient, c),
		tokensrc,
	)

	if err = daemons.HTTP(gctx, httpl); err != nil {
		return err
	}
	defer httpl.Close()

	if grpcl, err = daemons.DefaultAgentListener(); err != nil {
		return err
	}

	if err = daemons.Agent(gctx, grpcl); err != nil {
		return errorsx.Wrap(err, "unable to initialize daemon")
	}

	go func() {
		for {
			if cause := daemons.Ping(gctx, tlsc, &t.RuntimeResources, t.AccountID, t.MachineID, p2p, signer); cause != nil {
				log.Println("ping failed", cause)
			}

			select {
			case <-gctx.Context.Done():
				return
			default:
			}
		}
	}()

	if t.Autodownload {
		go runners.AutoDownload(gctx.Context, authclient, rm)
	}

	if _, found := os.LookupEnv("SSH_AUTH_SOCK"); !found {
		if err = daemons.SSHAgent(gctx, t.SSHAgentPath); err != nil {
			return errorsx.Wrap(err, "ssh agent failed")
		}
	}

	ctx, err := cmdopts.WithPodman(gctx.Context)
	if err != nil {
		return err
	}

	if err = runners.BuildRootContainer(ctx); err != nil {
		return err
	}

	return runners.Queue(
		ctx,
		rm,
		runners.QueueOptionCompletion(
			runners.NewCompletionClient(authclient),
		),
		runners.QueueOptionAgentOptions(
			runners.AgentOptionVolumes(
				runners.AgentMountReadWrite(
					envx.String(t.SSHAgentPath, "SSH_AUTH_SOCK"),
					eg.DefaultRuntimeDirectory("ssh.agent.socket"),
				),
				runners.AgentMountReadOnly(t.SSHKnownHosts, "/etc/ssh/ssh_known_hosts"),
			),
			runners.AgentOptionEnv("SSH_AUTH_SOCK", eg.DefaultRuntimeDirectory("ssh.agent.socket")),
			runners.AgentOptionVolumes(t.MountDirs...),
			runners.AgentOptionEnvKeys(t.EnvVars...),
			runners.AgentOptionEnv(eg.EnvComputeTLSInsecure, strconv.FormatBool(tlsc.Insecure)),
		),
		runners.QueueOptionLogVerbosity(gctx.Verbosity),
	)
}
