package compute

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"github.com/egdaemon/eg"
	"github.com/egdaemon/eg/cmd/cmdopts"
	"github.com/egdaemon/eg/compile"
	"github.com/egdaemon/eg/internal/envx"
	"github.com/egdaemon/eg/internal/errorsx"
	"github.com/egdaemon/eg/internal/fsx"
	"github.com/egdaemon/eg/internal/gitx"
	"github.com/egdaemon/eg/internal/md5x"
	"github.com/egdaemon/eg/internal/podmanx"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8sproxy"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid/v5"
)

type builtinLocal struct {
	cmdopts.RuntimeResources
	InvalidateCache  bool     `name:"invalidate-cache" help:"removes workload build cache"`
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_eg_root_directory}"`
	SSHKeyPath       string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	Endpoint         string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	Name             string   `arg:"" name:"workload" help:"name of the workload to run, i.e. the folder name within workload directory" default:"" predictor:"eg.workload"`
}

func (t builtinLocal) Run(gctx *cmdopts.Global, hotswapbin *cmdopts.HotswapPath) (err error) {
	var (
		tmpdir     string
		ws         workspaces.Context
		repo       *git.Repository
		environio  *os.File
		uid                            = uuid.Must(uuid.NewV7())
		sshmount   runners.AgentOption = runners.AgentOptionNoop
		sshenvvar  runners.AgentOption = runners.AgentOptionNoop
		envvar     runners.AgentOption = runners.AgentOptionNoop
		mounthome  runners.AgentOption = runners.AgentOptionNoop
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	ctx, err := podmanx.WithClient(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to podman")
	}

	if tmpdir, err = os.MkdirTemp("", "eg.builtin.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}
	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if ws, err = workspaces.NewLocal(
		gctx.Context,
		md5x.Digest(errorsx.Zero(cmdopts.BuildInfo())),
		tmpdir,
		t.Name,
		workspaces.OptionSymlinkCache(filepath.Join(t.Dir, eg.CacheDirectory)),
		workspaces.OptionSymlinkWorking(t.Dir),
		workspaces.OptionEnabled(workspaces.OptionInvalidateModuleCache, t.InvalidateCache),
	); err != nil {
		return errorsx.Wrap(err, "unable to initialize workspace")
	}

	if err = fsx.CloneTree(gctx.Context, eg.DefaultModuleDirectory(ws.Root), ".builtin", embeddedbuiltin); err != nil {
		return errorsx.Wrap(err, "unable to clone tree")
	}

	if err = compile.InitGolang(gctx.Context, eg.DefaultModuleDirectory(ws.Root), cmdopts.ModPath()); err != nil {
		return err
	}

	if err = compile.InitGolangTidy(gctx.Context, eg.DefaultModuleDirectory(ws.Root)); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(eg.DefaultModuleDirectory(ws.Root), ws)).Run(gctx.Context)
	if err != nil {
		return err
	}
	log.Println("DERP", roots)

	if err = compile.EnsureRequiredPackages(gctx.Context, filepath.Join(ws.Root, ws.TransDir)); err != nil {
		return err
	}

	modules, err := compile.FromTranspiled(gctx.Context, ws, roots...)
	if err != nil {
		return errorsx.Wrap(err, "unable to transpile")
	}

	if len(modules) == 0 {
		return errors.New("no usable modules detected")
	}

	log.Println("modules", modules)

	if err = runners.BuildRootContainerPath(gctx.Context, t.Dir, filepath.Join(ws.RuntimeDir, "Containerfile")); err != nil {
		return err
	}

	if repo, err = git.PlainOpen("."); err != nil {
		return errorsx.Wrap(err, "unable to open git repository")
	}

	environpath := filepath.Join(ws.RuntimeDir, eg.EnvironFile)
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

	canonicaluri := errorsx.Zero(gitx.CanonicalURI(repo, t.GitRemote))

	envb := envx.Build().
		FromPath(t.EnvironmentPaths...).
		FromEnv(t.Environment...).
		FromEnv(os.Environ()...).
		FromEnviron(errorsx.Zero(gitx.LocalEnv(repo, t.GitRemote, t.GitReference))...).
		Var(eg.EnvComputeBin, hotswapbin.String()).
		Var(eg.EnvUnsafeGitCloneEnabled, strconv.FormatBool(false)) // hack to disable cloning

	if err = envb.CopyTo(environio); err != nil {
		return errorsx.Wrap(err, "unable to generate environment")
	}

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.CacheDir, eg.DefaultModuleDirectory()))); err != nil {
		log.Println("unable to prewarm wasi directory cache", err)
	}

	ragent := runners.NewRunner(
		gctx.Context,
		ws,
		uid.String(),
		mounthome,
		mountegbin,
		envvar,
		sshmount,
		sshenvvar,
		runners.AgentOptionVolumes(
			runners.AgentMountReadWrite(ws.CacheDir, eg.DefaultMountRoot(eg.CacheDirectory)),
			runners.AgentMountReadWrite(ws.RuntimeDir, eg.DefaultMountRoot(eg.RuntimeDirectory)),
			runners.AgentMountReadWrite(ws.WorkingDir, eg.DefaultMountRoot(eg.WorkingDirectory)),
			runners.AgentMountReadWrite(ws.WorkspaceDir, eg.DefaultMountRoot(eg.WorkspaceDirectory)),
		),
		runners.AgentOptionLocalComputeCachingVolumes(canonicaluri),
		runners.AgentOptionEnvironFile(environpath),
		runners.AgentOptionCommandLine("--env-file", environpath), // required for tty to work correctly in local mode.
		runners.AgentOptionHostOS(),
		runners.AgentOptionEnv(eg.EnvComputeRunID, uid.String()),
		runners.AgentOptionEnv(eg.EnvComputeLoggingVerbosity, strconv.Itoa(gctx.Verbosity)),
	)

	prepcmd := func(cmd *exec.Cmd) *exec.Cmd {
		cmd.Dir = ws.Root
		cmd.Stdout = log.Writer()
		cmd.Stderr = log.Writer()
		cmd.Stdin = os.Stdin
		return cmd
	}

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

		// TODO REVISIT using t.ws.RuntimeDir as moduledir.
		err := c8sproxy.PodmanModule(ctx, prepcmd, eg.WorkingDirectory, fmt.Sprintf("eg-%s", uid.String()), ws.RuntimeDir, options...)
		if err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
