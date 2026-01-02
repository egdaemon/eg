package compute

import (
	"errors"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/contextx"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid/v5"
)

type local struct {
	cmdopts.RuntimeResources
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	Debug            bool     `name:"debug" help:"keep workspace around to debug issues, requires manual cleanup"`
	Privileged       bool     `name:"privileged" help:"run the initial container in privileged mode"`
	Dirty            bool     `name:"dirty" help:"include user directories and environment variables" hidden:"true"`
	InvalidateCache  bool     `name:"invalidate-cache" help:"removes workload build cache"`
	EnvironmentPaths string   `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Infinite         bool     `name:"infinite" help:"allow this module to run forever, used for running a workload like a webserver" hidden:"true"`
	Ports            []int    `name:"ports" help:"list of ports to publish to the host system" hidden:"true"`
	ContainerArgs    []string `name:"cargs" help:"list of command line arguments to pass to the root container" hidden:"true"`
	Name             string   `arg:"" name:"module" help:"name of the workload to run, i.e. the folder name within workload directory" default:"" predictor:"eg.workload"`
}

func (t local) Run(gctx *cmdopts.Global, hotswapbin *cmdopts.HotswapPath) (err error) {
	var (
		homedir    = userx.HomeDirectoryOrDefault("/root")
		ws         workspaces.Context
		repo       *git.Repository
		uid        = uuid.Must(uuid.NewV7())
		environio  *os.File
		sshmount   runners.AgentOption = runners.AgentOptionNoop
		sshenvvar  runners.AgentOption = runners.AgentOptionNoop
		envvar     runners.AgentOption = runners.AgentOptionNoop
		mounthome  runners.AgentOption = runners.AgentOptionNoop
		privileged runners.AgentOption = runners.AgentOptionNoop
		gnupghome  runners.AgentOption = runners.AgentOptionNoop
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	contextx.WaitGroupAdd(gctx.Context, 1)
	go contextx.WaitGroupDone(gctx.Context)

	ctx, err := podmanx.WithClient(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to podman")
	}

	if err := podmanx.EnsureSharedMount(ctx); err != nil {
		return errorsx.Wrap(err, "unable to ensure shared mount propagation")
	}

	if ws, err = workspaces.NewLocal(
		gctx.Context, md5x.Digest(cmdopts.BuildInfoSafe()), t.Dir, t.Name,
		workspaces.OptionSymlinkCache(filepath.Join(t.Dir, eg.CacheDirectory)),
		workspaces.OptionSymlinkWorking(t.Dir),
		workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, t.InvalidateCache),
	); err != nil {
		return errorsx.Wrap(err, "unable to setup workspace")
	}
	if !t.Debug {
		defer os.RemoveAll(ws.Root)
	} else {
		log.Println("debug enabled", ws.Root)
	}

	environpath := filepath.Join(ws.RuntimeDir, eg.EnvironFile)
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	if repo, err = git.PlainOpen(ws.WorkingDir); err != nil {
		return errorsx.Wrapf(err, "unable to open git repository: %s", ws.WorkingDir)
	}

	if t.Infinite {
		t.RuntimeResources.TTL = time.Duration(math.MaxInt)
	}

	envb := envx.Build().
		FromPath(t.EnvironmentPaths).
		FromEnv(t.Environment...).
		FromEnv(os.Environ()...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var(eg.EnvComputeWorkloadDirectory, eg.DefaultWorkloadDirectory()).
		Var(eg.EnvComputeWorkingDirectory, eg.DefaultWorkingDirectory()).
		Var(eg.EnvComputeCacheDirectory, eg.DefaultCacheDirectory()).
		Var(eg.EnvComputeRuntimeDirectory, eg.DefaultRuntimeDirectory()).
		Var(eg.EnvComputeWorkspaceDirectory, eg.DefaultWorkspaceDirectory()).
		Var(eg.EnvComputeRunID, uid.String()).
		Var(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)).
		Var(eg.EnvComputeBin, hotswapbin.String()).
		Var(eg.EnvUnsafeCacheID, ws.CachedID).
		Var(eg.EnvComputeTTL, t.RuntimeResources.TTL.String()).
		Var(eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(false)) // hack to disable cloning

	if t.Dirty {
		mounthome = runners.AgentOptionAutoMountHome(homedir)
	}

	gnupghome = runners.AgentOptionLocalGPGAgent(gctx.Context, envb)

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

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

	if err = runners.BuildRootContainerPath(gctx.Context, t.Dir, filepath.Join(ws.RuntimeDir, "Containerfile")); err != nil {
		return err
	}

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.CacheDir, eg.DefaultModuleDirectory()))); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	if t.Privileged {
		privileged = runners.AgentOptionCommandLine("--privileged")
	}

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))
	ragent := runners.NewRunner(
		gctx.Context,
		ws,
		uid.String(),
		privileged,
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
		gnupghome, // must come after the runtime directory mount to ensure correct mounting order.
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
			)...)

		prepcmd := func(cmd *exec.Cmd) *exec.Cmd {
			cmd.Dir = ws.Root
			cmd.Stdout = log.Writer()
			cmd.Stderr = log.Writer()
			cmd.Stdin = os.Stdin
			return cmd
		}

		// TODO REVISIT using t.ws.RuntimeDir as moduledir.
		if err := c8sproxy.PodmanModule(ctx, prepcmd, eg.WorkingDirectory, fmt.Sprintf("eg-%s", uid.String()), ws.RuntimeDir, options...); err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
