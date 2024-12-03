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
	"github.com/egdaemon/eg/internal/iox"
	"github.com/egdaemon/eg/internal/wasix"
	"github.com/egdaemon/eg/interp/c8s"
	"github.com/egdaemon/eg/runners"
	"github.com/egdaemon/eg/transpile"
	"github.com/egdaemon/eg/workspaces"
	"github.com/gofrs/uuid"
)

type c8sLocal struct {
	cmdopts.RuntimeResources
	Dir            string   `name:"directory" help:"root directory of the repository" default:"${vars_cwd}"`
	Containerfile  string   `arg:"" help:"path to the container file to run" default:"Containerfile"`
	SSHKeyPath     string   `name:"sshkeypath" help:"path to ssh key to use" default:"${vars_ssh_key_path}"`
	Environment    []string `name:"env" short:"e" help:"define environment variables and their values to be included"`
	GitRemote      string   `name:"git-remote" help:"name of the git remote to use" default:"${vars_git_default_remote_name}"`
	Endpoint       string   `name:"endpoint" help:"specify the endpoint to upload to" default:"${vars_endpoint}/c/manager/" hidden:"true"`
	ContainerCache string   `name:"croot" help:"container storage, ideally we'd be able to share with the host but for now" hidden:"true" default:"${vars_container_cache_directory}"`
}

func (t c8sLocal) Run(gctx *cmdopts.Global) (err error) {
	var (
		tmpdir     string
		ws         workspaces.Context
		environio  *os.File
		uid                            = uuid.Must(uuid.NewV7())
		sshmount   runners.AgentOption = runners.AgentOptionNoop
		sshenvvar  runners.AgentOption = runners.AgentOptionNoop
		envvar     runners.AgentOption = runners.AgentOptionNoop
		mounthome  runners.AgentOption = runners.AgentOptionNoop
		mountegbin runners.AgentOption = runners.AgentOptionEGBin(errorsx.Must(exec.LookPath(os.Args[0])))
	)

	if tmpdir, err = os.MkdirTemp("", "eg.c8s.upload.*"); err != nil {
		return errorsx.Wrap(err, "unable to create temporary directory")
	}
	defer func() {
		errorsx.Log(errorsx.Wrap(os.RemoveAll(tmpdir), "unable to remove temp directory"))
	}()

	if ws, err = workspaces.New(gctx.Context, tmpdir, ".eg", ""); err != nil {
		return errorsx.Wrap(err, "unable to initialize workspace")
	}

	egdir := filepath.Join(ws.Root, ws.ModuleDir)
	autoruncontainer := filepath.Join(ws.Root, ws.WorkingDir, "workspace", "Containerfile")
	if err = fsx.MkDirs(0700, filepath.Dir(autoruncontainer)); err != nil {
		return errorsx.Wrap(err, "unable to write autorunnable containerfil")
	}

	if err = fsx.CloneTree(gctx.Context, egdir, ".bootstrap.c8s", embeddedc8supload); err != nil {
		return errorsx.Wrap(err, "unable to clone tree")
	}

	if err = compile.InitGolang(gctx.Context, egdir, cmdopts.ModPath()); err != nil {
		return err
	}

	if err = iox.Copy(t.Containerfile, autoruncontainer); err != nil {
		return err
	}

	roots, err := transpile.Autodetect(transpile.New(ws)).Run(gctx.Context)
	if err != nil {
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

	environpath := filepath.Join(ws.Root, ws.RuntimeDir, "environ.env")
	if environio, err = os.Create(environpath); err != nil {
		return errorsx.Wrap(err, "unable to open the environment variable file")
	}
	defer environio.Close()

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
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.CacheDir), "/cache"),
			runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.RuntimeDir), "/opt/egruntime"),
			runners.AgentMountReadWrite(t.ContainerCache, "/var/lib/containers"),
		),
		runners.AgentOptionEnviron(environpath),
		runners.AgentOptionCommandLine("--env-file", environpath), // required for tty to work correctly in local mode.
		runners.AgentOptionCommandLine("--cap-add", "NET_ADMIN"),  // required for loopback device creation inside the container
		runners.AgentOptionCommandLine("--cap-add", "SYS_ADMIN"),  // required for rootless container building https://github.com/containers/podman/issues/4056#issuecomment-612893749
		runners.AgentOptionCommandLine("--device", "/dev/fuse"),
		runners.AgentOptionEnv(eg.EnvComputeRootModule, strconv.FormatBool(true)),
		runners.AgentOptionEnv(eg.EnvComputeModuleNestedLevel, strconv.Itoa(envx.Int(0, eg.EnvComputeModuleNestedLevel))),
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
				runners.AgentMountReadOnly(m.Path, "/opt/egmodule.wasm"),
				runners.AgentMountReadWrite(filepath.Join(ws.Root, ws.WorkingDir), eg.DefaultRootDirectory),
			)...)

		// TODO REVISIT using t.ws.RuntimeDir as moduledir.
		err := c8s.PodmanModule(gctx.Context, prepcmd, "eg", fmt.Sprintf("eg-%s", uid.String()), ws.RuntimeDir, options...)
		if err != nil {
			return errorsx.Wrap(err, "module execution failed")
		}
	}

	return nil
}
