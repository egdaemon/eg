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
	"strings"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/debugx"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/stringsx"
	"github.com/egdaemon/eg/internal/userx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid"
)

type serve struct {
	cmdopts.RuntimeResources
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	ModuleDir        string   `name:"moduledir" help:"must be a subdirectory in the provided directory" default:"${vars_workload_directory}"`
	Privileged       bool     `name:"privileged" help:"run the initial container in privileged mode"`
	Dirty            bool     `name:"dirty" help:"include user directories and environment variables" hidden:"true"`
	EnvironmentPaths string   `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_head_reference}"`
	Infinite         bool     `name:"infinite" help:"allow this module to run forever, used for running a workload like a webserver" hidden:"true"`
	Ports            []int    `name:"ports" help:"list of ports to publish to the host system"`
	Name             string   `arg:"" name:"module" help:"name of the module to run, i.e. the folder name within moduledir" default:"" predictor:"eg.workload"`
}

func (t serve) datadir(rels ...string) string {
	return filepath.Join(t.Dir, t.ModuleDir, t.Name, filepath.Join(rels...))
}

func (t serve) Run(gctx *cmdopts.Global, hotswapbin *cmdopts.HotswapPath) (err error) {
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

	ctx, err := podmanx.WithClient(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to podman")
	}

	if ws, err = workspaces.New(gctx.Context, md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())), t.Dir, t.ModuleDir, t.Name, false); err != nil {
		return errorsx.Wrap(err, "unable to setup workspace")
	}
	defer os.RemoveAll(filepath.Join(ws.Root, ws.RuntimeDir))

	if err = os.Remove(filepath.Join(ws.Root, ws.WorkingDir)); err != nil {
		return errorsx.Wrap(err, "unable to remove working directory")
	}

	if err = os.Symlink(ws.Root, filepath.Join(ws.Root, ws.WorkingDir)); err != nil {
		return errorsx.Wrap(err, "unable to symlink working directory")
	}

	environpath := filepath.Join(ws.Root, ws.RuntimeDir, eg.EnvironFile)
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	if repo, err = git.PlainOpen(ws.Root); err != nil {
		return errorsx.Wrap(err, "unable to open git repository")
	}

	if t.Infinite {
		t.RuntimeResources.TTL = time.Duration(math.MaxInt)
	}

	envb := envx.Build().
		FromPath(t.EnvironmentPaths).
		FromPath(t.datadir(".eg.env")).
		FromEnv(t.Environment...).
		FromEnv(os.Environ()...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var(eg.EnvComputeRunID, uid.String()).
		Var(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)).
		Var(eg.EnvComputeBin, hotswapbin.String()).
		Var(eg.EnvUnsafeCacheID, ws.CachedID).
		Var(eg.EnvComputeTTL, t.RuntimeResources.TTL.String()).
		Var(eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(false)) // hack to disable cloning

	for idx, p := range t.Ports {
		envb = envb.Var(fmt.Sprintf("EG_COMPUTE_PORT_%d", idx), strconv.Itoa(p))
	}

	if t.Dirty {
		mounthome = runners.AgentOptionAutoMountHome(homedir)
	}

	gnupghome = runners.AgentOptionLocalGPGAgent(gctx.Context, envb)

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	log.Println("cacheid", ws.CachedID)

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
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

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.Root, ws.CacheDir, eg.DefaultModuleDirectory()))); err != nil {
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
		sshmount,
		sshenvvar,
		mountegbin,
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.CacheDir), eg.DefaultMountRoot(eg.CacheDirectory)),
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.RuntimeDir), eg.DefaultMountRoot(eg.RuntimeDirectory)),
		),
		runners.AgentOptionLocalComputeCachingVolumes(canonicaluri),
		gnupghome, // must come after the runtime directory mount to ensure correct mounting order.
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
					filepath.Join(ws.Root, ws.BuildDir, ws.Module, "main.wasm.d"),
					eg.DefaultMountRoot(eg.RuntimeDirectory, ws.Module, "main.wasm.d"),
				),
				runners.AgentMountReadOnly(m.Path, eg.DefaultMountRoot(eg.ModuleBin)),
				runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.WorkingDir), eg.DefaultMountRoot(eg.WorkingDirectory)),
			)...)

		prepcmd := func(cmd *exec.Cmd) *exec.Cmd {
			cmd.Dir = ws.Root
			cmd.Stdout = log.Writer()
			cmd.Stderr = log.Writer()
			cmd.Stdin = os.Stdin
			return cmd
		}

		// TODO REVISIT using t.ws.RuntimeDir as moduledir.
		if err := c8sproxy.PodmanModule(ctx, prepcmd, imagename, fmt.Sprintf("eg-%s", uid.String()), ws.RuntimeDir, options...); err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
