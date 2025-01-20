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
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/go-git/go-git/v5"
	"github.com/gofrs/uuid"
)

type c8sLocal struct {
	cmdopts.RuntimeResources
	Dir              string   `name:"directory" help:"root directory of the repository" default:"${vars_git_directory}"`
	Containerfile    string   `arg:"" help:"path to the container file to run" default:"Containerfile"`
	SSHKeyPath       string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	EnvironmentPaths []string `name:"envpath" help:"environment files to pass to the module" default:""`
	Environment      []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote        string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	GitReference     string   `name:"git-ref" help:"name of the branch or commit to checkout" default:"${vars_git_default_reference}"`
	Endpoint         string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/q/" hidden:"true"`
	ContainerCache   string   `name:"croot" help:"container storage, ideally we'd be able to share with the host but for now" hidden:"true" default:"${vars_container_cache_directory}"`
}

func (t c8sLocal) Run(gctx *cmdopts.Global, hotswapbin *cmdopts.HotswapPath) (err error) {
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

	ctx, err := cmdopts.WithPodman(gctx.Context)
	if err != nil {
		return errorsx.Wrap(err, "unable to connect to podman")
	}

	if tmpdir, err = os.MkdirTemp("", "eg.c8s.upload.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}
	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if ws, err = workspaces.New(gctx.Context, tmpdir, eg.DefaultModuleDirectory(), ""); err != nil {
		return errorsx.Wrap(err, "unable to initialize workspace")
	}

	egdir := filepath.Join(ws.Root, ws.ModuleDir)
	autoruncontainer := filepath.Join(ws.Root, ws.RuntimeDir, "workspace", "Containerfile")
	if err = fsx.MkDirs(0700, filepath.Dir(autoruncontainer)); err != nil {
		return errorsx.Wrap(err, "unable to write autorunnable containerfil")
	}

	if err = fsx.CloneTree(gctx.Context, egdir, ".bootstrap.c8s", embeddedc8supload); err != nil {
		return errorsx.Wrap(err, "unable to clone tree")
	}

	if err = compile.InitGolang(gctx.Context, egdir, cmdopts.ModPath()); err != nil {
		return err
	}

	if err = compile.InitGolangTidy(gctx.Context, egdir); err != nil {
		return err
	}

	if err = iox.Copy(t.Containerfile, autoruncontainer); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
	if err != nil {
		return err
	}

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

	environpath := filepath.Join(ws.Root, ws.RuntimeDir, "environ.env")
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

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

	if err = wasix.WarmCacheDirectory(gctx.Context, filepath.Join(ws.Root, ws.BuildDir), wasix.WazCacheDir(filepath.Join(ws.Root, ws.RuntimeDir))); err != nil {
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
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.CacheDir), eg.DefaultCacheDirectory()),
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.RuntimeDir), eg.DefaultRuntimeDirectory()),
			runners.AgentMountReadWrite(t.ContainerCache, "/var/lib/containers"),
		),
		runners.AgentOptionEnviron(environpath),
		runners.AgentOptionCommandLine("--env-file", environpath), // required for tty to work correctly in local mode.
		runners.AgentOptionHostOS(),
		runners.AgentOptionCommandLine("--pids-limit", "-1"), // more bullshit. without this we get "Error: OCI runtime error: crun: the requested cgroup controller `pids` is not available"
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
				runners.AgentMountReadOnly(m.Path, eg.DefaultMountRoot(eg.ModuleBin)),
				runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.WorkingDir), eg.DefaultWorkingDirectory()),
			)...)

		// TODO REVISIT using t.ws.RuntimeDir as moduledir.
		err := c8s.PodmanModule(ctx, prepcmd, eg.WorkingDirectory, fmt.Sprintf("eg-%s", uid.String()), ws.Root, options...)
		if err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
