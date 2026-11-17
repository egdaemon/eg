package compute

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/egvm"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/uuidx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/interp/environ"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/secrets"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v6"
)

type local struct {
	cmdopts.RuntimeResources
	cmdopts.Publishing
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	ModuleDir        string   `name:"moduledir" help:"must be a subdirectory in the provided directory" default:"${vars_workload_directory}" hidden:"true"`
	Debug            bool     `name:"debug" help:"keep workspace around to debug issues, requires manual cleanup"`
	AutoPatch        bool     `name:"auto-patch" help:"commit the changes a workload made to its working directory to a new eg/ branch of the current repository once it completes, when disabled the changes are printed as a patch instead (dry run)" default:"true" negatable:""`
	Privileged       bool     `name:"privileged" help:"run the initial container in privileged mode"`
	Cgroups          bool     `name:"cgroups" help:"enable/disable cgroups for the module container" hidden:"true" negatable:"" default:"true"`
	Dirty            bool     `name:"dirty" help:"include user directories and environment variables" hidden:"true"`
	Wayland          bool     `name:"wayland" help:"bind-mount the host wayland display socket into the container"`
	GPU              bool     `name:"gpu" help:"enable gpu support, implied by a non zero --vram" hidden:"true"`
	GithubAuto       bool     `name:"github-token" help:"automatically detect and inject github credentials" negatable:"" default:"true" env:"${env_github_autotoken}"`
	GCPAuto          bool     `name:"gcp-auto" help:"use the default well known path for gcp's application default credentials"`
	GCP              string   `name:"gcp" help:"path to gcp's application default credentials"`
	InvalidateCache  bool     `name:"invalidate-cache" help:"removes workload build cache"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Ports            []int    `name:"ports" help:"list of ports to publish to the host system" hidden:"true"`
	ContainerArgs    []string `name:"cargs" help:"list of command line arguments to pass to the root container" hidden:"true"`
	Secrets          []string `name:"secret" help:"List of secret URIs to use. Examples: chachasm://passphrase@/path/to/file, gcpsm://project-id/secret-name/version, awssm://secret-name?region=us-east-1"`
	Profile          string   `name:"profile" help:"enable profiling of module runs (cpu,heap,mem,allocs,block)" enum:"cpu,heap,mem,allocs,block," default:""`
	Egiso            *string  `name:"egiso" help:"EXPERIMENTAL: run in a machine booted from the egiso at this path or https URL, with the digest of its table of contents as its fragment, built from the default root container when empty" hidden:"true"`
	Name             string   `arg:"" name:"module" help:"name of the workload to run, i.e. the folder name within workload directory" default:"" predictor:"eg.workload"`
}

// gpu reports whether gpu support is enabled, either explicitly or by requesting vram.
func (t local) gpu() bool {
	return t.GPU || t.RuntimeResources.Vram > 0
}

// Run compiles the workload's modules, runs them in containers or, given an egiso, a machine, and
// applies the changes they made.
func (t local) Run(gctx *cmdopts.Global, tlsc *cmdopts.TLSConfig, hotswapbin *cmdopts.HotswapPath, session *cmdopts.ComputeSession, clidb *cmdopts.CLIDB) (err error) {
	var (
		ws        workspaces.Context
		repo      *git.Repository
		rt        runners.T
		environio *os.File
		b         runners.B = runners.BFn(t.podman)
		uid                 = uuidx.FirstNonNil(session.Session, uuidx.NewV7())
	)

	if t.Egiso != nil {
		b = t.machine(tlsc.DefaultClient(), *t.Egiso)
	}

	// register the run so tools (i.e. the vscode extension) can list it while it is running. local runs
	// are not reachable over ssh and do not host an agent, they only patch their changes back when enabled.
	defer func() { errorsx.Log(clidb.Close()) }()
	registered, done := session.New(clidb).Branch(session.Branch, headbranch(t.Dir, t.Name, uid)).Patch(t.AutoPatch).Register(gctx.Context, uid, t.Name, t.Dir)
	defer func() { done(err) }()

	contextx.WaitGroupAdd(gctx.Context, 1)
	go contextx.WaitGroupDone(gctx.Context)

	if ws, err = workspaces.NewLocal(
		gctx.Context,
		uuidx.NewV7(),
		md5x.Digest(cmdopts.BuildInfoSafe()),
		t.Dir,
		t.Name,
		workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, t.InvalidateCache),
	); err != nil {
		return errorsx.Wrap(err, "unable to setup workspace")
	}
	if !t.Debug {
		defer os.RemoveAll(ws.Root)
	} else {
		log.Println("debug enabled", ws.Root)
	}

	defer ws.Cleanup(context.Background())

	if repo, err = git.PlainOpen(ws.WorkingDir); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository: %s", ws.WorkingDir)
	}

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))

	// the environment the enclosing workload was provided, everything below overrides it.
	inherited, err := environ.Inherited(gctx.Context)
	if err != nil {
		return err
	}

	envb := envx.Build().
		FromEnviron(inherited...).
		Var(eg.EnvGithubToken, githubToken(gctx.Context, t.GithubAuto, canonicaluri)).
		FromReader(secrets.NewReader(gctx.Context, t.Secrets...)).
		FromPath(t.EnvironmentPaths...).
		FromEnv(os.Environ()...).
		FromEnviron(envx.AutoEnviron(t.Environment...)...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var(eg.EnvComputeOS, runtime.GOOS).
		Var(eg.EnvComputeArch, runtime.GOARCH).
		Var(eg.EnvComputeWorkloadDirectory, eg.DefaultRootDirectory()).
		Var(eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory()).
		Var(eg.EnvComputeCacheDirectory, eg.DefaultCacheDirectory()).
		Var(eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory()).
		Var(eg.EnvComputeWorkspaceDirectory, eg.DefaultWorkspaceDirectory()).
		Var(eg.EnvComputeRunID, uid.String()).
		Var(eg.EnvComputeWorkload, t.Name).
		Var(eg.EnvComputeWorkloadName, t.Name).
		Var(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)).
		Var(eg.EnvComputeBin, hotswapbin.String()).
		Var(eg.EnvUnsafeCacheID, ws.CachedID).
		Var(eg.EnvComputeTTL, t.RuntimeResources.TTL.String()).
		Var(eg.EnvComputeInitiatedTimestamp, time.Now().UTC().Format(time.RFC3339Nano)).
		Var(eg.EnvComputeGPU, strconv.FormatBool(t.gpu())).
		Var(eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(false)). // hack to disable cloning
		Var(eg.EnvWorkloadDraft, strconv.FormatBool(t.Draft)).
		Var(eg.EnvWorkloadRelease, strconv.FormatBool(t.Release)).
		Var(eg.EnvComputeProfileMode, t.Profile)

	// envx.Debug(errorsx.Zero(envb.Environ())...)
	log.Println("cacheid", ws.CachedID)

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(t.Dir), ws)).Run(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to transpile")
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
	debugx.Println("runtime resources", spew.Sdump(t.RuntimeResources))

	if rt, err = b.New(gctx.Context, ws, uid, canonicaluri, envb); err != nil {
		return err
	}
	defer func() { err = errorsx.Compact(err, rt.Close()) }()

	if environio, err = os.Create(filepath.Join(ws.RuntimeDir, eg.EnvironFile)); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	for _, m := range modules {
		if err = rt.Do(gctx.Context, m); err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return gitx.AutoPatch(os.Stdout).DryRun(!t.AutoPatch).From(ws.WorkingDir).To(t.Dir).Branch(registered.Branch).Apply(gctx.Context)
}

// container runs modules in containers of the root container built from the repository's
// Containerfile.
type container struct {
	ctx   context.Context // carries the podman client.
	ws    workspaces.Context
	uid   uuidx.UUID
	agent *runners.Agent
	image string
	tty   bool // hands the module the host's stdin and stdout themselves, so a terminal is attached as one.
}

func (t container) Do(_ context.Context, m transpile.Compiled) error {
	options := append(
		t.agent.Options(),
		runners.AgentOptionVolumeSpecs(
			runners.AgentMountReadOnly(
				filepath.Join(t.ws.Root, t.ws.BuildDir, t.ws.Module, eg.ModuleDir),
				eg.DefaultMountRoot(eg.RuntimeDirectory, t.ws.Module, eg.ModuleDir),
			),
			runners.AgentMountReadOnly(m.Path, eg.ModuleMount()),
		)...)

	prepcmd := func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Dir = t.ws.Root
		cmd.Stdout = log.Writer()
		cmd.Stderr = log.Writer()
		cmd.Stdin = os.Stdin
		if t.tty {
			// the terminal attach needs the files themselves, a writer in between is not a terminal.
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
		}
		return cmd
	}

	// TODO REVISIT using t.ws.RuntimeDir as moduledir.
	return c8sproxy.PodmanModule(t.ctx, prepcmd, t.image, fmt.Sprintf("eg-%s", t.uid.String()), t.ws.RuntimeDir, options...)
}

func (t container) Close() error {
	return nil
}

// podman prepares containers of the root container built from the repository's Containerfile.
func (t local) podman(ctx context.Context, ws workspaces.Context, uid uuidx.UUID, canonicaluri string, envb *envx.Builder) (_ runners.T, err error) {
	var (
		homedir     = userx.HomeDirectoryOrDefault("/root")
		environpath = filepath.Join(ws.RuntimeDir, eg.EnvironFile)
		pctx        context.Context
		gnupghome   runners.AgentOption
		gpu         runners.AgentOption
		sshmount    runners.AgentOption = runners.AgentOptionNoop
		gcpcreds    runners.AgentOption = runners.AgentOptionNoop
		sshenvvar   runners.AgentOption = runners.AgentOptionNoop
		envvar      runners.AgentOption = runners.AgentOptionNoop
		mounthome   runners.AgentOption = runners.AgentOptionNoop
		wayland     runners.AgentOption = runners.AgentOptionNoop
		privileged  runners.AgentOption = runners.AgentOptionNoop
		cgroups                         = envx.Toggle(runners.AgentOptionNoop, runners.AgentOptionCommandLine("--cgroups", "disabled"), runners.Nested() && !t.Cgroups) // disable cgroups
		// nested containers cannot write the sysctls netavark needs for a bridge network (read-only /proc/sys), share the outer network instead.
		hostnet                        = envx.Toggle(runners.AgentOptionNoop, runners.AgentOptionCommandLine("--network", "host"), runners.Nested() && !envx.Boolean(false, eg.EnvExperimentalDisableHostNetwork))
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
		platform                       = podmanx.AutoPlatform(t.Arch)
	)

	if t.Dirty {
		mounthome = runners.AgentOptionAutoMountHome(homedir)
	}

	if t.GCPAuto {
		gcpcreds = runners.AgentOptionGcloudCredentials(ctx, envb, envx.String(userx.DefaultConfigDirectory("gcloud", "application_default_credentials.json"), eg.EnvGoogleApplicationCredentials))
	} else if stringsx.Present(t.GCP) {
		gcpcreds = runners.AgentOptionGcloudCredentials(ctx, envb, t.GCP)
	}

	if gpu, err = runners.AgentOptionGPU(t.gpu()); err != nil {
		return nil, err
	}

	gnupghome = runners.AgentOptionLocalGPGAgent(ctx, envb)

	if t.Wayland {
		wayland = runners.AgentOptionWayland(ctx, envb)
	}

	if pctx, err = podmanx.WithClient(ctx); err != nil {
		return nil, errorsx.Wrap(err, "unable to connect to podman")
	}

	if err = runners.BuildRootContainerPath(ctx, t.Dir, filepath.Join(ws.RuntimeDir, "Containerfile"), "--build-arg", envx.Format(eg.EnvBuildCommit, cmdopts.BuildCommit()), "--platform", platform); err != nil {
		return nil, err
	}

	if err = wasix.WarmCacheDirectory(ctx, filepath.Join(ws.Root, ws.BuildDir), ws.CacheDirWazero); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	if t.Privileged {
		privileged = runners.AgentOptionCommandLine("--privileged")
	}

	ragent := runners.NewRunner(
		ctx,
		ws,
		uid.String(),
		privileged,
		cgroups,
		hostnet,
		mounthome,
		envvar,
		sshmount,
		sshenvvar,
		mountegbin,
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
			runners.AgentMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
			runners.AgentMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
			runners.AgentMountReadWrite(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
		),
		runners.AgentOptionLocalComputeCachingVolumes(canonicaluri),
		runners.AgentOptionEnvironFile(environpath), // ensure we pick up the environment file with the container.
		runners.AgentOptionHostOS(t.ContainerArgs...),
		runners.AgentOptionPublish(t.Ports...),
		runners.AgentOptionCores(t.RuntimeResources.Cores),
		runners.AgentOptionMemory(uint64(t.RuntimeResources.Memory)),
		runners.AgentOptionPlatform(platform),
		gnupghome, // must come after the runtime directory mount to ensure correct mounting order.
		gcpcreds,  // must come after the mount directory to ensure correct mounting order.
		gpu,       // enable gpu support
		wayland,   // must come after the runtime directory mount to ensure correct mounting order.
	)

	return container{ctx: pctx, ws: ws, uid: uid, agent: ragent, image: eg.WorkingDirectory}, nil
}

// machine prepares a machine booted from the egiso at uri, fetched through c, or from one of the
// default root container when uri is empty.
func (t local) machine(c *http.Client, uri string) runners.BFn {
	return func(ctx context.Context, ws workspaces.Context, _ uuidx.UUID, canonicaluri string, envb *envx.Builder) (_ runners.T, err error) {
		var (
			containers = userx.DefaultCacheDirectory("egvm", runners.ContainersVolume(canonicaluri))
			// a machine runs the eg running the command, as a container does, when it runs the host's
			// OS; otherwise its image's own.
			mountegbin = envx.Toggle(runners.MachineOptionNoop, runners.MachineOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0]))), runtime.GOOS == "linux")
		)

		if stringsx.Blank(uri) {
			if uri, err = runners.BuildRootEgiso(ctx, userx.DefaultCacheDirectory("egiso"), t.Dir, filepath.Join(ws.RuntimeDir, "Containerfile"), "--build-arg", envx.Format(eg.EnvBuildCommit, cmdopts.BuildCommit()), "--platform", podmanx.AutoPlatform(t.Arch)); err != nil {
				return nil, err
			}
		}

		// as containers keep their container storage in a volume, a machine keeps it in a volume in the
		// user's cache, where overlay can layer it, which the machine's root and shares can not.
		return runners.NewMachine(ctx, c, ws, uri, log.Writer(),
			runners.MachineOptionVM(
				egvm.OptionCPUs(uint(t.RuntimeResources.Cores)),
				egvm.OptionMemory(uint64(t.RuntimeResources.Memory)),
				egvm.OptionMountReadOnly(filepath.Join(ws.Root, ws.BuildDir, ws.Module, eg.ModuleDir), eg.DefaultMountRoot(eg.RuntimeDirectory, ws.Module, eg.ModuleDir)),
				egvm.OptionMountVolume(containers, "/var/lib/containers"),
			),
			runners.MachineOptionLocalGPGAgent(ctx, envb),
			mountegbin,
		)
	}
}
