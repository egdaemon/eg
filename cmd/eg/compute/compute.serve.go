package compute

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/cmd/migrations"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/servesessions"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
	"github.com/gofrs/uuid/v5"
	"go.podman.io/podman/v6/pkg/bindings/containers"
)

type serve struct {
	cmdopts.RuntimeResources
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	ModuleDir        string   `name:"moduledir" help:"must be a subdirectory in the provided directory" default:"${vars_workload_directory}"`
	InvalidateCache  bool     `name:"invalidate-cache" help:"removes workload build cache"`
	Privileged       bool     `name:"privileged" help:"run the initial container in privileged mode"`
	Dirty            bool     `name:"dirty" help:"include user directories and environment variables" hidden:"true"`
	Wayland          bool     `name:"wayland" help:"bind-mount the host wayland display socket into the container"`
	GCPAuto          bool     `name:"gcp-auto" help:"use the default well known path for gcp's application default credentials"`
	GCP              string   `name:"gcp" help:"path to gcp's application default credentials"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	Secrets          []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Ports            []int    `name:"ports" help:"list of ports to publish to the host system"`
	SSHKeyPath       string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	SSH              int      `name:"ssh" type:"sshport" default:"-1" help:"enable ssh access into the container as the egd user, using eg's own managed ssh identity; bare --ssh publishes on a free host port, --ssh=<port> on a fixed one"`
	Session          string   `name:"session" help:"uuid to use as the run's id, also names the container and identifies the session for lookups by tools, i.e. the editor extension. generated when blank" default:""`
	Name             string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
}

// containersshport the host port podman published for the container's sshd, 0 until the container exists.
// containers are named by the run's id which clients can reuse, so a container left behind by an earlier run
// with the same name still reports its ports until it is replaced. only a container that is running and
// was created since this run started counts.
func containersshport(ctx context.Context, cname string, since time.Time) int64 {
	data, err := containers.Inspect(ctx, cname, nil)
	if err != nil || data.NetworkSettings == nil || data.State == nil || !data.State.Running || data.Created.Before(since) {
		return 0
	}

	for _, binding := range data.NetworkSettings.Ports["22/tcp"] {
		if port, err := strconv.ParseInt(binding.HostPort, 10, 64); err == nil && port > 0 {
			return port
		}
	}

	return 0
}

// registerssh keeps the serve session registered with the port podman actually published, until ctx is cancelled.
// the port is read from the container, which is what makes an automatic port possible: podman picks it.
// every module is a new container, so the port is re-registered when it changes.
func registerssh(ctx context.Context, db *sql.DB, s servesessions.Session, cname string) {
	var (
		registered int64
		started    = time.Now()
		tick       = time.NewTicker(250 * time.Millisecond)
	)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
		}

		port := containersshport(ctx, cname, started)
		if port == 0 || port == registered {
			continue
		}

		s.SSHPort = port
		if err := servesessions.Register(ctx, db, s); err != nil {
			log.Println("unable to register serve session", err)
			continue
		}
		registered = port

		log.Printf("ssh access enabled: ssh -i %s -p %d -o UserKnownHostsFile=/dev/null -o StrictHostKeyChecking=no -o LogLevel=ERROR %s@localhost\n", s.SSHKeyPath, port, s.SSHUser)
	}
}

func (t serve) datadir(rels ...string) string {
	return filepath.Join(t.Dir, t.ModuleDir, t.Name, filepath.Join(rels...))
}

func (t serve) Run(gctx *cmdopts.Global, hotswapbin *cmdopts.HotswapPath) (err error) {
	var (
		homedir    = userx.HomeDirectoryOrDefault("/root")
		uid        = uuid.Must(uuid.NewV7())
		ws         workspaces.Context
		repo       *git.Repository
		environio  *os.File
		gnupghome  runners.AgentOption
		envvar     runners.AgentOption = runners.AgentOptionNoop
		mounthome  runners.AgentOption = runners.AgentOptionNoop
		wayland    runners.AgentOption = runners.AgentOptionNoop
		gcpcreds   runners.AgentOption = runners.AgentOptionNoop
		privileged runners.AgentOption = runners.AgentOptionNoop
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	// clients that want to find the run again, i.e. the editor extension, pick the run's id.
	if stringsx.Present(t.Session) {
		if uid, err = uuid.FromString(t.Session); err != nil {
			return errorsx.Wrapf(err, "invalid session id %q, expected a uuid", t.Session)
		}
	}

	ctx, err := podmanx.WithClient(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to podman")
	}

	if ws, err = workspaces.NewLocal(
		gctx.Context,
		uuid.Must(uuid.NewV7()),
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		t.Dir,
		t.Name,
		workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, t.InvalidateCache),
	); err != nil {
		return errorsx.Wrap(err, "unable to setup workspace")
	}
	defer os.RemoveAll(ws.Root)

	environpath := filepath.Join(ws.RuntimeDir, eg.EnvironFile)
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	if repo, err = git.PlainOpen(ws.WorkingDir); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository: %s", ws.WorkingDir)
	}

	log.Println("loading environment file", t.datadir(".eg.env"))
	envb := envx.Build().
		FromEnv(os.Environ()...).
		FromReader(secrets.NewReader(gctx.Context, t.Secrets...)).
		FromPath(t.EnvironmentPaths...).
		FromPath(t.datadir(".eg.env")).
		FromEnv(t.Environment...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var(eg.EnvComputeRunID, uid.String()).
		Var(eg.EnvComputeWorkloadName, t.Name).
		Var(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)).
		Var(eg.EnvComputeBin, hotswapbin.String()).
		Var(eg.EnvUnsafeCacheID, ws.CachedID).
		Var(eg.EnvComputeTTL, t.RuntimeResources.TTL.String()).
		Var(eg.EnvComputeInitiatedTimestamp, time.Now().UTC().Format(time.RFC3339Nano)).
		Var(eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(false)) // hack to disable cloning

	for idx, p := range t.Ports {
		envb = envb.Var(fmt.Sprintf("EG_COMPUTE_PORT_%d", idx), strconv.Itoa(p))
	}

	if t.Dirty {
		mounthome = runners.AgentOptionAutoMountHome(homedir)
	}

	if t.GCPAuto {
		gcpcreds = runners.AgentOptionGcloudCredentials(gctx.Context, envb, envx.String(userx.DefaultConfigDirectory("gcloud", "application_default_credentials.json"), eg.EnvGoogleApplicationCredentials))
	} else if stringsx.Present(t.GCP) {
		gcpcreds = runners.AgentOptionGcloudCredentials(gctx.Context, envb, t.GCP)
	}

	gnupghome = runners.AgentOptionLocalGPGAgent(gctx.Context, envb)

	if t.Wayland {
		wayland = runners.AgentOptionWayland(gctx.Context, envb)
	}

	sshserver := runners.AgentOptionSSHServer(gctx.Context, ws, t.SSHKeyPath, t.SSH)

	// the cli's database is always opened, which applies its migrations. failure is not fatal, serve
	// does not otherwise depend on it.
	if db, derr := migrations.OpenLocked(gctx.Context, migrations.Path(migrations.CLI)); derr != nil {
		log.Println("unable to open the cli database", derr)
	} else {
		defer func() { errorsx.Log(db.Close()) }()

		// register the session so tools (i.e. the vscode extension) can discover how to connect.
		if t.SSH != cmdopts.SSHPortDisabled {
			wctx, stop := context.WithCancel(ctx)
			done := make(chan struct{})
			go func() {
				defer close(done)
				registerssh(wctx, db, servesessions.Session{
					ID:         uid.String(),
					Module:       t.Name,
					Directory:    t.Dir,
					SSHRequested: int64(t.SSH),
					SSHUser:      eg.DefaultUsername,
					SSHKeyPath:   t.SSHKeyPath,
				}, fmt.Sprintf("eg-%s", uid.String()))
			}()

			defer func() {
				// stop the watcher before marking the session stopped so it cannot register again, the session
				// context may already be cancelled so stopping uses its own.
				stop()
				<-done
				errorsx.Log(servesessions.Stop(context.Background(), db, uid.String()))
			}()
		}
	}

	if err = errorsx.Compact(envb.CopyTo(environio), environio.Close()); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	log.Println("cacheid", ws.CachedID)

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(t.Dir), ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

	if err = compile.EnsureRequiredPackages(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return err
	}

	if len(modules) == 0 {
		return errors.New("no usable modules detected")
	}

	debugx.Println("modules", modules)

	imagename := stringsx.Join(".", "eg.serve", strings.ReplaceAll(t.Name, string(filepath.Separator), "."))
	if err = runners.BuildContainer(gctx.Context, imagename, t.Dir, t.datadir("Containerfile")); err != nil {
		return errorsx.Wrap(err, "serve requires a containerfile to run")
	}

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), ws.CacheDirWazero); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	if t.Privileged {
		privileged = runners.AgentOptionCommandLine("--privileged")
	}

	debugx.Println("runtime resources", spew.Sdump(t.RuntimeResources))

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))
	ragent := runners.NewRunner(
		gctx.Context,
		ws,
		uid.String(),
		privileged,
		mounthome,
		envvar,
		sshserver,
		mountegbin,
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
			runners.AgentMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
			runners.AgentMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
			runners.AgentMountReadWrite(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
		),
		runners.AgentOptionLocalComputeCachingVolumes(canonicaluri),
		gnupghome, // must come after the runtime directory mount to ensure correct mounting order.
		gcpcreds,  // must come after the mount directory to ensure correct mounting order.
		wayland,   // must come after the runtime directory mount to ensure correct mounting order.
		runners.AgentOptionEnvironFile(environpath), // ensure we pick up the environment file with the container.
		runners.AgentOptionHostOS(),
		runners.AgentOptionPublish(t.Ports...),
		runners.AgentOptionCores(t.RuntimeResources.Cores),
		runners.AgentOptionMemory(uint64(t.RuntimeResources.Memory)),
	)

	for _, m := range modules {
		options := append(
			ragent.Options(),
			runners.AgentOptionVolumeSpecs(
				runners.AgentMountReadOnly(
					filepath.Join(ws.Root, ws.BuildDir, ws.Module, eg.ModuleDir),
					eg.DefaultMountRoot(eg.RuntimeDirectory, ws.Module, eg.ModuleDir),
				),
				runners.AgentMountReadOnly(m.Path, eg.ModuleMount()),
				runners.AgentMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
			)...)

		prepcmd := func(cmd *exec.Cmd) *exec.Cmd {
			cmd.Dir = ws.Root
			cmd.Stdout = log.Writer()
			cmd.Stderr = log.Writer()
			cmd.Stdin = os.Stdin
			return cmd
		}

		if err := c8sproxy.PodmanModule(ctx, prepcmd, imagename, fmt.Sprintf("eg-%s", uid.String()), ws.RuntimeDir, options...); err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
